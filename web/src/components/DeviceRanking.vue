<script setup lang="ts">
import { ref } from 'vue'
import { useRadarStore } from '../stores/radarStore'
import { formatSpeed, compressIP, formatDeviceName } from '../utils/format'
import {
  Smartphone,
  Laptop,
  Tv,
  HardDrive,
  Cpu,
  Layers,
  ArrowDown,
  ArrowUp,
  Edit2,
  Check,
  X
} from 'lucide-vue-next'

const radar = useRadarStore()

const editingIp = ref<string | null>(null)
const editNameInput = ref('')

const getIconComponent = (cat: string) => {
  switch (cat?.toLowerCase()) {
    case 'mobile':
      return Smartphone
    case 'pc':
      return Laptop
    case 'tv':
      return Tv
    case 'nas':
      return HardDrive
    default:
      return Cpu
  }
}

const startRename = (ip: string, currentName: string, event: Event) => {
  event.stopPropagation()
  editingIp.value = ip
  editNameInput.value = currentName
}

const saveRename = async (ip: string, event: Event) => {
  event.stopPropagation()
  if (editNameInput.value.trim()) {
    await radar.renameDevice(ip, editNameInput.value.trim())
  }
  editingIp.value = null
}

const cancelRename = (event: Event) => {
  event.stopPropagation()
  editingIp.value = null
}

const toggleSelectDevice = (ip: string) => {
  if (radar.selectedDeviceIp === ip) {
    radar.selectedDeviceIp = ''
  } else {
    radar.selectedDeviceIp = ip
  }
}
</script>

<template>
  <div class="apple-glass rounded-3xl p-4 sm:p-5 flex flex-col h-[320px]">
    <div class="flex items-center justify-between mb-2">
      <div class="flex items-center gap-2">
        <div class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
          <Layers class="w-4 h-4 text-emerald-500" />
        </div>
        <h3 class="text-xs sm:text-sm font-semibold text-slate-800 dark:text-slate-100">
          局域网终端榜
        </h3>
      </div>
      <span class="text-xs px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400">
        {{ radar.topDevices.length }} 设备
      </span>
    </div>

    <div class="flex-1 overflow-y-auto space-y-2 pr-1">
      <div
        v-for="dev in radar.topDevices"
        :key="(dev.node_id || '') + ':' + dev.ip"
        @click="toggleSelectDevice(dev.ip)"
        class="p-2.5 rounded-2xl border transition-all cursor-pointer"
        :class="radar.selectedDeviceIp === dev.ip
          ? 'bg-emerald-50/80 dark:bg-emerald-950/40 border-emerald-300 dark:border-emerald-700/60 shadow-xs'
          : 'bg-white/50 dark:bg-slate-800/40 border-slate-200/40 dark:border-slate-700/40 hover:bg-white/80 dark:hover:bg-slate-800/80'"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2 min-w-0 flex-1">
            <div class="w-7 h-7 rounded-xl bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 flex items-center justify-center flex-shrink-0">
              <component :is="getIconComponent(dev.category)" class="w-3.5 h-3.5" />
            </div>

            <div class="min-w-0 flex-1">
              <div v-if="editingIp === dev.ip" class="flex items-center gap-1.5 flex-1 min-w-0" @click.stop>
                <input
                  v-model="editNameInput"
                  type="text"
                  class="flex-1 min-w-[120px] px-2 py-1 text-xs rounded-xl bg-white dark:bg-slate-900 border border-emerald-500 focus:outline-none font-mono"
                  autofocus
                  @keydown.enter="saveRename(dev.ip, $event)"
                />
                <button
                  @click="saveRename(dev.ip, $event)"
                  class="p-1.5 rounded-xl bg-emerald-500/10 text-emerald-600 hover:bg-emerald-500/20 transition-colors flex-shrink-0"
                  title="确认保存"
                >
                  <Check class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="cancelRename($event)"
                  class="p-1.5 rounded-xl bg-slate-100 dark:bg-slate-700 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors flex-shrink-0"
                  title="取消"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>

              <div v-else class="flex items-center gap-1.5 min-w-0">
                <span class="text-xs font-semibold text-slate-800 dark:text-slate-200 truncate font-mono" :title="dev.ip">
                  {{ formatDeviceName(dev.name, dev.ip) }}
                </span>
                <span
                  v-if="radar.selectedNodeId === 'all' && dev.node_id"
                  class="text-[10px] px-1.5 py-0.2 rounded bg-slate-100 dark:bg-slate-700 text-slate-500 dark:text-slate-400 font-normal max-w-[70px] sm:max-w-none truncate flex-shrink-0"
                >
                  {{ radar.getNodeName(dev.node_id) }}
                </span>
                <span
                  v-if="dev.is_custom && dev.name !== dev.ip"
                  class="text-[10px] text-slate-400 font-mono truncate hidden sm:inline flex-shrink-0"
                >
                  ({{ compressIP(dev.ip) }})
                </span>
                <button
                  @click="startRename(dev.ip, formatDeviceName(dev.name, dev.ip), $event)"
                  class="p-0.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors flex-shrink-0"
                  title="重命名该设备"
                >
                  <Edit2 class="w-3 h-3" />
                </button>
              </div>
            </div>
          </div>

          <div v-if="editingIp !== dev.ip" class="flex flex-col sm:flex-row items-end sm:items-center gap-0.5 sm:gap-2 text-xs font-mono flex-shrink-0">
            <span class="flex items-center text-emerald-600 dark:text-emerald-400 font-semibold whitespace-nowrap text-[11px] sm:text-xs">
              <ArrowDown class="w-2.5 h-2.5 mr-0.5" />
              {{ formatSpeed(dev.rate_in_bps) }}
            </span>
            <span class="flex items-center text-sky-600 dark:text-sky-400 text-[10px] sm:text-[11px] whitespace-nowrap">
              <ArrowUp class="w-2.5 h-2.5 mr-0.5" />
              {{ formatSpeed(dev.rate_out_bps) }}
            </span>
          </div>
        </div>
      </div>

      <div
        v-if="radar.topDevices.length === 0"
        class="h-full flex flex-col items-center justify-center text-slate-400 text-xs py-8"
      >
        等待局域网流量上报...
      </div>
    </div>
  </div>
</template>
