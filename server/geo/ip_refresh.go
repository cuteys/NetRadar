package geo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type CyCDResponse struct {
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

type GatewayLocation struct {
	IPv4        string  `json:"ipv4"`
	IPv6        string  `json:"ipv6"`
	GatewayCity string  `json:"gateway_city"`
	GatewayLat  float64 `json:"gateway_lat"`
	GatewayLng  float64 `json:"gateway_lng"`
	LastUpdated string  `json:"last_updated"`
}

type IPRefreshService struct {
	mu       sync.RWMutex
	location GatewayLocation
	client   *http.Client
}

func NewIPRefreshService() *IPRefreshService {
	s := &IPRefreshService{
		location: GatewayLocation{
			GatewayCity: "上海",
			GatewayLat:  31.2304,
			GatewayLng:  121.4737,
		},
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}

	go s.runLoop()
	return s
}

func (s *IPRefreshService) runLoop() {
	s.RefreshNow()

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.RefreshNow()
	}
}

func (s *IPRefreshService) RefreshNow() error {
	// 网络请求在锁外执行，避免阻塞其他 goroutine
	var newIPv4, newIPv6, newCity string
	var newLat, newLng float64

	req4, err4 := http.NewRequest(http.MethodGet, "https://ipv4.cy.cd/api/json", nil)
	if err4 == nil {
		req4.Header.Set("User-Agent", "NetRadar")
		resp4, err := s.client.Do(req4)
		if err == nil && resp4.StatusCode == http.StatusOK {
			var data4 CyCDResponse
			if err := json.NewDecoder(resp4.Body).Decode(&data4); err == nil && data4.Data.IP != "" {
				newIPv4 = data4.Data.IP
				if data4.Data.Location.Latitude != 0 {
					newLat = data4.Data.Location.Latitude
					newLng = data4.Data.Location.Longitude
				}
				var regionStr string
				if len(data4.Data.Regions) > 0 {
					regionStr = strings.Join(data4.Data.Regions, "·")
				}
				newCity = fmt.Sprintf("%s (%s)", data4.Data.Country.Name, regionStr)
			}
			_ = resp4.Body.Close()
		}
	}

	req6, err6 := http.NewRequest(http.MethodGet, "https://ipv6.cy.cd/api/json", nil)
	if err6 == nil {
		req6.Header.Set("User-Agent", "NetRadar")
		resp6, err := s.client.Do(req6)
		if err == nil && resp6.StatusCode == http.StatusOK {
			var data6 CyCDResponse
			if err := json.NewDecoder(resp6.Body).Decode(&data6); err == nil {
				newIPv6 = data6.Data.IP
			}
			_ = resp6.Body.Close()
		}
	}

	s.mu.Lock()
	if newIPv4 != "" {
		s.location.IPv4 = newIPv4
	}
	if newIPv6 != "" {
		s.location.IPv6 = newIPv6
	}
	if newCity != "" {
		s.location.GatewayCity = newCity
	}
	if newLat != 0 && newLng != 0 {
		s.location.GatewayLat = newLat
		s.location.GatewayLng = newLng
	}
	s.location.LastUpdated = time.Now().Format("2006-01-02 15:04:05")
	currentLoc := s.location
	s.mu.Unlock()

	log.Printf("[IPRefresh] 网关公网 IP 已刷新: IPv4=%s, IPv6=%s (%.4f, %.4f)",
		currentLoc.IPv4, currentLoc.IPv6, currentLoc.GatewayLat, currentLoc.GatewayLng)
	return nil
}

func (s *IPRefreshService) GetLocation() GatewayLocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.location
}
