export type Device = {
  ip: string
  uploadToday: number
  downloadToday: number
  uploadSpeed: number
  downloadSpeed: number
  activeConnections: number
}

export type Connection = {
  id: string
  upload: number
  download: number
  chainValue: string
  rule: string
  rulePayload: string
  start: string
  closedAt?: string
  metadata: {
    sourceIP: string
    host: string
    destinationIP: string
    network: string
    processPath: string
    remoteDestination: string
    sniffHost: string
    type: string
  }
}

export type HistoryRow = {
  date: string
  ip: string
  upload: number
  download: number
  connections: number
}

export type Snapshot = { devices: Device[]; connections: Connection[] }
