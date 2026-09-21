package ws

import (
	"encoding/json"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"netradar/pkg/model"
	"netradar/pkg/utils"
	"netradar/server/auth"
	"netradar/server/config"
	"netradar/server/geo"
	"netradar/server/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	HandshakeTimeout: 5 * time.Second,
}

var stablePalette = []string{
	"#10b981", "#0ea5e9", "#f59e0b", "#ec4899",
	"#8b5cf6", "#06b6d4", "#f97316", "#84cc16",
}

func getStableColor(ip string) string {
	h := fnv.New32a()
	h.Write([]byte(ip))
	return stablePalette[int(h.Sum32())%len(stablePalette)]
}

type Hub struct {
	cfg         *config.ServerConfig
	authService *auth.AuthService
	geoService  *geo.GeoService
	ipRefresher *geo.IPRefreshService
	db          *store.Database

	mu              sync.RWMutex
	webConns        map[*websocket.Conn]bool
	nodes           map[string]*model.NodeInfo
	nodeLANMap      map[string]map[string]*model.DeviceStats // 按 nodeID 隔离局域网终端
	agentServerAddr string
	useTLS          bool
	stopHeartbeat   chan struct{}
}

func NewHub(cfg *config.ServerConfig, authSvc *auth.AuthService, geoSvc *geo.GeoService, ipRefresh *geo.IPRefreshService, db *store.Database) *Hub {
	h := &Hub{
		cfg:           cfg,
		authService:   authSvc,
		geoService:    geoSvc,
		ipRefresher:   ipRefresh,
		db:            db,
		webConns:      make(map[*websocket.Conn]bool),
		nodes:         make(map[string]*model.NodeInfo),
		nodeLANMap:    make(map[string]map[string]*model.DeviceStats),
		stopHeartbeat: make(chan struct{}),
	}

	if dbNodes, err := db.GetNodes(); err == nil {
		for _, n := range dbNodes {
			n.IsOnline = false
			h.nodes[n.ID] = n
		}
	}

	go h.runLivenessChecker()
	return h
}

func (h *Hub) runLivenessChecker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopHeartbeat:
			return
		case <-ticker.C:
			now := time.Now()

			h.mu.Lock()
			changed := false
			for _, node := range h.nodes {
				if node.IsOnline && now.Sub(node.LastSeen) > 15*time.Second {
					node.IsOnline = false
					changed = true
					log.Printf("[Hub] 探针节点超时离线: %s (%s)", node.ID, node.Name)
				}
			}
			h.mu.Unlock()

			if changed {
				h.BroadcastNodeStatus()
			}

			h.mu.RLock()
			for conn := range h.webConns {
				_ = conn.SetWriteDeadline(now.Add(2 * time.Second))
				_ = conn.WriteMessage(websocket.PingMessage, nil)
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) HandleAgentWS(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-NetRadar-Token")
	if token != h.cfg.AgentToken {
		log.Printf("[Hub] 拦截未经授权的探针连接: %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	nodeID := r.Header.Get("X-NetRadar-Node-ID")
	if nodeID == "" {
		nodeID = "node-" + utils.RandomHex(3)
	}
	rawName := r.Header.Get("X-NetRadar-Node-Name")
	nodeName, _ := url.QueryUnescape(rawName)

	agentPublicIP := r.Header.Get("X-NetRadar-Public-IP")
	var agentLat, agentLng float64
	if latStr := r.Header.Get("X-NetRadar-Lat"); latStr != "" {
		agentLat, _ = strconv.ParseFloat(latStr, 64)
	}
	if lngStr := r.Header.Get("X-NetRadar-Lng"); lngStr != "" {
		agentLng, _ = strconv.ParseFloat(lngStr, 64)
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Hub] 探针 WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	effectiveIP := agentPublicIP
	if effectiveIP == "" {
		effectiveIP = remoteIP
	}

	h.mu.Lock()
	node, exists := h.nodes[nodeID]
	if !exists {
		lat := agentLat
		lng := agentLng
		if lat == 0 && lng == 0 {
			lat = h.cfg.GatewayLat
			lng = h.cfg.GatewayLng
		}
		displayName := nodeName
		if displayName == "" {
			displayName = nodeID
		}
		node = &model.NodeInfo{
			ID:         nodeID,
			Name:       displayName,
			IP:         effectiveIP,
			LastSeen:   time.Now(),
			IsOnline:   true,
			GatewayLat: lat,
			GatewayLng: lng,
		}
		h.nodes[nodeID] = node
	} else {
		node.IsOnline = true
		node.LastSeen = time.Now()
		if !node.CustomLocation {
			if effectiveIP != "" {
				node.IP = effectiveIP
			}
			if agentLat != 0 && agentLng != 0 {
				node.GatewayLat = agentLat
				node.GatewayLng = agentLng
			}
		}
	}

	log.Printf("[Hub] 探针已连接: ID=%s 名称=%s 公网IP=%s (连接IP=%s)", nodeID, node.Name, effectiveIP, remoteIP)

	if _, ok := h.nodeLANMap[nodeID]; !ok {
		h.nodeLANMap[nodeID] = make(map[string]*model.DeviceStats)
	}
	h.mu.Unlock()
	_ = h.db.UpsertNode(node)
	h.BroadcastNodeStatus()

	conn.SetReadLimit(2 * 1024 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[Hub] 探针连接断开: %s (%v)", nodeID, err)
			h.mu.Lock()
			if n, ok := h.nodes[nodeID]; ok {
				n.IsOnline = false
				n.LastSeen = time.Now()
			}
			h.mu.Unlock()
			h.BroadcastNodeStatus()
			break
		}

		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		var payload model.NodeMetricsPayload
		if err := json.Unmarshal(msg, &payload); err != nil {
			continue
		}

		h.processAgentPayload(&payload, node)
	}
}

func (h *Hub) processAgentPayload(p *model.NodeMetricsPayload, node *model.NodeInfo) {
	node.LastSeen = time.Now()
	node.IsOnline = true
	node.RateInBps = p.RateInBps
	node.RateOutBps = p.RateOutBps
	node.ActiveConns = p.ActiveConns
	node.Hostname = p.Hostname
	node.OS = p.OS
	node.Arch = p.Arch

	statusChanged := false
	if (node.Name == "" || node.Name == node.ID) && p.Hostname != "" {
		node.Name = p.Hostname
		statusChanged = true
	}

	if !node.CustomLocation {
		if p.PublicIP != "" && node.IP != p.PublicIP {
			node.IP = p.PublicIP
			statusChanged = true
		}
		if p.GatewayLat != 0 && p.GatewayLng != 0 && (node.GatewayLat != p.GatewayLat || node.GatewayLng != p.GatewayLng) {
			node.GatewayLat = p.GatewayLat
			node.GatewayLng = p.GatewayLng
			statusChanged = true
		}
	}
	if statusChanged {
		_ = h.db.UpsertNode(node)
		h.BroadcastNodeStatus()
	}

	_ = h.db.RecordTraffic(p.NodeID, p.Timestamp, p.TotalBytesIn, p.TotalBytesOut, p.RateInBps, p.RateOutBps, p.ActiveConns)

	gatewayCoord := [2]float64{node.GatewayLng, node.GatewayLat}
	if gatewayCoord[0] == 0 && gatewayCoord[1] == 0 {
		loc := h.ipRefresher.GetLocation()
		gatewayCoord = [2]float64{loc.GatewayLng, loc.GatewayLat}
	}
	if gatewayCoord[0] == 0 && gatewayCoord[1] == 0 {
		gatewayCoord = [2]float64{h.cfg.GatewayLng, h.cfg.GatewayLat}
	}

	var particleFlows []*model.ParticleFlow
	protoDist := make(map[string]int64)
	countryDist := make(map[string]int64)

	interval := p.Interval
	if interval <= 0 {
		interval = 2.0
	}

	h.mu.Lock()
	lanDevices, exists := h.nodeLANMap[p.NodeID]
	if !exists {
		lanDevices = make(map[string]*model.DeviceStats)
		h.nodeLANMap[p.NodeID] = lanDevices
	}
	for _, dev := range lanDevices {
		dev.RateInBps = 0
		dev.RateOutBps = 0
	}
	h.mu.Unlock()

	for _, flow := range p.Flows {
		loc := h.geoService.Lookup(flow.DstIP)
		flow.Country = loc.Country
		flow.Region = loc.Region
		flow.City = loc.City
		flow.ISP = loc.ISP
		flow.Latitude = loc.Latitude
		flow.Longitude = loc.Longitude

		protoDist[flow.Protocol] += flow.BytesIn + flow.BytesOut
		countryDist[loc.Country] += flow.BytesIn + flow.BytesOut

		pf := &model.ParticleFlow{
			ID:        flow.ID,
			NodeID:    p.NodeID,
			SrcIP:     flow.SrcIP,
			SrcPort:   flow.SrcPort,
			DstIP:     flow.DstIP,
			DstPort:   flow.DstPort,
			Protocol:  flow.Protocol,
			BytesIn:   flow.BytesIn,
			BytesOut:  flow.BytesOut,
			Country:   loc.Country,
			City:      loc.City,
			ISP:       loc.ISP,
			Color:     getStableColor(flow.DstIP),
			FromCoord: gatewayCoord,
			ToCoord:   [2]float64{loc.Longitude, loc.Latitude},
		}
		particleFlows = append(particleFlows, pf)

		if flow.SrcIP != "" {
			h.mu.Lock()
			dev, ok := lanDevices[flow.SrcIP]
			if !ok {
				customName, isCustom := h.db.GetDeviceAliasDirect(flow.SrcIP)
				name := customName
				if name == "" {
					name = flow.SrcIP
				}

				dev = &model.DeviceStats{
					IP:         flow.SrcIP,
					Name:       name,
					Category:   "device",
					IsCustom:   isCustom,
					NodeID:     p.NodeID,
					LastActive: time.Now().Unix(),
				}
				lanDevices[flow.SrcIP] = dev
			}

			if alias, ok := h.db.GetDeviceAliasDirect(flow.SrcIP); ok && alias != "" {
				dev.Name = alias
				dev.IsCustom = true
			}

			dev.RateInBps += float64(flow.BytesIn) / interval
			dev.RateOutBps += float64(flow.BytesOut) / interval
			dev.TotalIn += flow.BytesIn
			dev.TotalOut += flow.BytesOut
			dev.LastActive = time.Now().Unix()
			h.mu.Unlock()
		}
	}

	if len(particleFlows) > 0 {
		go h.db.RecordDestinations(p.NodeID, particleFlows)
	}

	h.mu.RLock()
	var topLAN []*model.DeviceStats
	for _, d := range lanDevices {
		topLAN = append(topLAN, d)
	}
	h.mu.RUnlock()

	sort.Slice(topLAN, func(i, j int) bool {
		return (topLAN[i].RateInBps + topLAN[i].RateOutBps) > (topLAN[j].RateInBps + topLAN[j].RateOutBps)
	})
	if len(topLAN) > 15 {
		topLAN = topLAN[:15]
	}

	totalReqs := int64(len(particleFlows))
	sort.Slice(particleFlows, func(i, j int) bool {
		return (particleFlows[i].BytesIn + particleFlows[i].BytesOut) > (particleFlows[j].BytesIn + particleFlows[j].BytesOut)
	})
	if len(particleFlows) > 40 {
		particleFlows = particleFlows[:40]
	}

	broadcastMsg := &model.LiveBroadcastMessage{
		Type:      "metrics",
		NodeID:    p.NodeID,
		Timestamp: p.Timestamp,
		Summary: &model.NodeMetricsSummary{
			RateInBps:     p.RateInBps,
			RateOutBps:    p.RateOutBps,
			TotalBytesIn:  p.TotalBytesIn,
			TotalBytesOut: p.TotalBytesOut,
			TotalRequests: totalReqs,
			ActiveConns:   p.ActiveConns,
		},
		Flows:       particleFlows,
		TopLAN:      topLAN,
		ProtoDist:   protoDist,
		CountryDist: countryDist,
	}

	h.BroadcastToWeb(broadcastMsg)
}

func (h *Hub) HandleDashboardWS(w http.ResponseWriter, r *http.Request) {
	if !h.authService.ValidateRequest(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	h.webConns[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.webConns, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	// 保持连接读取心跳
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (h *Hub) BroadcastToWeb(msg *model.LiveBroadcastMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.webConns {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (h *Hub) BroadcastNodeStatus() {
	nodes := h.GetActiveNodes()
	data, err := json.Marshal(map[string]interface{}{
		"type":  "node_status",
		"nodes": nodes,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.webConns {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (h *Hub) GetActiveNodes() []*model.NodeInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	list := make([]*model.NodeInfo, 0, len(h.nodes))
	for _, n := range h.nodes {
		list = append(list, n)
	}
	return list
}

func (h *Hub) DeleteNode(nodeID string) error {
	h.mu.Lock()
	delete(h.nodes, nodeID)
	delete(h.nodeLANMap, nodeID)
	h.mu.Unlock()
	h.BroadcastNodeStatus()
	return h.db.DeleteNode(nodeID)
}

func (h *Hub) UpdateNodeInfo(req *model.UpdateNodeRequest) error {
	h.mu.Lock()
	if n, ok := h.nodes[req.ID]; ok {
		if req.Name != "" {
			n.Name = req.Name
		}
		if req.IP != "" {
			n.IP = req.IP
		}
		if req.GatewayLat != 0 {
			n.GatewayLat = req.GatewayLat
		}
		if req.GatewayLng != 0 {
			n.GatewayLng = req.GatewayLng
		}
		n.CustomLocation = req.CustomLocation
	}
	h.mu.Unlock()
	h.BroadcastNodeStatus()
	return h.db.UpdateNodeInfo(req)
}

func (h *Hub) UpdateAgentToken(token string) {
	h.mu.Lock()
	h.cfg.AgentToken = token
	h.mu.Unlock()
}

func (h *Hub) GetAgentToken() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.cfg.AgentToken
}

func (h *Hub) GetListenAddr() string {
	return h.cfg.ListenAddr
}

func (h *Hub) SetAgentSettings(addr string, useTLS bool) {
	h.mu.Lock()
	h.agentServerAddr = addr
	h.useTLS = useTLS
	h.mu.Unlock()
}

func (h *Hub) GetAgentServerAddr() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.agentServerAddr
}

func (h *Hub) GetUseTLS() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.useTLS
}

func (h *Hub) RenameDevice(ip, newName string) error {
	h.mu.Lock()
	for _, lanDevices := range h.nodeLANMap {
		if dev, ok := lanDevices[ip]; ok {
			dev.Name = newName
			dev.IsCustom = true
		}
	}
	h.mu.Unlock()
	return h.db.SetDeviceAlias(ip, newName)
}
