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
