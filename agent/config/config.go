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
	NodeName   string
	Interval   int
	Mock       bool
}

func LoadConfig() *AgentConfig {
	cfg := &AgentConfig{}

	flag.StringVar(&cfg.ConfigFile, "c", getEnv("NETRADAR_CONFIG", "config.yaml"), "配置文件路径")
	flag.StringVar(&cfg.ServerURL, "server", "", "服务端 WebSocket 地址")
	flag.StringVar(&cfg.Token, "token", "", "通信密钥 Token")
	flag.StringVar(&cfg.NodeID, "node-id", "", "节点唯一标识")
	flag.StringVar(&cfg.NodeName, "node-name", "", "节点显示名称")
	flag.IntVar(&cfg.Interval, "interval", 0, "采集上报间隔 (秒)")
	flag.BoolVar(&cfg.Mock, "mock", false, "模拟流量生成测试")

	flag.Parse()

	loadedFromFile := false
	if fileData, err := os.ReadFile(cfg.ConfigFile); err == nil {
		parseSimpleYAML(string(fileData), cfg)
		loadedFromFile = true
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
	if cfg.NodeName == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "Home-Router"
		}
		cfg.NodeName = getEnv("NETRADAR_NODE_NAME", hostname)
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 2
	}

	if !loadedFromFile {
		saveSimpleYAML(cfg.ConfigFile, cfg)
		log.Printf("[探针] 已创建配置文件: %s", cfg.ConfigFile)
	}

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
		case "server":
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
		case "name", "node_name":
			if cfg.NodeName == "" {
				cfg.NodeName = v
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

func saveSimpleYAML(path string, cfg *AgentConfig) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	content := fmt.Sprintf(`# NetRadar Agent 配置文件
server: "%s"
token: "%s"
uuid: "%s"
name: "%s"
interval: %d
`, cfg.ServerURL, cfg.Token, cfg.NodeID, cfg.NodeName, cfg.Interval)

	_ = os.WriteFile(path, []byte(content), 0644)
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}
