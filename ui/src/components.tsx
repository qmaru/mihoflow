import { useMemo, type ReactNode } from "react"
import type { Connection, Device, HistoryRow } from "./types"
import { bytes } from "./format"

const chainLabel = (value: string) => value.replace(/\s*->\s*/g, " / ")
const processLabel = (value: string) => value.split(/[\\/]/).pop() || value

const compareIP = (left: string, right: string) => {
  const leftParts = left.split(".").map(Number)
  const rightParts = right.split(".").map(Number)
  for (let index = 0; index < Math.max(leftParts.length, rightParts.length); index += 1) {
    const difference = (leftParts[index] ?? -1) - (rightParts[index] ?? -1)
    if (difference !== 0) return difference
  }
  return left.localeCompare(right)
}

export const Icon = ({
  name,
}: {
  name: "moon" | "sun" | "activity" | "download" | "upload" | "server" | "clock"
}) => {
  const labels: Record<typeof name, string> = {
    moon: "🌙",
    sun: "☀️",
    activity: "📈",
    download: "⬇️",
    upload: "⬆️",
    server: "🖥️",
    clock: "🕒",
  }
  return (
    <span className={`icon icon-${name}`} aria-hidden="true">
      {labels[name]}
    </span>
  )
}

type SelectOption<T extends string | number> = { label: string; value: T }

export function SelectField<T extends string | number>({
  value,
  options,
  onChange,
  ariaLabel,
}: {
  value: T
  options: SelectOption<T>[]
  onChange: (value: T) => void
  ariaLabel: string
}) {
  return (
    <select
      className="field-select"
      value={value}
      onChange={(event) => {
        const option = options.find((item) => String(item.value) === event.target.value)
        if (option) onChange(option.value)
      }}
      aria-label={ariaLabel}
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  )
}

function PanelHeader({
  eyebrow,
  title,
  children,
  className = "",
}: {
  eyebrow: string
  title: ReactNode
  children?: ReactNode
  className?: string
}) {
  return (
    <div className={`panel-heading ${className}`}>
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h2>{title}</h2>
      </div>
      {children}
    </div>
  )
}

export function StatCard({
  label,
  value,
  detail,
  icon,
  tone,
}: {
  label: string
  value: string
  detail: string
  icon: Parameters<typeof Icon>[0]["name"]
  tone: string
}) {
  return (
    <article className="stat-card">
      <div className={`stat-icon ${tone}`}>
        <Icon name={icon} />
      </div>
      <div>
        <p className="eyebrow">{label}</p>
        <strong>{value}</strong>
        <p className="muted">{detail}</p>
      </div>
    </article>
  )
}

export function DeviceTable({
  devices,
  onSelect,
}: {
  devices: Device[]
  onSelect: (ip: string) => void
}) {
  const orderedDevices = useMemo(
    () => [...devices].sort((left, right) => compareIP(left.ip, right.ip)),
    [devices],
  )

  return (
    <section className="panel">
      <PanelHeader eyebrow="实时状态" title="设备概览">
        <span className="count-badge">点击设备进入详情</span>
      </PanelHeader>
      {orderedDevices.length ? (
        <div className="device-grid">
          {orderedDevices.map((device) => (
            <button
              className="device-row device-button"
              key={device.ip}
              onClick={() => onSelect(device.ip)}
            >
              <div className="device-name">
                <span className="device-dot" />
                <strong>{device.ip}</strong>
              </div>
              <div>
                <span className="label">今日上传</span>
                <span className="value upload-text">{bytes(device.uploadToday)}</span>
              </div>
              <div>
                <span className="label">今日下载</span>
                <span className="value download-text">{bytes(device.downloadToday)}</span>
              </div>
              <div>
                <span className="label">今日流量</span>
                <span className="value">{bytes(device.uploadToday + device.downloadToday)}</span>
              </div>
              <div>
                <span className="label">连接</span>
                <span className="value">{device.activeConnections}</span>
              </div>
            </button>
          ))}
        </div>
      ) : (
        <Empty text="暂无设备数据" />
      )}
    </section>
  )
}

const groupLabels: Record<string, string> = {
  all: "全部连接",
  host: "按目的地",
  process: "按进程",
  rule: "按规则",
  chain: "按链路",
}
const groupOptions = Object.entries(groupLabels).map(([value, label]) => ({ value, label }))
type TrafficGroup = { key: string; upload: number; download: number; count: number }

export function ConnectionTable({
  connections,
  group,
  onGroup,
}: {
  connections: Connection[]
  group: string
  onGroup: (value: string) => void
}) {
  const groups = useMemo<TrafficGroup[]>(() => {
    if (group === "all") return []
    const result = new Map<string, TrafficGroup>()
    connections.forEach((item) => {
      const key =
        group === "host"
          ? item.metadata.host || item.metadata.destinationIP || "未解析目标"
          : group === "process"
            ? processLabel(item.metadata.processPath || "未识别进程")
            : group === "rule"
              ? item.rule || "MATCH"
              : chainLabel(item.chainValue || "直连")
      const old = result.get(key) || { key, upload: 0, download: 0, count: 0 }
      result.set(key, {
        key,
        upload: old.upload + item.upload,
        download: old.download + item.download,
        count: old.count + 1,
      })
    })
    return [...result.values()].sort((a, b) => b.upload + b.download - a.upload - a.download)
  }, [connections, group])
  return (
    <section className="panel">
      <PanelHeader
        eyebrow="连接明细"
        className="table-heading"
        title={
          <>
            {group === "all" ? "活跃连接" : `${groupLabels[group]}流量`}{" "}
            <span className="inline-count">
              {group === "all" ? connections.length : groups.length}
            </span>
          </>
        }
      >
        <SelectField
          value={group}
          options={groupOptions}
          onChange={onGroup}
          ariaLabel="连接分组方式"
        />
      </PanelHeader>
      {group === "all" ? (
        <ConnectionRows connections={connections} />
      ) : groups.length ? (
        <div className="group-grid">
          {groups.map((item) => (
            <div className="traffic-group" key={item.key}>
              <div>
                <strong>{item.key}</strong>
                <small>{item.count} 条连接</small>
              </div>
              <div className="group-values">
                <span className="traffic-up">↑ {bytes(item.upload)}</span>
                <span className="traffic-down">↓ {bytes(item.download)}</span>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <Empty text="暂无连接数据" />
      )}
    </section>
  )
}

function ConnectionRows({ connections }: { connections: Connection[] }) {
  return connections.length ? (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>目标</th>
            <th>链路</th>
            <th>规则</th>
            <th>流量</th>
            <th>开始时间</th>
          </tr>
        </thead>
        <tbody>
          {connections.map((item) => (
            <tr key={item.id}>
              <td>
                <span className="host">
                  {item.metadata.host || item.metadata.destinationIP || "未解析目标"}
                </span>
                {item.metadata.host && item.metadata.destinationIP ? (
                  <small>{item.metadata.destinationIP}</small>
                ) : null}
                {item.metadata.remoteDestination &&
                item.metadata.remoteDestination !== item.metadata.destinationIP ? (
                  <small>目标：{item.metadata.remoteDestination}</small>
                ) : null}
                {item.metadata.sniffHost &&
                item.metadata.sniffHost !== item.metadata.host &&
                item.metadata.sniffHost !== item.metadata.destinationIP ? (
                  <small>嗅探：{item.metadata.sniffHost}</small>
                ) : null}
              </td>
              <td>
                <span className="chain">{chainLabel(item.chainValue || "直连")}</span>
              </td>
              <td>
                <span className="rule-tag">{item.rule || "MATCH"}</span>
              </td>
              <td>
                <span className="traffic-up">↑ {bytes(item.upload)}</span>
                <span className="traffic-down">↓ {bytes(item.download)}</span>
              </td>
              <td className="time">
                {item.start
                  ? new Date(item.start).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })
                  : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  ) : (
    <Empty text="暂无活跃连接" />
  )
}

export function ChainBreakdown({ connections }: { connections: Connection[] }) {
  const items = useMemo(() => {
    const map = new Map<string, { upload: number; download: number; count: number }>()
    connections.forEach((item) => {
      const key = chainLabel(item.chainValue || "直连")
      const old = map.get(key) || { upload: 0, download: 0, count: 0 }
      map.set(key, {
        upload: old.upload + item.upload,
        download: old.download + item.download,
        count: old.count + 1,
      })
    })
    return [...map.entries()]
      .map(([key, value]) => ({ key, ...value }))
      .sort((a, b) => b.upload + b.download - a.upload - a.download)
  }, [connections])
  const max = Math.max(...items.map((item) => item.upload + item.download), 1)
  return (
    <section className="panel">
      <PanelHeader eyebrow="流量构成" title="链路使用情况">
        <span className="count-badge">{items.length} 条链路</span>
      </PanelHeader>
      {items.length ? (
        <div className="chain-list">
          {items.map((item) => (
            <div className="chain-row" key={item.key}>
              <div className="chain-info">
                <strong>{item.key}</strong>
                <small>
                  {item.count} 条连接 · {bytes(item.upload + item.download)}
                </small>
              </div>
              <div className="chain-meter">
                <span
                  style={{ width: `${Math.max(((item.upload + item.download) / max) * 100, 4)}%` }}
                />
              </div>
              <div className="chain-total">
                <span className="traffic-up">↑ {bytes(item.upload)}</span>
                <span className="traffic-down">↓ {bytes(item.download)}</span>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <Empty text="该设备暂无 Chain 数据" />
      )}
    </section>
  )
}

export function HistoryPanel({
  rows,
  days,
  onDays,
}: {
  rows: HistoryRow[]
  days: number
  onDays: (value: number) => void
}) {
  const max = Math.max(...rows.map((row) => row.upload + row.download), 1)
  return (
    <section className="panel history-panel">
      <PanelHeader eyebrow="流量趋势" title="历史记录">
        <SelectField
          value={days}
          options={[
            { value: 7, label: "近 7 天" },
            { value: 30, label: "近 30 天" },
            { value: 90, label: "近 90 天" },
          ]}
          onChange={onDays}
          ariaLabel="历史记录范围"
        />
      </PanelHeader>
      {rows.length ? (
        <div className="history-chart">
          {rows.slice(-14).map((row) => (
            <div
              className="history-bar"
              key={`${row.date}-${row.ip}`}
              title={`${row.date} · ${bytes(row.upload + row.download)}`}
            >
              <div className="bar-stack">
                <span style={{ height: `${Math.max((row.download / max) * 100, 2)}%` }} />
                <i style={{ height: `${Math.max((row.upload / max) * 100, 2)}%` }} />
              </div>
              <small>{row.date.slice(5)}</small>
            </div>
          ))}
        </div>
      ) : (
        <Empty text="暂无历史数据" />
      )}
    </section>
  )
}
export function Empty({ text }: { text: string }) {
  return (
    <div className="empty">
      <Icon name="activity" />
      <span>{text}</span>
    </div>
  )
}
