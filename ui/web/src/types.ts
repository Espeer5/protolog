export interface LogDTO {
  topic: string
  timestamp: string
  level: string
  host: string
  service: string
  summary: string
  type: string
  payloadJson?: any
}

export interface TopicsResponse {
  topics: string[]
}
