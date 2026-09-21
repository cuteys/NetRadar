package api

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"netradar/pkg/model"
	"netradar/server/auth"
	"netradar/server/store"
	"netradar/server/ws"
)

type APIHandler struct {
	authService *auth.AuthService
	hub         *ws.Hub
	db          *store.Database
}

func NewAPIHandler(authSvc *auth.AuthService, hub *ws.Hub, db *store.Database) *APIHandler {
	return &APIHandler{
		authService: authSvc,
		hub:         hub,
		db:          db,
	}
}

func (h *APIHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "请求参数格式无效"})
		return
	}

	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	token, err := h.authService.CheckLogin(clientIP, req.Username, req.Password)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "netradar_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	respondJSON(w, http.StatusOK, model.AuthResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix(),
		Username:  req.Username,
	})
}

func (h *APIHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "netradar_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	username, _ := r.Context().Value(auth.UserContextKey).(string)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"username":      username,
	})
}

func (h *APIHandler) HandleGetNodes(w http.ResponseWriter, r *http.Request) {
	activeNodes := h.hub.GetActiveNodes()
	if len(activeNodes) == 0 {
		dbNodes, err := h.db.GetNodes()
		if err == nil && len(dbNodes) > 0 {
			activeNodes = dbNodes
		}
	}
	if activeNodes == nil {
		activeNodes = []*model.NodeInfo{}
	}
	respondJSON(w, http.StatusOK, activeNodes)
}

func (h *APIHandler) HandleDeleteNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("id")
	if nodeID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少节点 ID"})
		return
	}

	if err := h.hub.DeleteNode(nodeID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "deleted_id": nodeID})
}

func (h *APIHandler) HandleUpdateNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "请求参数无效或缺少节点 ID"})
		return
	}

	if err := h.hub.UpdateNodeInfo(&req); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "node": req})
}

func (h *APIHandler) HandleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, model.SystemSettings{
		Username:        h.authService.GetUsername(),
		AgentToken:      h.hub.GetAgentToken(),
		AgentServerAddr: h.hub.GetAgentServerAddr(),
		UseTLS:          h.hub.GetUseTLS(),
	})
}

func (h *APIHandler) HandleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.UpdateSystemSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "参数无效"})
		return
	}

	if req.NewPassword != "" {
		if !h.authService.VerifyPassword(req.CurrentPassword) {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "当前原密码错误"})
			return
		}
	}

	newUsername := req.Username
	if newUsername == "" {
		newUsername = h.authService.GetUsername()
	}

	var newPwdHash []byte
	var err error
	if req.NewPassword != "" {
		newPwdHash, err = h.authService.UpdateCredentials(newUsername, req.NewPassword)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if req.AgentToken != "" {
		h.hub.UpdateAgentToken(req.AgentToken)
	}
	h.hub.SetAgentSettings(req.AgentServerAddr, req.UseTLS)

	currentUsername, currentHash, currentToken, currentPort, currentAddr, currentTLS, currentSecret, _ := h.db.GetAdminCredentials()
	if newUsername != "" {
		currentUsername = newUsername
	}
	if len(newPwdHash) > 0 {
		currentHash = string(newPwdHash)
	}
	if req.AgentToken != "" {
		currentToken = req.AgentToken
	}
	currentAddr = req.AgentServerAddr
	currentTLS = req.UseTLS

	_ = h.db.SaveAdminCredentials(currentUsername, currentHash, currentToken, currentPort, currentAddr, currentTLS, currentSecret)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "ok",
		"username":          currentUsername,
		"token":             currentToken,
		"agent_server_addr": currentAddr,
		"use_tls":           currentTLS,
	})
}

func (h *APIHandler) HandleGetHistory(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	rangeParam := r.URL.Query().Get("range")

	now := time.Now().Unix()
	var sinceTs int64
	limit := 100

	switch rangeParam {
	case "1h":
		sinceTs = now - 3600
	case "24h":
		sinceTs = now - 86400
	case "7d":
		sinceTs = now - 7*86400
	default:
		sinceTs = now - 600
		limit = 60
	}

	points, err := h.db.GetHistory(nodeID, sinceTs, limit)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, points)
}

func (h *APIHandler) HandleGetAggregatedStats(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	rangeParam := r.URL.Query().Get("range")

	now := time.Now().Unix()
	var sinceTs int64

	switch rangeParam {
	case "1h":
		sinceTs = now - 3600
	case "24h":
		sinceTs = now - 86400
	case "7d":
		sinceTs = now - 7*86400
	default:
		sinceTs = now - 3600
	}

	stats, err := h.db.GetAggregatedTotals(nodeID, sinceTs)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (h *APIHandler) HandleRenameDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.RenameDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "IP 或名称不能为空"})
		return
	}

	if err := h.hub.RenameDevice(req.IP, req.Name); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"ip":     req.IP,
		"name":   req.Name,
	})
}

func (h *APIHandler) HandleGetSummary(w http.ResponseWriter, r *http.Request) {
	nodes := h.hub.GetActiveNodes()
	var totalRateIn, totalRateOut float64
	var totalConns int
	onlineCount := 0

	for _, n := range nodes {
		if n.IsOnline {
			onlineCount++
			totalRateIn += n.RateInBps
			totalRateOut += n.RateOutBps
			totalConns += n.ActiveConns
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_nodes":    len(nodes),
		"online_nodes":   onlineCount,
		"total_rate_in":  totalRateIn,
		"total_rate_out": totalRateOut,
		"active_conns":   totalConns,
	})
}

func (h *APIHandler) HandleGetDestinations(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	rangeParam := r.URL.Query().Get("range")

	now := time.Now().Unix()
	var sinceTs int64
	limit := 300

	switch rangeParam {
	case "1h":
		sinceTs = now - 3600
	case "24h":
		sinceTs = now - 86400
	case "7d":
		sinceTs = now - 7*86400
	default:
		sinceTs = now - 3600
	}

	points, err := h.db.GetHistoricalDestinations(nodeID, sinceTs, limit)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, points)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
