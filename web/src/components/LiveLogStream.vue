<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRadarStore } from '../stores/radarStore'
import { formatBytes, formatSpeed, compressIP, formatDeviceName } from '../utils/format'
import { formatCountry } from '../utils/countryNames'
import {
  Terminal,
  Search,
  ArrowDown,
  ArrowUp,
  Activity,
  ArrowRight,
  X,
} from 'lucide-vue-next'

const radar = useRadarStore()

const listContainer = ref<HTMLDivElement | null>(null)

watch(() => radar.searchFilter, () => {
  if (listContainer.value) {
    listContainer.value.scrollTop = 0
  }
})

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
  const query = radar.searchFilter.trim().toLowerCase()
  const tokens = query ? query.split(/\s+/).filter(Boolean) : []
  const matchTokens = (haystack: string) => {
    if (tokens.length === 0) return true
    return tokens.every((t) => haystack.includes(t))
  }

  // 1. 历史模式
  if (radar.selectedTimeRange !== 'realtime') {
    let list = radar.historicalDestinations || []
    if (tokens.length > 0) {
      list = list.filter((h) => {
        const hNodeName = radar.getNodeName(h.node_id)
        const text = [
          h.dst_ip,
          compressIP(h.dst_ip),
          h.dst_port ? String(h.dst_port) : '',
          h.dst_port ? `:${h.dst_port}` : '',
          formatCountry(h.country),
          h.country || '',
          h.city || '',
          h.isp || '',
          h.protocol || '',
          hNodeName,
        ].join(' ').toLowerCase()
        return matchTokens(text)
      })
    }
    const copy = [...list]
    if (sortBy.value === 'time') {
      list.sort((a, b) => (b.last_seen || 0) - (a.last_seen || 0))
    } else {
      list.sort((a, b) => ((b.bytes_in || 0) + (b.bytes_out || 0)) - ((a.bytes_in || 0) + (a.bytes_out || 0)))
    }
    return list.map((h, idx) => ({
      isHistorical: true,
      id: `hist_${idx}_${h.dst_ip}_${h.dst_port}_${h.node_id || ''}`,
      protocol: (h.protocol || 'TCP').toUpperCase(),
      src_ip: '历史聚合',
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
      nodeName: radar.getNodeName(h.node_id),
    }))
  }

  let conns = radar.activeConnections
  if (radar.selectedNodeId && radar.selectedNodeId !== 'all') {
    conns = conns.filter((c) => c.node_id === radar.selectedNodeId)
  }
  if (radar.selectedDeviceIp) {
    conns = conns.filter((c) => c.src_ip === radar.selectedDeviceIp)
  }
  if (statusFilter.value === 'active') {
    conns = conns.filter((c) => c.status === 'active')
  } else if (statusFilter.value === 'closed') {
    conns = conns.filter((c) => c.status === 'closed')
  }

  if (tokens.length > 0) {
    conns = conns.filter((c) => {
      const devName = radar.getDeviceName(c.src_ip)
      const nodeName = radar.getNodeName(c.node_id)
      const text = [
        c.src_ip,
        compressIP(c.src_ip),
        c.src_port ? String(c.src_port) : '',
        c.src_port ? `:${c.src_port}` : '',
        devName,
        c.dst_ip,
        compressIP(c.dst_ip),
        c.dst_port ? String(c.dst_port) : '',
        c.dst_port ? `:${c.dst_port}` : '',
        formatCountry(c.country),
        c.country,
        c.city,
        c.isp,
        c.protocol,
        nodeName,
      ].join(' ').toLowerCase()
      return matchTokens(text)
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
    devName: radar.getDeviceName(c.src_ip),
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
    nodeName: radar.selectedNodeId === 'all' ? radar.getNodeName(c.node_id) : '',
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
          v-model="radar.searchFilter"
          type="text"
          class="w-full pl-8 pr-8 py-1.5 rounded-xl bg-white/70 dark:bg-slate-800/70 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-200 focus:outline-none focus:border-emerald-500 transition-all font-mono"
          placeholder="搜索 IP / 端口 / 归属地 / 终端名称 / 协议..."
        />
        <button
          v-if="radar.searchFilter"
          @click="radar.searchFilter = ''"
          class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors p-0.5"
          title="清空搜索"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>

    <!-- Clash-Style Connection Card List -->
    <div ref="listContainer" class="flex-1 overflow-y-auto space-y-2 pr-1 text-xs">
      <div
        v-for="item in displayItems"
        :key="item.id"
        class="p-2.5 sm:p-3 rounded-2xl transition-all font-mono shadow-2xs space-y-1.5 border"
        :class="item.status === 'active' && (item.speed_in > 0 || item.speed_out > 0)
          ? 'bg-emerald-50/30 dark:bg-emerald-950/20 border-emerald-400/60 dark:border-emerald-500/50 shadow-xs'
          : 'bg-white/60 dark:bg-slate-800/50 border-slate-200/50 dark:border-slate-700/50 hover:bg-white/90 dark:hover:bg-slate-800/90'"
      >
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-1.5 sm:gap-2">
          <div class="flex items-center justify-between sm:justify-start gap-1.5 min-w-0 sm:flex-1">
            <div class="flex items-center gap-1.5 flex-shrink-0">
              <span
                class="px-1.5 py-0.5 rounded-md text-[10px] font-bold tracking-wider border flex-shrink-0"
                :class="getProtocolBadgeClass(item.protocol)"
              >
                {{ item.protocol }}
              </span>
              <span v-if="item.nodeName" class="text-[9px] px-1 py-0.2 rounded bg-slate-200/60 dark:bg-slate-700 text-slate-500 dark:text-slate-400 truncate flex-shrink-0">
                {{ item.nodeName }}
              </span>
            </div>

            <div class="flex sm:hidden items-center gap-1.5 text-[10px] text-slate-400 flex-shrink-0 font-mono">
              <div
                v-if="!item.isHistorical && (item.speed_in > 0 || item.speed_out > 0)"
                class="flex items-center gap-1 px-1.5 py-0.2 rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-medium whitespace-nowrap text-[9px]"
              >
                <span class="flex items-center"><ArrowDown class="w-2 h-2 mr-0.5" />{{ formatSpeed(item.speed_in) }}</span>
                <span class="flex items-center text-sky-600 dark:text-sky-400"><ArrowUp class="w-2 h-2 mr-0.5" />{{ formatSpeed(item.speed_out) }}</span>
              </div>
              <span class="flex items-center gap-1 whitespace-nowrap text-[10px]">
                <span class="w-1.5 h-1.5 rounded-full" :class="item.status === 'active' ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'"></span>
                <span>{{ formatAgo(item.last_active) }}</span>
              </span>
            </div>

            <div class="hidden sm:flex items-center gap-1.5 min-w-0 flex-1 ml-1">
              <div class="flex items-center gap-1 min-w-0 text-xs text-slate-700 dark:text-slate-300">
                <span class="font-medium truncate max-w-[200px] md:max-w-[280px]" :title="formatDeviceName(item.devName, item.src_ip)">
                  {{ formatDeviceName(item.devName, item.src_ip) }}
                </span>
                <span v-if="item.src_port" class="text-slate-400 text-[10px] flex-shrink-0">:{{ item.src_port }}</span>
              </div>
              <ArrowRight class="w-3 h-3 text-emerald-500 flex-shrink-0 mx-0.5" />
              <div class="flex items-center gap-1 min-w-0 text-xs text-slate-800 dark:text-slate-100 font-semibold">
                <span class="truncate max-w-[220px] md:max-w-[320px]" :title="`${compressIP(item.dst_ip)}:${item.dst_port}`">
                  {{ compressIP(item.dst_ip) }}:{{ item.dst_port }}
                </span>
              </div>
            </div>
          </div>

          <div class="flex sm:hidden items-center gap-1.5 min-w-0 w-full text-xs py-0.5">
            <div class="flex items-center gap-0.5 min-w-0 flex-1 text-slate-700 dark:text-slate-300">
              <span class="font-medium truncate text-slate-900 dark:text-white" :title="formatDeviceName(item.devName, item.src_ip)">
                {{ formatDeviceName(item.devName, item.src_ip) }}
              </span>
              <span v-if="item.src_port" class="text-slate-400 text-[10px] flex-shrink-0">:{{ item.src_port }}</span>
            </div>
            <ArrowRight class="w-3 h-3 text-emerald-500 flex-shrink-0 mx-0.5" />
            <div class="flex items-center gap-0.5 min-w-0 flex-1 justify-end text-slate-800 dark:text-slate-100 font-semibold">
              <span class="truncate text-right" :title="`${compressIP(item.dst_ip)}:${item.dst_port}`">
                {{ compressIP(item.dst_ip) }}:{{ item.dst_port }}
              </span>
            </div>
          </div>

          <div class="hidden sm:flex items-center gap-2 text-[10px] text-slate-400 flex-shrink-0 font-mono">
            <div
              v-if="!item.isHistorical && (item.speed_in > 0 || item.speed_out > 0)"
              class="flex items-center gap-1.5 px-1.5 py-0.5 rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-medium whitespace-nowrap"
            >
              <span class="flex items-center">
                <ArrowDown class="w-2.5 h-2.5 mr-0.5" />
                {{ formatSpeed(item.speed_in) }}
              </span>
              <span class="flex items-center text-sky-600 dark:text-sky-400">
                <ArrowUp class="w-2.5 h-2.5 mr-0.5" />
                {{ formatSpeed(item.speed_out) }}
              </span>
            </div>

            <span v-if="item.duration" class="hidden md:inline text-slate-400 whitespace-nowrap">
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

        <!-- Bottom Row: Geo & ISP (Left) | Cumulative Traffic (Right) -->
        <div class="flex items-center justify-between text-[11px] pt-1 border-t border-slate-100/60 dark:border-slate-700/40 text-slate-500 dark:text-slate-400 gap-2">
          <!-- Geo & ISP Info -->
          <div class="truncate text-slate-400 text-[10px] sm:text-[11px] min-w-0 flex-1">
            <span>{{ formatCountry(item.country) }}</span><span v-if="item.city && item.city !== item.country && item.city !== formatCountry(item.country)">·{{ item.city }}</span>
            <span v-if="item.isp" class="hidden sm:inline"> ({{ item.isp }})</span>
          </div>

          <!-- Total Accumulated Traffic -->
          <div class="flex items-center gap-1 text-slate-600 dark:text-slate-300 font-medium whitespace-nowrap font-mono text-[10px] sm:text-[11px] flex-shrink-0">
            <span title="下行流量">↓{{ formatBytes(item.total_in) }}</span>
            <span class="text-slate-300 dark:text-slate-600">/</span>
            <span title="上行流量">↑{{ formatBytes(item.total_out) }}</span>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div
        v-if="displayItems.length === 0"
        class="h-44 flex flex-col items-center justify-center text-slate-400 text-xs py-8 space-y-1"
      >
        <Activity class="w-6 h-6 text-slate-300 dark:text-slate-600 mb-1" />
        <span>{{ radar.searchFilter ? '未找到匹配的连接审计记录' : '暂无外联网络连接数据，探针正在持续监听...' }}</span>
      </div>
    </div>
  </div>
</template>
