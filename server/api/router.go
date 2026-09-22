package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"

	"netradar/pkg/version"
	"netradar/server/auth"
	"netradar/server/ws"
)

func NewRouter(authSvc *auth.AuthService, handler *APIHandler, hub *ws.Hub, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws/agent", hub.HandleAgentWS)
	mux.HandleFunc("/ws/dashboard", hub.HandleDashboardWS)

	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version": version.Version,
		})
	})

	mux.HandleFunc("/api/auth/login", handler.HandleLogin)
	mux.HandleFunc("/api/auth/logout", handler.HandleLogout)

	mux.HandleFunc("/api/auth/me", authSvc.RequireAuth(handler.HandleMe))
	mux.HandleFunc("/api/nodes", authSvc.RequireAuth(handler.HandleGetNodes))
	mux.HandleFunc("/api/nodes/update", authSvc.RequireAuth(handler.HandleUpdateNode))
	mux.HandleFunc("/api/nodes/delete", authSvc.RequireAuth(handler.HandleDeleteNode))
	mux.HandleFunc("/api/stats/summary", authSvc.RequireAuth(handler.HandleGetSummary))
	mux.HandleFunc("/api/destinations", authSvc.RequireAuth(handler.HandleGetDestinations))
	mux.HandleFunc("/api/stats/history", authSvc.RequireAuth(handler.HandleGetHistory))
	mux.HandleFunc("/api/history", authSvc.RequireAuth(handler.HandleGetHistory)) // 兼容历史别名，杜绝 404
	mux.HandleFunc("/api/stats/aggregated", authSvc.RequireAuth(handler.HandleGetAggregatedStats))
	mux.HandleFunc("/api/device/rename", authSvc.RequireAuth(handler.HandleRenameDevice))
	mux.HandleFunc("/api/settings/system", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.HandleUpdateSystemSettings(w, r)
		} else {
			handler.HandleGetSystemSettings(w, r)
		}
	}))

	mux.HandleFunc("/install-agent.sh", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("install-agent.sh")
		if err != nil {
			http.Redirect(w, r, "https://raw.githubusercontent.com/cuteys/NetRadar/master/install-agent.sh", http.StatusFound)
			return
		}

		serverAddr := hub.GetAgentServerAddr()
		if serverAddr == "" {
			serverAddr = r.Host
		}
		token := hub.GetAgentToken()
		useTLS := "false"
		if hub.GetUseTLS() {
			useTLS = "true"
		}

		content := string(data)
		content = strings.Replace(content, `SERVER_ADDR=""`, `SERVER_ADDR="`+serverAddr+`"`, 1)
		content = strings.Replace(content, `AGENT_TOKEN=""`, `AGENT_TOKEN="`+token+`"`, 1)
		content = strings.Replace(content, `USE_TLS=false`, `USE_TLS=`+useTLS, 1)

		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		_, _ = w.Write([]byte(content))
	})

	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
				http.NotFound(w, r)
				return
			}

			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" || path == "index.html" {
				path = "index.html"
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			}
			f, err := staticFS.Open(path)
			if err != nil {
				r.URL.Path = "/"
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			} else {
				_ = f.Close()
			}

			fileServer.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-NetRadar-Token")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedOrigin(origin, host string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	// 允许同主机访问或本地开发端口 (如 Vite: 5173)
	if strings.EqualFold(u.Host, host) || strings.HasPrefix(u.Host, "localhost:") || strings.HasPrefix(u.Host, "127.0.0.1:") || u.Host == "localhost" || u.Host == "127.0.0.1" {
		return true
	}
	if allowed := os.Getenv("NETRADAR_ALLOWED_ORIGINS"); allowed != "" {
		for _, o := range strings.Split(allowed, ",") {
			if strings.TrimSpace(o) == origin || strings.TrimSpace(o) == "*" {
				return true
			}
		}
	}
	return false
}
