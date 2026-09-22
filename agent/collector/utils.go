package collector

import (
	"net"
	"strconv"
	"strings"
)

var privateCIDRs []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"100.64.0.0/10",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateCIDRs = append(privateCIDRs, block)
		}
	}
}

func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, block := range privateCIDRs {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func GuessDeviceCategory(ipStr string) string {
	parts := strings.Split(ipStr, ".")
	if len(parts) == 4 {
		octetNum, err := strconv.Atoi(parts[3])
		if err != nil {
			return "device"
		}
		switch {
		case octetNum == 1:
			return "gateway"
		case octetNum >= 2 && octetNum <= 50:
			return "nas"
		case octetNum >= 51 && octetNum <= 150:
			return "pc"
		case octetNum >= 151 && octetNum <= 200:
			return "mobile"
		default:
			return "iot"
		}
	}
	return "device"
}

func GuessDeviceName(ipStr string) string {
	parts := strings.Split(ipStr, ".")
	if len(parts) == 4 {
		lastOctet := parts[3]
		octetNum, err := strconv.Atoi(lastOctet)
		if err != nil {
			return "Client " + ipStr
		}
		switch {
		case octetNum == 1:
			return "Router Gateway"
		case octetNum >= 2 && octetNum <= 10:
			return "Home NAS / Server (" + lastOctet + ")"
		case octetNum >= 51 && octetNum <= 70:
			return "MacBook Pro (" + lastOctet + ")"
		case octetNum >= 71 && octetNum <= 100:
			return "Gaming PC (" + lastOctet + ")"
		case octetNum >= 151 && octetNum <= 170:
			return "iPhone (" + lastOctet + ")"
		case octetNum >= 171 && octetNum <= 190:
			return "iPad / Tablet (" + lastOctet + ")"
		case octetNum >= 191 && octetNum <= 210:
			return "Smart TV (" + lastOctet + ")"
		default:
			return "Device ." + lastOctet
		}
	}
	return "Client " + ipStr
}
