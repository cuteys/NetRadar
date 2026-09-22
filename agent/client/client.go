package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"netradar/agent/config"
	"netradar/pkg/model"
)

type GeoInfo struct {
	IP        string  `json:"ip"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
}

func DetectPublicIPAndGeo() *GeoInfo {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	fetch := func(endpoint string) (*GeoInfo, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "NetRadar")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}

		var res struct {
			Code int `json:"code"`
			Data struct {
				IP       string `json:"ip"`
				Location struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"location"`
				Country struct {
					Name string `json:"name"`
				} `json:"country"`
				Regions []string `json:"regions"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}

		if res.Data.IP == "" {
			return nil, errors.New("empty ip")
		}

		city := res.Data.Country.Name
		if len(res.Data.Regions) > 0 {
			city = fmt.Sprintf("%s (%s)", city, strings.Join(res.Data.Regions, "·"))
		}

		return &GeoInfo{
			IP:        res.Data.IP,
			Latitude:  res.Data.Location.Latitude,
			Longitude: res.Data.Location.Longitude,
			City:      city,
		}, nil
	}

	if info, err := fetch("https://ipv4.cy.cd/api/json"); err == nil {
		return info
	}
	if info, err := fetch("https://ipv6.cy.cd/api/json"); err == nil {
		return info
	}
	return nil
}

type AgentWSClient struct {
	cfg       *config.AgentConfig
	conn      *websocket.Conn
	mu        sync.Mutex
	isClosed  bool
	connected bool

	geoMu   sync.RWMutex
	geoInfo *GeoInfo
}

func NewAgentWSClient(cfg *config.AgentConfig) *AgentWSClient {
	return &AgentWSClient{
		cfg: cfg,
	}
}

func (c *AgentWSClient) Start() {
	c.refreshGeo()
	go c.geoRefreshLoop()
	go c.connectLoop()
}

func (c *AgentWSClient) geoRefreshLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.isClosed {
				return
			}
			c.refreshGeo()
		}
	}
}

func (c *AgentWSClient) refreshGeo() {
	if info := DetectPublicIPAndGeo(); info != nil {
		c.geoMu.Lock()
		c.geoInfo = info
		c.geoMu.Unlock()
		log.Printf("[探针] 自身公网 IP 与坐标探测成功: %s (%.4f, %.4f)", info.IP, info.Latitude, info.Longitude)
	}
}

func (c *AgentWSClient) GetGeoInfo() *GeoInfo {
	c.geoMu.RLock()
	defer c.geoMu.RUnlock()
	return c.geoInfo
}

func (c *AgentWSClient) connectLoop() {
	backoff := 1 * time.Second

	for !c.isClosed {
		err := c.connect()
		if err != nil {
			log.Printf("[探针] 连接服务端失败 (%s): %v，%v 后重试...", c.cfg.ServerURL, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}

		backoff = 1 * time.Second
		log.Printf("[探针] 已成功连接到 NetRadar 服务端: %s", c.cfg.ServerURL)

		for {
			_, _, err := c.conn.ReadMessage()
			if err != nil {
				log.Printf("[探针] 连接中断: %v", err)
				c.mu.Lock()
				c.connected = false
				if c.conn != nil {
					_ = c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				break
			}
		}
	}
}

func (c *AgentWSClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	header := http.Header{}
	header.Set("X-NetRadar-Token", c.cfg.Token)
	header.Set("X-NetRadar-Node-ID", c.cfg.NodeID)
	if c.cfg.Version != "" {
		header.Set("X-NetRadar-Version", c.cfg.Version)
	}

	if geo := c.GetGeoInfo(); geo != nil {
		if geo.IP != "" {
			header.Set("X-NetRadar-Public-IP", geo.IP)
		}
		if geo.Latitude != 0 && geo.Longitude != 0 {
			header.Set("X-NetRadar-Lat", strconv.FormatFloat(geo.Latitude, 'f', 6, 64))
			header.Set("X-NetRadar-Lng", strconv.FormatFloat(geo.Longitude, 'f', 6, 64))
		}
		if geo.City != "" {
			header.Set("X-NetRadar-City", url.QueryEscape(geo.City))
		}
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, resp, err := dialer.Dial(c.cfg.ServerURL, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			log.Printf("[探针] 认证失败 (HTTP 401)，请检查通信密钥 Token 是否正确！")
		}
		return err
	}

	c.conn = conn
	c.connected = true
	return nil
}

func (c *AgentWSClient) SendPayload(payload *model.NodeMetricsPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *AgentWSClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isClosed = true
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
