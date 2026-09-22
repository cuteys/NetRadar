<script setup lang="ts">
import { watch, computed } from 'vue'
import { useRadarStore } from '../stores/radarStore'
import { formatBytes, formatSpeed, compressIP } from '../utils/format'
import AppleSelect, { type DropdownOption } from './AppleSelect.vue'
import {
  Clock,
  ArrowDownCircle,
  ArrowUpCircle,
  Activity,
  Zap,
  Filter
} from 'lucide-vue-next'

const radar = useRadarStore()

const timeRanges: { label: string; value: 'realtime' | '1h' | '24h' | '7d' }[] = [
  { label: '实时', value: 'realtime' },
  { label: '1小时', value: '1h' },
  { label: '24小时', value: '24h' },
  { label: '7天', value: '7d' },
]

const getNodeName = (nodeId?: string) => {
  if (!nodeId) return ''
  const n = (radar.nodes || []).find((item) => item.id === nodeId)
  return n ? n.name : ''
}

const setTimeRange = (val: 'realtime' | '1h' | '24h' | '7d') => {
  radar.selectedTimeRange = val
  radar.fetchCumulativeStats()
  if (val !== 'realtime') {
    radar.fetchHistory(val)
    radar.fetchHistoricalDestinations(val)
  }
}

watch(() => radar.selectedNodeId, () => {
  radar.fetchCumulativeStats()
  if (radar.selectedTimeRange !== 'realtime') {
    radar.fetchHistory()
    radar.fetchHistoricalDestinations()
  }
})

const deviceOptions = computed<DropdownOption[]>(() => {
  const list: DropdownOption[] = [
    { label: '全部内网终端', value: '' },
  ]
  const isAll = radar.selectedNodeId === 'all'
  const filtered = isAll
    ? radar.topDevices
    : radar.topDevices.filter((d) => !d.node_id || d.node_id === radar.selectedNodeId)

  for (const dev of filtered) {
    const ipDisplay = compressIP(dev.ip)
    const nodeName = isAll && dev.node_id ? getNodeName(dev.node_id) : ''
    const sub = nodeName ? `${ipDisplay} · ${nodeName}` : (dev.ip !== dev.name ? ipDisplay : undefined)
    list.push({
      label: dev.name,
      value: dev.ip,
      sublabel: sub,
    })
  }
  return list
})
</script>

<template>
  <div class="apple-glass rounded-3xl p-3.5 sm:p-5 mb-4 sm:mb-5 flex flex-col gap-3 sm:gap-4">
    <!-- Top Row: Time Range Selector & Device Filter -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200/50 dark:border-slate-800/50 pb-3">
      <!-- Time Range Pills -->
      <div class="flex items-center gap-1.5 sm:gap-2 flex-wrap">
        <span class="text-xs font-medium text-slate-500 dark:text-slate-400 flex items-center gap-1 whitespace-nowrap">
          <Clock class="w-3.5 h-3.5" />
          时间范围:
        </span>
        <div class="flex items-center p-0.5 sm:p-1 rounded-xl bg-slate-100 dark:bg-slate-800 text-xs">
          <button
            v-for="tr in timeRanges"
            :key="tr.value"
            @click="setTimeRange(tr.value)"
            class="px-2 sm:px-2.5 py-1 rounded-lg font-medium transition-all text-xs whitespace-nowrap"
            :class="radar.selectedTimeRange === tr.value ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-xs' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'"
          >
            {{ tr.label }}
          </button>
        </div>
      </div>

      <!-- Device Filter Dropdown -->
      <div class="flex items-center gap-1.5 sm:gap-2">
        <span class="text-xs font-medium text-slate-500 dark:text-slate-400 flex items-center gap-1 whitespace-nowrap">
          <Filter class="w-3.5 h-3.5" />
          设备过滤:
        </span>
        <AppleSelect
          v-model="radar.selectedDeviceIp"
          :options="deviceOptions"
          buttonClass="bg-slate-100/90 dark:bg-slate-800/90 hover:bg-slate-200/80 dark:hover:bg-slate-700/80 border border-slate-200/80 dark:border-slate-700/80 rounded-xl px-2.5 py-1 text-xs text-slate-700 dark:text-slate-200 min-w-[140px] max-w-[220px] sm:max-w-xs cursor-pointer transition-colors shadow-2xs"
          menuWidthClass="w-64"
          placement="left"
        />
      </div>
    </div>

    <!-- Bottom Row: 4 Metric Cards (Relocated Live Speeds + Cumulative Totals) -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-2.5 sm:gap-4">
      <!-- 1. Download Card -->
      <div class="p-3 sm:p-4 rounded-2xl bg-white/60 dark:bg-slate-800/40 border border-slate-200/50 dark:border-slate-700/50 flex items-center gap-2.5 sm:gap-3.5 shadow-2xs hover:bg-white/80 dark:hover:bg-slate-800/70 transition-all">
        <div class="p-2 sm:p-2.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 flex-shrink-0">
          <ArrowDownCircle class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-[11px] text-slate-400 dark:text-slate-500 truncate">
            {{ radar.selectedTimeRange === 'realtime' ? '实时下行' : '区间累计下行' }}
          </div>
          <div class="text-sm sm:text-base lg:text-lg font-bold text-emerald-600 dark:text-emerald-400 font-mono tracking-tight whitespace-nowrap overflow-hidden text-ellipsis leading-tight">
            {{ radar.selectedTimeRange === 'realtime' ? formatSpeed(radar.currentRateIn) : formatBytes(radar.cumulativeStats.total_bytes_in) }}
          </div>
          <div class="text-[10px] text-slate-400 font-mono truncate mt-0.5">
            {{ radar.selectedTimeRange === 'realtime' ? `累计 ${formatBytes(radar.totalIn)}` : '下行流量' }}
          </div>
        </div>
      </div>

      <!-- 2. Upload Card -->
      <div class="p-3 sm:p-4 rounded-2xl bg-white/60 dark:bg-slate-800/40 border border-slate-200/50 dark:border-slate-700/50 flex items-center gap-2.5 sm:gap-3.5 shadow-2xs hover:bg-white/80 dark:hover:bg-slate-800/70 transition-all">
        <div class="p-2 sm:p-2.5 rounded-xl bg-sky-50 dark:bg-sky-950/40 text-sky-600 dark:text-sky-400 flex-shrink-0">
          <ArrowUpCircle class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-[11px] text-slate-400 dark:text-slate-500 truncate">
            {{ radar.selectedTimeRange === 'realtime' ? '实时上行' : '区间累计上行' }}
          </div>
          <div class="text-sm sm:text-base lg:text-lg font-bold text-sky-600 dark:text-sky-400 font-mono tracking-tight whitespace-nowrap overflow-hidden text-ellipsis leading-tight">
            {{ radar.selectedTimeRange === 'realtime' ? formatSpeed(radar.currentRateOut) : formatBytes(radar.cumulativeStats.total_bytes_out) }}
          </div>
          <div class="text-[10px] text-slate-400 font-mono truncate mt-0.5">
            {{ radar.selectedTimeRange === 'realtime' ? `累计 ${formatBytes(radar.totalOut)}` : '上行流量' }}
          </div>
        </div>
      </div>

      <!-- 3. Active Connections Card -->
      <div class="p-3 sm:p-4 rounded-2xl bg-white/60 dark:bg-slate-800/40 border border-slate-200/50 dark:border-slate-700/50 flex items-center gap-2.5 sm:gap-3.5 shadow-2xs hover:bg-white/80 dark:hover:bg-slate-800/70 transition-all">
        <div class="p-2 sm:p-2.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 flex-shrink-0">
          <Activity class="w-4 h-4 sm:w-5 sm:h-5 text-slate-600 dark:text-slate-400" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-[11px] text-slate-400 dark:text-slate-500 truncate">
            {{ radar.selectedTimeRange === 'realtime' ? '活跃连接' : '区间连接数' }}
          </div>
          <div class="text-sm sm:text-base lg:text-lg font-bold text-slate-800 dark:text-slate-100 font-mono tracking-tight whitespace-nowrap overflow-hidden text-ellipsis leading-tight">
            {{ radar.selectedTimeRange === 'realtime' ? radar.activeConns : radar.cumulativeStats.total_requests }}
            <span class="text-xs font-normal text-slate-400">个</span>
          </div>
          <div class="text-[10px] text-slate-400 font-mono truncate mt-0.5">
            {{ radar.selectedTimeRange === 'realtime' ? `请求 ${radar.totalRequests} 次` : '会话总数' }}
          </div>
        </div>
      </div>

      <!-- 4. Total Throughput Card -->
      <div class="p-3 sm:p-4 rounded-2xl bg-white/60 dark:bg-slate-800/40 border border-slate-200/50 dark:border-slate-700/50 flex items-center gap-2.5 sm:gap-3.5 shadow-2xs hover:bg-white/80 dark:hover:bg-slate-800/70 transition-all">
        <div class="p-2 sm:p-2.5 rounded-xl bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400 flex-shrink-0">
          <Zap class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-[11px] text-slate-400 dark:text-slate-500 truncate">
            {{ radar.selectedTimeRange === 'realtime' ? '实时总吞吐' : '区间峰值速率' }}
          </div>
          <div class="text-sm sm:text-base lg:text-lg font-bold text-slate-800 dark:text-slate-100 font-mono tracking-tight whitespace-nowrap overflow-hidden text-ellipsis leading-tight">
            {{ radar.selectedTimeRange === 'realtime' ? formatSpeed(radar.currentRateIn + radar.currentRateOut) : formatSpeed(radar.cumulativeStats.peak_rate_in + radar.cumulativeStats.peak_rate_out) }}
          </div>
          <div class="text-[10px] text-slate-400 font-mono truncate mt-0.5">
            {{ radar.selectedTimeRange === 'realtime' ? '双向总速率' : '双向峰值' }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
