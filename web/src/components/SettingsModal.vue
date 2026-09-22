<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRadarStore } from '../stores/radarStore'
import { clearAppCache } from '../utils/cache'
import {
  X,
  Shield,
  KeyRound,
  User,
  Dices,
  Check,
  AlertCircle,
  Network,
  Lock,
  RotateCcw
} from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const radar = useRadarStore()

const form = ref({
  username: 'admin',
  current_password: '',
  new_password: '',
  confirm_password: '',
  agent_token: '',
  agent_server_addr: '',
  use_tls: false,
})

const isSaving = ref(false)
const saveSuccess = ref(false)
const errorMessage = ref('')
const isClearingCache = ref(false)

const handleClearCache = async () => {
  if (isClearingCache.value) return
  isClearingCache.value = true
  try {
    await clearAppCache({ reload: true, preserveAuth: true })
  } catch (e) {
    console.error('清理缓存失败:', e)
    isClearingCache.value = false
  }
}

const hasNewVersion = computed(() => {
  const current = (radar.systemSettings?.version || '').replace(/^v/, '')
  const latest = (radar.systemSettings?.latest_version || '').replace(/^v/, '')
  if (!current || !latest || current === latest) return false
  return latest > current
})

const syncForm = () => {
  if (radar.systemSettings) {
    form.value.username = radar.systemSettings.username || 'admin'
    form.value.agent_token = radar.systemSettings.agent_token || ''
    form.value.agent_server_addr = radar.systemSettings.agent_server_addr || (typeof window !== 'undefined' ? window.location.host : '127.0.0.1:8899')
    form.value.use_tls = !!radar.systemSettings.use_tls
  }
  form.value.current_password = ''
  form.value.new_password = ''
  form.value.confirm_password = ''
  errorMessage.value = ''
  saveSuccess.value = false
}

watch(() => props.open, async (val) => {
  if (val) {
    await radar.fetchSystemSettings()
    syncForm()
  }
})

const generateRandomToken = () => {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
  const array = new Uint8Array(24)
  window.crypto.getRandomValues(array)
  let token = 'nr_'
  for (let i = 0; i < 24; i++) {
    token += chars[array[i] % chars.length]
  }
  form.value.agent_token = token
}

const handleSave = async () => {
  errorMessage.value = ''
  
  if (form.value.new_password) {
    if (!form.value.current_password) {
      errorMessage.value = '修改密码时必须输入当前原密码'
      return
    }
    if (form.value.new_password !== form.value.confirm_password) {
      errorMessage.value = '两次输入的新密码不一致'
      return
    }
    if (form.value.new_password.length < 6) {
      errorMessage.value = '新密码长度至少需要 6 个字符'
      return
    }
  }

  isSaving.value = true
  const res = await radar.updateSystemSettings({
    username: form.value.username.trim(),
    current_password: form.value.current_password,
    new_password: form.value.new_password,
    agent_token: form.value.agent_token.trim(),
    agent_server_addr: form.value.agent_server_addr.trim(),
    use_tls: form.value.use_tls,
  })
  isSaving.value = false

  if (res.success) {
    saveSuccess.value = true
    setTimeout(() => {
      saveSuccess.value = false
      emit('close')
    }, 800)
  } else {
    errorMessage.value = res.error || '保存失败'
  }
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.open) {
    emit('close')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
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
          class="w-full max-w-lg apple-glass-heavy rounded-3xl p-6 sm:p-7 shadow-2xl border border-slate-200/80 dark:border-slate-800/80 text-slate-800 dark:text-slate-100 relative max-h-[90vh] flex flex-col animate-scale-in"
        >
          <!-- Close button -->
          <button
            @click="emit('close')"
            class="absolute top-5 right-5 p-2 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-all"
          >
            <X class="w-4 h-4" />
          </button>

          <!-- Header with Version Info -->
          <div class="flex items-center justify-between mb-5 pr-8">
            <div class="flex items-center gap-3">
              <div class="p-2.5 rounded-2xl bg-slate-100 dark:bg-slate-800 text-slate-800 dark:text-slate-200 border border-slate-200/60 dark:border-slate-700/60">
                <Shield class="w-5 h-5 text-emerald-500" />
              </div>
              <div>
                <h3 class="font-bold text-base text-slate-900 dark:text-white leading-tight">系统与安全配置</h3>
                <div class="flex flex-wrap items-center gap-1.5 mt-1 text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                  <span>当前版本: <strong class="text-slate-700 dark:text-slate-300 font-semibold">{{ radar.systemSettings?.version || '---' }}</strong></span>
                  <span v-if="radar.systemSettings?.latest_version" class="text-slate-300 dark:text-slate-600">·</span>
                  <span v-if="radar.systemSettings?.latest_version">
                    最新版本: <strong class="font-semibold" :class="hasNewVersion ? 'text-amber-600 dark:text-amber-400' : 'text-emerald-600 dark:text-emerald-400'">{{ radar.systemSettings.latest_version }}</strong>
                  </span>
                  <span v-if="hasNewVersion" class="px-1.5 py-0.5 rounded-md bg-amber-500/10 text-amber-600 dark:text-amber-400 text-[10px] font-sans font-medium">
                    有新版本
                  </span>
                  <span v-else-if="radar.systemSettings?.latest_version" class="px-1.5 py-0.5 rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-[10px] font-sans font-medium">
                    最新
                  </span>

                  <!-- 清理缓存按钮 (适配 Safari 深度清理与强制刷新) -->
                  <span class="text-slate-300 dark:text-slate-600">·</span>
                  <button
                    type="button"
                    @click="handleClearCache"
                    :disabled="isClearingCache"
                    class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-sans font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 bg-slate-200/60 dark:bg-slate-700/60 hover:bg-slate-200 dark:hover:bg-slate-700 transition-all cursor-pointer disabled:opacity-50"
                    title="清理浏览器本地缓存与存储，并强制重新加载（已针对 Safari 深度适配）"
                  >
                    <RotateCcw class="w-2.5 h-2.5" :class="isClearingCache ? 'animate-spin' : ''" />
                    <span>{{ isClearingCache ? '正在清理...' : '清理缓存' }}</span>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Error Alert Banner -->
          <div
            v-if="errorMessage"
            class="mb-4 p-3 rounded-2xl bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800/40 text-red-600 dark:text-red-400 text-xs flex items-center gap-2"
          >
            <AlertCircle class="w-4 h-4 flex-shrink-0" />
            <span>{{ errorMessage }}</span>
          </div>

          <!-- Settings Form -->
          <div class="flex-1 overflow-y-auto space-y-4 pr-1">
            <!-- Admin Credentials Section -->
            <div class="p-4 rounded-2xl bg-slate-100/70 dark:bg-slate-800/50 border border-slate-200/60 dark:border-slate-700/60 space-y-3">
              <div class="flex items-center gap-2 text-xs font-semibold text-slate-800 dark:text-slate-200">
                <User class="w-3.5 h-3.5 text-emerald-500" />
                <span>管理员账号与密码</span>
              </div>

              <div>
                <label class="block text-[11px] text-slate-500 dark:text-slate-400 mb-1">
                  用户名
                </label>
                <input
                  v-model="form.username"
                  type="text"
                  class="w-full px-3.5 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:border-emerald-500 font-medium"
                  placeholder="admin"
                />
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5 pt-1">
                <div>
                  <label class="block text-[11px] text-slate-500 dark:text-slate-400 mb-1">
                    当前原密码
                  </label>
                  <input
                    v-model="form.current_password"
                    type="password"
                    class="w-full px-3 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:border-emerald-500"
                    placeholder="改密需提供"
                  />
                </div>
                <div>
                  <label class="block text-[11px] text-slate-500 dark:text-slate-400 mb-1">
                    新密码 (留空不改)
                  </label>
                  <input
                    v-model="form.new_password"
                    type="password"
                    class="w-full px-3 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:border-emerald-500"
                    placeholder="不少于6位"
                  />
                </div>
                <div>
                  <label class="block text-[11px] text-slate-500 dark:text-slate-400 mb-1">
                    确认新密码
                  </label>
                  <input
                    v-model="form.confirm_password"
                    type="password"
                    class="w-full px-3 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 focus:outline-none focus:border-emerald-500"
                    placeholder="重复新密码"
                  />
                </div>
              </div>
            </div>

            <!-- Agent Server Connection Address & TLS Section -->
            <div class="p-4 rounded-2xl bg-slate-100/70 dark:bg-slate-800/50 border border-slate-200/60 dark:border-slate-700/60 space-y-3">
              <div class="flex items-center gap-2 text-xs font-semibold text-slate-800 dark:text-slate-200">
                <Network class="w-3.5 h-3.5 text-emerald-500" />
                <span>Agent 对接地址</span>
              </div>

              <div>
                <label class="block text-[11px] text-slate-500 dark:text-slate-400 mb-1">
                  面板公网域名或 IP 及端口 (供探针上报)
                </label>
                <input
                  v-model="form.agent_server_addr"
                  type="text"
                  class="w-full px-3.5 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 font-mono focus:outline-none focus:border-emerald-500"
                  placeholder="例如: radar.example.com:8899 或 114.240.1.2:8899"
                />
              </div>

              <!-- Use TLS Toggle -->
              <div class="flex items-center justify-between pt-1">
                <div>
                  <div class="text-xs font-medium text-slate-800 dark:text-slate-200 flex items-center gap-1.5">
                    <Lock class="w-3.5 h-3.5 text-sky-500" />
                    <span>使用 TLS 加密连接 (wss://)</span>
                  </div>
                  <div class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                    开启后启动命令将采用安全 WebSocket 协议（需经由 Nginx/Caddy 等配置 SSL 证书）
                  </div>
                </div>
                <label class="relative inline-flex items-center cursor-pointer flex-shrink-0 ml-3">
                  <input type="checkbox" v-model="form.use_tls" class="sr-only peer" />
                  <div class="w-10 h-5 bg-slate-300 dark:bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-emerald-500"></div>
                </label>
              </div>
            </div>

            <!-- Agent Token Section -->
            <div class="p-4 rounded-2xl bg-slate-100/70 dark:bg-slate-800/50 border border-slate-200/60 dark:border-slate-700/60 space-y-2.5">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2 text-xs font-semibold text-slate-800 dark:text-slate-200">
                  <KeyRound class="w-3.5 h-3.5 text-emerald-500" />
                  <span>Agent 探针接入 Token</span>
                </div>
                <button
                  type="button"
                  @click="generateRandomToken"
                  class="flex items-center gap-1 px-2.5 py-1 rounded-lg text-[11px] font-medium bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800/40 hover:bg-emerald-100 dark:hover:bg-emerald-900/50 active:scale-95 transition-all shadow-2xs"
                  title="生成高强度随机 Token"
                >
                  <Dices class="w-3.5 h-3.5" />
                  <span>随机生成</span>
                </button>
              </div>

              <div>
                <input
                  v-model="form.agent_token"
                  type="text"
                  class="w-full px-3.5 py-2 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-xs text-slate-800 dark:text-slate-100 font-mono focus:outline-none focus:border-emerald-500"
                  placeholder="nr_..."
                />
              </div>
              <p class="text-[11px] text-slate-500 dark:text-slate-400 leading-normal">
                探针连接时使用此 Token 进行身份校验，将同步自动填充至新探针启动命令。
              </p>
            </div>
          </div>

          <!-- Action Buttons -->
          <div class="mt-5 pt-3 border-t border-slate-200/50 dark:border-slate-800/50 flex items-center justify-end gap-2.5">
            <button
              @click="emit('close')"
              class="px-4 py-2 rounded-xl bg-slate-100 dark:bg-slate-800 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700 transition-all"
            >
              取消
            </button>
            <button
              @click="handleSave"
              :disabled="isSaving"
              class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 text-xs font-medium hover:opacity-90 active:scale-95 transition-all shadow-sm disabled:opacity-50"
            >
              <Check v-if="saveSuccess" class="w-3.5 h-3.5 text-emerald-500" />
              <span>{{ saveSuccess ? '已生效' : (isSaving ? '保存中...' : '保存配置') }}</span>
            </button>
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
    opacity: 0.95;
    transform: scale(0.95);
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
