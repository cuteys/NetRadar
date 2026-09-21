import { defineStore } from 'pinia'
import { ref, watchEffect } from 'vue'

export type ThemeMode = 'system' | 'light' | 'dark'

export const useThemeStore = defineStore('theme', () => {
  const saved = (localStorage.getItem('netradar_theme') as ThemeMode) || 'system'
  const mode = ref<ThemeMode>(saved)
  const isDark = ref(false)

  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

  const updateActualTheme = () => {
    if (mode.value === 'system') {
      isDark.value = mediaQuery.matches
    } else {
      isDark.value = mode.value === 'dark'
    }

    if (isDark.value) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }

  mediaQuery.addEventListener('change', () => {
    if (mode.value === 'system') {
      updateActualTheme()
    }
  })

  watchEffect(() => {
    localStorage.setItem('netradar_theme', mode.value)
    updateActualTheme()
  })

  const setMode = (newMode: ThemeMode) => {
    mode.value = newMode
  }

  const toggle = () => {
    if (mode.value === 'light') mode.value = 'dark'
    else if (mode.value === 'dark') mode.value = 'system'
    else mode.value = 'light'
  }

  return {
    mode,
    isDark,
    setMode,
    toggle,
  }
})
