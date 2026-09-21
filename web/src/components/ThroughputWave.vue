<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as echarts from 'echarts'
import { useRadarStore } from '../stores/radarStore'
import { useThemeStore } from '../stores/themeStore'
import { formatSpeed } from '../utils/format'
import { Activity, ArrowDown, ArrowUp } from 'lucide-vue-next'

const radar = useRadarStore()
const theme = useThemeStore()

const chartContainer = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

let isInitialized = false

const initChart = () => {
  if (!chart) return

  const isDark = theme.isDark
  const points = radar.historyPoints || []

  const xData = points.map((p) => p.time)
  const inData = points.map((p) => p.inRate)
  const outData = points.map((p) => p.outRate)

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    animation: false,
    animationDuration: 0,
    animationDurationUpdate: 0,
    grid: {
      top: '16%',
      left: '2%',
      right: '2%',
      bottom: '6%',
      containLabel: true,
    },
    tooltip: {
      trigger: 'axis',
      confine: true,
      backgroundColor: isDark ? 'rgba(15, 23, 42, 0.94)' : 'rgba(255, 255, 255, 0.95)',
      borderColor: isDark ? '#334155' : '#e2e8f0',
      borderWidth: 1,
      textStyle: {
        color: isDark ? '#f8fafc' : '#1e293b',
        fontSize: 12,
        fontFamily: '-apple-system, sans-serif',
      },
      formatter: (params: any) => {
        let html = `<div style="font-size: 11px; margin-bottom: 4px; opacity: 0.7;">${params[0]?.axisValue || ''}</div>`
        params.forEach((item: any) => {
          const val = formatSpeed(item.value)
          html += `
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 16px; font-size: 12px; margin-top: 2px;">
              <span style="display: flex; align-items: center; gap: 6px;">
                <span style="width: 8px; height: 8px; border-radius: 50%; background: ${item.color};"></span>
                ${item.seriesName}
              </span>
              <strong style="font-family: monospace;">${val}</strong>
            </div>
          `
        })
        return html
      },
    },
    xAxis: {
      type: 'category',
      data: xData,
      boundaryGap: false,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: isDark ? '#64748b' : '#94a3b8',
        fontSize: 10,
        interval: Math.floor(xData.length / 5) || 1,
      },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: {
        lineStyle: {
          color: isDark ? 'rgba(51, 65, 85, 0.4)' : 'rgba(226, 232, 240, 0.6)',
          type: 'dashed',
        },
      },
      axisLabel: {
        color: isDark ? '#64748b' : '#94a3b8',
        fontSize: 10,
        formatter: (val: number) => formatSpeed(val),
      },
    },
    series: [
      {
        name: '下行速率',
        type: 'line',
        smooth: 0.35,
        symbol: 'none',
        lineStyle: {
          width: 2,
          color: '#10b981',
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(16, 185, 129, 0.28)' },
            { offset: 1, color: 'rgba(16, 185, 129, 0.01)' },
          ]),
        },
        data: inData,
      },
      {
        name: '上行速率',
        type: 'line',
        smooth: 0.35,
        symbol: 'none',
        lineStyle: {
          width: 2,
          color: '#0284c7',
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(14, 165, 233, 0.28)' },
            { offset: 1, color: 'rgba(14, 165, 233, 0.01)' },
          ]),
        },
        data: outData,
      },
    ],
  }

  chart.setOption(option, true)
  isInitialized = true
}

let updateWaveRaf: number | null = null
const updateData = () => {
  if (!chart || !isInitialized || radar.isPaused) return
  if (updateWaveRaf) return
  updateWaveRaf = requestAnimationFrame(() => {
    updateWaveRaf = null
    if (!chart || !isInitialized || radar.isPaused) return
    const points = radar.historyPoints || []
    const xData = points.map((p) => p.time)
    const inData = points.map((p) => p.inRate)
    const outData = points.map((p) => p.outRate)

    chart.setOption({
      xAxis: {
        data: xData,
        axisLabel: {
          interval: Math.floor(xData.length / 5) || 1,
        },
      },
      series: [
        { data: inData },
        { data: outData },
      ],
    }, false)
  })
}

onMounted(() => {
  if (chartContainer.value) {
    chart = echarts.init(chartContainer.value, null, {
      renderer: 'canvas',
      devicePixelRatio: Math.min(window.devicePixelRatio || 1, 1.5),
    })
    initChart()
    window.addEventListener('resize', () => chart?.resize())
  }
})

onUnmounted(() => {
  if (updateWaveRaf) cancelAnimationFrame(updateWaveRaf)
  chart?.dispose()
  chart = null
})

watch(() => radar.historyPoints, () => {
  updateData()
})

watch(() => radar.selectedTimeRange, () => {
  updateData()
})

watch(() => theme.isDark, () => {
  initChart()
})
</script>

<template>
  <div class="apple-glass rounded-3xl p-4 sm:p-5 flex flex-col h-[320px]">
    <!-- Header -->
    <div class="flex items-center justify-between mb-2 gap-2">
      <div class="flex items-center gap-2 flex-shrink-0">
        <div class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
          <Activity class="w-4 h-4 text-emerald-500" />
        </div>
        <h3 class="text-xs sm:text-sm font-semibold text-slate-800 dark:text-slate-100 whitespace-nowrap">
          {{ radar.selectedTimeRange === 'realtime' ? '实时吞吐波形' : '区间吞吐历史' }}
        </h3>
      </div>

      <!-- Legend -->
      <div class="flex items-center gap-1.5 sm:gap-2.5 text-[11px] sm:text-xs font-mono flex-shrink-0">
        <div class="flex items-center gap-0.5 sm:gap-1 text-emerald-600 dark:text-emerald-400 font-medium whitespace-nowrap">
          <span class="w-1.5 h-1.5 sm:w-2 sm:h-2 rounded-full bg-emerald-500"></span>
          <ArrowDown class="w-2.5 h-2.5 sm:w-3 sm:h-3" />
          <span>{{ radar.selectedTimeRange === 'realtime' ? formatSpeed(radar.currentRateIn) : '下行吞吐' }}</span>
        </div>
        <div class="flex items-center gap-0.5 sm:gap-1 text-sky-600 dark:text-sky-400 font-medium whitespace-nowrap">
          <span class="w-1.5 h-1.5 sm:w-2 sm:h-2 rounded-full bg-sky-500"></span>
          <ArrowUp class="w-2.5 h-2.5 sm:w-3 sm:h-3" />
          <span>{{ radar.selectedTimeRange === 'realtime' ? formatSpeed(radar.currentRateOut) : '上行吞吐' }}</span>
        </div>
      </div>
    </div>

    <!-- Chart container -->
    <div ref="chartContainer" class="w-full flex-1 min-h-0"></div>
  </div>
</template>
