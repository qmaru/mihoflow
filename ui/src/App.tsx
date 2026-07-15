import { useEffect, useMemo, useRef, useState } from "react"
import { getConnections, getDevices, getHistory, subscribe } from "./api"
import {
  ChainBreakdown,
  ConnectionTable,
  DeviceTable,
  HistoryPanel,
  Icon,
  StatCard,
} from "./components"
import { bytes } from "./format"
import type { Connection, Device, HistoryRow } from "./types"
import "./App.css"

function App() {
  const [devices, setDevices] = useState<Device[]>([])
  const [connections, setConnections] = useState<Connection[]>([])
  const [connectionRecords, setConnectionRecords] = useState<Connection[]>([])
  const [connectionRecordsIP, setConnectionRecordsIP] = useState("")
  const [history, setHistory] = useState<HistoryRow[]>([])
  const [days, setDays] = useState(30)
  const [group, setGroup] = useState("all")
  const [selectedIP, setSelectedIP] = useState<string | null>(null)
  const [connected, setConnected] = useState(false)
  const [theme, setTheme] = useState(() => localStorage.getItem("mihoflow-theme") || "light")
  const liveSnapshotSeen = useRef(false)
  const [snapshotVersion, setSnapshotVersion] = useState(0)

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem("mihoflow-theme", theme)
  }, [theme])
  useEffect(() => {
    let current = true
    const load = async () => {
      try {
        const deviceData = await getDevices()
        if (current && !liveSnapshotSeen.current) setDevices(deviceData)
      } catch {
        // 保留最近一次成功数据，避免短暂网络错误导致界面闪现空状态。
      }
    }
    void load()
    return () => {
      current = false
    }
  }, [])
  const historyIP = selectedIP || ""
  useEffect(() => {
    if (!selectedIP) return
    let current = true
    const load = async () => {
      try {
        const records = await getConnections(days, selectedIP)
        if (current) {
          setConnectionRecords(records)
          setConnectionRecordsIP(selectedIP)
        }
      } catch {
        // 保留最近一次成功的总流量记录，避免短暂请求失败导致统计归零。
      }
    }
    void load()
    return () => {
      current = false
    }
  }, [days, selectedIP, snapshotVersion])
  useEffect(() => {
    if (!historyIP) return
    let current = true
    const load = async () => {
      try {
        const rows = await getHistory(days, historyIP)
        const byDate = new Map<string, HistoryRow>()
        rows.forEach((row) => {
          const old = byDate.get(row.date)
          byDate.set(
            row.date,
            old
              ? {
                  ...old,
                  upload: old.upload + row.upload,
                  download: old.download + row.download,
                  connections: old.connections + row.connections,
                }
              : row,
          )
        })
        if (current) setHistory([...byDate.values()])
      } catch {
        // 保留最近一次成功历史，避免筛选时出现闪烁。
      }
    }
    void load()
    return () => {
      current = false
    }
  }, [days, historyIP])
  useEffect(
    () =>
      subscribe(
        (snapshot) => {
          liveSnapshotSeen.current = true
          setDevices(snapshot.devices)
          setConnections(snapshot.connections)
          setSnapshotVersion((version) => version + 1)
          setConnected(true)
        },
        () => setConnected(false),
      ),
    [],
  )

  const totals = useMemo(
    () =>
      devices.reduce(
        (all, item) => ({
          upload: all.upload + item.uploadToday,
          download: all.download + item.downloadToday,
        }),
        { upload: 0, download: 0 },
      ),
    [devices],
  )
  const selected = devices.find((device) => device.ip === selectedIP)
  const selectedConnections = connectionRecordsIP === selectedIP ? connectionRecords : []

  return (
    <div className="app-shell">
      <header className="topbar">
        <button className="brand" onClick={() => setSelectedIP(null)}>
          <span className="brand-mark">
            <Icon name="activity" />
          </span>
          <span>Mihoflow</span>
        </button>
        <div className="header-actions">
          <span className={`connection-state ${connected ? "online" : "offline"}`}>
            <span />
            {connected ? "实时连接" : "连接中"}
          </span>
          <button
            className="icon-button"
            onClick={() => setTheme(theme === "light" ? "dark" : "light")}
            aria-label="切换主题"
          >
            <Icon name={theme === "light" ? "moon" : "sun"} />
          </button>
        </div>
      </header>
      <main className="content">
        {selected ? (
          <>
            <div className="detail-intro">
              <button className="back-button" onClick={() => setSelectedIP(null)}>
                返回总览
              </button>
              <p className="eyebrow">设备详情 / {selected.ip}</p>
              <h1>{selected.ip}</h1>
              <p className="intro-copy">只显示该设备的连接、链路使用情况与历史流量。</p>
            </div>
            <section className="stat-grid">
              <StatCard
                label="今日上传"
                value={bytes(selected.uploadToday)}
                detail="累计上传流量"
                icon="upload"
                tone="teal"
              />
              <StatCard
                label="今日下载"
                value={bytes(selected.downloadToday)}
                detail="累计下载流量"
                icon="download"
                tone="blue"
              />
              <StatCard
                label="活跃连接"
                value={String(selected.activeConnections)}
                detail="当前连接数"
                icon="activity"
                tone="amber"
              />
              <StatCard
                label="Chain 数量"
                value={String(
                  new Set(selectedConnections.map((item) => item.chainValue || "直连")).size,
                )}
                detail="已使用链路"
                icon="server"
                tone="violet"
              />
            </section>
            <ChainBreakdown connections={selectedConnections} />
            <ConnectionTable connections={selectedConnections} group={group} onGroup={setGroup} />
            <HistoryPanel rows={history} days={days} onDays={setDays} />
          </>
        ) : (
          <>
            <section className="page-intro">
              <div>
                <p className="eyebrow">网络监测 / 总览</p>
                <h1>流量概览</h1>
                <p className="intro-copy">选择一个设备，进入它的流量详情。</p>
              </div>
              <div className="live-indicator">
                <span />每 5 秒更新
              </div>
            </section>
            <section className="stat-grid">
              <StatCard
                label="今日上传"
                value={bytes(totals.upload)}
                detail="累计上传流量"
                icon="upload"
                tone="teal"
              />
              <StatCard
                label="今日下载"
                value={bytes(totals.download)}
                detail="累计下载流量"
                icon="download"
                tone="blue"
              />
              <StatCard
                label="活跃连接"
                value={String(connections.length)}
                detail={`${devices.length} 个设备在线`}
                icon="activity"
                tone="amber"
              />
              <StatCard
                label="监测设备"
                value={String(devices.length)}
                detail="局域网设备"
                icon="server"
                tone="violet"
              />
            </section>
            <DeviceTable devices={devices} onSelect={setSelectedIP} />
          </>
        )}
      </main>
      <footer className="footer">
        <span>Mihoflow · Network monitor</span>
        <span className="footer-status">
          <span />
          数据来自本地服务
        </span>
      </footer>
    </div>
  )
}

export default App
