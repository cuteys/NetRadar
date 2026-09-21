<script setup lang="ts">
import { useRadarStore } from '../stores/radarStore'
import { formatBytes } from '../utils/format'
import { formatCountry } from '../utils/countryNames'
import { Terminal, Shield, ArrowDown, ArrowUp } from 'lucide-vue-next'

const radar = useRadarStore()

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
</script>

<template>
  <div class="apple-glass rounded-3xl p-4 sm:p-5 flex flex-col h-[320px] sm:h-[380px] lg:h-[480px] xl:h-[540px]">
    <!-- Header -->
    <div class="flex items-center justify-between mb-2">
      <div class="flex items-center gap-2">
        <div class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
          <Terminal class="w-4 h-4 text-emerald-500" />
        </div>
        <h3 class="text-xs sm:text-sm font-semibold text-slate-800 dark:text-slate-100">
          外联态势审计流水
        </h3>
      </div>
      <div class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
        <Shield class="w-3.5 h-3.5 text-emerald-500" />
        <span class="hidden sm:inline">实时监听流</span>
        <span class="text-[10px] font-mono">({{ radar.filteredLogs.length }})</span>
      </div>
    </div>

    <!-- Logs Table / Feed -->
    <div class="flex-1 overflow-y-auto space-y-1.5 pr-1 text-xs">
      <div
        v-for="log in radar.filteredLogs"
        :key="log.id || (log.src_ip + '-' + log.dst_ip + '-' + log.dst_port)"
        class="p-2 sm:p-2.5 rounded-2xl bg-white/50 dark:bg-slate-800/40 border border-slate-200/40 dark:border-slate-700/40 flex items-center justify-between gap-2 sm:gap-3 hover:bg-white/80 dark:hover:bg-slate-800/80 transition-all font-mono"
      >
        <!-- Left: Proto & IPs -->
        <div class="flex items-center gap-2 min-w-0 flex-1">
          <span
            class="px-2 py-0.5 rounded-md text-[10px] font-bold tracking-wider border flex-shrink-0"
            :class="getProtocolBadgeClass(log.protocol)"
          >
            {{ log.protocol }}
          </span>

          <div class="min-w-0 flex-1 flex flex-col sm:flex-row sm:items-center sm:gap-2">
            <!-- Target IP & Port: Always prominent and clearly visible -->
            <div class="flex items-center gap-1 min-w-0">
              <span class="text-slate-300 dark:text-slate-600 sm:hidden text-[10px]">➔</span>
              <span class="text-slate-800 dark:text-slate-100 font-semibold text-[11px] sm:text-xs truncate font-mono">
                {{ log.dst_ip }}:{{ log.dst_port }}
              </span>
            </div>

            <!-- Source Device & Location Info -->
            <div class="text-[10px] sm:text-[11px] text-slate-400 dark:text-slate-500 truncate flex items-center gap-1 font-mono">
              <span class="hidden sm:inline text-slate-300 dark:text-slate-600">◀</span>
              <span>{{ log.src_ip }}</span>
              <span class="sm:hidden text-slate-300 dark:text-slate-600">·</span>
              <span class="sm:hidden truncate">{{ formatCountry(log.country) }}</span>
            </div>
          </div>
        </div>

        <!-- Center: Geo & ISP (Tablet & Desktop) -->
        <div class="hidden sm:block text-[11px] text-slate-500 dark:text-slate-400 text-right truncate max-w-[140px] md:max-w-[180px]">
          {{ formatCountry(log.country) }} · {{ log.city || '数据中心' }}
        </div>

        <!-- Right: Speed / Delta -->
        <div class="flex items-center gap-2 text-right flex-shrink-0 text-[11px]">
          <span class="text-emerald-600 dark:text-emerald-400 font-semibold flex items-center">
            <ArrowDown class="w-2.5 h-2.5 mr-0.5" />
            {{ formatBytes(log.bytes_in) }}
          </span>
          <span class="text-sky-600 dark:text-sky-400 flex items-center">
            <ArrowUp class="w-2.5 h-2.5 mr-0.5" />
            {{ formatBytes(log.bytes_out) }}
          </span>
        </div>
      </div>

      <div
        v-if="radar.filteredLogs.length === 0"
        class="h-full flex items-center justify-center text-slate-400 text-xs py-8"
      >
        等待实时网络连接中...
      </div>
    </div>
  </div>
</template>
