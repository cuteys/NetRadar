package config

import (
	"flag"
	"netradar/pkg/utils"
	"strconv"
)

type ServerConfig struct {
	ListenAddr    string
	AgentToken    string
	AdminUser     string
	AdminPassword string
	JWTSecret     string
	DBPath        string
	GeoIPDir      string
	GatewayLat    float64
	GatewayLng    float64
	GatewayCity   string
}

func LoadConfig() *ServerConfig {
	cfg := &ServerConfig{}

	flag.StringVar(&cfg.ListenAddr, "listen", utils.GetEnv("NETRADAR_LISTEN", ":8899"), "监听地址 (默认 :8899)")
	flag.StringVar(&cfg.AgentToken, "agent-token", utils.GetEnv("NETRADAR_AGENT_TOKEN", ""), "探针通信 Token")
	flag.StringVar(&cfg.AdminUser, "admin-user", utils.GetEnv("NETRADAR_ADMIN_USER", "admin"), "控制台管理员用户名")
	flag.StringVar(&cfg.AdminPassword, "admin-password", utils.GetEnv("NETRADAR_ADMIN_PASSWORD", ""), "控制台管理员密码")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", utils.GetEnv("NETRADAR_JWT_SECRET", ""), "JWT 签名密钥")
	flag.StringVar(&cfg.DBPath, "db-path", utils.GetEnv("NETRADAR_DB_PATH", "/data/sqlite/sqlite.db"), "SQLite 数据库路径")
	flag.StringVar(&cfg.GeoIPDir, "geoip-dir", utils.GetEnv("NETRADAR_GEOIP_DIR", "/data/geoip"), "GeoIP 离线库目录")

	latStr := utils.GetEnv("NETRADAR_GATEWAY_LAT", "31.2304")
	lngStr := utils.GetEnv("NETRADAR_GATEWAY_LNG", "121.4737")
	cfg.GatewayCity = utils.GetEnv("NETRADAR_GATEWAY_CITY", "上海")

	cfg.GatewayLat, _ = strconv.ParseFloat(latStr, 64)
	cfg.GatewayLng, _ = strconv.ParseFloat(lngStr, 64)

	flag.Parse()

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = utils.RandomHex(32)
	}

	return cfg
}
