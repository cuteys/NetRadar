<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import * as echarts from 'echarts'
import worldGeoJson from '../assets/world.json'
import chinaGeoJson from '../assets/china.json'
import { useRadarStore } from '../stores/radarStore'
import { useThemeStore } from '../stores/themeStore'
import { formatBytes } from '../utils/format'
import { Globe, RotateCcw } from 'lucide-vue-next'

const radar = useRadarStore()
const theme = useThemeStore()

const mapContainer = ref<HTMLDivElement | null>(null)
const particleCanvas = ref<HTMLCanvasElement | null>(null)
let chart: echarts.ECharts | null = null
let isInitialized = false
let resizeObserver: ResizeObserver | null = null

const mapMode = ref<'world' | 'china'>('world')
const activeTrackCount = ref(0)

// 注册地图
echarts.registerMap('world', worldGeoJson as any)
echarts.registerMap('china', chinaGeoJson as any)

const gatewayCoord = computed<[number, number] | null>(() => {
  const active = radar.activeNode
  if (active && active.gateway_lng && active.gateway_lat) {
    return [active.gateway_lng, active.gateway_lat]
  }
  return null
})

const getThemeColors = () => {
  const isDark = theme.isDark
  return {
    areaColor: isDark ? '#1e293b' : '#f1f5f9',
    borderColor: isDark ? '#334155' : '#cbd5e1',
    emphasisAreaColor: isDark ? '#283548' : '#e2e8f0',
    routerPoint: isDark ? '#38bdf8' : '#0284c7',
  }
}

// 判断经纬度是否在中国境内
const isInsideChina = (lng: number, lat: number) => {
  return lng >= 73 && lng <= 136 && lat >= 18 && lat <= 54
}

// 飞线颜色组
const colorPalette = [
  '#10b981',
  '#0ea5e9',
  '#f59e0b',
  '#ec4899',
  '#8b5cf6',
  '#06b6d4',
  '#f97316',
  '#14b8a6',
  '#6366f1',
  '#84cc16',
]

const getHistoricalColor = (identifier: string) => {
  let hash = 0
  for (let i = 0; i < identifier.length; i++) {
    hash = ((hash << 5) - hash) + identifier.charCodeAt(i)
    hash |= 0
  }
  return colorPalette[Math.abs(hash) % colorPalette.length]
}

// 实时飞线模型
interface FlylineTrack {
  key: string
  color: string
  originCoord: [number, number]
  targetCoord: [number, number]
  p0: [number, number] | null
  p1: [number, number] | null
  control: [number, number] | null
  t: number
  speed: number
  flow: any
}

// 历史静态轨迹线
interface StaticArcTrack {
  originCoord: [number, number]
  targetCoord: [number, number]
  p0: [number, number] | null
  p1: [number, number] | null
  control: [number, number] | null
  color: string
}

let activeTracks: FlylineTrack[] = []
let historicalTracks: StaticArcTrack[] = []
let animFrameId: number | null = null
let isInteracting = false
let pendingUpdate = false
let lastFlowSignature = ''
let updateTimer: any = null
let lastFrameTime = 0

// 实时流缓存，方便悬浮提示快速读取
const flowDetailMap = new Map<string, any>()

const projectGeoCoord = (coord?: [number, number] | null): [number, number] | null => {
  if (!chart || !isInitialized || !coord) return null
  try {
    const pt = chart.convertToPixel({ geoIndex: 0 }, coord)
    if (pt && Number.isFinite(pt[0]) && Number.isFinite(pt[1])) {
      return [pt[0], pt[1]]
    }
  } catch {
    try {
      const pt = chart.convertToPixel('geo', coord)
      if (pt && Number.isFinite(pt[0]) && Number.isFinite(pt[1])) {
        return [pt[0], pt[1]]
      }
    } catch {
      return null
    }
  }
  return null
}

// 计算贝塞尔曲线控制点（向北偏拱弧）
const computeControlPoint = (
  p0: [number, number],
  p1: [number, number]
): [number, number] => {
  const mx = (p0[0] + p1[0]) / 2
  const my = (p0[1] + p1[1]) / 2
  const dx = p1[0] - p0[0]
  const dy = p1[1] - p0[1]
  const dist = Math.hypot(dx, dy)
  if (dist < 1) return [mx, my]

  const nx = -dy / dist
  const ny = dx / dist

  const sign = ny > 0 ? -1 : 1
  const offset = Math.min(dist * 0.22, 60) * sign

  return [mx + nx * offset, my + ny * offset]
}

// 缩放/平移后重新映射像素坐标
const updateTrackPixels = () => {
  for (const track of activeTracks) {
    track.p0 = projectGeoCoord(track.originCoord)
    track.p1 = projectGeoCoord(track.targetCoord)
    if (track.p0 && track.p1) {
      track.control = computeControlPoint(track.p0, track.p1)
    } else {
      track.control = null
    }
  }
}

const updateHistoricalTrackPixels = () => {
  for (const track of historicalTracks) {
    track.p0 = projectGeoCoord(track.originCoord)
    track.p1 = projectGeoCoord(track.targetCoord)
    if (track.p0 && track.p1) {
      track.control = computeControlPoint(track.p0, track.p1)
    } else {
      track.control = null
    }
  }
}

// 计算二次贝塞尔曲线上的插值点
const getBezierPoint = (
  p0: [number, number],
  pc: [number, number],
  p1: [number, number],
  t: number
): [number, number] => {
  const inv = 1 - t
  const inv2 = inv * inv
  const t2 = t * t
  const twoInvT = 2 * inv * t
  return [
    inv2 * p0[0] + twoInvT * pc[0] + t2 * p1[0],
    inv2 * p0[1] + twoInvT * pc[1] + t2 * p1[1],
  ]
}

const clearCanvas = () => {
  const canvas = particleCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  ctx.clearRect(0, 0, canvas.width / dpr, canvas.height / dpr)
}

// 渲染历史静态轨迹线
const renderHistoricalLines = () => {
  const canvas = particleCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  const width = canvas.width / dpr
  const height = canvas.height / dpr

  ctx.clearRect(0, 0, width, height)
  const isDark = theme.isDark

  for (const track of historicalTracks) {
    if (!track.p0 || !track.p1 || !track.control) continue
    const { p0, p1, control, color } = track

    ctx.beginPath()
    ctx.moveTo(p0[0], p0[1])
    ctx.quadraticCurveTo(control[0], control[1], p1[0], p1[1])
    ctx.strokeStyle = color
    ctx.lineWidth = isDark ? 1.1 : 1.3
    ctx.globalAlpha = isDark ? 0.35 : 0.42
    ctx.stroke()
  }
}

// 实时飞线动画渲染循环
const renderCanvas = (time: number) => {
  if (radar.selectedTimeRange !== 'realtime') {
    animFrameId = null
    return
  }

  if (!lastFrameTime) lastFrameTime = time
  const dt = Math.min(time - lastFrameTime, 100)
  lastFrameTime = time
  const timeScale = dt / 16.667

  const canvas = particleCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  const width = canvas.width / dpr
  const height = canvas.height / dpr

  ctx.clearRect(0, 0, width, height)
  const isDark = theme.isDark

  for (const track of activeTracks) {
    if (!track.p0 || !track.p1 || !track.control) continue

    const { p0, p1, control, color } = track

    // 绘制弧线底轨
    ctx.beginPath()
    ctx.moveTo(p0[0], p0[1])
    ctx.quadraticCurveTo(control[0], control[1], p1[0], p1[1])
    ctx.strokeStyle = color
    ctx.lineWidth = isDark ? 1.2 : 1.4
    ctx.globalAlpha = isDark ? 0.3 : 0.4
    ctx.stroke()

    // 步进粒子位置
    track.t += track.speed * timeScale
    if (track.t > 1) {
      track.t -= 1
    }

    const t = track.t
    const head = getBezierPoint(p0, control, p1, t)

    // 拖尾渐变
    const tailLength = 0.06
    const steps = 6
    let prevPoint: [number, number] | null = null

    for (let i = 0; i <= steps; i++) {
      const u = t - tailLength * (1 - i / steps)
      if (u < 0) continue

      const pt = getBezierPoint(p0, control, p1, u)
      if (prevPoint) {
        const factor = i / steps
        const lineWidth = 0.6 + factor * 3.6
        const alpha = 0.04 + factor * 0.65

        ctx.beginPath()
        ctx.moveTo(prevPoint[0], prevPoint[1])
        ctx.lineTo(pt[0], pt[1])
        ctx.strokeStyle = color
        ctx.lineWidth = lineWidth
        ctx.lineCap = 'round'
        ctx.globalAlpha = alpha
        ctx.stroke()
      }
      prevPoint = pt
    }

    // 粒子光核
    ctx.beginPath()
    ctx.arc(head[0], head[1], 5.0, 0, Math.PI * 2)
    ctx.fillStyle = color
    ctx.globalAlpha = 0.7
    ctx.fill()

    ctx.beginPath()
    ctx.arc(head[0], head[1], 2.0, 0, Math.PI * 2)
    ctx.fillStyle = '#ffffff'
    ctx.globalAlpha = 0.98
    ctx.fill()
  }

  animFrameId = requestAnimationFrame(renderCanvas)
}

const startAnimation = () => {
  if (animFrameId) return
  lastFrameTime = 0
  animFrameId = requestAnimationFrame(renderCanvas)
}

const stopAnimation = () => {
  if (animFrameId) {
    cancelAnimationFrame(animFrameId)
    animFrameId = null
  }
}

// 画布尺寸同步
const resizeCanvas = () => {
  if (!particleCanvas.value || !mapContainer.value) return
  const rect = mapContainer.value.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) return

  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  const canvas = particleCanvas.value
  canvas.width = Math.round(rect.width * dpr)
  canvas.height = Math.round(rect.height * dpr)
  canvas.style.width = `${rect.width}px`
  canvas.style.height = `${rect.height}px`

  const ctx = canvas.getContext('2d')
  if (ctx) {
    ctx.resetTransform()
    ctx.scale(dpr, dpr)
  }

  if (radar.selectedTimeRange === 'realtime') {
    updateTrackPixels()
  } else {
    updateHistoricalTrackPixels()
    renderHistoricalLines()
  }
}

const buildSeries = (scatterData: any[] = [], isHistorical = false): any[] => {
  const colors = getThemeColors()
  const series: any[] = []

  const isAll = radar.selectedNodeId === 'all'
  const nodesToShow = isAll
    ? (radar.nodes || []).filter((n) => n.is_online && n.gateway_lng && n.gateway_lat)
    : (radar.activeNode && radar.activeNode.gateway_lng && radar.activeNode.gateway_lat ? [radar.activeNode] : [])

  if (nodesToShow.length > 0) {
    series.push({
      type: 'scatter',
      coordinateSystem: 'geo',
      symbol: 'circle',
      symbolSize: 12,
      itemStyle: {
        color: colors.routerPoint,
        borderColor: '#ffffff',
        borderWidth: 2,
        shadowColor: colors.routerPoint,
        shadowBlur: 6,
      },
      label: {
        show: true,
        position: 'right',
        formatter: (params: any) => params.name,
        color: theme.isDark ? '#e2e8f0' : '#334155',
        fontSize: 12,
        fontWeight: 600,
      },
      data: nodesToShow.map((n) => ({
        name: n.name,
        value: [n.gateway_lng, n.gateway_lat],
      })),
    })
  }

  series.push({
    type: 'scatter',
    coordinateSystem: 'geo',
      symbolSize: isHistorical ? 6.5 : 7.5,
      itemStyle: {
        borderColor: '#ffffff',
        borderWidth: isHistorical ? 1.0 : 1.2,
        opacity: isHistorical ? 0.9 : 0.95,
      },
      emphasis: {
        scale: 1.35,
        itemStyle: {
          borderColor: '#ffffff',
          borderWidth: 2.0,
        },
      },
      data: scatterData,
  })

  return series
}

// 同步地图数据与散点图层
const syncMapData = (immediate = false) => {
  if (!chart || !isInitialized || radar.isPaused) return

  // 历史模式：显示静态轨迹与点位
  if (radar.selectedTimeRange !== 'realtime') {
    stopAnimation()
    activeTracks = []
    activeTrackCount.value = 0

    const hist = radar.historicalDestinations || []
    const scatterData: any[] = []
    const newHistTracks: StaticArcTrack[] = []
    const seen = new Set<string>()
    const origin = gatewayCoord.value

    for (const h of hist) {
      if (!h.to_coord || h.to_coord.length < 2) continue
      if (h.to_coord[0] === 0 && h.to_coord[1] === 0) continue

      if (mapMode.value === 'china') {
        if (!isInsideChina(h.to_coord[0], h.to_coord[1]) && h.country !== '中国') {
          continue
        }
      }

      const coordKey = `${h.to_coord[0].toFixed(2)},${h.to_coord[1].toFixed(2)}`
      if (seen.has(coordKey)) continue
      seen.add(coordKey)

      const totalBytes = (h.bytes_in || 0) + (h.bytes_out || 0)
      const dotColor = getHistoricalColor(h.dst_ip || coordKey)

      scatterData.push({
        name: h.city || h.country || h.dst_ip,
        value: [h.to_coord[0], h.to_coord[1], totalBytes],
        coordKey,
        historicalData: h,
        itemStyle: {
          color: dotColor,
        },
      })

      if (origin) {
        const p0 = projectGeoCoord(origin)
        const p1 = projectGeoCoord([h.to_coord[0], h.to_coord[1]])
        const control = (p0 && p1) ? computeControlPoint(p0, p1) : null

        newHistTracks.push({
          originCoord: origin,
          targetCoord: [h.to_coord[0], h.to_coord[1]],
          p0,
          p1,
          control,
          color: dotColor,
        })
      }
    }

    historicalTracks = newHistTracks
    renderHistoricalLines()

    const currentSig = `hist_${scatterData.length}_${scatterData.map(s => s.coordKey).sort().join('|')}`
    if (immediate || currentSig !== lastFlowSignature) {
      lastFlowSignature = currentSig
      chart.setOption({
        series: buildSeries(scatterData, true),
      }, false)
    }
    return
  }

  // 实时模式：展示动态飞线
  historicalTracks = []
  startAnimation()

  if (isInteracting) {
    pendingUpdate = true
    return
  }

  const flows = radar.filteredFlows || []
  const origin = gatewayCoord.value
  const sortedFlows = [...flows].sort((a, b) => (b.bytes_in + b.bytes_out) - (a.bytes_in + a.bytes_out))

  const newTracks: FlylineTrack[] = []
  const scatterData: any[] = []
  const seenCoords = new Set<string>()

  for (const f of sortedFlows) {
    if (!f.to_coord || f.to_coord.length < 2) continue

    if (mapMode.value === 'china') {
      if (!isInsideChina(f.to_coord[0], f.to_coord[1]) && f.country !== '中国') {
        continue
      }
    }

    const coordKey = `${f.to_coord[0].toFixed(2)},${f.to_coord[1].toFixed(2)}`
    if (seenCoords.has(coordKey)) continue
    seenCoords.add(coordKey)

    flowDetailMap.set(coordKey, f)

    const color = f.color || '#10b981'
    const key = `${f.dst_ip}_${coordKey}`

    const existing = activeTracks.find(t => t.key === key)
    const t = existing ? existing.t : Math.random() * 0.8
    const totalBytes = f.bytes_in + f.bytes_out
    const speed = 0.0011 + Math.min(totalBytes / 10000000, 0.0009)

    const p0 = immediate ? null : (existing?.p0 || null)
    const p1 = immediate ? null : (existing?.p1 || null)
    const control = immediate ? null : (existing?.control || null)

    const trackOrigin = (f.from_coord && f.from_coord[0] && f.from_coord[1]) ? f.from_coord : origin
    if (trackOrigin) {
      newTracks.push({
        key,
        color,
        originCoord: trackOrigin,
        targetCoord: [f.to_coord[0], f.to_coord[1]],
        p0,
        p1,
        control,
        t,
        speed,
        flow: f,
      })
    }

    scatterData.push({
      name: f.city || f.country || f.dst_ip,
      value: [f.to_coord[0], f.to_coord[1], totalBytes],
      coordKey,
      flowData: f,
      itemStyle: {
        color: color,
      },
    })

    if (newTracks.length >= 20) break
  }

  activeTracks = newTracks
  activeTrackCount.value = activeTracks.length

  for (const track of activeTracks) {
    if (!track.p0 || !track.p1 || !track.control) {
      track.p0 = projectGeoCoord(track.originCoord)
      track.p1 = projectGeoCoord(track.targetCoord)
      if (track.p0 && track.p1) {
        track.control = computeControlPoint(track.p0, track.p1)
      }
    }
  }

  const currentSig = `real_${scatterData.map(s => s.coordKey).sort().join('|')}`
  if (immediate || currentSig !== lastFlowSignature) {
    lastFlowSignature = currentSig
    chart.setOption({
      series: buildSeries(scatterData, false),
    }, false, true)
  }
}

const updateSeriesOnly = (immediate = false) => {
  if (immediate) {
    if (updateTimer) {
      clearTimeout(updateTimer)
      updateTimer = null
    }
    syncMapData(true)
    return
  }
  if (updateTimer) return
  updateTimer = setTimeout(() => {
    updateTimer = null
    syncMapData(false)
  }, 1000)
}

const initMapOption = () => {
  if (!chart) return

  const colors = getThemeColors()

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    animation: false,
    animationDuration: 0,
    animationDurationUpdate: 0,
    tooltip: {
      trigger: 'item',
      confine: true,
      enterable: false,
      extraCssText: 'pointer-events: none; z-index: 50;',
      backgroundColor: theme.isDark ? 'rgba(15, 23, 42, 0.94)' : 'rgba(255, 255, 255, 0.95)',
      borderColor: theme.isDark ? '#334155' : '#e2e8f0',
      borderWidth: 1,
      padding: [8, 12],
      textStyle: {
        color: theme.isDark ? '#f8fafc' : '#1e293b',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", sans-serif',
        fontSize: 12,
      },
      formatter: (params: any) => {
        if (params.seriesType === 'scatter') {
          // 历史卡片
          const h = params.data?.historicalData
          if (h) {
            const totalBytes = (h.bytes_in || 0) + (h.bytes_out || 0)
            const lastSeenStr = h.last_seen ? new Date(h.last_seen * 1000).toLocaleString('zh-CN', { hour12: false }) : '近期'
            return `
              <div style="font-weight: 600; margin-bottom: 4px; display: flex; align-items: center; gap: 6px;">
                <span style="display:inline-block;width:9px;height:9px;border-radius:50%;background:${params.color || '#38bdf8'}"></span>
                ${h.country || '外联节点'} · ${h.city || '数据中心'}
              </div>
              <div style="font-size: 11px; opacity: 0.88; line-height: 1.6;">
                <div>目标 IP: <span style="font-family: monospace; font-weight: 600;">${h.dst_ip}:${h.dst_port || '-'}</span></div>
                <div>协议网络: <span style="font-weight: 500;">${h.protocol || 'TCP'}</span> · ${h.isp || '骨干网络'}</div>
                <div>区间下行: <span style="color: #10b981; font-weight: 600;">${formatBytes(h.bytes_in || 0)}</span></div>
                <div>区间上行: <span style="color: #0ea5e9; font-weight: 600;">${formatBytes(h.bytes_out || 0)}</span></div>
                <div>区间总计: <span style="color: #a855f7; font-weight: 600;">${formatBytes(totalBytes)}</span></div>
                <div>最近活跃: <span style="opacity: 0.75;">${lastSeenStr}</span></div>
              </div>
            `
          }

          // 实时卡片
          const d = flowDetailMap.get(params.data?.coordKey) || params.data?.flowData
          if (!d) {
            return `<div style="font-weight: 600; padding: 2px 4px;">${params.name || '边缘节点'}</div>`
          }
          return `
            <div style="font-weight: 600; margin-bottom: 4px; display: flex; align-items: center; gap: 6px;">
              <span style="display:inline-block;width:9px;height:9px;border-radius:50%;background:${params.color || '#10b981'}"></span>
              ${d.country || '外联节点'} · ${d.city || '数据中心'}
            </div>
            <div style="font-size: 11px; opacity: 0.88; line-height: 1.6;">
              <div>目标 IP: <span style="font-family: monospace; font-weight: 600;">${d.dst_ip}:${d.dst_port}</span></div>
              <div>来源设备: <span style="font-family: monospace;">${d.src_ip || '-'}</span></div>
              <div>协议类型: <span style="font-weight: 500;">${d.protocol}</span> · ${d.isp || '骨干网络'}</div>
              <div>实时下行: <span style="color: #10b981; font-weight: 600;">${formatBytes(d.bytes_in)}</span></div>
              <div>实时上行: <span style="color: #0ea5e9; font-weight: 600;">${formatBytes(d.bytes_out)}</span></div>
            </div>
          `
        }

        if (params.name) {
          return `<div style="font-weight: 600; padding: 2px 4px;">${params.name}</div>`
        }
        return ''
      },
    },
    geo: {
      map: mapMode.value,
      roam: true,
      zoom: mapMode.value === 'china' ? 1.55 : 1.38,
      center: mapMode.value === 'china' ? [104.5, 35.5] : [12, 12],
      label: {
        show: false,
      },
      itemStyle: {
        areaColor: colors.areaColor,
        borderColor: colors.borderColor,
        borderWidth: 0.6,
      },
      emphasis: {
        itemStyle: {
          areaColor: colors.emphasisAreaColor,
          borderColor: theme.isDark ? '#60a5fa' : '#3b82f6',
          borderWidth: 1,
        },
        label: {
          show: false,
        },
      },
      select: {
        disabled: true,
      },
    },
    series: buildSeries([]) as any,
  }

  chart.setOption(option, true)
  isInitialized = true

  setTimeout(() => {
    syncMapData(true)
  }, 40)
}

const setMapMode = (mode: 'world' | 'china') => {
  if (mapMode.value === mode) return
  mapMode.value = mode
  activeTracks = []
  historicalTracks = []
  activeTrackCount.value = 0
  lastFlowSignature = ''
  clearCanvas()
  initMapOption()
}

const resetView = () => {
  activeTracks = []
  historicalTracks = []
  activeTrackCount.value = 0
  lastFlowSignature = ''
  clearCanvas()
  initMapOption()
}

const handleInteractionStart = () => {
  isInteracting = true
}

const handleInteractionEnd = () => {
  isInteracting = false
  if (pendingUpdate) {
    pendingUpdate = false
    updateSeriesOnly(true)
  }
}

const onGeoRoam = () => {
  if (radar.selectedTimeRange === 'realtime') {
    updateTrackPixels()
  } else {
    updateHistoricalTrackPixels()
    renderHistoricalLines()
  }
}

const handleVisibilityChange = () => {
  if (document.hidden) {
    stopAnimation()
  } else if (radar.selectedTimeRange === 'realtime') {
    startAnimation()
  }
}

onMounted(() => {
  if (mapContainer.value) {
    chart = echarts.init(mapContainer.value, null, {
      renderer: 'canvas',
      devicePixelRatio: Math.min(window.devicePixelRatio || 1, 1.5),
      useDirtyRect: true,
    })
    initMapOption()

    chart.on('georoam', onGeoRoam)

    // 监听容器尺寸调整
    resizeObserver = new ResizeObserver(() => {
      chart?.resize()
      resizeCanvas()
    })
    resizeObserver.observe(mapContainer.value)

    const el = mapContainer.value
    el.addEventListener('touchstart', handleInteractionStart, { passive: true })
    el.addEventListener('touchend', handleInteractionEnd, { passive: true })
    el.addEventListener('touchcancel', handleInteractionEnd, { passive: true })
    el.addEventListener('mousedown', handleInteractionStart)
    window.addEventListener('mouseup', handleInteractionEnd)
    document.addEventListener('visibilitychange', handleVisibilityChange)

    resizeCanvas()
    if (radar.selectedTimeRange === 'realtime') {
      startAnimation()
    } else {
      radar.fetchHistoricalDestinations()
    }
  }
})

onUnmounted(() => {
  stopAnimation()
  if (updateTimer) clearTimeout(updateTimer)
  resizeObserver?.disconnect()

  const el = mapContainer.value
  if (el) {
    el.removeEventListener('touchstart', handleInteractionStart)
    el.removeEventListener('touchend', handleInteractionEnd)
    el.removeEventListener('touchcancel', handleInteractionEnd)
    el.removeEventListener('mousedown', handleInteractionStart)
  }
  window.removeEventListener('mouseup', handleInteractionEnd)
  document.removeEventListener('visibilitychange', handleVisibilityChange)

  chart?.dispose()
  chart = null
})

// 监听实时流数据更新
watch(() => radar.activeFlows, () => {
  if (radar.selectedTimeRange === 'realtime') {
    updateSeriesOnly()
  }
})

// 监听历史数据更新
watch(() => radar.historicalDestinations, () => {
  if (radar.selectedTimeRange !== 'realtime') {
    syncMapData(true)
  }
})

// 切换时间范围
watch(() => radar.selectedTimeRange, (range) => {
  lastFlowSignature = ''
  clearCanvas()
  if (range === 'realtime') {
    historicalTracks = []
    startAnimation()
    syncMapData(true)
  } else {
    stopAnimation()
    activeTracks = []
    activeTrackCount.value = 0
    radar.fetchHistoricalDestinations(range)
    syncMapData(true)
  }
})

watch(() => radar.selectedDeviceIp, () => {
  updateSeriesOnly(true)
})

watch(() => radar.selectedNodeId, () => {
  if (radar.selectedTimeRange === 'realtime') {
    updateSeriesOnly(true)
  } else {
    radar.fetchHistoricalDestinations()
  }
})

watch(() => radar.activeNode, () => {
  updateSeriesOnly(true)
})

watch(() => radar.isPaused, (paused) => {
  if (!paused && radar.selectedTimeRange === 'realtime') {
    updateSeriesOnly(true)
  }
})

watch(() => theme.isDark, () => {
  initMapOption()
})
</script>

<template>
  <div class="apple-glass rounded-3xl p-4 sm:p-5 relative overflow-hidden flex flex-col h-[380px] sm:h-[480px] lg:h-[620px] xl:h-[680px]">
    <!-- Top Bar Controls -->
    <div class="flex items-center justify-between z-20 mb-2 gap-2">
      <div class="flex items-center gap-2">
        <div class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
          <Globe class="w-4 h-4" />
        </div>
        <h2 class="text-xs sm:text-sm font-semibold text-slate-800 dark:text-slate-100 flex items-center gap-1.5">
          外联态势感知
        </h2>
      </div>

      <!-- Map View Switcher & Reset Button -->
      <div class="flex items-center gap-1.5 flex-wrap">
        <!-- 全球 vs 全国 -->
        <div class="flex items-center p-0.5 rounded-xl bg-slate-100/90 dark:bg-slate-800/90 border border-slate-200/60 dark:border-slate-700/60">
          <button
            @click="setMapMode('world')"
            class="px-2 py-0.5 rounded-lg text-xs font-medium transition-all"
            :class="mapMode === 'world' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-xs' : 'text-slate-500 hover:text-slate-800 dark:hover:text-slate-200'"
          >
            全球
          </button>
          <button
            @click="setMapMode('china')"
            class="px-2 py-0.5 rounded-lg text-xs font-medium transition-all"
            :class="mapMode === 'china' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-xs' : 'text-slate-500 hover:text-slate-800 dark:hover:text-slate-200'"
          >
            全国
          </button>
        </div>

        <button
          @click="resetView"
          class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 transition-colors"
          title="重置地图视角"
        >
          <RotateCcw class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>

    <!-- Dual-Layer Map Stage -->
    <div class="relative w-full flex-1 min-h-0 overflow-hidden">
      <!-- Base Layer: ECharts Map Canvas (Using GeoIP's lightweight world map + interactive target dots with tooltips) -->
      <div ref="mapContainer" class="w-full h-full cursor-grab active:cursor-grabbing"></div>

      <!-- Overlay Layer: Dedicated Native Canvas for 60FPS Conical Comet Particles & Static Historical Arcs -->
      <canvas
        ref="particleCanvas"
        class="absolute inset-0 w-full h-full pointer-events-none z-10"
      ></canvas>

      <!-- Empty state when no nodes connected -->
      <div
        v-if="(radar.nodes || []).length === 0"
        class="absolute inset-0 z-20 flex items-center justify-center pointer-events-none p-4"
      >
        <div class="px-4 py-2.5 rounded-2xl apple-glass-heavy border border-slate-200/80 dark:border-slate-700/80 shadow-lg text-center backdrop-blur-md">
          <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">暂无在线探针节点</p>
          <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">请点击顶部「管理」获取一键安装命令接入节点</p>
        </div>
      </div>
    </div>

    <!-- Bottom Indicator -->
    <div class="absolute bottom-3 left-4 sm:left-6 z-20 pointer-events-none flex items-center gap-2 text-[11px] text-slate-500 dark:text-slate-400">
      <div v-if="radar.selectedTimeRange === 'realtime'" class="flex items-center gap-1.5 px-3 py-1 rounded-full apple-glass-heavy shadow-xs">
        <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
        <span>活动飞线: <strong class="text-slate-800 dark:text-slate-200">{{ activeTrackCount }}</strong> 条</span>
      </div>
      <div v-else class="flex items-center gap-1.5 px-3 py-1 rounded-full apple-glass-heavy shadow-xs">
        <span class="w-2 h-2 rounded-full bg-sky-500"></span>
        <span v-if="radar.isLoadingDestinations">正在加载历史外联分布...</span>
        <span v-else>历史外联: <strong class="text-slate-800 dark:text-slate-200">{{ historicalTracks.length }}</strong> 条轨迹 · {{ radar.historicalDestinations.length }} 个节点 (静态全景)</span>
      </div>
    </div>
  </div>
</template>
