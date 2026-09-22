import { defineStore } from 'pinia'
import { ref, shallowRef, computed, watch } from 'vue'
import { useAuthStore } from './authStore'

export interface ParticleFlow {
  id: string
  node_id: string
  src_ip: string
  src_port?: number
  dst_ip: string
  dst_port: number
  protocol: string
  bytes_in: number
  bytes_out: number
  country: string
  city: string
  isp: string
  color: string
  from_coord: [number, number]
  to_coord: [number, number]
}

export interface ConnectionItem {
  id: string
  key: string
  node_id: string
  protocol: string
  src_ip: string
  src_port?: number
  dst_ip: string
  dst_port: number
  country: string
  city: string
  isp: string
  color: string
  from_coord?: [number, number]
  to_coord?: [number, number]
  speed_in_bps: number
  speed_out_bps: number
  total_in: number
  total_out: number
  created_at: number
  last_active: number
  status: 'active' | 'closed'
}

export interface DeviceStats {
  ip: string
  name: string
  category: string
  rate_in_bps: number
  rate_out_bps: number
  total_in: number
  total_out: number
  conn_count: number
  last_active: number
  is_custom?: boolean
  node_id?: string
}

export interface NodeInfo {
  id: string
  name: string
  hostname: string
  os: string
  arch: string
  ip: string
  version?: string
  is_online: boolean
  rate_in_bps: number
  rate_out_bps: number
  gateway_lat: number
  gateway_lng: number
  custom_location?: boolean
  last_seen?: string
}

export interface WavePoint {
  time: string
  inRate: number
  outRate: number
}

export interface SystemSettings {
  username: string
  agent_token: string
  agent_server_addr: string
  use_tls: boolean
  version?: string
  latest_version?: string
}

interface NodeSnapshot {
  node_id: string
  timestamp: number
  lastReceived: number
  summary: {
    rate_in_bps: number
    rate_out_bps: number
    total_bytes_in: number
    total_bytes_out: number
    active_conns: number
    total_requests: number
  }
  flows: ParticleFlow[]
  top_lan: DeviceStats[]
  proto_dist: Record<string, number>
  country_dist: Record<string, number>
}

export const useRadarStore = defineStore('radar', () => {
  const authStore = useAuthStore()

  const isConnected = ref(false)
  const isPaused = ref(false)

  const currentRateIn = ref(0)
  const currentRateOut = ref(0)
  const totalIn = ref(0)
  const totalOut = ref(0)
  const activeConns = ref(0)
  const totalRequests = ref(0)

  const nodes = ref<NodeInfo[]>([])
  const selectedNodeId = ref<string>('all')
  const selectedDeviceIp = ref<string>('')
  const selectedTimeRange = ref<'realtime' | '1h' | '24h' | '7d'>('realtime')

  const topDevices = shallowRef<DeviceStats[]>([])
  const protoDist = shallowRef<Record<string, number>>({})
  const countryDist = shallowRef<Record<string, number>>({})
  const activeFlows = shallowRef<ParticleFlow[]>([])
  const activeConnections = shallowRef<ConnectionItem[]>([])
  const historyPoints = shallowRef<WavePoint[]>([])
  const logStream = shallowRef<ParticleFlow[]>([])
  const historicalDestinations = shallowRef<any[]>([])
  const isLoadingDestinations = ref(false)
  const isLoadingHistory = ref(false)

  const cumulativeStats = ref({
    total_bytes_in: 0,
    total_bytes_out: 0,
    total_requests: 0,
    peak_rate_in: 0,
    peak_rate_out: 0,
  })

  const systemSettings = ref<SystemSettings>({
    username: 'admin',
    agent_token: '',
    agent_server_addr: '',
    use_tls: false,
  })

  let ws: WebSocket | null = null
  let reconnectTimer: any = null
  let tickerTimer: any = null

  // 内部节点快照池与连接池
  const nodeSnapshotMap = new Map<string, NodeSnapshot>()
  const connMap = new Map<string, ConnectionItem>()

  const togglePause = () => {
    isPaused.value = !isPaused.value
  }

  // 切换节点时如果选中设备不在该节点下则重置为全部终端
  watch(selectedNodeId, (newNodeId) => {
    if (selectedDeviceIp.value) {
      if (newNodeId !== 'all') {
        const devExists = topDevices.value.some(
          (d) => d.ip === selectedDeviceIp.value && (!d.node_id || d.node_id === newNodeId)
        )
        if (!devExists) {
          selectedDeviceIp.value = ''
        }
      }
    }
    if (selectedTimeRange.value !== 'realtime') {
      fetchHistory(selectedTimeRange.value)
      fetchHistoricalDestinations(selectedTimeRange.value)
    }
  })

  // 联动所有容器的流过滤
  const filteredFlows = computed(() => {
    let list = activeFlows.value
    if (selectedNodeId.value && selectedNodeId.value !== 'all') {
      list = list.filter((f) => f.node_id === selectedNodeId.value)
    }
    if (selectedDeviceIp.value) {
      list = list.filter((f) => f.src_ip === selectedDeviceIp.value)
    }
    return list
  })

  const filteredLogs = computed(() => {
    let list = logStream.value
    if (selectedNodeId.value && selectedNodeId.value !== 'all') {
      list = list.filter((f) => f.node_id === selectedNodeId.value)
    }
    if (selectedDeviceIp.value) {
      list = list.filter((f) => f.src_ip === selectedDeviceIp.value)
    }
    return list
  })

  // 联动所有容器的连接池过滤
  const filteredConnections = computed(() => {
    let list = activeConnections.value
    if (selectedNodeId.value && selectedNodeId.value !== 'all') {
      list = list.filter((c) => c.node_id === selectedNodeId.value)
    }
    if (selectedDeviceIp.value) {
      list = list.filter((c) => c.src_ip === selectedDeviceIp.value)
    }
    return list
  })

  // 历史模式下动态从历史目的地聚合协议与国家分布
  const effectiveProtoDist = computed<Record<string, number>>(() => {
    if (selectedTimeRange.value === 'realtime') {
      return protoDist.value
    }
    const map: Record<string, number> = {}
    for (const h of historicalDestinations.value) {
      const proto = (h.protocol || 'OTHER').toUpperCase()
      const bytes = (h.bytes_in || 0) + (h.bytes_out || 0)
      map[proto] = (map[proto] || 0) + bytes
    }
    return map
  })

  const effectiveCountryDist = computed<Record<string, number>>(() => {
    if (selectedTimeRange.value === 'realtime') {
      return countryDist.value
    }
    const map: Record<string, number> = {}
    for (const h of historicalDestinations.value) {
      const country = h.country || '未知'
      const bytes = (h.bytes_in || 0) + (h.bytes_out || 0)
      map[country] = (map[country] || 0) + bytes
    }
    return map
  })

  const activeNode = computed<NodeInfo | null>(() => {
    if (selectedNodeId.value === 'all') {
      return nodes.value.find((n) => n.is_online) || nodes.value[0] || null
    }
    return nodes.value.find((n) => n.id === selectedNodeId.value) || null
  })

  // ----------------------------------------------------
  // 固定 2 秒平稳时钟节流器与聚合计算引擎
  // ----------------------------------------------------
  const startTicker = () => {
    if (tickerTimer) clearInterval(tickerTimer)
    tickerTimer = setInterval(() => {
      onClockTick()
    }, 2000)
  }

  const stopTicker = () => {
    if (tickerTimer) {
      clearInterval(tickerTimer)
      tickerTimer = null
    }
  }

  const onClockTick = () => {
    if (isPaused.value) return

    // 历史模式下由 API 单独驱动，不推实时点
    if (selectedTimeRange.value !== 'realtime') {
      return
    }

    const now = Date.now()
    const isAll = selectedNodeId.value === 'all'
    const targetNodeId = selectedNodeId.value

    let aggRateIn = 0
    let aggRateOut = 0
    let aggConns = 0
    let combinedFlows: ParticleFlow[] = []
    const combinedProto: Record<string, number> = {}
    const combinedCountry: Record<string, number> = {}
    const devMap = new Map<string, DeviceStats>()

    for (const [nodeId, snapshot] of nodeSnapshotMap.entries()) {
      // 超过 10 秒无心跳视为离线
      const isAlive = (now - snapshot.lastReceived) < 10000

      if (isAll || nodeId === targetNodeId) {
        if (isAlive) {
          aggRateIn += snapshot.summary.rate_in_bps || 0
          aggRateOut += snapshot.summary.rate_out_bps || 0
          aggConns += snapshot.summary.active_conns || 0
          totalIn.value += snapshot.summary.total_bytes_in || 0
          totalOut.value += snapshot.summary.total_bytes_out || 0
          totalRequests.value += snapshot.summary.total_requests || 0

          if (snapshot.flows && snapshot.flows.length > 0) {
            combinedFlows.push(...snapshot.flows)
          }

          for (const [k, v] of Object.entries(snapshot.proto_dist || {})) {
            combinedProto[k] = (combinedProto[k] || 0) + v
          }
          for (const [k, v] of Object.entries(snapshot.country_dist || {})) {
            combinedCountry[k] = (combinedCountry[k] || 0) + v
          }

          for (const dev of (snapshot.top_lan || [])) {
            const compositeKey = `${nodeId}:${dev.ip}`
            const existing = devMap.get(compositeKey)
            if (!existing) {
              devMap.set(compositeKey, { ...dev, node_id: nodeId })
            } else {
              existing.rate_in_bps += dev.rate_in_bps
              existing.rate_out_bps += dev.rate_out_bps
              existing.total_in += dev.total_in
              existing.total_out += dev.total_out
              existing.conn_count += dev.conn_count
              if (dev.last_active > existing.last_active) {
                existing.last_active = dev.last_active
              }
            }
          }
        }
      }
    }

    // 更新聚合指标
    currentRateIn.value = aggRateIn
    currentRateOut.value = aggRateOut
    activeConns.value = aggConns
    protoDist.value = combinedProto
    countryDist.value = combinedCountry
    activeFlows.value = combinedFlows

    // 更新设备列表并按当前总速率降序
    const sortedDevs = Array.from(devMap.values()).sort(
      (a, b) => (b.rate_in_bps + b.rate_out_bps) - (a.rate_in_bps + a.rate_out_bps)
    )
    topDevices.value = sortedDevs.slice(0, 30)

    // 推进固定 2 秒波形数据点（平滑单点推移，杜绝跳变）
    const nowStr = new Date().toLocaleTimeString('zh-CN', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
    const newWave = [...historyPoints.value, {
      time: nowStr,
      inRate: aggRateIn,
      outRate: aggRateOut,
    }]
    if (newWave.length > 50) {
      newWave.shift()
    }
    historyPoints.value = newWave

    // 仿 Clash 连接池合并与老化
    updateClashConnections(combinedFlows, now)
  }

  // 仿 Clash 连接池状态管理
  const updateClashConnections = (flows: ParticleFlow[], now: number) => {
    const seenThisTick = new Set<string>()

    for (const f of flows) {
      if (!f.dst_ip) continue
      const sPort = f.src_port || 0
      const connKey = `${f.node_id}_${f.protocol}_${f.src_ip}:${sPort}_${f.dst_ip}:${f.dst_port}`
      seenThisTick.add(connKey)

      const existing = connMap.get(connKey)
      if (existing) {
        existing.speed_in_bps = Math.round(f.bytes_in / 2.0)
        existing.speed_out_bps = Math.round(f.bytes_out / 2.0)
        existing.total_in += f.bytes_in
        existing.total_out += f.bytes_out
        existing.last_active = now
        existing.status = 'active'
      } else {
        const item: ConnectionItem = {
          id: f.id || connKey,
          key: connKey,
          node_id: f.node_id,
          protocol: f.protocol || 'TCP',
          src_ip: f.src_ip,
          src_port: f.src_port,
          dst_ip: f.dst_ip,
          dst_port: f.dst_port,
          country: f.country || '',
          city: f.city || '',
          isp: f.isp || '',
          color: f.color || '',
          from_coord: f.from_coord,
          to_coord: f.to_coord,
          speed_in_bps: Math.round(f.bytes_in / 2.0),
          speed_out_bps: Math.round(f.bytes_out / 2.0),
          total_in: f.bytes_in,
          total_out: f.bytes_out,
          created_at: now,
          last_active: now,
          status: 'active',
        }
        connMap.set(connKey, item)
      }
    }

    // 老化处理：本周期没有增量的连接降速，超过 60 秒无活跃的删除
    for (const [key, conn] of connMap.entries()) {
      if (!seenThisTick.has(key)) {
        conn.speed_in_bps = 0
        conn.speed_out_bps = 0
        if (now - conn.last_active > 15000) {
          conn.status = 'closed'
        }
        if (now - conn.last_active > 60000) {
          connMap.delete(key)
        }
      }
    }

    // 转换为数组供展示，限制最大保留 100 条
    const list = Array.from(connMap.values())
    if (list.length > 100) {
      list.sort((a, b) => b.last_active - a.last_active)
      list.length = 100
    }
    activeConnections.value = list
  }

  // ----------------------------------------------------
  // WebSocket 通信与消息接收
  // ----------------------------------------------------
  const connect = () => {
    if (!authStore.token) return
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const wsUrl = `${protocol}//${host}/ws/dashboard?token=${encodeURIComponent(authStore.token)}`

    try {
      ws = new WebSocket(wsUrl)

      ws.onopen = () => {
        isConnected.value = true
        fetchNodes()
        fetchCumulativeStats()
        fetchSystemSettings()
        startTicker()
      }

      ws.onmessage = (event) => {
        if (isPaused.value) return
        try {
          const data = JSON.parse(event.data)
          handleMessage(data)
        } catch (e) {
          console.error('Error parsing WS message', e)
        }
      }

      ws.onclose = () => {
        isConnected.value = false
        ws = null
        scheduleReconnect()
      }

      ws.onerror = () => {
        isConnected.value = false
        if (ws) {
          ws.close()
          ws = null
        }
      }
    } catch {
      scheduleReconnect()
    }
  }

  const scheduleReconnect = () => {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = setTimeout(async () => {
      if (!authStore.isAuthenticated) return
      const isValid = await authStore.checkAuth()
      if (isValid && authStore.isAuthenticated) {
        connect()
      }
    }, 2500)
  }

  const disconnect = () => {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    stopTicker()
    if (ws) {
      ws.close()
      ws = null
    }
    isConnected.value = false
  }

  // 接收探针指标，暂存到对应节点的快照缓存中
  const handleMessage = (data: any) => {
    if (data.type === 'metrics') {
      const nodeId = data.node_id || 'unknown'
      nodeSnapshotMap.set(nodeId, {
        node_id: nodeId,
        timestamp: data.timestamp || Date.now(),
        lastReceived: Date.now(),
        summary: data.summary || {
          rate_in_bps: 0,
          rate_out_bps: 0,
          total_bytes_in: 0,
          total_bytes_out: 0,
          active_conns: 0,
          total_requests: 0,
        },
        flows: Array.isArray(data.flows) ? data.flows : [],
        top_lan: Array.isArray(data.top_lan) ? data.top_lan : [],
        proto_dist: data.proto_dist || {},
        country_dist: data.country_dist || {},
      })
    } else if (data.type === 'node_status') {
      if (Array.isArray(data.nodes)) {
        nodes.value = data.nodes
      }
    }
  }

  // ----------------------------------------------------
  // REST API 交互
  // ----------------------------------------------------
  const fetchNodes = async () => {
    if (!authStore.token) return
    try {
      const res = await fetch('/api/nodes', {
        headers: { Authorization: `Bearer ${authStore.token}` },
      })
      if (res.ok) {
        const data = await res.json()
        nodes.value = Array.isArray(data) ? data : []
      }
    } catch {
      nodes.value = []
    }
  }

  const updateNode = async (req: { id: string; name: string; ip: string; gateway_lat: number; gateway_lng: number; custom_location?: boolean }) => {
    if (!authStore.token) return false
    try {
      const res = await fetch('/api/nodes/update', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${authStore.token}`,
        },
        body: JSON.stringify(req),
      })
      if (res.ok) {
        const target = nodes.value.find((n) => n.id === req.id)
        if (target) {
          target.name = req.name
          target.ip = req.ip
          target.gateway_lat = req.gateway_lat
          target.gateway_lng = req.gateway_lng
          if (typeof req.custom_location === 'boolean') {
            target.custom_location = req.custom_location
          }
        }
        return true
      }
    } catch {}
    return false
  }

  const deleteNode = async (nodeId: string) => {
    if (!authStore.token) return false
    try {
      const res = await fetch(`/api/nodes/delete?id=${encodeURIComponent(nodeId)}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${authStore.token}` },
      })
      if (res.ok) {
        nodes.value = nodes.value.filter((n) => n.id !== nodeId)
        nodeSnapshotMap.delete(nodeId)
        if (selectedNodeId.value === nodeId) {
          selectedNodeId.value = 'all'
        }
        return true
      }
    } catch {}
    return false
  }

  const fetchCumulativeStats = async () => {
    if (!authStore.token) return
    try {
      const nodeParam = selectedNodeId.value || 'all'
      const res = await fetch(`/api/stats/aggregated?node_id=${nodeParam}&range=${selectedTimeRange.value}`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      })
      if (res.ok) {
        cumulativeStats.value = await res.json()
      }
    } catch {}
  }

  const fetchHistory = async (range?: string) => {
    if (!authStore.token) return
    const targetRange = range || selectedTimeRange.value
    if (targetRange === 'realtime') return

    isLoadingHistory.value = true
    try {
      const nodeParam = selectedNodeId.value || 'all'
      const res = await fetch(`/api/history?node_id=${encodeURIComponent(nodeParam)}&range=${encodeURIComponent(targetRange)}`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      })
      if (res.ok) {
        const points = await res.json()
        if (Array.isArray(points)) {
          historyPoints.value = points.map((p: any) => {
            const d = new Date(p.timestamp * 1000)
            const timeStr = targetRange === '7d'
              ? `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
              : d.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' })
            return {
              time: timeStr,
              inRate: p.rate_in_bps || 0,
              outRate: p.rate_out_bps || 0,
            }
          })
        }
      }
    } catch (e) {
      console.error('Failed to fetch history', e)
    } finally {
      isLoadingHistory.value = false
    }
  }

  const fetchHistoricalDestinations = async (range?: string) => {
    if (!authStore.token) return
    const targetRange = range || selectedTimeRange.value
    if (targetRange === 'realtime') {
      historicalDestinations.value = []
      return
    }

    isLoadingDestinations.value = true
    try {
      const nodeParam = selectedNodeId.value || 'all'
      const res = await fetch(`/api/destinations?node_id=${encodeURIComponent(nodeParam)}&range=${encodeURIComponent(targetRange)}`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      })
      if (res.ok) {
        const data = await res.json()
        historicalDestinations.value = Array.isArray(data) ? data : []
      }
    } catch (e) {
      console.error('Failed to fetch historical destinations', e)
    } finally {
      isLoadingDestinations.value = false
    }
  }

  const fetchSystemSettings = async () => {
    if (!authStore.token) return
    try {
      const res = await fetch('/api/settings/system', {
        headers: { Authorization: `Bearer ${authStore.token}` },
      })
      if (res.ok) {
        systemSettings.value = await res.json()
      }
    } catch {}
  }

  const updateSystemSettings = async (payload: any) => {
    if (!authStore.token) return { success: false, error: '未登录' }
    try {
      const res = await fetch('/api/settings/system', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${authStore.token}`,
        },
        body: JSON.stringify(payload),
      })
      const data = await res.json()
      if (res.ok) {
        systemSettings.value.username = data.username
        systemSettings.value.agent_token = data.token
        systemSettings.value.agent_server_addr = data.agent_server_addr
        systemSettings.value.use_tls = data.use_tls
        return { success: true }
      } else {
        return { success: false, error: data.error || '保存失败' }
      }
    } catch (e: any) {
      return { success: false, error: e.message }
    }
  }

  const renameDevice = async (ip: string, newName: string) => {
    if (!authStore.token) return false
    try {
      const res = await fetch('/api/device/rename', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${authStore.token}`,
        },
        body: JSON.stringify({ ip, name: newName }),
      })
      if (res.ok) {
        const dev = topDevices.value.find((d) => d.ip === ip)
        if (dev) {
          dev.name = newName
          dev.is_custom = true
        }
        return true
      }
    } catch {}
    return false
  }

  return {
    isConnected,
    isPaused,
    currentRateIn,
    currentRateOut,
    totalIn,
    totalOut,
    activeConns,
    totalRequests,
    nodes,
    activeNode,
    selectedNodeId,
    selectedDeviceIp,
    selectedTimeRange,
    topDevices,
    protoDist,
    countryDist,
    effectiveProtoDist,
    effectiveCountryDist,
    activeFlows,
    filteredFlows,
    activeConnections,
    filteredConnections,
    historyPoints,
    logStream,
    filteredLogs,
    cumulativeStats,
    systemSettings,
    historicalDestinations,
    isLoadingDestinations,
    isLoadingHistory,
    togglePause,
    connect,
    disconnect,
    fetchNodes,
    updateNode,
    deleteNode,
    fetchCumulativeStats,
    fetchHistory,
    fetchHistoricalDestinations,
    fetchSystemSettings,
    updateSystemSettings,
    renameDevice,
  }
})
