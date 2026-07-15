import type { Connection, Device, HistoryRow, Snapshot } from "./types"

const json = async <T>(path: string): Promise<T> => {
  const response = await fetch(path)
  if (!response.ok) throw new Error(`${response.status} ${response.statusText}`)
  return (await response.json()) as T
}

export const getDevices = () => json<Device[]>("/api/devices")
export const getConnections = (days: number, ip: string) =>
  json<Connection[]>(`/api/connections?days=${days}&ip=${encodeURIComponent(ip)}`)
export const getHistory = (days: number, ip: string) =>
  json<HistoryRow[]>(`/api/history?days=${days}${ip ? `&ip=${encodeURIComponent(ip)}` : ""}`)

export const subscribe = (onMessage: (snapshot: Snapshot) => void, onError: () => void) => {
  const source = new EventSource("/api/events")
  source.onmessage = (event) => {
    try {
      onMessage(JSON.parse(event.data) as Snapshot)
    } catch {
      onError()
    }
  }
  source.onerror = onError
  return () => source.close()
}
