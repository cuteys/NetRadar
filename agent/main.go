package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"netradar/agent/client"
	"netradar/agent/collector"
	"netradar/agent/config"
	"netradar/agent/updater"
)

var Version = "v0.1.6"

func main() {
	cfg := config.LoadConfig()
	cfg.Version = Version

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
	mockGen := collector.NewMockCollector()

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

			payload, _ := tracker.ProcessConntrack(cfg.NodeID, entries)
			payload.Hostname, _ = os.Hostname()
			payload.OS = runtime.GOOS
			payload.Arch = runtime.GOARCH
			payload.Version = Version

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
