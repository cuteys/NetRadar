import { defineStore } from 'pinia'
import { ref, shallowRef, computed } from 'vue'
import { useAuthStore } from './authStore'

export interface ParticleFlow {
  id: string
  node_id: string
  src_ip: string
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
}

export interface NodeInfo {
  id: string
  name: string
  hostname: string
  os: string
  arch: string
  ip: string
  is_online: boolean
  rate_in_bps: number
  rate_out_bps: number
  gateway_lat: number
  gateway_lng: number
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
  const historyPoints = shallowRef<WavePoint[]>([])
  const logStream = shallowRef<ParticleFlow[]>([])
  const historicalDestinations = shallowRef<any[]>([])
  const isLoadingDestinations = ref(false)

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

  const togglePause = () => {
    isPaused.value = !isPaused.value
  }

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

  const activeNode = computed<NodeInfo | null>(() => {
    if (selectedNodeId.value === 'all') {
      return nodes.value.find((n) => n.is_online) || nodes.value[0] || null
    }
    return nodes.value.find((n) => n.id === selectedNodeId.value) || null
  })

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
    if (ws) {
      ws.close()
      ws = null
    }
    isConnected.value = false
  }

  const handleMessage = (data: any) => {
    if (data.type === 'metrics') {
      const matchesNode = selectedNodeId.value === 'all' || data.node_id === selectedNodeId.value

      if (matchesNode && data.summary) {
        currentRateIn.value = data.summary.rate_in_bps || 0
        currentRateOut.value = data.summary.rate_out_bps || 0
        totalIn.value += data.summary.total_bytes_in || 0
        totalOut.value += data.summary.total_bytes_out || 0
        activeConns.value = data.summary.active_conns || 0
        totalRequests.value += data.summary.total_requests || 0

        const nowStr = new Date().toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
        const newHistory = [...historyPoints.value, {
          time: nowStr,
          inRate: currentRateIn.value,
          outRate: currentRateOut.value,
        }]
        if (newHistory.length > 50) {
          newHistory.shift()
        }
        historyPoints.value = newHistory
      }

      if (data.flows && Array.isArray(data.flows)) {
        activeFlows.value = data.flows
        const activeItems = data.flows.filter((f: ParticleFlow) => f.bytes_in > 0 || f.bytes_out > 0)
        if (activeItems.length > 0) {
          logStream.value = [...activeItems, ...logStream.value].slice(0, 40)
        }
      }

      if (data.top_lan && Array.isArray(data.top_lan)) {
        topDevices.value = data.top_lan
      }

      if (data.proto_dist) {
        protoDist.value = data.proto_dist
      }

      if (data.country_dist) {
        countryDist.value = data.country_dist
      }
    } else if (data.type === 'node_status') {
      if (Array.isArray(data.nodes)) {
        nodes.value = data.nodes
      }
    }
  }

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

  const updateNode = async (req: { id: string; name: string; ip: string; gateway_lat: number; gateway_lng: number }) => {
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
    activeFlows,
    filteredFlows,
    historyPoints,
    logStream,
    filteredLogs,
    cumulativeStats,
    systemSettings,
    historicalDestinations,
    isLoadingDestinations,
    togglePause,
    connect,
    disconnect,
    fetchNodes,
    updateNode,
    deleteNode,
    fetchCumulativeStats,
    fetchHistoricalDestinations,
    fetchSystemSettings,
    updateSystemSettings,
    renameDevice,
  }
})
