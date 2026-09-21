package geo

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DBDownloadTarget struct {
	FileName string
	URLs     []string
	MinSize  int64
}

var DefaultTargets = []DBDownloadTarget{
	{
		FileName: "GeoLite2-City.mmdb",
		URLs: []string{
			"https://gh-proxy.com/https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb",
			"https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb",
		},
		MinSize: 10 * 1024 * 1024, // > 10MB
	},
	{
		FileName: "GeoCN.mmdb",
		URLs: []string{
			"https://gh-proxy.com/https://github.com/cuteys/GeoCN/releases/latest/download/GeoCN.mmdb",
			"https://github.com/cuteys/GeoCN/releases/latest/download/GeoCN.mmdb",
		},
		MinSize: 2 * 1024 * 1024, // > 2MB
	},
	{
		FileName: "GeoLite2-ASN.mmdb",
		URLs: []string{
			"https://gh-proxy.com/https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb",
			"https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb",
		},
		MinSize: 2 * 1024 * 1024, // > 2MB
	},
}

type GeoDownloader struct {
	dbDir       string
	client      *http.Client
	isUpdating  bool
	mu          sync.Mutex
	onCompleted func()
}

func NewGeoDownloader(dbDir string, onCompleted func()) *GeoDownloader {
	return &GeoDownloader{
		dbDir: dbDir,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		onCompleted: onCompleted,
	}
}

func (d *GeoDownloader) EnsureDatabases() {
	if err := os.MkdirAll(d.dbDir, 0755); err != nil {
		log.Printf("[GeoDownloader] 创建目录失败 %s: %v", d.dbDir, err)
		return
	}

	missing := false
	for _, target := range DefaultTargets {
		fullPath := filepath.Join(d.dbDir, target.FileName)
		if fi, err := os.Stat(fullPath); err != nil || fi.Size() < target.MinSize {
			missing = true
			break
		}
	}

	if missing {
		log.Printf("[GeoDownloader] 本地缺少 IP 离线库，后台开始下载...")
		go d.DownloadAll()
	}
}

func (d *GeoDownloader) DownloadAll() {
	d.mu.Lock()
	if d.isUpdating {
		d.mu.Unlock()
		return
	}
	d.isUpdating = true
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.isUpdating = false
		d.mu.Unlock()
	}()

	updatedCount := 0
	for _, target := range DefaultTargets {
		fullPath := filepath.Join(d.dbDir, target.FileName)
		if d.downloadTarget(target, fullPath) {
			updatedCount++
		}
	}

	if updatedCount > 0 && d.onCompleted != nil {
		log.Printf("[GeoDownloader] 已更新 %d 个数据库，正在重载引擎...", updatedCount)
		d.onCompleted()
	}
}

func (d *GeoDownloader) downloadTarget(target DBDownloadTarget, destPath string) bool {
	tmpPath := destPath + ".tmp"
	defer os.Remove(tmpPath)

	for attempt, url := range target.URLs {
		log.Printf("[GeoDownloader] 正在下载 %s (源 %d/%d)...", target.FileName, attempt+1, len(target.URLs))

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "NetRadar-GeoIP-Updater")

		resp, err := d.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				_ = resp.Body.Close()
			}
			log.Printf("[GeoDownloader] 下载失败 %s (%s)", target.FileName, url)
			continue
		}

		out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			_ = resp.Body.Close()
			return false
		}

		written, err := io.Copy(out, resp.Body)
		_ = out.Close()
		_ = resp.Body.Close()

		if err != nil || written < target.MinSize {
			log.Printf("[GeoDownloader] 下载数据不完整 (%d 字节)，尝试下一镜像源...", written)
			_ = os.Remove(tmpPath)
			continue
		}

		// 内容一致则跳过替换
		if fileExists(destPath) {
			oldHash := fileSHA256(destPath)
			newHash := fileSHA256(tmpPath)
			if oldHash != "" && oldHash == newHash {
				_ = os.Remove(tmpPath)
				return false
			}
		}

		if err := os.Rename(tmpPath, destPath); err != nil {
			log.Printf("[GeoDownloader] 替换文件失败 %s: %v", destPath, err)
			return false
		}

		log.Printf("[GeoDownloader] 下载完成 %s (%.2f MB)", target.FileName, float64(written)/(1024*1024))
		return true
	}

	log.Printf("[GeoDownloader] 警告: 所有镜像源均无法下载 %s", target.FileName)
	return false
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func fileSHA256(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
