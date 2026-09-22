<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRadarStore } from '../stores/radarStore'
import { formatBytes, formatSpeed, compressIP } from '../utils/format'
import { formatCountry } from '../utils/countryNames'
import {
  Terminal,
  Search,
  ArrowDown,
  ArrowUp,
  Activity,
  ArrowRight,
} from 'lucide-vue-next'

const radar = useRadarStore()

const searchQuery = ref('')
const sortBy = ref<'traffic' | 'speed' | 'time'>('traffic')
const statusFilter = ref<'all' | 'active' | 'closed'>('all')

const getProtocolBadgeClass = (proto: string) => {
  switch (proto?.toUpperCase()) {
    case 'TCP':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300 border-emerald-200 dark:border-emerald-800/40'
    case 'UDP':
      return 'bg-sky-50 text-sky-700 dark:bg-sky-950/50 dark:text-sky-300 border-sky-200 dark:border-sky-800/40'
    case 'ICMP':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300 border-amber-200 dark:border-amber-800/40'
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border-slate-200 dark:border-slate-700'
  }
}

// 获取设备名称
const getDeviceName = (ip: string, nodeId?: string) => {
  const dev = radar.topDevices.find((d) => d.ip === ip && (!nodeId || !d.node_id || d.node_id === nodeId))
  return dev ? dev.name : ''
}

// 获取节点名称
const getNodeName = (nodeId?: string) => {
  if (!nodeId) return ''
  const n = (radar.nodes || []).find((item) => item.id === nodeId)
  return n ? n.name : ''
}

// 格式化相对时间
const formatAgo = (timestampMs: number) => {
  if (!timestampMs) return '刚刚'
  const diffSec = Math.max(0, Math.floor((Date.now() - timestampMs) / 1000))
  if (diffSec < 3) return '刚刚'
  if (diffSec < 60) return `${diffSec}s前`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m前`
  const diffHour = Math.floor(diffMin / 60)
  return `${diffHour}h前`
}

// 格式化持续时长
const formatDuration = (startMs: number, endMs: number) => {
  const diffSec = Math.max(1, Math.floor((endMs - startMs) / 1000))
  if (diffSec < 60) return `${diffSec}s`
  const min = Math.floor(diffSec / 60)
  const sec = diffSec % 60
  return `${min}m${sec}s`
}

// 实时处理连接列表
const displayItems = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()

  // 1. 历史模式
  if (radar.selectedTimeRange !== 'realtime') {
    let list = radar.historicalDestinations || []
    if (query) {
      list = list.filter((h) => {
        const dest = `${compressIP(h.dst_ip)}:${h.dst_port}`.toLowerCase()
        const geo = `${formatCountry(h.country)} ${h.city || ''} ${h.isp || ''}`.toLowerCase()
        const proto = (h.protocol || '').toLowerCase()
        return dest.includes(query) || geo.includes(query) || proto.includes(query)
      })
    }
    const copy = [...list]
    if (sortBy.value === 'time') {
      copy.sort((a, b) => (b.last_seen || 0) - (a.last_seen || 0))
    } else {
      copy.sort((a, b) => ((b.bytes_in || 0) + (b.bytes_out || 0)) - ((a.bytes_in || 0) + (a.bytes_out || 0)))
    }
    return copy.map((h, idx) => ({
      isHistorical: true,
      id: `hist_${idx}_${h.dst_ip}_${h.dst_port}`,
      protocol: (h.protocol || 'TCP').toUpperCase(),
      src_ip: '全网关聚合',
      src_port: undefined as number | undefined,
      devName: '全网关归档',
      dst_ip: h.dst_ip,
      dst_port: h.dst_port,
      country: h.country,
      city: h.city,
      isp: h.isp,
      total_in: h.bytes_in || 0,
      total_out: h.bytes_out || 0,
      speed_in: 0,
      speed_out: 0,
      last_active: h.last_seen ? h.last_seen * 1000 : Date.now(),
      status: 'closed' as const,
      duration: '',
      nodeName: '',
    }))
  }

  // 2. 实时模式：仿 Clash 连接面板展示
  let conns = radar.filteredConnections
  if (statusFilter.value === 'active') {
    conns = conns.filter((c) => c.status === 'active')
  } else if (statusFilter.value === 'closed') {
    conns = conns.filter((c) => c.status === 'closed')
  }

  if (query) {
    conns = conns.filter((c) => {
      const src = `${compressIP(c.src_ip)}:${c.src_port || ''} ${getDeviceName(c.src_ip, c.node_id)}`.toLowerCase()
      const dst = `${compressIP(c.dst_ip)}:${c.dst_port}`.toLowerCase()
      const geo = `${formatCountry(c.country)} ${c.city || ''} ${c.isp || ''}`.toLowerCase()
      const proto = c.protocol.toLowerCase()
      const node = getNodeName(c.node_id).toLowerCase()
      return src.includes(query) || dst.includes(query) || geo.includes(query) || proto.includes(query) || node.includes(query)
    })
  }

  const copy = [...conns]
  if (sortBy.value === 'speed') {
    copy.sort((a, b) => (b.speed_in_bps + b.speed_out_bps) - (a.speed_in_bps + a.speed_out_bps))
  } else if (sortBy.value === 'time') {
    copy.sort((a, b) => b.last_active - a.last_active)
  } else {
    copy.sort((a, b) => (b.total_in + b.total_out) - (a.total_in + a.total_out))
  }

  return copy.map((c) => ({
    isHistorical: false,
    id: c.id,
    protocol: c.protocol,
    src_ip: c.src_ip,
    src_port: c.src_port,
    devName: getDeviceName(c.src_ip, c.node_id),
    dst_ip: c.dst_ip,
    dst_port: c.dst_port,
    country: c.country,
    city: c.city,
    isp: c.isp,
    total_in: c.total_in,
    total_out: c.total_out,
    speed_in: c.speed_in_bps,
    speed_out: c.speed_out_bps,
    last_active: c.last_active,
    status: c.status,
    duration: formatDuration(c.created_at, c.last_active),
    nodeName: radar.selectedNodeId === 'all' ? getNodeName(c.node_id) : '',
  }))
})
</script>

<template>
  <div class="apple-glass rounded-3xl p-3.5 sm:p-5 flex flex-col h-[520px] sm:h-[600px] lg:h-[680px] xl:h-[720px]">
    <!-- Header Controls: Title & Filters -->
    <div class="flex flex-col gap-2.5 mb-3 border-b border-slate-200/50 dark:border-slate-800/50 pb-3">
      <!-- Top Row: Title + Status/Sort Capsules -->
      <div class="flex items-center justify-between gap-2 flex-wrap">
        <div class="flex items-center gap-2">
          <div class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
            <Terminal class="w-4 h-4 text-emerald-500" />
          </div>
          <h3 class="text-xs sm:text-sm font-semibold text-slate-800 dark:text-slate-100 flex items-center gap-1.5">
            <span>外联态势审计流水</span>
            <span class="text-[11px] font-mono text-slate-400 font-normal">({{ displayItems.length }})</span>
          </h3>
        </div>

        <!-- Status and Sort Pills in clean row -->
        <div class="flex items-center gap-1.5 flex-wrap">
          <!-- Realtime Status Pill Filter -->
          <div v-if="radar.selectedTimeRange === 'realtime'" class="flex items-center p-0.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-[11px]">
            <button
              @click="statusFilter = 'all'"
              class="px-2 py-0.5 rounded-lg transition-all"
              :class="statusFilter === 'all' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-2xs font-medium' : 'text-slate-500'"
            >
              全部
            </button>
            <button
              @click="statusFilter = 'active'"
              class="px-2 py-0.5 rounded-lg transition-all"
              :class="statusFilter === 'active' ? 'bg-white dark:bg-slate-700 text-emerald-600 dark:text-emerald-400 shadow-2xs font-medium' : 'text-slate-500'"
            >
              活跃
            </button>
            <button
              @click="statusFilter = 'closed'"
              class="px-2 py-0.5 rounded-lg transition-all"
              :class="statusFilter === 'closed' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-2xs font-medium' : 'text-slate-500'"
            >
              关闭
            </button>
          </div>

          <!-- Sort Switcher -->
          <div class="flex items-center p-0.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-[11px]">
            <button
              @click="sortBy = 'traffic'"
              class="px-2 py-0.5 rounded-lg transition-all"
              :class="sortBy === 'traffic' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-2xs font-medium' : 'text-slate-500'"
              title="按累计总流量降序"
            >
              流量
            </button>
            <button
              v-if="radar.selectedTimeRange === 'realtime'"
              @click="sortBy = 'speed'"
              class="px-2 py-0.5 rounded-lg transition-all"
              :class="sortBy === 'speed' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-2xs font-medium' : 'text-slate-500'"
              title="按当前实时速率降序"
            >
              速度
            </button>
            <button
              @click="sortBy = 'time'"
              class="px-2 py-0.5 rounded-lg transition-all"
              :class="sortBy === 'time' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-2xs font-medium' : 'text-slate-500'"
              title="按最近活跃时间排序"
            >
              时间
            </button>
          </div>
        </div>
      </div>

      <!-- Search Box Row (Full Width on mobile, expands cleanly) -->
      <div class="relative w-full">
        <Search class="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
        <input
          v-model="searchQuery"
          type="text"
          class="w-full pl-8 pr-3 py-1.5 rounded-xl bg-white/70 dark:bg-slate-800/70 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-200 focus:outline-none focus:border-emerald-500 transition-all font-mono"
          placeholder="搜索 IP / 端口 / 归属地 / 终端名称 / 协议..."
        />
      </div>
    </div>

    <!-- Clash-Style Connection Card List -->
    <div class="flex-1 overflow-y-auto space-y-2 pr-1 text-xs">
      <div
        v-for="item in displayItems"
        :key="item.id"
        class="p-2.5 sm:p-3 rounded-2xl bg-white/60 dark:bg-slate-800/50 border border-slate-200/50 dark:border-slate-700/50 hover:bg-white/90 dark:hover:bg-slate-800/90 transition-all font-mono shadow-2xs space-y-1.5"
      >
        <!-- Top Row: Protocol + Source ➔ Target + Status & Time -->
        <div class="flex items-start justify-between gap-2">
          <!-- Left: Protocol Badge + Dual Endpoints Flow -->
          <div class="flex items-center gap-1.5 min-w-0 flex-1 flex-wrap sm:flex-nowrap">
            <span
              class="px-1.5 py-0.5 rounded-md text-[10px] font-bold tracking-wider border flex-shrink-0"
              :class="getProtocolBadgeClass(item.protocol)"
            >
              {{ item.protocol }}
            </span>

            <!-- Source Device / IP -->
            <div class="flex items-center gap-1 min-w-0 text-xs text-slate-700 dark:text-slate-300">
              <span class="font-medium truncate max-w-[130px] sm:max-w-[180px] text-slate-900 dark:text-white" :title="item.devName || item.src_ip">
                {{ item.devName || compressIP(item.src_ip) || '本机/网关' }}
              </span>
              <span v-if="item.src_port" class="text-slate-400 text-[10px] flex-shrink-0">
                :{{ item.src_port }}
              </span>
              <span v-if="item.nodeName" class="text-[9px] px-1 py-0.2 rounded bg-slate-200/60 dark:bg-slate-700 text-slate-500 dark:text-slate-400 truncate hidden sm:inline flex-shrink-0">
                {{ item.nodeName }}
              </span>
            </div>

            <!-- Flow Arrow -->
            <ArrowRight class="w-3 h-3 text-emerald-500 flex-shrink-0 mx-0.5" />

            <!-- Destination Endpoint -->
            <div class="flex items-center gap-1 min-w-0 text-xs text-slate-800 dark:text-slate-100 font-semibold">
              <span class="truncate max-w-[150px] sm:max-w-[220px]" :title="`${compressIP(item.dst_ip)}:${item.dst_port}`">
                {{ compressIP(item.dst_ip) }}:{{ item.dst_port }}
              </span>
            </div>
          </div>

          <!-- Right: Active Status & Time -->
          <div class="flex items-center gap-1.5 text-[10px] text-slate-400 flex-shrink-0 pt-0.5">
            <span v-if="item.duration" class="hidden md:inline text-slate-400">
              {{ item.duration }}
            </span>
            <span class="flex items-center gap-1 whitespace-nowrap">
              <span
                class="w-1.5 h-1.5 rounded-full"
                :class="item.status === 'active' ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'"
              ></span>
              <span>{{ formatAgo(item.last_active) }}</span>
            </span>
          </div>
        </div>

        <!-- Bottom Row: Geo & ISP (Left) | Speeds & Cumulative Traffic (Right) -->
        <div class="flex items-center justify-between text-[11px] pt-1 border-t border-slate-100/60 dark:border-slate-700/40 text-slate-500 dark:text-slate-400 gap-2">
          <!-- Geo & ISP Info -->
          <div class="truncate text-slate-400 text-[10px] sm:text-[11px] min-w-0 flex-1">
            <span>{{ formatCountry(item.country) }}</span>
            <span v-if="item.city"> · {{ item.city }}</span>
            <span v-if="item.isp" class="hidden sm:inline"> ({{ item.isp }})</span>
          </div>

          <!-- Speeds & Total Traffic Stats (Clash Layout) -->
          <div class="flex items-center gap-2.5 flex-shrink-0 font-mono text-[10px] sm:text-[11px]">
            <!-- Live Speed (only in realtime) -->
            <div v-if="!item.isHistorical && (item.speed_in > 0 || item.speed_out > 0)" class="flex items-center gap-1.5">
              <span class="text-emerald-600 dark:text-emerald-400 font-medium flex items-center whitespace-nowrap">
                <ArrowDown class="w-2.5 h-2.5 mr-0.5" />
                {{ formatSpeed(item.speed_in) }}
              </span>
              <span class="text-sky-600 dark:text-sky-400 font-medium flex items-center whitespace-nowrap">
                <ArrowUp class="w-2.5 h-2.5 mr-0.5" />
                {{ formatSpeed(item.speed_out) }}
              </span>
            </div>

            <!-- Total Accumulated Traffic -->
            <div class="flex items-center gap-1 text-slate-600 dark:text-slate-300 font-medium whitespace-nowrap">
              <span title="下行流量">↓{{ formatBytes(item.total_in) }}</span>
              <span class="text-slate-300 dark:text-slate-600">/</span>
              <span title="上行流量">↑{{ formatBytes(item.total_out) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div
        v-if="displayItems.length === 0"
        class="h-44 flex flex-col items-center justify-center text-slate-400 text-xs py-8 space-y-1"
      >
        <Activity class="w-6 h-6 text-slate-300 dark:text-slate-600 mb-1" />
        <span>{{ searchQuery ? '未找到匹配的连接审计记录' : '暂无外联网络连接数据，探针正在持续监听...' }}</span>
      </div>
    </div>
  </div>
</template>
