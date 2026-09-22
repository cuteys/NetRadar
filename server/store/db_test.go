package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"netradar/pkg/model"
)

func TestDatabaseOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "netradar_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	defer db.Close()

	// 1. Credentials test
	err = db.SaveAdminCredentials("admin", "hash123", "token123", ":8899", "127.0.0.1:8899", false, "secret123")
	if err != nil {
		t.Fatalf("SaveAdminCredentials failed: %v", err)
	}

	user, hash, token, port, addr, tls, secret, err := db.GetAdminCredentials()
	if err != nil {
		t.Fatalf("GetAdminCredentials failed: %v", err)
	}
	if user != "admin" || hash != "hash123" || token != "token123" || port != ":8899" || addr != "127.0.0.1:8899" || tls != false || secret != "secret123" {
		t.Errorf("GetAdminCredentials mismatch: user=%s, hash=%s, token=%s, port=%s, addr=%s, tls=%v, secret=%s",
			user, hash, token, port, addr, tls, secret)
	}

	// 2. Node Upsert & Query with CPU, Mem, Uptime
	node := &model.NodeInfo{
		ID:          "node-1",
		Name:        "Router 1",
		Hostname:    "OpenWrt",
		OS:          "linux",
		Arch:        "mipsle",
		IP:          "192.168.1.1",
		Version:     "v1.0.0",
		CPUUsage:    3.5,
		MemUsage:    45.2,
		Uptime:      123456,
		RateInBps:   1000,
		RateOutBps:  2000,
		GatewayLat:  31.23,
		GatewayLng:  121.47,
		IsOnline:    true,
	}

	err = db.UpsertNode(node)
	if err != nil {
		t.Fatalf("UpsertNode failed: %v", err)
	}

	nodes, err := db.GetNodes()
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.ID != "node-1" || n.Name != "Router 1" || n.CPUUsage != 3.5 || n.MemUsage != 45.2 || n.Uptime != 123456 {
		t.Errorf("Node data mismatch: %+v", n)
	}

	// 3. History Downsampled Test
	now := time.Now().Unix()
	for i := 0; i < 10; i++ {
		_ = db.RecordTraffic("node-1", now-int64(i*5), 1000, 500, 200, 100, 5)
	}

	// Range 1h (since 3600s ago) with 30s buckets
	history, err := db.GetHistory("node-1", now-3600, 100, 30)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) == 0 {
		t.Fatalf("Expected history records, got 0")
	}
}
