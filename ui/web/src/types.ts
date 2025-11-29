export interface LogDTO {
  topic: string
  timestamp: string
  level: string
  host: string
  service: string
  summary: string
  type: string
}

export interface TopicsResponse {
  topics: string[]
}
