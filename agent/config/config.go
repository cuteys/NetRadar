package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type AgentConfig struct {
	ConfigFile string
	ServerURL  string
	Token      string
	NodeID     string
	Version    string
	Interval   int
	Mock       bool
}

func LoadConfig() *AgentConfig {
	cfg := &AgentConfig{}
	var deprecatedNodeName string

	flag.StringVar(&cfg.ConfigFile, "c", getEnv("NETRADAR_CONFIG", "config.yaml"), "配置文件路径")
	flag.StringVar(&cfg.ServerURL, "server", "", "服务端 WebSocket 地址")
	flag.StringVar(&cfg.Token, "token", "", "通信密钥 Token")
	flag.StringVar(&cfg.NodeID, "node-id", "", "节点唯一标识")
	flag.StringVar(&deprecatedNodeName, "node-name", "", "已废弃，节点名称由服务端统一管理")
	flag.IntVar(&cfg.Interval, "interval", 0, "采集上报间隔 (秒)")
	flag.BoolVar(&cfg.Mock, "mock", false, "模拟流量生成测试")

	flag.Parse()

	if fileData, err := os.ReadFile(cfg.ConfigFile); err == nil {
		parseSimpleYAML(string(fileData), cfg)
	}

	if cfg.ServerURL == "" {
		cfg.ServerURL = getEnv("NETRADAR_SERVER", "ws://127.0.0.1:8899/ws/agent")
	}
	if cfg.Token == "" {
		cfg.Token = getEnv("NETRADAR_TOKEN", "netradar_secret_token_12345")
	}
	if cfg.NodeID == "" {
		cfg.NodeID = getEnv("NETRADAR_NODE_ID", uuid.New().String())
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 2
	}

	saveStandardYAML(cfg.ConfigFile, cfg)

	return cfg
}

func parseSimpleYAML(content string, cfg *AgentConfig) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch k {
		case "server", "server_url":
			if cfg.ServerURL == "" {
				cfg.ServerURL = v
			}
		case "token":
			if cfg.Token == "" {
				cfg.Token = v
			}
		case "uuid", "node_id":
			if cfg.NodeID == "" {
				cfg.NodeID = v
			}
		case "interval":
			if cfg.Interval == 0 {
				if iv, err := strconv.Atoi(v); err == nil {
					cfg.Interval = iv
				}
			}
		}
	}
}

func saveStandardYAML(path string, cfg *AgentConfig) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	content := fmt.Sprintf(`# NetRadar Agent 配置文件
server: "%s"
token: "%s"
uuid: "%s"
interval: %d
`, cfg.ServerURL, cfg.Token, cfg.NodeID, cfg.Interval)

	if err := os.WriteFile(path, []byte(content), 0644); err == nil {
		log.Printf("[探针] 配置文件已格式化更新: %s", path)
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}
