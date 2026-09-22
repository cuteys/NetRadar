package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"netradar/agent/client"
	"netradar/agent/collector"
	"netradar/agent/config"
	"netradar/agent/updater"
	"netradar/pkg/version"
)

var Version = version.Version

var (
	lastCPUTotal uint64
	lastCPUIdle  uint64
)

func main() {
	cfg := config.LoadConfig()
	cfg.Version = Version

	if strings.TrimSpace(cfg.Token) == "" {
		log.Fatalf("[探针错误] 必须配置通信 Token！请通过 -token 命令行参数、NETRADAR_TOKEN 环境变量或在配置文件 %s 中指定有效 Token。", cfg.ConfigFile)
	}

	log.Printf("=== NetRadar 探针 (%s) ===", Version)
	log.Printf("节点 ID:   %s", cfg.NodeID)
	log.Printf("系统架构:   %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("目标服务:   %s", cfg.ServerURL)
	log.Printf("上报周期:   %d 秒", cfg.Interval)

	updater.StartAutoUpdater(Version)

	conntrackPath, hasConntrack := collector.CheckConntrackAvailable()
	useMock := cfg.Mock || !hasConntrack

	if useMock {
		if !hasConntrack {
			log.Printf("[探针] 未检测到 nf_conntrack，启用模拟模式运行")
		} else {
			log.Printf("[探针] 已指定 Mock 模式运行")
		}
	} else {
		log.Printf("[探针] 捕获内核连接跟踪表: %s", conntrackPath)
	}

	wsClient := client.NewAgentWSClient(cfg)
	wsClient.Start()
	defer wsClient.Close()

	tracker := collector.NewDeltaTracker()
	var mockGen *collector.MockCollector
	if useMock {
		mockGen = collector.NewMockCollector()
	}

	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			log.Println("[探针] 正在退出...")
			return
		case <-ticker.C:
			var entries []*collector.RawConntrackEntry
			var err error

			if useMock {
				entries = mockGen.GenerateNextTick()
			} else {
				entries, err = collector.ReadConntrackEntries(conntrackPath)
				if err != nil {
					log.Printf("[探针] 读取连接跟踪表失败: %v", err)
					continue
				}
			}

			payload := tracker.ProcessConntrack(cfg.NodeID, entries)
			payload.Hostname, _ = os.Hostname()
			payload.OS = runtime.GOOS
			payload.Arch = runtime.GOARCH
			payload.Version = Version

			cpuUsage, memUsage, uptime := readSystemMetrics()
			payload.CPUUsage = cpuUsage
			payload.MemUsage = memUsage
			payload.Uptime = uptime

			if geo := wsClient.GetGeoInfo(); geo != nil {
				payload.PublicIP = geo.IP
				payload.GatewayLat = geo.Latitude
				payload.GatewayLng = geo.Longitude
			}

			if err := wsClient.SendPayload(payload); err != nil {
				log.Printf("[探针] 上报流量指标失败: %v", err)
			}
		}
	}
}

// readSystemMetrics 轻量级采集宿主机的 CPU 占用、内存占用与运行时间 (<0.1ms)
func readSystemMetrics() (cpuUsage float64, memUsage float64, uptime int64) {
	if runtime.GOOS != "linux" {
		return 0, 0, 0
	}

	// 1. 读取 Uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		var up float64
		if _, err := fmt.Sscanf(string(data), "%f", &up); err == nil {
			uptime = int64(up)
		}
	}

	// 2. 读取内存利用率
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		var total, free, available, buffers, cached uint64
		lines := strings.Split(string(data), "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "MemTotal:") {
				fmt.Sscanf(l, "MemTotal: %d", &total)
			} else if strings.HasPrefix(l, "MemFree:") {
				fmt.Sscanf(l, "MemFree: %d", &free)
			} else if strings.HasPrefix(l, "MemAvailable:") {
				fmt.Sscanf(l, "MemAvailable: %d", &available)
			} else if strings.HasPrefix(l, "Buffers:") {
				fmt.Sscanf(l, "Buffers: %d", &buffers)
			} else if strings.HasPrefix(l, "Cached:") {
				fmt.Sscanf(l, "Cached: %d", &cached)
			}
		}
		if total > 0 {
			if available > 0 {
				memUsage = float64(total-available) / float64(total) * 100
			} else {
				actualFree := free + buffers + cached
				if actualFree < total {
					memUsage = float64(total-actualFree) / float64(total) * 100
				}
			}
		}
	}

	// 3. 读取 CPU 使用率
	if data, err := os.ReadFile("/proc/stat"); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "cpu ") {
			var u, n, s, idle, iowait, irq, softirq, steal uint64
			fmt.Sscanf(lines[0], "cpu %d %d %d %d %d %d %d %d", &u, &n, &s, &idle, &iowait, &irq, &softirq, &steal)
			tot := u + n + s + idle + iowait + irq + softirq + steal
			id := idle + iowait
			if lastCPUTotal > 0 && tot > lastCPUTotal {
				dTot := tot - lastCPUTotal
				dId := id - lastCPUIdle
				if dTot > 0 && dTot >= dId {
					cpuUsage = float64(dTot-dId) / float64(dTot) * 100
				}
			}
			lastCPUTotal = tot
			lastCPUIdle = id
		}
	}

	return cpuUsage, memUsage, uptime
}
