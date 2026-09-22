<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import * as echarts from 'echarts'
import { useRadarStore } from '../stores/radarStore'
import { useThemeStore } from '../stores/themeStore'
import { formatBytes } from '../utils/format'
import { formatCountry } from '../utils/countryNames'
import { PieChart } from 'lucide-vue-next'

const radar = useRadarStore()
const theme = useThemeStore()

const chartContainer = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

const viewMode = ref<'proto' | 'country'>('proto')

const palette = [
  '#10b981', '#0ea5e9', '#f59e0b', '#ec4899',
  '#8b5cf6', '#06b6d4', '#84cc16', '#f97316',
]

const itemsList = computed(() => {
  const isProto = viewMode.value === 'proto'
  const source = isProto ? radar.effectiveProtoDist : radar.effectiveCountryDist

  let total = 0
  const list: { name: string; value: number; percent: number; color: string }[] = []

  for (const [k, v] of Object.entries(source || {})) {
    if (v <= 0) continue
    total += v
    list.push({
      name: isProto ? k : formatCountry(k),
      value: v,
      percent: 0,
      color: '',
    })
  }

  if (list.length === 0) {
    return []
  }

  list.sort((a, b) => b.value - a.value)
  list.forEach((item, idx) => {
    item.percent = total > 0 ? Math.round((item.value / total) * 100) : 0
    item.color = palette[idx % palette.length]
  })

  return list.slice(0, 5)
})

const topItem = computed(() => {
  return itemsList.value[0] || { name: '等待采样', percent: 0 }
})

let updateDonutRaf: number | null = null
const updateChart = () => {
  if (!chart || radar.isPaused) return
  if (updateDonutRaf) return
  updateDonutRaf = requestAnimationFrame(() => {
    updateDonutRaf = null
    if (!chart || radar.isPaused) return

    const isDark = theme.isDark
    const hasData = itemsList.value.length > 0
    const data = hasData
      ? itemsList.value.map((item) => ({
          name: item.name,
          value: item.value,
          itemStyle: { color: item.color },
        }))
      : [{ value: 1, name: '等待流量', itemStyle: { color: isDark ? '#334155' : '#e2e8f0' } }]

    const option: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      animation: false,
      tooltip: {
        trigger: 'item',
        confine: true,
        backgroundColor: isDark ? 'rgba(15, 23, 42, 0.94)' : 'rgba(255, 255, 255, 0.95)',
        borderColor: isDark ? '#334155' : '#e2e8f0',
        borderWidth: 1,
        textStyle: {
          color: isDark ? '#f8fafc' : '#1e293b',
          fontSize: 12,
        },
        formatter: (params: any) => {
          if (!hasData) return '等待捕获连接流量'
          return `
            <div style="font-weight: 600; margin-bottom: 2px;">${params.name}</div>
            <div>流量占比: <strong>${params.percent}%</strong></div>
            <div style="font-size: 11px; opacity: 0.7;">累计字节: ${formatBytes(params.value)}</div>
          `
        },
      },
      title: {
        text: hasData ? topItem.value.name : '暂无数据',
        subtext: hasData ? `${topItem.value.percent}% 主导` : '等待探针上报',
        left: 'center',
        top: '38%',
        textStyle: {
          fontSize: 13,
          fontWeight: 'bold',
          color: isDark ? '#f1f5f9' : '#1e293b',
        },
        subtextStyle: {
          fontSize: 11,
          color: isDark ? '#94a3b8' : '#64748b',
        },
      },
      series: [
        {
          name: viewMode.value === 'proto' ? '协议类型' : '目的国家',
          type: 'pie',
          radius: ['52%', '74%'],
          center: ['50%', '48%'],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 6,
            borderColor: isDark ? '#131926' : '#ffffff',
            borderWidth: 2,
          },
          label: {
            show: false,
          },
          emphasis: {
            scale: hasData,
            scaleSize: 5,
          },
          data: data,
        },
      ],
    }

    chart.setOption(option)
  })
}

const toggleMode = (m: 'proto' | 'country') => {
  viewMode.value = m
  updateChart()
}

const handleResize = () => {
  chart?.resize()
}

onMounted(() => {
  if (chartContainer.value) {
    chart = echarts.init(chartContainer.value, null, {
      renderer: 'canvas',
      devicePixelRatio: Math.min(window.devicePixelRatio || 1, 1.5),
    })
    updateChart()
    window.addEventListener('resize', handleResize)
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (updateDonutRaf) cancelAnimationFrame(updateDonutRaf)
  chart?.dispose()
  chart = null
})

watch(() => radar.effectiveProtoDist, () => updateChart())
watch(() => radar.effectiveCountryDist, () => updateChart())
watch(() => radar.selectedTimeRange, () => updateChart())
watch(() => theme.isDark, () => updateChart())
</script>

<template>
  <div class="apple-glass rounded-3xl p-4 sm:p-5 flex flex-col h-[320px]">
    <div class="flex items-center justify-between mb-1">
      <div class="flex items-center gap-2">
        <div class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
          <PieChart class="w-4 h-4 text-emerald-500" />
        </div>
        <h3 class="text-xs sm:text-sm font-semibold text-slate-800 dark:text-slate-100">
          {{ viewMode === 'proto' ? '传输协议分布' : '外联地区分布' }}
        </h3>
      </div>

      <div class="flex items-center p-0.5 rounded-lg bg-slate-100 dark:bg-slate-800 text-[11px]">
        <button
          @click="toggleMode('proto')"
          class="px-2.5 py-0.5 rounded-md transition-all font-medium"
          :class="viewMode === 'proto' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-xs' : 'text-slate-500'"
        >
          协议
        </button>
        <button
          @click="toggleMode('country')"
          class="px-2.5 py-0.5 rounded-md transition-all font-medium"
          :class="viewMode === 'country' ? 'bg-white dark:bg-slate-700 text-slate-800 dark:text-white shadow-xs' : 'text-slate-500'"
        >
          地区
        </button>
      </div>
    </div>

    <div ref="chartContainer" class="w-full h-[180px] relative"></div>

    <div class="mt-1 pt-2 border-t border-slate-100 dark:border-slate-800/60 grid grid-cols-2 gap-x-3 gap-y-1.5 text-[11px]">
      <template v-if="itemsList.length > 0">
        <div
          v-for="item in itemsList.slice(0, 4)"
          :key="item.name"
          class="flex items-center justify-between min-w-0"
        >
          <div class="flex items-center gap-1.5 min-w-0">
            <span
              class="w-2 h-2 rounded-full flex-shrink-0"
              :style="{ backgroundColor: item.color }"
            ></span>
            <span class="truncate text-slate-600 dark:text-slate-400 font-medium">
              {{ item.name }}
            </span>
          </div>
          <span class="font-mono text-slate-800 dark:text-slate-200 font-semibold flex-shrink-0">
            {{ item.percent }}%
          </span>
        </div>
      </template>
      <div v-else class="col-span-2 text-center text-slate-400 text-xs py-1">
        等待流量数据捕获...
      </div>
    </div>
  </div>
</template>
