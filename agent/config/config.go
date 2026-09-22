package config

import (
	"crypto/rand"
	"crypto/sha256"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"netradar/pkg/utils"
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

// getStableHardwareUUID 基于设备机器码与网卡 MAC 派生确定性 UUID
func getStableHardwareUUID() string {
	var hardwareKey string

	// 1. 读取 machine-id
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id", "/sys/class/dmi/id/product_uuid"} {
		if data, err := os.ReadFile(p); err == nil {
			id := strings.TrimSpace(string(data))
			if len(id) >= 16 && !strings.Contains(id, "00000000") {
				hardwareKey = "mid:" + id
				break
			}
		}
	}

	// 2. 若未获取到 machine-id，读取物理网卡 MAC
	if hardwareKey == "" {
		if ifaces, err := net.Interfaces(); err == nil {
			// 优先匹配路由常见 LAN/WAN 接口
			preferred := []string{"br-lan", "eth0", "eth1", "lan", "wan"}
			for _, name := range preferred {
				for _, iface := range ifaces {
					if strings.EqualFold(iface.Name, name) && len(iface.HardwareAddr) == 6 {
						hardwareKey = "mac:" + iface.HardwareAddr.String()
						break
					}
				}
				if hardwareKey != "" {
					break
				}
			}
			// 兜底取首个有效物理 MAC
			if hardwareKey == "" {
				for _, iface := range ifaces {
					if iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) == 6 {
						hardwareKey = "mac:" + iface.HardwareAddr.String()
						break
					}
				}
			}
		}
	}

	// 3. 读取 CPU/主板硬件特征
	if hardwareKey == "" {
		if cpuData, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(cpuData), "\n") {
				if strings.HasPrefix(strings.ToLower(line), "serial") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						s := strings.TrimSpace(parts[1])
						if len(s) > 4 && s != "0000000000000000" {
							hardwareKey = "cpu:" + s
							break
						}
					}
				}
			}
		}
	}

	if hardwareKey != "" {
		h := sha256.Sum256([]byte("netradar-node-v1:" + hardwareKey))
		h[6] = (h[6] & 0x0f) | 0x40 // Version 4
		h[8] = (h[8] & 0x3f) | 0x80 // Variant RFC 4122
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
	}

	return generateUUID()
}

func resolveDefaultConfigPath() string {
	if env := os.Getenv("NETRADAR_CONFIG"); env != "" {
		return env
	}

	var execDir string
	if execPath, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
			execDir = filepath.Dir(resolved)
		} else {
			execDir = filepath.Dir(execPath)
		}
	}

	candidates := []string{}
	if execDir != "" {
		candidates = append(candidates, filepath.Join(execDir, "config.yaml"))
	}
	candidates = append(candidates,
		"/data/netradar/agent/config.yaml",
		"/opt/netradar/agent/config.yaml",
		"/etc/netradar/agent/config.yaml",
		"./config.yaml",
	)

	// 若已有文件存在，优先采用已存在的文件
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	// 否则优先选择可执行程序所在目录以持久化
	if execDir != "" {
		return filepath.Join(execDir, "config.yaml")
	}

	return "config.yaml"
}

func LoadConfig() *AgentConfig {
	cfg := &AgentConfig{}

	defaultConfigPath := resolveDefaultConfigPath()
	flag.StringVar(&cfg.ConfigFile, "c", defaultConfigPath, "")
	flag.StringVar(&cfg.ServerURL, "server", "", "")
	flag.StringVar(&cfg.Token, "token", "", "")
	flag.StringVar(&cfg.NodeID, "node-id", "", "")
	flag.IntVar(&cfg.Interval, "interval", 0, "")
	flag.BoolVar(&cfg.Mock, "mock", false, "")

	flag.Parse()

	// 尝试从配置文件加载
	if fileData, err := os.ReadFile(cfg.ConfigFile); err == nil {
		parseSimpleYAML(string(fileData), cfg)
	}

	if cfg.ServerURL == "" {
		cfg.ServerURL = utils.GetEnv("NETRADAR_SERVER", "ws://127.0.0.1:8899/ws/agent")
	}
	if cfg.Token == "" {
		cfg.Token = utils.GetEnv("NETRADAR_TOKEN", "netradar_secret_token_12345")
	}
	if cfg.NodeID == "" {
		cfg.NodeID = utils.GetEnv("NETRADAR_NODE_ID", getStableHardwareUUID())
	}
	if cfg.Interval <= 0 {
		envInterval, _ := strconv.Atoi(os.Getenv("NETRADAR_INTERVAL"))
		if envInterval > 0 {
			cfg.Interval = envInterval
		} else {
			cfg.Interval = 3
		}
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
				if iv, err := strconv.Atoi(v); err == nil && iv > 0 {
					cfg.Interval = iv
				}
			}
		}
	}
}

func saveStandardYAML(path string, cfg *AgentConfig) {
	content := fmt.Sprintf(`server: "%s"
token: "%s"
uuid: "%s"
interval: %d
`, cfg.ServerURL, cfg.Token, cfg.NodeID, cfg.Interval)

	// 内容一致则跳过写入，避免闪存介质频繁擦写
	if existing, err := os.ReadFile(path); err == nil {
		if strings.TrimSpace(string(existing)) == strings.TrimSpace(content) {
			return
		}
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err == nil {
		log.Printf("[探针] 配置文件已更新: %s", path)
	}
}
