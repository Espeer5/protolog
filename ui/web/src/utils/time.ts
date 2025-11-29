export function formatTimestamp(raw: string): string {
  if (!raw) return ""

  const date = new Date(raw)
  if (isNaN(date.getTime())) return raw // fallback for invalid timestamps

  // --- Absolute time ---
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, "0")
  const d = String(date.getDate()).padStart(2, "0")

  const hh = String(date.getHours()).padStart(2, "0")
  const mm = String(date.getMinutes()).padStart(2, "0")
  const ss = String(date.getSeconds()).padStart(2, "0")

  const ms = String(date.getMilliseconds()).padStart(3, "0")

  const abs = `${y}-${m}-${d} ${hh}:${mm}:${ss}.${ms}`

  // --- Relative time ---
  const diff = Date.now() - date.getTime()

  let rel: string
  if (diff < 1000) rel = "just now"
  else if (diff < 60_000) rel = `${Math.floor(diff / 1000)}s ago`
  else if (diff < 3_600_000) rel = `${Math.floor(diff / 60_000)}m ago`
  else rel = `${Math.floor(diff / 3_600_000)}h ago`

  return `${abs} (${rel})`
}
