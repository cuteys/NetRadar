export function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}

export function formatSpeed(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec <= 0) return '0 B/s'
  return formatBytes(bytesPerSec) + '/s'
}

export function getDeviceIcon(category: string): string {
  switch (category?.toLowerCase()) {
    case 'mobile':
      return 'Smartphone'
    case 'pc':
      return 'Laptop'
    case 'tv':
      return 'Tv'
    case 'nas':
      return 'HardDrive'
    case 'gateway':
      return 'Router'
    default:
      return 'Cpu'
  }
}

export function getProtocolColor(proto: string): string {
  switch (proto?.toUpperCase()) {
    case 'TCP':
      return '#a78bfa'
    case 'UDP':
      return '#60a5fa'
    case 'ICMP':
      return '#fca5a5'
    default:
      return '#4ade80'
  }
}

// RFC 5952 IPv6 精简压缩（去除前导 0，将连续为 0 的块压缩为 ::）
export function compressIP(ipStr: string): string {
  if (!ipStr || !ipStr.includes(':')) {
    return ipStr || ''
  }

  let ip = ipStr.trim()
  let portSuffix = ''
  if (ip.startsWith('[') && ip.includes(']')) {
    const endBracket = ip.indexOf(']')
    portSuffix = ip.slice(endBracket + 1)
    ip = ip.slice(1, endBracket)
  }

  const parts = ip.split(':')
  if (parts.length < 3) {
    return ipStr
  }

  // 1. 去除每组 16 位 hextet 的前导 0
  const normalized = parts.map((p) => {
    if (p === '') return ''
    const hex = p.replace(/^0+/, '')
    return hex === '' ? '0' : hex.toLowerCase()
  })

  // 如果原本已包含 ::，只处理各块的前导 0
  if (ip.includes('::')) {
    const joined = normalized.join(':').replace(/:{3,}/g, '::')
    return portSuffix ? `[${joined}]${portSuffix}` : joined
  }

  // 2. 找到最长的连续 '0' 区间替换为 '::'
  let bestStart = -1
  let bestLen = 0
  let curStart = -1
  let curLen = 0

  for (let i = 0; i < normalized.length; i++) {
    if (normalized[i] === '0') {
      if (curStart === -1) {
        curStart = i
        curLen = 1
      } else {
        curLen++
      }
      if (curLen > bestLen) {
        bestStart = curStart
        bestLen = curLen
      }
    } else {
      curStart = -1
      curLen = 0
    }
  }

  let result = ''
  if (bestLen >= 2) {
    const before = normalized.slice(0, bestStart).join(':')
    const after = normalized.slice(bestStart + bestLen).join(':')
    result = `${before}::${after}`
    if (result.startsWith(':') && !result.startsWith('::')) {
      result = ':' + result
    }
  } else {
    result = normalized.join(':')
  }

  return portSuffix ? `[${result}]${portSuffix}` : result
}

