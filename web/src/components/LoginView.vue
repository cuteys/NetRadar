<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '../stores/authStore'
import { useThemeStore } from '../stores/themeStore'
import { Radar, Lock, User, ArrowRight, ShieldCheck, Sun, Moon, Monitor } from 'lucide-vue-next'

const auth = useAuthStore()
const theme = useThemeStore()
const username = ref('')
const password = ref('')

const handleLogin = async () => {
  if (!password.value) return
  await auth.login(username.value, password.value)
}
</script>

<template>
  <div class="relative min-h-screen w-full flex items-center justify-center p-4 bg-slate-100/70 dark:bg-[#0b0f19] overflow-hidden transition-colors duration-300">
    <!-- Background Ambience Glow -->
    <div class="absolute -top-40 -left-40 w-96 h-96 rounded-full bg-emerald-500/10 dark:bg-emerald-500/5 blur-3xl pointer-events-none"></div>
    <div class="absolute -bottom-40 -right-40 w-96 h-96 rounded-full bg-sky-500/10 dark:bg-sky-500/5 blur-3xl pointer-events-none"></div>

    <!-- Top Right Theme Switcher -->
    <div class="absolute top-5 right-5 z-10">
      <button
        @click="theme.toggle()"
        class="w-9 h-9 rounded-xl inline-flex items-center justify-center bg-white/80 dark:bg-slate-800/80 hover:bg-white dark:hover:bg-slate-800 text-slate-600 dark:text-slate-300 border border-slate-200/80 dark:border-slate-700/80 shadow-xs transition-all backdrop-blur-md"
        :title="'当前模式: ' + theme.mode"
      >
        <Sun v-if="theme.mode === 'light'" class="w-4 h-4 text-amber-500" />
        <Moon v-else-if="theme.mode === 'dark'" class="w-4 h-4 text-sky-400" />
        <Monitor v-else class="w-4 h-4 text-slate-400" />
      </button>
    </div>

    <!-- Main Apple Glass Card -->
    <div class="relative w-full max-w-sm sm:max-w-md apple-glass-heavy rounded-3xl p-8 sm:p-10 shadow-2xl border border-slate-200/80 dark:border-slate-800/80 transition-all">
      <!-- Icon & Title -->
      <div class="flex flex-col items-center text-center mb-8">
        <div class="relative flex items-center justify-center w-14 h-14 rounded-2xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 shadow-md mb-4">
          <Radar class="w-7 h-7" />
          <span class="absolute -top-1 -right-1 flex h-3 w-3">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
          </span>
        </div>
        <h2 class="text-xl sm:text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
          NetRadar 控制台
        </h2>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-1 flex items-center gap-1">
          <ShieldCheck class="w-3.5 h-3.5 text-emerald-500" />
          授权访问 · 边缘网络态势感知
        </p>
      </div>

      <!-- Error message -->
      <div
        v-if="auth.errorMsg"
        class="mb-5 px-3.5 py-2.5 rounded-2xl bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800/40 text-red-600 dark:text-red-400 text-xs text-center font-medium"
      >
        {{ auth.errorMsg }}
      </div>

      <!-- Form -->
      <form @submit.prevent="handleLogin" class="space-y-4">
        <div>
          <label class="block text-xs font-medium text-slate-600 dark:text-slate-300 mb-1.5 ml-1">
            用户名
          </label>
          <div class="relative flex items-center">
            <User class="absolute left-3.5 w-4 h-4 text-slate-400" />
            <input
              v-model="username"
              type="text"
              required
              class="w-full pl-10 pr-4 py-2.5 rounded-xl bg-slate-100/80 dark:bg-slate-800/80 border border-slate-200/80 dark:border-slate-700/80 text-sm text-slate-800 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-400 dark:focus:ring-slate-600 focus:border-transparent transition-all"
              placeholder="请输入用户名"
            />
          </div>
        </div>

        <div>
          <label class="block text-xs font-medium text-slate-600 dark:text-slate-300 mb-1.5 ml-1">
            密码
          </label>
          <div class="relative flex items-center">
            <Lock class="absolute left-3.5 w-4 h-4 text-slate-400" />
            <input
              v-model="password"
              type="password"
              required
              class="w-full pl-10 pr-4 py-2.5 rounded-xl bg-slate-100/80 dark:bg-slate-800/80 border border-slate-200/80 dark:border-slate-700/80 text-sm text-slate-800 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-400 dark:focus:ring-slate-600 focus:border-transparent transition-all"
              placeholder="请输入密码"
              autofocus
            />
          </div>
        </div>

        <button
          type="submit"
          :disabled="auth.loading"
          class="w-full mt-4 flex items-center justify-center gap-2 py-3 px-4 rounded-xl bg-slate-900 dark:bg-white text-white dark:text-slate-900 font-medium text-sm shadow-sm hover:opacity-90 active:scale-[0.98] transition-all disabled:opacity-60"
        >
          <span v-if="auth.loading" class="animate-spin inline-block w-4 h-4 border-2 border-current border-t-transparent rounded-full"></span>
          <span v-else class="flex items-center gap-1.5">
            进入网络态势大屏
            <ArrowRight class="w-4 h-4" />
          </span>
        </button>
      </form>
    </div>
  </div>
</template>
