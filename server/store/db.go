package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"netradar/pkg/model"
)

type Database struct {
	db         *sql.DB
	mu         sync.Mutex
	aliasCache sync.Map
	stopClean  chan struct{}
}

func NewDatabase(dbPath string) (*Database, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}

	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL;")
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")

	d := &Database{
		db:        db,
		stopClean: make(chan struct{}),
	}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("数据库表迁移失败: %w", err)
	}

	d.loadAliasesToCache()
	d.startRetentionCleaner(7)

	return d, nil
}

func (d *Database) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		name TEXT,
		hostname TEXT,
		os TEXT,
		arch TEXT,
		ip TEXT,
		version TEXT,
		first_seen DATETIME,
		last_seen DATETIME,
		gateway_lat REAL,
		gateway_lng REAL
	);

	CREATE TABLE IF NOT EXISTS traffic_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id TEXT,
		timestamp INTEGER,
		bytes_in INTEGER,
		bytes_out INTEGER,
		rate_in REAL,
		rate_out REAL,
		active_conns INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_traffic_history_node_time ON traffic_history(node_id, timestamp);

	CREATE TABLE IF NOT EXISTS device_aliases (
		ip TEXT PRIMARY KEY,
		custom_name TEXT,
		updated_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS admin_credentials (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		username TEXT,
		password_hash TEXT,
		agent_token TEXT,
		listen_port TEXT,
		agent_server_addr TEXT DEFAULT '',
		use_tls INTEGER DEFAULT 0,
		jwt_secret TEXT DEFAULT '',
		updated_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS destination_history (
		node_id TEXT,
		dst_ip TEXT,
		dst_port INTEGER,
		protocol TEXT,
		country TEXT,
		city TEXT,
		isp TEXT,
		latitude REAL,
		longitude REAL,
		bytes_in INTEGER,
		bytes_out INTEGER,
		last_seen INTEGER,
		PRIMARY KEY(node_id, dst_ip)
	);
	CREATE INDEX IF NOT EXISTS idx_dest_history_node_time ON destination_history(node_id, last_seen);
	`
	_, err := d.db.Exec(schema)
	if err == nil {
		_, _ = d.db.Exec(`ALTER TABLE admin_credentials ADD COLUMN agent_server_addr TEXT DEFAULT ''`)
		_, _ = d.db.Exec(`ALTER TABLE admin_credentials ADD COLUMN use_tls INTEGER DEFAULT 0`)
		_, _ = d.db.Exec(`ALTER TABLE admin_credentials ADD COLUMN jwt_secret TEXT DEFAULT ''`)
		_, _ = d.db.Exec(`ALTER TABLE nodes ADD COLUMN custom_location INTEGER DEFAULT 0`)
	}
	return err
}

// 启动数据保留策略定时清理器（默认保留 retentionDays 天流量历史，30 天外联历史）
func (d *Database) startRetentionCleaner(retentionDays int) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// 启动后先执行一次清理
		time.Sleep(10 * time.Second)
		d.CleanExpiredHistory(retentionDays)

		for {
			select {
			case <-d.stopClean:
				return
			case <-ticker.C:
				d.CleanExpiredHistory(retentionDays)
			}
		}
	}()
}

// CleanExpiredHistory 清理过期历史数据并释放空间
func (d *Database) CleanExpiredHistory(retentionDays int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	trafficThreshold := time.Now().AddDate(0, 0, -retentionDays).Unix()
	destThreshold := time.Now().AddDate(0, 0, -30).Unix()

	resTraffic, err1 := d.db.Exec(`DELETE FROM traffic_history WHERE timestamp < ?`, trafficThreshold)
	resDest, err2 := d.db.Exec(`DELETE FROM destination_history WHERE last_seen < ?`, destThreshold)

	if err1 == nil && err2 == nil {
		n1, _ := resTraffic.RowsAffected()
		n2, _ := resDest.RowsAffected()
		if n1 > 0 || n2 > 0 {
			log.Printf("[DB] 历史数据自动清理完成: 清理过期流量点 %d 条, 过期外联记录 %d 条", n1, n2)
			_, _ = d.db.Exec(`PRAGMA optimize;`)
		}
	}
}

func (d *Database) loadAliasesToCache() {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`SELECT ip, custom_name FROM device_aliases`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ip, name string
		if err := rows.Scan(&ip, &name); err == nil {
			d.aliasCache.Store(ip, name)
		}
	}
}

func (d *Database) GetAdminCredentials() (username, pwdHash, token, port, agentServerAddr string, useTLS bool, jwtSecret string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var tlsInt int
	row := d.db.QueryRow(`SELECT username, password_hash, agent_token, listen_port, COALESCE(agent_server_addr, ''), COALESCE(use_tls, 0), COALESCE(jwt_secret, '') FROM admin_credentials WHERE id = 1`)
	err = row.Scan(&username, &pwdHash, &token, &port, &agentServerAddr, &tlsInt, &jwtSecret)
	useTLS = (tlsInt == 1)
	return
}

func (d *Database) SaveAdminCredentials(username, pwdHash, token, port, agentServerAddr string, useTLS bool, jwtSecret string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tlsInt := 0
	if useTLS {
		tlsInt = 1
	}

	query := `
	INSERT INTO admin_credentials (id, username, password_hash, agent_token, listen_port, agent_server_addr, use_tls, jwt_secret, updated_at)
	VALUES (1, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		username=excluded.username,
		password_hash=excluded.password_hash,
		agent_token=excluded.agent_token,
		listen_port=excluded.listen_port,
		agent_server_addr=excluded.agent_server_addr,
		use_tls=excluded.use_tls,
		jwt_secret=excluded.jwt_secret,
		updated_at=CURRENT_TIMESTAMP;
	`
	_, err := d.db.Exec(query, username, pwdHash, token, port, agentServerAddr, tlsInt, jwtSecret)
	return err
}

func (d *Database) UpsertNode(node *model.NodeInfo) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	customLoc := 0
	if node.CustomLocation {
		customLoc = 1
	}

	query := `
	INSERT INTO nodes (id, name, hostname, os, arch, ip, version, first_seen, last_seen, gateway_lat, gateway_lng, custom_location)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name=CASE WHEN nodes.name != '' THEN nodes.name WHEN excluded.name != '' THEN excluded.name ELSE nodes.name END,
		hostname=excluded.hostname,
		os=excluded.os,
		arch=excluded.arch,
		ip=CASE WHEN nodes.custom_location = 1 THEN nodes.ip WHEN excluded.ip != '' THEN excluded.ip ELSE nodes.ip END,
		gateway_lat=CASE WHEN nodes.custom_location = 1 THEN nodes.gateway_lat WHEN excluded.gateway_lat != 0 THEN excluded.gateway_lat ELSE nodes.gateway_lat END,
		gateway_lng=CASE WHEN nodes.custom_location = 1 THEN nodes.gateway_lng WHEN excluded.gateway_lng != 0 THEN excluded.gateway_lng ELSE nodes.gateway_lng END,
		last_seen=excluded.last_seen;
	`
	_, err := d.db.Exec(query,
		node.ID, node.Name, node.Hostname, node.OS, node.Arch, node.IP, node.Version,
		time.Now(), node.LastSeen, node.GatewayLat, node.GatewayLng, customLoc,
	)
	return err
}

func (d *Database) UpdateNodeInfo(req *model.UpdateNodeRequest) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	customLoc := 0
	if req.CustomLocation {
		customLoc = 1
	}

	query := `
	UPDATE nodes
	SET name = ?, ip = ?, gateway_lat = ?, gateway_lng = ?, custom_location = ?
	WHERE id = ?
	`
	_, err := d.db.Exec(query, req.Name, req.IP, req.GatewayLat, req.GatewayLng, customLoc, req.ID)
	return err
}

func (d *Database) RecordTraffic(nodeID string, timestamp int64, bytesIn, bytesOut int64, rateIn, rateOut float64, conns int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO traffic_history (node_id, timestamp, bytes_in, bytes_out, rate_in, rate_out, active_conns)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, nodeID, timestamp, bytesIn, bytesOut, rateIn, rateOut, conns)
	return err
}

func (d *Database) SetDeviceAlias(ip, customName string) error {
	d.aliasCache.Store(ip, customName)

	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO device_aliases (ip, custom_name, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(ip) DO UPDATE SET
		custom_name=excluded.custom_name,
		updated_at=CURRENT_TIMESTAMP;
	`
	_, err := d.db.Exec(query, ip, customName)
	return err
}

// GetDeviceAliasDirect 从内存缓存直接读取别名，免去数据库读 IO
func (d *Database) GetDeviceAliasDirect(ip string) (string, bool) {
	val, ok := d.aliasCache.Load(ip)
	if !ok {
		return "", false
	}
	return val.(string), true
}

func (d *Database) GetDeviceAliases() (map[string]string, error) {
	aliases := make(map[string]string)
	d.aliasCache.Range(func(key, value any) bool {
		aliases[key.(string)] = value.(string)
		return true
	})
	return aliases, nil
}

func (d *Database) GetNodes() ([]*model.NodeInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`SELECT id, name, hostname, os, arch, ip, version, last_seen, gateway_lat, gateway_lng, COALESCE(custom_location, 0) FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]*model.NodeInfo, 0)
	now := time.Now()

	for rows.Next() {
		var n model.NodeInfo
		var lastSeen time.Time
		var customLoc int
		if err := rows.Scan(&n.ID, &n.Name, &n.Hostname, &n.OS, &n.Arch, &n.IP, &n.Version, &lastSeen, &n.GatewayLat, &n.GatewayLng, &customLoc); err == nil {
			n.LastSeen = lastSeen
			n.IsOnline = now.Sub(lastSeen) < 15*time.Second
			n.CustomLocation = customLoc == 1
			nodes = append(nodes, &n)
		}
	}
	return nodes, nil
}

func (d *Database) GetHistory(nodeID string, sinceTimestamp int64, limit int) ([]map[string]interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var query string
	var args []interface{}

	if nodeID == "" || nodeID == "all" {
		query = `
			SELECT timestamp, SUM(rate_in) as r_in, SUM(rate_out) as r_out, SUM(active_conns) as conns
			FROM traffic_history
			WHERE timestamp >= ?
			GROUP BY timestamp
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{sinceTimestamp, limit}
	} else {
		query = `
			SELECT timestamp, rate_in, rate_out, active_conns
			FROM traffic_history
			WHERE node_id = ? AND timestamp >= ?
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{nodeID, sinceTimestamp, limit}
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []map[string]interface{}
	for rows.Next() {
		var ts int64
		var rateIn, rateOut float64
		var conns int
		if err := rows.Scan(&ts, &rateIn, &rateOut, &conns); err == nil {
			points = append(points, map[string]interface{}{
				"timestamp":    ts,
				"rate_in_bps":  rateIn,
				"rate_out_bps": rateOut,
				"active_conns": conns,
			})
		}
	}

	// 逆转为按时间正序
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}

	return points, nil
}

func (d *Database) GetAggregatedTotals(nodeID string, sinceTimestamp int64) (map[string]interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var query string
	var args []interface{}

	if nodeID == "" || nodeID == "all" {
		query = `
			SELECT
				COALESCE(SUM(bytes_in), 0),
				COALESCE(SUM(bytes_out), 0),
				COUNT(*),
				COALESCE(MAX(rate_in), 0),
				COALESCE(MAX(rate_out), 0)
			FROM traffic_history
			WHERE timestamp >= ?
		`
		args = []interface{}{sinceTimestamp}
	} else {
		query = `
			SELECT
				COALESCE(SUM(bytes_in), 0),
				COALESCE(SUM(bytes_out), 0),
				COUNT(*),
				COALESCE(MAX(rate_in), 0),
				COALESCE(MAX(rate_out), 0)
			FROM traffic_history
			WHERE node_id = ? AND timestamp >= ?
		`
		args = []interface{}{nodeID, sinceTimestamp}
	}

	var bytesIn, bytesOut, count int64
	var peakIn, peakOut float64

	err := d.db.QueryRow(query, args...).Scan(&bytesIn, &bytesOut, &count, &peakIn, &peakOut)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_bytes_in":  bytesIn,
		"total_bytes_out": bytesOut,
		"total_requests": count,
		"peak_rate_in":   peakIn,
		"peak_rate_out":  peakOut,
	}, nil
}

func (d *Database) DeleteNode(nodeID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, _ = d.db.Exec(`DELETE FROM traffic_history WHERE node_id = ?`, nodeID)
	_, _ = d.db.Exec(`DELETE FROM destination_history WHERE node_id = ?`, nodeID)
	_, err := d.db.Exec(`DELETE FROM nodes WHERE id = ?`, nodeID)
	return err
}

func (d *Database) RecordDestinations(nodeID string, flows []*model.ParticleFlow) {
	if len(flows) == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO destination_history (node_id, dst_ip, dst_port, protocol, country, city, isp, latitude, longitude, bytes_in, bytes_out, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id, dst_ip) DO UPDATE SET
			dst_port=excluded.dst_port,
			protocol=excluded.protocol,
			country=excluded.country,
			city=excluded.city,
			isp=excluded.isp,
			latitude=excluded.latitude,
			longitude=excluded.longitude,
			bytes_in=destination_history.bytes_in + excluded.bytes_in,
			bytes_out=destination_history.bytes_out + excluded.bytes_out,
			last_seen=excluded.last_seen;
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, f := range flows {
		if f.DstIP == "" || (f.ToCoord[0] == 0 && f.ToCoord[1] == 0) {
			continue
		}
		_, _ = stmt.Exec(
			nodeID, f.DstIP, f.DstPort, f.Protocol,
			f.Country, f.City, f.ISP,
			f.ToCoord[1], f.ToCoord[0],
			f.BytesIn, f.BytesOut, now,
		)
	}
	_ = tx.Commit()
}

func (d *Database) GetHistoricalDestinations(nodeID string, sinceTimestamp int64, limit int) ([]map[string]interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if limit <= 0 || limit > 500 {
		limit = 300
	}

	var query string
	var args []interface{}

	if nodeID == "" || nodeID == "all" {
		query = `
			SELECT
				dst_ip, dst_port, protocol, country, city, isp,
				latitude, longitude,
				SUM(bytes_in) as total_in,
				SUM(bytes_out) as total_out,
				MAX(last_seen) as max_seen
			FROM destination_history
			WHERE last_seen >= ?
			GROUP BY dst_ip
			ORDER BY (total_in + total_out) DESC
			LIMIT ?
		`
		args = []interface{}{sinceTimestamp, limit}
	} else {
		query = `
			SELECT
				dst_ip, dst_port, protocol, country, city, isp,
				latitude, longitude,
				bytes_in, bytes_out, last_seen
			FROM destination_history
			WHERE node_id = ? AND last_seen >= ?
			ORDER BY (bytes_in + bytes_out) DESC
			LIMIT ?
		`
		args = []interface{}{nodeID, sinceTimestamp, limit}
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var dstIP, protocol, country, city, isp string
		var dstPort int
		var lat, lng float64
		var bytesIn, bytesOut, lastSeen int64

		if err := rows.Scan(&dstIP, &dstPort, &protocol, &country, &city, &isp, &lat, &lng, &bytesIn, &bytesOut, &lastSeen); err == nil {
			results = append(results, map[string]interface{}{
				"dst_ip":    dstIP,
				"dst_port":  dstPort,
				"protocol":  protocol,
				"country":   country,
				"city":      city,
				"isp":       isp,
				"latitude":  lat,
				"longitude": lng,
				"to_coord":  []float64{lng, lat},
				"bytes_in":  bytesIn,
				"bytes_out": bytesOut,
				"last_seen": lastSeen,
			})
		}
	}

	return results, nil
}

func (d *Database) Close() error {
	close(d.stopClean)
	return d.db.Close()
}
