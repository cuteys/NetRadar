<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRadarStore } from '../stores/radarStore'
import {
  X,
  Server,
  Trash2,
  Edit3,
  CheckCircle2,
  XCircle,
  Copy,
  Check,
  Terminal,
  MapPin,
  ArrowLeft,
  Globe2,
  Compass
} from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const radar = useRadarStore()
const copied = ref(false)
const deletingId = ref<string | null>(null)
const editingNode = ref<any | null>(null)

const editForm = ref({
  name: '',
  locationMode: 'auto' as 'auto' | 'manual',
  manualType: 'ip' as 'ip' | 'coords',
  ip: '',
  lat: 0,
  lng: 0,
  geoInfo: '',
})
const saveSuccess = ref(false)
const saveError = ref('')

const useGhProxy = ref(false)

watch(() => props.open, (val) => {
  if (val) {
    radar.fetchNodes()
    radar.fetchSystemSettings()
  }
})

const installCommand = computed(() => {
  const defaultHost = typeof window !== 'undefined' ? window.location.host : '127.0.0.1:8899'
  const host = radar.systemSettings?.agent_server_addr || defaultHost
  const token = radar.systemSettings?.agent_token || 'netradar_secret_token_12345'
  const tls = radar.systemSettings?.use_tls ? ' --tls' : ''
  const proxyPrefix = useGhProxy.value ? 'https://gh-proxy.com/' : ''
  return `curl -fsSL -k ${proxyPrefix}https://raw.githubusercontent.com/cuteys/NetRadar/master/install-agent.sh | sh -s -- -s "${host}" -t "${token}"${tls}`
})

const uninstallCommand = computed(() => {
  const proxyPrefix = useGhProxy.value ? 'https://gh-proxy.com/' : ''
  return `curl -fsSL -k ${proxyPrefix}https://raw.githubusercontent.com/cuteys/NetRadar/master/install-agent.sh | sh -s -- --uninstall`
})

const rawBinaryCommand = computed(() => {
  const proto = radar.systemSettings?.use_tls ? 'wss:' : 'ws:'
  const defaultHost = typeof window !== 'undefined' ? window.location.host : '127.0.0.1:8899'
  const host = radar.systemSettings?.agent_server_addr || defaultHost
  const token = radar.systemSettings?.agent_token || 'netradar_secret_token_12345'
  return `./agent -server "${proto}//${host}/ws/agent" -token "${token}"`
})

const copyCommand = (text?: string) => {
  const toCopy = typeof text === 'string' ? text : installCommand.value
  navigator.clipboard.writeText(toCopy)
  copied.value = true
  setTimeout(() => {
    copied.value = false
  }, 2000)
}

const startEdit = (node: any) => {
  editingNode.value = node
  editForm.value = {
    name: node.name || '',
    locationMode: node.custom_location ? 'manual' : 'auto',
    manualType: 'ip',
    ip: node.ip || '',
    lat: node.gateway_lat || 0,
    lng: node.gateway_lng || 0,
    geoInfo: '',
  }
  saveSuccess.value = false
  saveError.value = ''
}

const cancelEdit = () => {
  editingNode.value = null
  saveSuccess.value = false
  saveError.value = ''
}

const handleSaveNode = async () => {
  if (!editingNode.value) return
  saveError.value = ''

  const isManual = editForm.value.locationMode === 'manual'
  const ok = await radar.updateNode({
    id: editingNode.value.id,
    name: editForm.value.name.trim() || editingNode.value.name,
    ip: editForm.value.ip.trim(),
    gateway_lat: Number(editForm.value.lat) || 0,
    gateway_lng: Number(editForm.value.lng) || 0,
    custom_location: isManual,
  })

  if (ok) {
    saveSuccess.value = true
    setTimeout(() => {
      editingNode.value = null
      saveSuccess.value = false
    }, 600)
  } else {
    saveError.value = '保存失败，请检查参数'
  }
}

const handleDelete = async (id: string, name: string) => {
  if (confirm(`确定要彻底删除探针节点 "${name}" (${id}) 吗？相关历史流量记录也将被清理。`)) {
    deletingId.value = id
    await radar.deleteNode(id)
    deletingId.value = null
    if (editingNode.value?.id === id) {
      editingNode.value = null
    }
  }
}

const formatDate = (d?: string) => {
  if (!d) return '刚刚'
  try {
    return new Date(d).toLocaleString('zh-CN', { hour12: false })
  } catch {
    return d
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="open"
        @click.self="emit('close')"
        class="fixed inset-0 z-[9999] flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/75 transition-all"
      >
        <div
          class="w-full max-w-2xl apple-glass-heavy rounded-3xl p-6 sm:p-7 shadow-2xl border border-slate-200/80 dark:border-slate-800/80 text-slate-800 dark:text-slate-100 relative max-h-[90vh] flex flex-col animate-scale-in"
        >
          <!-- Close Button -->
          <button
            @click="emit('close')"
            class="absolute top-5 right-5 p-2 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-all"
          >
            <X class="w-4 h-4" />
          </button>

          <!-- Header -->
          <div class="flex items-center gap-3 mb-5">
            <div class="p-2.5 rounded-2xl bg-slate-100 dark:bg-slate-800 text-slate-800 dark:text-slate-200 border border-slate-200/60 dark:border-slate-700/60">
              <Server class="w-5 h-5 text-emerald-500" />
            </div>
            <h3 class="font-bold text-base text-slate-900 dark:text-white">边缘探针节点管理</h3>
          </div>

          <!-- EDIT NODE VIEW -->
          <div v-if="editingNode" class="flex-1 overflow-y-auto space-y-4 pr-1">
            <div class="flex items-center justify-between pb-2 border-b border-slate-200/60 dark:border-slate-800/60">
              <button
                @click="cancelEdit"
                class="flex items-center gap-1.5 text-xs font-medium text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white transition-colors"
              >
                <ArrowLeft class="w-3.5 h-3.5" />
                <span>返回节点列表</span>
              </button>
              <span class="text-xs text-slate-400 font-mono">UUID: {{ editingNode.id }}</span>
            </div>

            <!-- Edit Node Name -->
            <div class="p-3.5 rounded-2xl bg-slate-100/70 dark:bg-slate-800/50 border border-slate-200/60 dark:border-slate-700/60 space-y-2">
              <label class="block text-xs font-semibold text-slate-800 dark:text-slate-200">
                节点名称
              </label>
              <input
                v-model="editForm.name"
                type="text"
                class="w-full px-3.5 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-500/20 focus:border-emerald-500 transition-all font-medium"
                placeholder="例如: 客厅小米AX6000 或 东京核心节点"
              />
            </div>

            <!-- Positioning Settings (Auto Fetch vs Manual Fixed) -->
            <div class="p-3.5 rounded-2xl bg-slate-100/70 dark:bg-slate-800/50 border border-slate-200/60 dark:border-slate-700/60 space-y-3">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <MapPin class="w-4 h-4 text-emerald-500" />
                  <span class="text-xs font-semibold text-slate-800 dark:text-slate-200">
                    网关地理定位与 IP 设置
                  </span>
                </div>
                <!-- Mode Switcher: 自动获取 / 手动固定 -->
                <div class="flex items-center p-0.5 rounded-xl bg-slate-200/80 dark:bg-slate-700/80 text-[11px]">
                  <button
                    type="button"
                    @click="editForm.locationMode = 'auto'"
                    class="px-2.5 py-1 rounded-lg font-medium transition-all"
                    :class="editForm.locationMode === 'auto' ? 'bg-white dark:bg-slate-800 text-slate-800 dark:text-white shadow-xs' : 'text-slate-500 dark:text-slate-400'"
                  >
                    自动获取
                  </button>
                  <button
                    type="button"
                    @click="editForm.locationMode = 'manual'"
                    class="px-2.5 py-1 rounded-lg font-medium transition-all"
                    :class="editForm.locationMode === 'manual' ? 'bg-white dark:bg-slate-800 text-slate-800 dark:text-white shadow-xs' : 'text-slate-500 dark:text-slate-400'"
                  >
                    手动固定
                  </button>
                </div>
              </div>

              <!-- AUTO FETCH MODE -->
              <div v-if="editForm.locationMode === 'auto'" class="space-y-2.5">
                <div class="p-2.5 rounded-xl bg-white/70 dark:bg-slate-900/50 border border-slate-200/50 dark:border-slate-800/50 text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
                  由探针节点自身向接口探测公网 IP 与地理坐标并实时上报。
                </div>

                <!-- Auto Detected IP & Geolocation Result Card -->
                <div class="p-3 rounded-xl bg-white/60 dark:bg-slate-900/40 border border-slate-200/40 dark:border-slate-800/40 space-y-1.5 text-xs font-mono">
                  <div class="flex items-center justify-between">
                    <span class="text-slate-400 text-[11px]">公网 IP 地址:</span>
                    <span class="font-semibold text-slate-800 dark:text-slate-200">{{ editForm.ip || '探针自动上报中' }}</span>
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-slate-400 text-[11px]">地理坐标:</span>
                    <span class="text-emerald-600 dark:text-emerald-400">[{{ editForm.lng ? editForm.lng.toFixed(4) : '0' }}, {{ editForm.lat ? editForm.lat.toFixed(4) : '0' }}]</span>
                  </div>
                </div>
              </div>

              <!-- MANUAL FIXED MODE (IP / Coords 二选一) -->
              <div v-else class="space-y-3">
                <div class="flex items-center gap-2 text-[11px] text-slate-500 dark:text-slate-400">
                  <span>选择录入方式:</span>
                  <div class="inline-flex rounded-lg bg-slate-200/70 dark:bg-slate-700/60 p-0.5">
                    <button
                      type="button"
                      @click="editForm.manualType = 'ip'"
                      class="px-2 py-0.5 rounded-md transition-all font-medium"
                      :class="editForm.manualType === 'ip' ? 'bg-white dark:bg-slate-800 text-slate-800 dark:text-white shadow-2xs' : 'text-slate-500'"
                    >
                      固定 IP 地址
                    </button>
                    <button
                      type="button"
                      @click="editForm.manualType = 'coords'"
                      class="px-2 py-0.5 rounded-md transition-all font-medium"
                      :class="editForm.manualType === 'coords' ? 'bg-white dark:bg-slate-800 text-slate-800 dark:text-white shadow-2xs' : 'text-slate-500'"
                    >
                      经纬度坐标
                    </button>
                  </div>
                </div>

                <!-- Manual Fixed IP -->
                <div v-if="editForm.manualType === 'ip'" class="space-y-1.5">
                  <label class="block text-[11px] text-slate-500 dark:text-slate-400">
                    固定公网/内网入口 IP 地址
                  </label>
                  <input
                    v-model="editForm.ip"
                    type="text"
                    class="w-full px-3.5 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:border-emerald-500 font-mono"
                    placeholder="例如: 114.240.12.34 或 240e:..."
                  />
                </div>

                <!-- Manual Coords -->
                <div v-else class="grid grid-cols-2 gap-3">
                  <div>
                    <label class="block text-[11px] text-slate-500 dark:text-slate-400 mb-1">
                      经度 (Longitude)
                    </label>
                    <input
                      v-model.number="editForm.lng"
                      type="number"
                      step="0.0001"
                      class="w-full px-3 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:border-emerald-500 font-mono"
                      placeholder="121.4737"
                    />
                  </div>
                  <div>
                    <label class="block text-[11px] text-slate-500 dark:text-slate-400 mb-1">
                      纬度 (Latitude)
                    </label>
                    <input
                      v-model.number="editForm.lat"
                      type="number"
                      step="0.0001"
                      class="w-full px-3 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:border-emerald-500 font-mono"
                      placeholder="31.2304"
                    />
                  </div>
                </div>
              </div>
            </div>

            <!-- Error message -->
            <div v-if="saveError" class="text-xs text-red-500 px-1">
              {{ saveError }}
            </div>

            <!-- Edit Action Buttons -->
            <div class="pt-2 flex items-center justify-end gap-2.5">
              <button
                @click="cancelEdit"
                class="px-4 py-2 rounded-xl bg-slate-100 dark:bg-slate-800 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700 transition-all"
              >
                取消
              </button>
              <button
                @click="handleSaveNode"
                class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 text-xs font-medium hover:opacity-90 active:scale-95 transition-all shadow-sm"
              >
                <Check v-if="saveSuccess" class="w-3.5 h-3.5 text-emerald-500" />
                <span>{{ saveSuccess ? '已保存' : '保存修改' }}</span>
              </button>
            </div>
          </div>

          <!-- NODE LIST VIEW -->
          <div v-else class="flex-1 overflow-y-auto space-y-4 pr-1">
            <!-- Quick Install Snippet -->
            <div class="p-3.5 rounded-2xl bg-slate-100/70 dark:bg-slate-800/50 border border-slate-200/60 dark:border-slate-700/60 space-y-2.5">
              <!-- Top Row: Title & Accelerator -->
              <div class="flex items-center justify-between flex-wrap gap-2 text-xs font-medium">
                <span class="flex items-center gap-1.5 text-slate-800 dark:text-slate-100">
                  <Terminal class="w-3.5 h-3.5 text-emerald-500" />
                  <span>一键自动部署探针（开机自启与进程保活）</span>
                </span>
                
                <label class="flex items-center gap-1.5 cursor-pointer text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 text-[11px] select-none">
                  <input
                    type="checkbox"
                    v-model="useGhProxy"
                    class="rounded border-slate-300 dark:border-slate-600 text-emerald-500 focus:ring-emerald-400 focus:ring-offset-0"
                  />
                  <span>大陆加速 (gh-proxy.com)</span>
                </label>
              </div>

              <!-- Install Command Code Box -->
              <div class="relative group">
                <div class="font-mono text-[11px] text-slate-700 dark:text-slate-300 overflow-x-auto p-2.5 pr-20 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 break-all select-all leading-relaxed">
                  {{ installCommand }}
                </div>
                <button
                  @click="copyCommand(installCommand)"
                  class="absolute top-2 right-2 flex items-center gap-1 px-2.5 py-1 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-[11px] font-medium transition-all cursor-pointer"
                >
                  <Check v-if="copied" class="w-3 h-3" />
                  <Copy v-else class="w-3 h-3" />
                  <span>{{ copied ? '已复制' : '复制命令' }}</span>
                </button>
              </div>

              <!-- Bottom Row: Install Path & Actions -->
              <div class="flex items-center justify-between flex-wrap gap-2 text-[11px] text-slate-400 pt-0.5">
                <span class="flex items-center gap-1">
                  <span>安装目录:</span>
                  <code class="px-1.5 py-0.5 rounded bg-slate-200/60 dark:bg-slate-800 text-slate-600 dark:text-slate-300 font-mono text-[10px]">
                    /data/netradar/agent (或 /opt/netradar/agent)
                  </code>
                </span>

                <div class="flex items-center gap-3">
                  <button
                    @click="copyCommand(uninstallCommand)"
                    class="hover:text-red-500 dark:hover:text-red-400 transition-colors cursor-pointer"
                    title="复制一键卸载脚本命令"
                  >
                    复制一键卸载命令
                  </button>
                  <button
                    @click="copyCommand(rawBinaryCommand)"
                    class="hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors cursor-pointer"
                    title="直接运行单二进制命令"
                  >
                    复制单二进制命令
                  </button>
                </div>
              </div>
            </div>

            <!-- Node List -->
            <div class="space-y-2.5">
              <div
                v-for="node in (radar.nodes || [])"
                :key="node.id"
                class="p-3.5 rounded-2xl bg-white/60 dark:bg-slate-800/40 border border-slate-200/50 dark:border-slate-700/50 flex items-center justify-between gap-3 hover:bg-white/90 dark:hover:bg-slate-800/90 transition-all"
              >
                <!-- Node details -->
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="font-semibold text-xs text-slate-800 dark:text-slate-100 truncate">
                      {{ node.name }}
                    </span>
                    <!-- Status badge -->
                    <span
                      class="flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium border"
                      :class="node.is_online
                        ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800/40'
                        : 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400 border-slate-200 dark:border-slate-700'"
                    >
                      <CheckCircle2 v-if="node.is_online" class="w-2.5 h-2.5" />
                      <XCircle v-else class="w-2.5 h-2.5" />
                      {{ node.is_online ? '在线' : '离线' }}
                    </span>
                  </div>

                  <div class="flex flex-wrap items-center gap-x-3 gap-y-1 mt-1 text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                    <span>UUID: <strong class="text-slate-700 dark:text-slate-300 font-normal">{{ node.id }}</strong></span>
                    <span v-if="node.ip">IP: {{ node.ip }}</span>
                    <span v-if="node.gateway_lat && node.gateway_lng">坐标: [{{ node.gateway_lng.toFixed(2) }}, {{ node.gateway_lat.toFixed(2) }}]</span>
                    <span v-if="node.os">系统: {{ node.os }}/{{ node.arch }}</span>
                    <span>最后在线: {{ formatDate(node.last_seen) }}</span>
                  </div>
                </div>

                <!-- Actions: Edit & Delete -->
                <div class="flex items-center gap-2 flex-shrink-0">
                  <button
                    @click="startEdit(node)"
                    class="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 transition-all shadow-2xs"
                    title="修改节点名称与定位"
                  >
                    <Edit3 class="w-3.5 h-3.5 text-emerald-500" />
                    <span>编辑</span>
                  </button>
                  <button
                    @click="handleDelete(node.id, node.name)"
                    :disabled="deletingId === node.id"
                    class="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-950/30 hover:bg-red-100 dark:hover:bg-red-900/40 border border-red-200/60 dark:border-red-800/40 transition-all disabled:opacity-50"
                    title="彻底删除此节点"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                    <span class="hidden sm:inline">删除</span>
                  </button>
                </div>
              </div>

              <div
                v-if="(radar.nodes || []).length === 0"
                class="text-center py-10 text-xs text-slate-400"
              >
                暂无注册的探针节点，按照上方命令启动探针即可自动注册入网。
              </div>
            </div>

            <!-- Footer for Node List -->
            <div class="pt-3 border-t border-slate-200/50 dark:border-slate-800/50 flex justify-end">
              <button
                @click="emit('close')"
                class="px-4 py-2 rounded-xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 text-xs font-medium hover:opacity-90 transition-all shadow-xs"
              >
                完成
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@keyframes scaleIn {
  from {
    opacity: 0.96;
    transform: scale(0.96);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.animate-scale-in {
  animation: scaleIn 0.2s ease-out forwards;
}
</style>
