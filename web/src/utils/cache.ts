/**
 * NetRadar 浏览器缓存管理与 Safari 适配工具
 */

export interface ClearCacheOptions {
  reload?: boolean
  preserveAuth?: boolean
}

/**
 * 彻底清理前端缓存并强制硬刷新（针对 Safari / WebKit 特性专项优化）
 */
export async function clearAppCache(options: ClearCacheOptions = {}) {
  const { reload = true, preserveAuth = true } = options

  // 1. 保留身份鉴权凭证（避免清理后需重复登录）
  let token: string | null = null
  let user: string | null = null
  let appVersion: string | null = null

  try {
    if (preserveAuth) {
      token = localStorage.getItem('netradar_token')
      user = localStorage.getItem('netradar_user')
    }
    appVersion = localStorage.getItem('netradar_app_version')
  } catch (e) {
    console.error('[Cache] 读取 localStorage 异常:', e)
  }

  // 2. 清理 localStorage
  try {
    localStorage.clear()
    if (token) localStorage.setItem('netradar_token', token)
    if (user) localStorage.setItem('netradar_user', user)
    if (appVersion) localStorage.setItem('netradar_app_version', appVersion)
  } catch (e) {
    console.error('[Cache] 清理 localStorage 异常:', e)
  }

  // 3. 清理 sessionStorage
  try {
    sessionStorage.clear()
  } catch (e) {
    console.error('[Cache] 清理 sessionStorage 异常:', e)
  }

  // 4. 清理 CacheStorage API (Safari 11.1+ / PWA 强缓存)
  if (typeof window !== 'undefined' && 'caches' in window) {
    try {
      const keys = await window.caches.keys()
      await Promise.all(keys.map((k) => window.caches.delete(k)))
    } catch (e) {
      console.error('[Cache] 清理 CacheStorage 异常:', e)
    }
  }

  // 5. 注销可能存在的 Service Worker
  if (typeof window !== 'undefined' && 'serviceWorker' in navigator) {
    try {
      const registrations = await navigator.serviceWorker.getRegistrations()
      await Promise.all(registrations.map((r) => r.unregister()))
    } catch (e) {
      console.error('[Cache] 注销 ServiceWorker 异常:', e)
    }
  }

  // 6. 适配 Safari / WebKit 强制硬刷新（绕过 Safari 激进的 Disk Cache 与 BFCache）
  if (reload && typeof window !== 'undefined') {
    try {
      const url = new URL(window.location.href)
      // 添加随机防缓存时间戳参数，强制 WebKit 引擎发起真实的服务器网络请求
      url.searchParams.set('_t', Date.now().toString())
      window.location.replace(url.toString())
    } catch {
      window.location.reload()
    }
  }
}

/**
 * 版本检查与自动强制缓存清理
 * 当检测到浏览器记录的版本与服务端最新版本不一致时，自动执行深度清理并刷新
 */
export async function checkVersionAndClearCache(backendVersion?: string): Promise<boolean> {
  if (typeof window === 'undefined') return false

  let serverVer = backendVersion

  // 如果未直接传入后端版本号，主动请求非鉴权的 /api/version
  if (!serverVer) {
    try {
      const res = await fetch('/api/version', {
        cache: 'no-store',
        headers: {
          'Cache-Control': 'no-cache, no-store, must-revalidate',
          Pragma: 'no-cache',
        },
      })
      if (res.ok) {
        const data = await res.json()
        serverVer = data.version
      }
    } catch (e) {
      console.warn('[Cache] 获取服务端版本号失败:', e)
      return false
    }
  }

  if (!serverVer) return false

  try {
    const storedVer = localStorage.getItem('netradar_app_version')
    if (!storedVer) {
      // 初次记录当前版本
      localStorage.setItem('netradar_app_version', serverVer)
      return false
    }

    if (storedVer !== serverVer) {
      console.warn(`[NetRadar] 检测到系统版本更新 (${storedVer} -> ${serverVer})，正在执行强制清理缓存并刷新...`)
      localStorage.setItem('netradar_app_version', serverVer)
      await clearAppCache({ reload: true, preserveAuth: true })
      return true
    }
  } catch (e) {
    console.error('[Cache] 对比版本号异常:', e)
  }

  return false
}
