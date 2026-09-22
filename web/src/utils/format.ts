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

// RFC 5952 IPv6 规范压缩（彻底去除前导 0，将最长连续为 0 的块压缩为 ::）
export function compressIP(ipStr: string): string {
  if (!ipStr) return ''
  let ip = ipStr.trim()
  if (!ip.includes(':')) {
    return ip
  }

  let prefix = ''
  if (ip.startsWith('Client ')) {
    prefix = 'Client '
    ip = ip.slice(7).trim()
  }

  let portSuffix = ''
  if (ip.startsWith('[') && ip.includes(']')) {
    const endBracket = ip.indexOf(']')
    portSuffix = ip.slice(endBracket + 1)
    ip = ip.slice(1, endBracket)
  } else if (!ip.includes('[') && ip.includes('.')) {
    // 可能是 IPv4 映射地址或混合形式
    return ipStr
  }

  // 分离可能存在的端口号 (例如未带括号的末尾端口如 ::1:8080)
  // 标准 IPv6 最多 8 组（7 个冒号），如果超过则最后一部分可能是端口
  const rawParts = ip.split(':')
  if (rawParts.length < 3) {
    return ipStr
  }

  // 展开现有的 :: 以便寻找最长的全零序列
  let parts: string[] = []
  if (ip.includes('::')) {
    const halves = ip.split('::')
    const left = halves[0] ? halves[0].split(':') : []
    const right = halves[1] ? halves[1].split(':') : []
    const missing = 8 - (left.length + right.length)
    parts = [...left]
    for (let i = 0; i < missing; i++) {
      parts.push('0')
    }
    parts.push(...right)
  } else {
    parts = rawParts
  }

  // 1. 去除每组 16 位 hextet 的前导 0
  const normalized = parts.map((p) => {
    if (!p) return '0'
    const hex = p.replace(/^0+/, '')
    return hex === '' ? '0' : hex.toLowerCase()
  })

  // 2. 找到最长的连续 '0' 区间替换为 '::' (至少连续2个0才压缩)
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
    if (before === '' && after === '') {
      result = '::'
    } else if (before === '') {
      result = `::${after}`
    } else if (after === '') {
      result = `${before}::`
    } else {
      result = `${before}::${after}`
    }
  } else {
    result = normalized.join(':')
  }

  const out = portSuffix ? `[${result}]${portSuffix}` : result
  return prefix ? `${prefix}${out}` : out
}

// 终端名称格式化：如果自定义名称直接返回，否则将未压缩的 IPv6 彻底精简化
export function formatDeviceName(devName?: string, ip?: string): string {
  if (!devName && !ip) return '本机/网关'
  const target = devName || ip || ''
  if (target.includes(':')) {
    return compressIP(target)
  }
  return target
}

