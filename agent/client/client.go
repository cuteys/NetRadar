package client

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"netradar/agent/config"
	"netradar/pkg/model"
)

type AgentWSClient struct {
	cfg       *config.AgentConfig
	conn      *websocket.Conn
	mu        sync.Mutex
	isClosed  bool
	connected bool
}

func NewAgentWSClient(cfg *config.AgentConfig) *AgentWSClient {
	return &AgentWSClient{
		cfg: cfg,
	}
}

func (c *AgentWSClient) Start() {
	go c.connectLoop()
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
	header.Set("X-NetRadar-Node-Name", url.QueryEscape(c.cfg.NodeName))

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
