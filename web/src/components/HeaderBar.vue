<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRadarStore } from '../stores/radarStore'
import { useThemeStore } from '../stores/themeStore'
import { useAuthStore } from '../stores/authStore'
import { formatSpeed } from '../utils/format'
import SettingsModal from './SettingsModal.vue'
import NodeManagerModal from './NodeManagerModal.vue'
import AppleSelect, { type DropdownOption } from './AppleSelect.vue'
import {
  Radar,
  ArrowDown,
  ArrowUp,
  Activity,
  Sun,
  Moon,
  Monitor,
  LogOut,
  Settings,
  Server,
  ChevronDown,
  Pause,
  Play
} from 'lucide-vue-next'

const radar = useRadarStore()
const theme = useThemeStore()
const auth = useAuthStore()

const showSettings = ref(false)
const showNodeManager = ref(false)

const activeNodeName = computed(() => {
  if (radar.selectedNodeId === 'all') return '全部节点'
  const n = radar.nodes.find((item) => item.id === radar.selectedNodeId)
  return n ? n.name : '边缘节点'
})

const nodeOptions = computed<DropdownOption[]>(() => {
  const list: DropdownOption[] = [
    { label: '全部节点', value: 'all' },
  ]
  for (const node of radar.nodes) {
    list.push({
      label: node.name,
      value: node.id,
      badge: node.is_online ? '在线' : '离线',
      badgeColor: node.is_online ? 'emerald' : 'slate',
    })
  }
  return list
})
</script>

<template>
  <header class="apple-glass rounded-2xl px-4 sm:px-6 py-3 mb-4 flex flex-wrap items-center justify-between gap-3 sm:gap-4 transition-all">
    <!-- Left: Sleek Logo & Multi-Agent Node Switcher -->
    <div class="flex items-center gap-3 sm:gap-4">
      <!-- Apple Pure Monochromatic Radar Icon -->
      <div class="relative flex items-center justify-center w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 shadow-sm flex-shrink-0">
        <Radar class="w-5 h-5 text-emerald-500 dark:text-emerald-600" />
        <span class="absolute -top-0.5 -right-0.5 flex h-2 w-2 rounded-full bg-emerald-500"></span>
      </div>

      <div>
        <div class="flex items-center gap-2">
          <h1 class="font-bold text-base sm:text-lg tracking-tight text-slate-900 dark:text-white">
            NetRadar
          </h1>
        </div>

        <!-- Node Selector Capsule Pill & Manage Button -->
        <div class="inline-flex items-center mt-1 p-0.5 rounded-xl bg-slate-100/90 dark:bg-slate-800/90 border border-slate-200/70 dark:border-slate-700/70 shadow-2xs">
          <AppleSelect
            v-model="radar.selectedNodeId"
            :options="nodeOptions"
            buttonClass="px-2 py-0.5 text-xs text-slate-700 dark:text-slate-200 hover:text-slate-900 dark:hover:text-white"
            menuWidthClass="w-52"
          >
            <template #prefix>
              <Server class="w-3.5 h-3.5 text-slate-400 dark:text-slate-400 mr-1 flex-shrink-0" />
            </template>
          </AppleSelect>
          <div class="h-3 w-[1px] bg-slate-200 dark:bg-slate-700 my-auto mx-0.5"></div>
          <button
            @click="showNodeManager = true"
            class="text-[11px] px-2 py-0.5 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 hover:bg-white dark:hover:bg-slate-700/80 transition-all font-medium"
            title="探针节点管理与删除"
          >
            管理
          </button>
        </div>
      </div>
    </div>

    <!-- Right: Pause, Settings, Theme Switcher & Logout (Pixel-perfect h-9 capsules) -->
    <div class="flex items-center gap-2">
      <!-- Pause / Resume Real-time Stream Toggle -->
      <button
        @click="radar.togglePause()"
        class="h-9 px-3 rounded-xl inline-flex items-center justify-center gap-1.5 text-xs font-medium transition-all shadow-2xs"
        :class="radar.isPaused
          ? 'bg-amber-500 text-white animate-pulse'
          : 'bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300'"
        :title="radar.isPaused ? '大屏已暂停实时刷新，点击恢复' : '点击暂停实时刷新'"
      >
        <Play v-if="radar.isPaused" class="w-3.5 h-3.5 fill-current" />
        <Pause v-else class="w-3.5 h-3.5" />
        <span class="text-[11px]">{{ radar.isPaused ? '已暂停' : '暂停' }}</span>
      </button>

      <!-- System & Security Settings Button -->
      <button
        @click="showSettings = true"
        class="w-9 h-9 rounded-xl inline-flex items-center justify-center bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 transition-colors shadow-2xs"
        title="系统与安全配置"
      >
        <Settings class="w-4 h-4" />
      </button>

      <!-- Theme Mode Toggle -->
      <button
        @click="theme.toggle()"
        class="w-9 h-9 rounded-xl inline-flex items-center justify-center bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 transition-colors shadow-2xs"
        :title="'当前模式: ' + theme.mode"
      >
        <Sun v-if="theme.mode === 'light'" class="w-4 h-4 text-amber-500" />
        <Moon v-else-if="theme.mode === 'dark'" class="w-4 h-4 text-sky-400" />
        <Monitor v-else class="w-4 h-4 text-slate-400" />
      </button>

      <!-- Logout -->
      <button
        @click="auth.logout()"
        class="h-9 px-3 rounded-xl inline-flex items-center justify-center gap-1.5 bg-slate-100 dark:bg-slate-800 text-xs text-slate-600 dark:text-slate-300 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors shadow-2xs"
        title="退出控制台"
      >
        <LogOut class="w-3.5 h-3.5" />
        <span class="hidden sm:inline">退出</span>
      </button>
    </div>

    <!-- Settings Dialog -->
    <SettingsModal :open="showSettings" @close="showSettings = false" />

    <!-- Node Manager Dialog -->
    <NodeManagerModal :open="showNodeManager" @close="showNodeManager = false" />
  </header>
</template>
