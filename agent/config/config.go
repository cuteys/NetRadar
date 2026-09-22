package config

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func LoadConfig() *AgentConfig {
	cfg := &AgentConfig{}
	var deprecatedNodeName string

	flag.StringVar(&cfg.ConfigFile, "c", getEnv("NETRADAR_CONFIG", "config.yaml"), "")
	flag.StringVar(&cfg.ServerURL, "server", "", "")
	flag.StringVar(&cfg.Token, "token", "", "")
	flag.StringVar(&cfg.NodeID, "node-id", "", "")
	flag.StringVar(&deprecatedNodeName, "node-name", "", "")
	flag.IntVar(&cfg.Interval, "interval", 0, "")
	flag.BoolVar(&cfg.Mock, "mock", false, "")

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
		cfg.NodeID = getEnv("NETRADAR_NODE_ID", generateUUID())
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

	content := fmt.Sprintf(`server: "%s"
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
