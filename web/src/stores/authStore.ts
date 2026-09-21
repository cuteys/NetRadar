import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('netradar_token') || '')
  const username = ref<string>(localStorage.getItem('netradar_user') || '')
  const isAuthenticated = ref<boolean>(!!token.value)
  const loading = ref<boolean>(false)
  const errorMsg = ref<string>('')

  const checkAuth = async () => {
    if (!token.value) {
      isAuthenticated.value = false
      return false
    }

    try {
      const res = await fetch('/api/auth/me', {
        headers: {
          Authorization: `Bearer ${token.value}`,
        },
      })
      if (res.ok) {
        const data = await res.json()
        isAuthenticated.value = true
        username.value = data.username || 'admin'
        return true
      } else {
        logout()
        return false
      }
    } catch {
      return isAuthenticated.value
    }
  }

  const login = async (user: string, pass: string) => {
    loading.value = true
    errorMsg.value = ''

    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          username: user,
          password: pass,
        }),
      })

      const data = await res.json()
      if (!res.ok) {
        errorMsg.value = data.error || '登录失败，请检查账号密码'
        return false
      }

      token.value = data.token
      username.value = data.username
      isAuthenticated.value = true
      localStorage.setItem('netradar_token', data.token)
      localStorage.setItem('netradar_user', data.username)
      return true
    } catch (err: any) {
      errorMsg.value = '无法连接到控制台服务: ' + (err.message || '网络异常')
      return false
    } finally {
      loading.value = false
    }
  }

  const logout = async () => {
    try {
      await fetch('/api/auth/logout', { method: 'POST' })
    } catch {}

    token.value = ''
    username.value = ''
    isAuthenticated.value = false
    localStorage.removeItem('netradar_token')
    localStorage.removeItem('netradar_user')
  }

  return {
    token,
    username,
    isAuthenticated,
    loading,
    errorMsg,
    checkAuth,
    login,
    logout,
  }
})
