package model

import "time"

type FlowRecord struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`

	Protocol string `json:"protocol"`
	SrcIP    string `json:"src_ip"`
	SrcPort  int    `json:"src_port"`
	DstIP    string `json:"dst_ip"`
	DstPort  int    `json:"dst_port"`

	BytesIn  int64 `json:"bytes_in"`
	BytesOut int64 `json:"bytes_out"`
	Packets  int64 `json:"packets"`

	State string `json:"state"`

	Country   string  `json:"country,omitempty"`
	Region    string  `json:"region,omitempty"`
	City      string  `json:"city,omitempty"`
	ISP       string  `json:"isp,omitempty"`
	Latitude  float64 `json:"lat,omitempty"`
	Longitude float64 `json:"lng,omitempty"`
}

type NodeMetricsPayload struct {
	NodeID    string        `json:"node_id"`
	Hostname  string        `json:"hostname"`
	OS        string        `json:"os"`
	Arch      string        `json:"arch"`
	Timestamp int64         `json:"timestamp"`
	Interval  float64       `json:"interval"`
	Flows     []*FlowRecord `json:"flows"`

	TotalBytesIn  int64   `json:"total_bytes_in"`
	TotalBytesOut int64   `json:"total_bytes_out"`
	TotalPackets  int64   `json:"total_packets"`
	RateInBps     float64 `json:"rate_in_bps"`
	RateOutBps    float64 `json:"rate_out_bps"`
	ActiveConns   int     `json:"active_conns"`
}

type NodeInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Hostname    string    `json:"hostname"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	IP          string    `json:"ip"`
	Version     string    `json:"version"`
	LastSeen    time.Time `json:"last_seen"`
	IsOnline    bool      `json:"is_online"`
	RateInBps   float64   `json:"rate_in_bps"`
	RateOutBps  float64   `json:"rate_out_bps"`
	ActiveConns int       `json:"active_conns"`

	GatewayLat float64 `json:"gateway_lat"`
	GatewayLng float64 `json:"gateway_lng"`
}

type UpdateNodeRequest struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	IP         string  `json:"ip"`
	GatewayLat float64 `json:"gateway_lat"`
	GatewayLng float64 `json:"gateway_lng"`
}

type DeviceStats struct {
	IP         string  `json:"ip"`
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	RateInBps  float64 `json:"rate_in_bps"`
	RateOutBps float64 `json:"rate_out_bps"`
	TotalIn    int64   `json:"total_in"`
	TotalOut   int64   `json:"total_out"`
	ConnCount  int     `json:"conn_count"`
	LastActive int64   `json:"last_active"`
	IsCustom   bool    `json:"is_custom"`
	NodeID     string  `json:"node_id,omitempty"`
}

type RenameDeviceRequest struct {
	IP   string `json:"ip"`
	Name string `json:"name"`
}

type SystemSettings struct {
	Username        string `json:"username"`
	AgentToken      string `json:"agent_token"`
	AgentServerAddr string `json:"agent_server_addr"`
	UseTLS          bool   `json:"use_tls"`
}

type UpdateSystemSettingsRequest struct {
	Username        string `json:"username"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	AgentToken      string `json:"agent_token"`
	AgentServerAddr string `json:"agent_server_addr"`
	UseTLS          bool   `json:"use_tls"`
}

type LiveBroadcastMessage struct {
	Type        string              `json:"type"`
	NodeID      string              `json:"node_id"`
	Timestamp   int64               `json:"timestamp"`
	Summary     *NodeMetricsSummary `json:"summary,omitempty"`
	Flows       []*ParticleFlow     `json:"flows,omitempty"`
	TopLAN      []*DeviceStats      `json:"top_lan,omitempty"`
	ProtoDist   map[string]int64    `json:"proto_dist,omitempty"`
	CountryDist map[string]int64    `json:"country_dist,omitempty"`
}

type ParticleFlow struct {
	ID        string     `json:"id"`
	NodeID    string     `json:"node_id"`
	SrcIP     string     `json:"src_ip"`
	DstIP     string     `json:"dst_ip"`
	DstPort   int        `json:"dst_port"`
	Protocol  string     `json:"protocol"`
	BytesIn   int64      `json:"bytes_in"`
	BytesOut  int64      `json:"bytes_out"`
	Country   string     `json:"country"`
	City      string     `json:"city"`
	ISP       string     `json:"isp"`
	Color     string     `json:"color"`
	FromCoord [2]float64 `json:"from_coord"`
	ToCoord   [2]float64 `json:"to_coord"`
}

type NodeMetricsSummary struct {
	RateInBps     float64 `json:"rate_in_bps"`
	RateOutBps    float64 `json:"rate_out_bps"`
	TotalBytesIn  int64   `json:"total_bytes_in"`
	TotalBytesOut int64   `json:"total_bytes_out"`
	TotalRequests int64   `json:"total_requests"`
	ActiveConns   int     `json:"active_conns"`
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	Username  string `json:"username"`
}
