import { useEffect, useState, useRef } from 'react'
import type { LogDTO, TopicsResponse } from './types'
import './App.css'

type StatusState =
  | { kind: 'disconnected' }
  | { kind: 'connecting'; topic: string | null }
  | { kind: 'connected'; topic: string | null }
  | { kind: 'error'; message: string }

function App() {
  const [topics, setTopics] = useState<string[]>([])
  const [activeTopic, setActiveTopic] = useState<string | null>(null)
  const [logs, setLogs] = useState<LogDTO[]>([])
  const [status, setStatus] = useState<StatusState>({ kind: 'disconnected' })

  const wsRef = useRef<WebSocket | null>(null)
  const logsRef = useRef<HTMLDivElement | null>(null)

  // Fetch topics once on mount, and when user clicks "Refresh"
  async function fetchTopics() {
    try {
      const res = await fetch('/api/topics')
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`)
      }
      const data = (await res.json()) as TopicsResponse
      const t = data.topics ?? []
      setTopics(t)

      if (!t.length) {
        setActiveTopic(null)
        setLogs([])
        setStatus({ kind: 'disconnected' })
        return
      }

      // If we have no active topic or it disappeared, pick the first
      setActiveTopic((prev) => (prev && t.includes(prev) ? prev : t[0]))
    } catch (err: any) {
      console.error('Failed to fetch topics:', err)
      setStatus({ kind: 'error', message: 'Failed to fetch topics' })
    }
  }

  // WebSocket connection logic when activeTopic changes
  useEffect(() => {
    const topic = activeTopic
    if (!topic) {
      // no topic selected / available
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      return
    }

    // Close previous WS if any
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }

    setLogs([])
    setStatus({ kind: 'connecting', topic })

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const base = `${proto}//${window.location.host}`
    const url = `${base}/ws/logs?topic=${encodeURIComponent(topic)}`

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      setStatus({ kind: 'connected', topic })
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as LogDTO
        setLogs((prev) => [...prev, msg])
      } catch (e) {
        console.error('Bad WS message:', e)
      }
    }

    ws.onerror = (e) => {
      console.error('WebSocket error:', e)
      setStatus({ kind: 'error', message: 'WebSocket error' })
    }

    ws.onclose = () => {
      // Only mark disconnected if this is the current WS
      if (wsRef.current === ws) {
        wsRef.current = null
        setStatus({ kind: 'disconnected' })
      }
    }

    return () => {
      // Cleanup when topic changes / component unmounts
      if (wsRef.current === ws) {
        ws.close()
        wsRef.current = null
      }
    }
  }, [activeTopic])

  // Auto-scroll logs to bottom when new logs arrive
  useEffect(() => {
    if (logsRef.current) {
      logsRef.current.scrollTop = logsRef.current.scrollHeight
    }
  }, [logs])

  useEffect(() => {
    fetchTopics()
  }, [])

  function statusText() {
    switch (status.kind) {
      case 'disconnected':
        return 'Disconnected'
      case 'connecting':
        return status.topic ? `Connecting (${status.topic})...` : 'Connecting...'
      case 'connected':
        return status.topic ? `Connected (${status.topic})` : 'Connected'
      case 'error':
        return `Error: ${status.message}`
      default:
        return ''
    }
  }

  const currentTopicLabel = activeTopic ?? '(none)'

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
            <h2>Topics</h2>
            <button onClick={fetchTopics}>Refresh</button>
          </div>
          <ul id="topicsList">
            {topics.map((t) => (
              <li
                key={t || '(untitled)'}
                className={t === activeTopic ? 'active' : ''}
                onClick={() => setActiveTopic(t)}
              >
                <span className="topic">{t || '(untitled)'}</span>
              </li>
            ))}
            {!topics.length && (
              <li className="empty">No topics yet. Is the publisher running?</li>
            )}
          </ul>
        </aside>

        <section>
          <div className="toolbar">
            <span className="label">Topic:</span>
            <span id="currentTopic">{currentTopicLabel}</span>
          </div>
          <div id="logs" ref={logsRef}>
            {logs.length === 0 ? (
              <span className="placeholder">Waiting for logs...</span>
            ) : (
              logs.map((l, idx) => (
                <div key={idx} className="log-line">
                  <div className="log-meta">
                    <span className="ts">{l.timestamp}</span>{' '}
                    <span className={`level level-${l.level.toLowerCase()}`}>
                      {l.level}
                    </span>{' '}
                    <span className="host-service">
                      {l.host}/{l.service}
                    </span>{' '}
                    <span className="type">({l.type})</span>
                  </div>
                  <div className="log-summary">{l.summary}</div>
                </div>
              ))
            )}
          </div>
        </section>
      </main>
    </div>
  )
}

export default App
