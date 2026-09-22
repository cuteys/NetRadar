package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner     = "cuteys"
	repoName      = "NetRadar"
	checkInterval = 10 * time.Minute
	initialDelay  = 30 * time.Second
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func StartAutoUpdater(currentVersion string) {
	if os.Getenv("NETRADAR_DISABLE_AUTO_UPDATE") == "true" || os.Getenv("NETRADAR_DISABLE_AUTO_UPDATE") == "1" {
		log.Println("[更新] 自动更新已通过环境变量禁用")
		return
	}

	go func() {
		time.Sleep(initialDelay)
		checkAndUpdate(currentVersion)

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for range ticker.C {
			checkAndUpdate(currentVersion)
		}
	}()
}

func checkAndUpdate(currentVersion string) {
	if currentVersion == "" || currentVersion == "dev" {
		return
	}

	rel, err := fetchLatestRelease()
	if err != nil {
		return
	}

	latestTag := strings.TrimSpace(rel.TagName)
	if latestTag == "" {
		return
	}

	if isNewerVersion(currentVersion, latestTag) {
		log.Printf("[更新] 检测到新版本: %s (当前: %s)，准备升级...", latestTag, currentVersion)
		if err := performUpdate(rel, latestTag); err != nil {
			log.Printf("[更新] 自动升级失败: %v", err)
		}
	}
}

func fetchLatestRelease() (*githubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	urls := []string{
		"https://gh-proxy.com/" + apiURL,
		apiURL,
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error

	for _, u := range urls {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "NetRadar-Agent-Updater")
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("GitHub API returned status %d for %s", resp.StatusCode, u)
			continue
		}

		var rel githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			lastErr = err
			continue
		}

		return &rel, nil
	}

	return nil, lastErr
}

func isNewerVersion(current, latest string) bool {
	c := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(current)), "v")
	l := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(latest)), "v")
	if l == "" || c == l {
		return false
	}

	cParts := strings.Split(c, ".")
	lParts := strings.Split(l, ".")
	maxLen := len(cParts)
	if len(lParts) > maxLen {
		maxLen = len(lParts)
	}

	for i := 0; i < maxLen; i++ {
		var cVal, lVal int
		if i < len(cParts) {
			cVal, _ = strconv.Atoi(cParts[i])
		}
		if i < len(lParts) {
			lVal, _ = strconv.Atoi(lParts[i])
		}
		if lVal > cVal {
			return true
		}
		if lVal < cVal {
			return false
		}
	}

	return false
}

func performUpdate(rel *githubRelease, targetTag string) error {
	assetName := getTargetAssetName()
	downloadURL := ""

	for _, asset := range rel.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		downloadURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", repoOwner, repoName, targetTag, assetName)
	}

	urls := []string{
		"https://gh-proxy.com/" + downloadURL,
		downloadURL,
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("解析软链接失败: %w", err)
	}

	tmpFile := execPath + ".download.tmp"
	defer os.Remove(tmpFile)

	client := &http.Client{Timeout: 60 * time.Second}
	var downloaded bool

	for _, u := range urls {
		log.Printf("[更新] 正在从 %s 下载新版本...", u)
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "NetRadar-Agent-Updater")

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		out, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("创建临时文件失败: %w", err)
		}

		_, err = io.Copy(out, resp.Body)
		out.Close()
		resp.Body.Close()

		if err != nil {
			continue
		}

		if fi, err := os.Stat(tmpFile); err == nil && fi.Size() > 500*1024 {
			downloaded = true
			break
		}
	}

	if !downloaded {
		return fmt.Errorf("下载失败")
	}

	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("修改权限失败: %w", err)
	}
	if err := os.Rename(tmpFile, execPath); err != nil {
		return fmt.Errorf("替换二进制失败: %w", err)
	}

	log.Printf("[更新] 已升级到 %s，正在重启...", targetTag)
	os.Exit(0)
	return nil
}

func getTargetAssetName() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	if arch == "arm" {
		return "agent-linux-armv7"
	}

	return fmt.Sprintf("agent-%s-%s", osName, arch)
}
