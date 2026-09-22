package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"netradar/pkg/utils"
	"netradar/pkg/version"
	"netradar/server/api"
	"netradar/server/auth"
	"netradar/server/config"
	"netradar/server/geo"
	"netradar/server/store"
	"netradar/server/ws"
)

//go:embed dist/*
var embeddedDist embed.FS

var Version = version.Version

func formatDisplayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "http://127.0.0.1:" + strings.TrimPrefix(addr, "0.0.0.0:")
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return "http://" + addr
	}
	return addr
}

func main() {
	cfg := config.LoadConfig()

	log.Printf("==================================================")
	log.Printf("   NetRadar 控制端 (%s)", Version)
	log.Printf("==================================================")
	log.Printf("监听地址: %s", cfg.ListenAddr)
	log.Printf("数据路径: %s", cfg.DBPath)
	log.Printf("==================================================")

	db, err := store.NewDatabase(cfg.DBPath)
	if err != nil {
		log.Fatalf("[Server] 初始化 SQLite 数据库失败: %v", err)
	}
	defer db.Close()

	// 账号与密钥初始化
	adminUser, pwdHash, storedToken, storedPort, storedAddr, storedTLS, storedSecret, err := db.GetAdminCredentials()
	var authSvc *auth.AuthService
	if err != nil || adminUser == "" {
		adminUser = cfg.AdminUser
		initialPwd := cfg.AdminPassword
		generated := false
		if initialPwd == "" {
			initialPwd = utils.RandomString(10, "nr_")
			generated = true
		}
		if cfg.AgentToken == "" {
			cfg.AgentToken = utils.RandomString(24, "nr_")
		}
		storedAddr = ""
		storedTLS = false

		authSvc, err = auth.NewAuthService(adminUser, initialPwd, cfg.JWTSecret)
		if err != nil {
			log.Fatalf("[Server] 初始化认证服务失败: %v", err)
		}

		newHash, _ := authSvc.UpdateCredentials(adminUser, initialPwd)
		_ = db.SaveAdminCredentials(adminUser, string(newHash), cfg.AgentToken, cfg.ListenAddr, storedAddr, storedTLS, cfg.JWTSecret)

		if generated {
			banner := fmt.Sprintf("\n"+
				"*****************************************************************\n"+
				"*             NetRadar 控制台首次部署初始化成功                *\n"+
				"*                                                               *\n"+
				"*   管理后台地址: %-45s *\n"+
				"*   管理员用户名: %-20s                          *\n"+
				"*   随机初始密码: %-20s                          *\n"+
				"*   探针通信密钥: %-20s                          *\n"+
				"*                                                               *\n"+
				"*   (提示：首次登录后可在右上角【设置】中随时修改配置)         *\n"+
				"*****************************************************************\n",
				formatDisplayAddr(cfg.ListenAddr), adminUser, initialPwd, cfg.AgentToken)
			log.Printf("%s", banner)
		}
	} else {
		if storedToken != "" {
			cfg.AgentToken = storedToken
		}
		if storedPort != "" {
			cfg.ListenAddr = storedPort
		}
		if storedSecret != "" {
			cfg.JWTSecret = storedSecret
		} else {
			_ = db.SaveAdminCredentials(adminUser, pwdHash, cfg.AgentToken, cfg.ListenAddr, storedAddr, storedTLS, cfg.JWTSecret)
		}

		authSvc, err = auth.NewAuthService(adminUser, "dummy", cfg.JWTSecret)
		if err != nil {
			log.Fatalf("[Server] 初始化认证服务失败: %v", err)
		}
		if pwdHash != "" {
			authSvc.SetPasswordHash(adminUser, []byte(pwdHash))
		}

		// 允许通过 -admin-password 命令行参数强制重置密码
		if cfg.AdminPassword != "" {
			newHash, err := authSvc.UpdateCredentials(adminUser, cfg.AdminPassword)
			if err == nil {
				_ = db.SaveAdminCredentials(adminUser, string(newHash), cfg.AgentToken, cfg.ListenAddr, storedAddr, storedTLS, cfg.JWTSecret)
				log.Printf("[Server] 管理员密码已通过命令行参数成功重置")
			}
		}
	}

	geoSvc := geo.NewGeoService(cfg.GeoIPDir)
	defer geoSvc.Close()

	ipRefresher := geo.NewIPRefreshService()
	hub := ws.NewHub(cfg, authSvc, geoSvc, ipRefresher, db)
	hub.SetAgentSettings(storedAddr, storedTLS)

	handler := api.NewAPIHandler(authSvc, hub, db, Version)

	var staticFS fs.FS
	if subFS, err := fs.Sub(embeddedDist, "dist"); err == nil {
		if _, err := subFS.Open("index.html"); err == nil {
			staticFS = subFS
		}
	}
	if staticFS == nil {
		if fi, err := os.Stat("./server/dist"); err == nil && fi.IsDir() {
			staticFS = os.DirFS("./server/dist")
		}
	}

	router := api.NewRouter(authSvc, handler, hub, staticFS)

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	go func() {
		log.Printf("[Server] NetRadar 控制台已启动: %s", formatDisplayAddr(cfg.ListenAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Server] 运行异常: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[Server] 正在平滑关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[Server] 服务强制关闭: %v", err)
	}
	log.Println("[Server] 服务已安全退出")
}
