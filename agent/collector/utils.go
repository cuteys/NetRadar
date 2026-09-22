package collector

import (
	"net"
	"strings"
)

// IsPrivateIP 判断是否为局域网/私有/保留 IP 地址
// 采用轻量无分配解析，避免高频调用 net.ParseIP 引起 GC 停顿
func IsPrivateIP(ipStr string) bool {
	if ipStr == "" {
		return false
	}

	// IPv4 快速段位比对（零内存分配）
	if dotIdx := strings.IndexByte(ipStr, '.'); dotIdx > 0 {
		var b0, b1 int
		for i := 0; i < dotIdx; i++ {
			c := ipStr[i]
			if c < '0' || c > '9' {
				return false
			}
			b0 = b0*10 + int(c-'0')
		}

		rem := ipStr[dotIdx+1:]
		nextDot := strings.IndexByte(rem, '.')
		if nextDot > 0 {
			for i := 0; i < nextDot; i++ {
				c := rem[i]
				if c < '0' || c > '9' {
					break
				}
				b1 = b1*10 + int(c-'0')
			}
		}

		switch b0 {
		case 10, 127: // 10.0.0.0/8, 127.0.0.0/8
			return true
		case 192: // 192.168.0.0/16
			return b1 == 168
		case 172: // 172.16.0.0/12
			return b1 >= 16 && b1 <= 31
		case 169: // 169.254.0.0/16
			return b1 == 254
		case 100: // 100.64.0.0/10 (CGNAT)
			return b1 >= 64 && b1 <= 127
		case 0:
			return true
		default:
			return false
		}
	}

	// IPv6 私有/本地网段判断
	if strings.Contains(ipStr, ":") {
		lower := strings.ToLower(ipStr)
		if lower == "::1" || strings.HasPrefix(lower, "fe80:") || strings.HasPrefix(lower, "fc") || strings.HasPrefix(lower, "fd") {
			return true
		}
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return false
		}
		return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate()
	}

	return false
}
