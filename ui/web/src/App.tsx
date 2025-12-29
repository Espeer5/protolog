import { useEffect, useRef, useState } from 'react'
import type { LogDTO, TopicsResponse } from './types'
import { formatTimestamp } from './utils/time'
import './App.css'

type StatusState =
  | { kind: 'disconnected' }
  | { kind: 'connecting' }
  | { kind: 'connected' }
  | { kind: 'error'; message: string }

type Mode = 'live' | 'history'

function App() {
  // Raw data
  const [logs, setLogs] = useState<LogDTO[]>([])
  const [knownTopics, setKnownTopics] = useState<string[]>([])
  const MAX_LIVE_LOGS = 5000

  // Selection / filters
  const [selectedLog, setSelectedLog] = useState<LogDTO | null>(null)
  const [topicFilter, setTopicFilter] = useState<string[]>([])
  const [hostFilter, setHostFilter] = useState<string[]>([])
  const [levelFilter, setLevelFilter] = useState<string[]>([])
  const [typeFilter, setTypeFilter] = useState<string[]>([])

  // UI state
  const [mode, setMode] = useState<Mode>('live')
  const [startTime, setStartTime] = useState<string>(() => {
    // default: last 15 minutes
    const d = new Date(Date.now() - 15 * 60 * 1000)
    return d.toISOString().slice(0, 19) // "YYYY-MM-DDTHH:MM:SS" for datetime-local
  })
  const [endTime, setEndTime] = useState<string>(() => {
    const d = new Date()
    return d.toISOString().slice(0, 19)
  })
  const [historyCursor, setHistoryCursor] = useState<string>('')
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyHasMore, setHistoryHasMore] = useState(false)


  const [status, setStatus] = useState<StatusState>({ kind: 'disconnected' })
  const [paused, setPaused] = useState(false)
  const [hasNewLogs, setHasNewLogs] = useState(false)

  // Refs
  const wsRef = useRef<WebSocket | null>(null)
  const logsRef = useRef<HTMLDivElement | null>(null)
  const pausedRef = useRef(false)
  const autoScrollRef = useRef(true)
  const modeRef = useRef<Mode>('live')

  // --- Helpers for filters ---

  const allTopicsFromLogs = Array.from(
    new Set(logs.map((l) => l.topic).filter(Boolean)),
  ).sort()

  const topicOptions = Array.from(
    new Set([...(knownTopics || []), ...allTopicsFromLogs]),
  )
    .filter(Boolean)
    .sort()

  const hostOptions = Array.from(
    new Set(logs.map((l) => l.host).filter(Boolean)),
  ).sort()

  const levelOptions = Array.from(
    new Set(logs.map((l) => l.level).filter(Boolean)),
  ).sort()

  const typeOptions = Array.from(
    new Set(logs.map((l) => l.type).filter(Boolean)),
  ).sort()

  function toggleValue(
    value: string,
    current: string[],
    setter: (v: string[]) => void,
  ) {
    setter(
      current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value],
    )
  }

  const [serviceFilter, setServiceFilter] = useState<string[]>([])

  const serviceOptions = Array.from(
    new Set(logs.map((l) => l.service).filter(Boolean)),
  ).sort()

  function passesFilters(log: LogDTO): boolean {
    if (topicFilter.length && !topicFilter.includes(log.topic)) return false
    if (hostFilter.length && !hostFilter.includes(log.host)) return false
    if (levelFilter.length && !levelFilter.includes(log.level)) return false
    if (typeFilter.length && !typeFilter.includes(log.type)) return false
    if (serviceFilter.length && !serviceFilter.includes(log.service)) return false
    return true
  }

  const filteredLogs = logs.filter(passesFilters)

  // If the selected log is no longer visible due to filters, clear selection
  useEffect(() => {
    if (selectedLog && !passesFilters(selectedLog)) {
      setSelectedLog(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topicFilter, hostFilter, levelFilter, typeFilter, serviceFilter])

  useEffect(() => {
    if (mode === 'history') {
      setLogs([])
      setHistoryCursor('')
      setSelectedLog(null)
      fetchHistoryPage(true)
    }
  }, [mode])

  // --- Fetch historical logs from the API ---
  async function fetchHistoryPage(reset: boolean) {
    if (historyLoading) return
    setHistoryLoading(true)
    try {
      const startIso = new Date(startTime).toISOString()
      const endIso = new Date(endTime).toISOString()

      const params = new URLSearchParams()
      params.set('start', startIso)
      params.set('end', endIso)
      topicFilter.forEach(t => params.append('topic', t))
      serviceFilter.forEach(s => params.append('service', s))
      hostFilter.forEach(h => params.append('host', h))
      typeFilter.forEach(t => params.append('type', t))
      levelFilter.forEach(l => params.append('level', l))

      params.set('limit', '500')
      if (!reset && historyCursor) params.set('cursor', historyCursor)

      const res = await fetch(`/api/logs?${params.toString()}`)
      if (!res.ok) {
        const body = await res.text().catch(() => '')
        throw new Error(`HTTP ${res.status} ${res.statusText}: ${body}`)
      }
      const data = (await res.json()) as { items: LogDTO[]; next_cursor?: string }

      setHistoryCursor(data.next_cursor ?? '')
      setHistoryHasMore(Boolean(data.next_cursor))

      setLogs((prev) => (reset ? data.items : [...prev, ...data.items]))
      setSelectedLog(null)
      setHasNewLogs(false)
    } catch (err) {
      console.error('History query failed:', err)
      setStatus({ kind: 'error', message: 'History query failed' })
    } finally {
      setHistoryLoading(false)
    }
  }

  // --- Fetch topics from API (for initial topic list) ---

  async function fetchTopics() {
    try {
      const res = await fetch('/api/topics')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = (await res.json()) as TopicsResponse
      const t = data.topics ?? []
      setKnownTopics(t)
    } catch (err: any) {
      console.error('Failed to fetch topics:', err)
      setStatus({ kind: 'error', message: 'Failed to fetch topics' })
    }
  }

  // --- WebSocket connection: subscribe to ALL topics (no filter) ---

  useEffect(() => {
    setStatus({ kind: 'connecting' })

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const base = `${proto}//${window.location.host}`
    const url = `${base}/ws/logs` // no topic param -> all topics

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      setStatus({ kind: 'connected' })
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as LogDTO

        // If paused, ignore incoming messages
        if (pausedRef.current || modeRef.current === 'history') return

        if (!autoScrollRef.current) {
          setHasNewLogs(true)
        }

        setLogs((prev) => {
          const next = [...prev, msg]
          if (next.length <= MAX_LIVE_LOGS) return next
          return next.slice(next.length - MAX_LIVE_LOGS)
        })
      } catch (e) {
        console.error('Bad WS message:', e)
      }
    }

    ws.onerror = (e) => {
      console.error('WebSocket error:', e)
      setStatus({ kind: 'error', message: 'WebSocket error' })
    }

    ws.onclose = () => {
      if (wsRef.current === ws) {
        wsRef.current = null
        setStatus({ kind: 'disconnected' })
      }
    }

    return () => {
      if (wsRef.current === ws) {
        ws.close()
        wsRef.current = null
      }
    }
  }, [])

  // --- Auto-scroll when filtered logs change, unless paused or user scrolled up ---

  useEffect(() => {
    if (!paused && autoScrollRef.current && logsRef.current) {
      logsRef.current.scrollTop = logsRef.current.scrollHeight
    }
  }, [filteredLogs, paused])

  // Initial topics load
  useEffect(() => {
    fetchTopics()
  }, [])

  // --- Controls ---

  function togglePaused() {
    setPaused((prev) => {
      const next = !prev
      pausedRef.current = next
      return next
    })
  }

  function clearScreen() {
    setLogs([])
    setSelectedLog(null)
    setHasNewLogs(false)
  }

  function jumpToBottom() {
    const el = logsRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
    autoScrollRef.current = true
    setHasNewLogs(false)
  }

  function onLogsScroll() {
    const el = logsRef.current
    if (!el) return

    // How far are we from the bottom?
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight

    // Treat user as "at bottom" if we're within 1px
    const atBottom = Math.abs(distanceFromBottom) < 1

    const wasAuto = autoScrollRef.current
    autoScrollRef.current = atBottom

    // If user just came back to bottom, clear "new logs" pill
    if (!wasAuto && atBottom) {
      setHasNewLogs(false)
    }
  }

  function statusText() {
    switch (status.kind) {
      case 'disconnected':
        return 'Disconnected'
      case 'connecting':
        return 'Connecting...'
      case 'connected':
        return 'Connected'
      case 'error':
        return `Error: ${status.message}`
      default:
        return ''
    }
  }

  return (
    <div className="app-root">
      <header>
        <h1>Protolog</h1>
        <div
          className={[
            'status',
            status.kind === 'connected' ? 'connected' : '',
            status.kind === 'error' ? 'error' : '',
          ].join(' ')}
        >
          {statusText()}
        </div>
      </header>

      <main>
        <aside>
          <div className="aside-header">
            <h2>Filters</h2>
            <button onClick={fetchTopics}>Refresh</button>
          </div>

          <div className="filter-section">
            <div className="filter-title">Topics</div>
            <ul className="filter-list">
              {topicOptions.map((t) => (
                <li key={t}>
                  <label>
                    <input
                      type="checkbox"
                      checked={topicFilter.includes(t)}
                      onChange={() =>
                        toggleValue(t, topicFilter, setTopicFilter)
                      }
                    />
                    <span className="filter-label-text">{t}</span>
                  </label>
                </li>
              ))}
              {topicOptions.length === 0 && (
                <li className="empty">No topics yet</li>
              )}
            </ul>
          </div>

          <div className="filter-section">
            <div className="filter-title">Hosts</div>
            <ul className="filter-list">
              {hostOptions.map((h) => (
                <li key={h}>
                  <label>
                    <input
                      type="checkbox"
                      checked={hostFilter.includes(h)}
                      onChange={() =>
                        toggleValue(h, hostFilter, setHostFilter)
                      }
                    />
                    <span className="filter-label-text">{h}</span>
                  </label>
                </li>
              ))}
              {hostOptions.length === 0 && (
                <li className="empty">No hosts yet</li>
              )}
            </ul>
          </div>

          <div className="filter-section">
            <div className="filter-title">Services</div>
            <ul className="filter-list">
              {serviceOptions.map((s) => (
                <li key={s}>
                  <label>
                    <input
                      type="checkbox"
                      checked={serviceFilter.includes(s)}
                      onChange={() => toggleValue(s, serviceFilter, setServiceFilter)}
                    />
                    <span className="filter-label-text">{s}</span>
                  </label>
                </li>
              ))}
              {serviceOptions.length === 0 && (
                <li className="empty">No services yet</li>
              )}
            </ul>
          </div>

          <div className="filter-section">
            <div className="filter-title">Levels</div>
            <ul className="filter-list">
              {levelOptions.map((lvl) => (
                <li key={lvl}>
                  <label>
                    <input
                      type="checkbox"
                      checked={levelFilter.includes(lvl)}
                      onChange={() =>
                        toggleValue(lvl, levelFilter, setLevelFilter)
                      }
                    />
                    <span className="filter-label-text">{lvl}</span>
                  </label>
                </li>
              ))}
              {levelOptions.length === 0 && (
                <li className="empty">No levels yet</li>
              )}
            </ul>
          </div>

          <div className="filter-section">
            <div className="filter-title">Types</div>
            <ul className="filter-list">
              {typeOptions.map((ty) => (
                <li key={ty}>
                  <label>
                    <input
                      type="checkbox"
                      checked={typeFilter.includes(ty)}
                      onChange={() =>
                        toggleValue(ty, typeFilter, setTypeFilter)
                      }
                    />
                    <span className="filter-label-text">{ty}</span>
                  </label>
                </li>
              ))}
              {typeOptions.length === 0 && (
                <li className="empty">No types yet</li>
              )}
            </ul>
          </div>
        </aside>

        <section className="logs-and-detail">
          <div className="toolbar">
            <div className="toolbar-left">
              <span className="label">Logs</span>
            </div>
            <div className="toolbar-right">
              <button onClick={togglePaused}>
                {paused ? 'Resume' : 'Pause'}
              </button>
              <button onClick={jumpToBottom}>Bottom</button>
              <button onClick={clearScreen}>Clear screen</button>
              <button onClick={() => setMode(mode === 'live' ? 'history' : 'live')}>
                {mode === 'live' ? 'History' : 'Live'}
              </button>
              {mode === 'history' && (
                <div className="history-controls">
                  <label>
                    Start
                    <input
                      type="datetime-local"
                      value={startTime}
                      onChange={(e) => setStartTime(e.target.value)}
                    />
                  </label>
                  <label>
                    End
                    <input
                      type="datetime-local"
                      value={endTime}
                      onChange={(e) => setEndTime(e.target.value)}
                    />
                  </label>
                  <button onClick={() => { setHistoryCursor(''); fetchHistoryPage(true) }}>
                    Run
                  </button>
                  <button
                    disabled={!historyHasMore || historyLoading}
                    onClick={() => fetchHistoryPage(false)}
                  >
                    Load more
                  </button>
                </div>
              )}
            </div>
          </div>

          <div className="logs-layout">
            <div className="logs-container">
              <div
                id="logs"
                ref={logsRef}
                className="logs-list"
                onScroll={onLogsScroll}
              >
                {filteredLogs.length === 0 ? (
                  <span className="placeholder">No logs (check filters?)</span>
                ) : (
                  <div className="log-table">
                    <div className="log-header-row">
                      <div className="log-col log-col-ts">Time</div>
                      <div className="log-col log-col-level">Level</div>
                      <div className="log-col log-col-topic">Topic</div>
                      <div className="log-col log-col-host">Service</div>
                      <div className="log-col log-col-type">Type</div>
                      <div className="log-col log-col-summary">Summary</div>
                    </div>

                    {filteredLogs.map((l, idx) => (
                      <div
                        key={idx}
                        className={
                          'log-row' +
                          (selectedLog === l ? ' log-row-selected' : '')
                        }
                        onClick={() => setSelectedLog(l)}
                      >
                        <div className="log-col log-col-ts">
                          {formatTimestamp(l.timestamp)}
                        </div>
                        <div className="log-col log-col-level">
                          <span
                            className={`level-badge level-${l.level.toLowerCase()}`}
                          >
                            {l.level.replace('LOG_LEVEL_', '')}
                          </span>
                        </div>
                        <div className="log-col log-col-topic">
                          {l.topic}
                        </div>
                        <div className="log-col log-col-host">
                          {l.service}
                        </div>
                        <div className="log-col log-col-type">
                          {l.type}
                        </div>
                        <div className="log-col log-col-summary">
                          {l.summary}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {hasNewLogs && !paused && (
                <button
                  className="new-logs-indicator"
                  onClick={jumpToBottom}
                >
                  New logs ↓
                </button>
              )}
            </div>

            <div className="detail-panel">
              <div className="detail-header">Details</div>
              {selectedLog ? (
                <div className="detail-body">
                  <div className="detail-row">
                    <span className="detail-label">Timestamp</span>
                    <span className="detail-value">
                      {formatTimestamp(selectedLog.timestamp)}
                    </span>
                  </div>
                  <div className="detail-row">
                    <span className="detail-label">Level</span>
                    <span className="detail-value">{selectedLog.level}</span>
                  </div>
                  <div className="detail-row">
                    <span className="detail-label">Host / Service</span>
                    <span className="detail-value">
                      {selectedLog.host}/{selectedLog.service}
                    </span>
                  </div>
                  <div className="detail-row">
                    <span className="detail-label">Type</span>
                    <span className="detail-value">
                      {selectedLog.type}
                    </span>
                  </div>
                  <div className="detail-row">
                    <span className="detail-label">Summary</span>
                    <span className="detail-value">
                      {selectedLog.summary}
                    </span>
                  </div>

                  <div className="detail-payload-label">Payload</div>
                  <pre className="detail-payload">
                    {selectedLog.payloadJson
                      ? JSON.stringify(selectedLog.payloadJson, null, 2)
                      : '// no payload or unknown type'}
                  </pre>
                </div>
              ) : (
                <div className="detail-body placeholder">
                  Click a log entry to see details
                </div>
              )}
            </div>
          </div>
        </section>
      </main>
    </div>
  )
}

export default App
