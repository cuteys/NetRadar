package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

	// 1. 获取并解析官方发布的 SHA-256 校验清单
	checksums, err := fetchChecksums(rel, targetTag)
	if err != nil {
		return fmt.Errorf("获取 SHA-256 校验清单失败: %w", err)
	}

	expectedHash, ok := checksums[assetName]
	if !ok || expectedHash == "" {
		return fmt.Errorf("发布清单中未找到 %s 的 SHA-256 校验和", assetName)
	}

	urls := []string{
		downloadURL,
		"https://gh-proxy.com/" + downloadURL,
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
		return fmt.Errorf("下载失败或文件不完整")
	}

	// 2. 严格校验文件头部魔数 (ELF/PE/Mach-O，防止中间人注入 HTML 报错页或损坏文件)
	if err := validateExecutableHeader(tmpFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("二进制格式合法性校验失败: %w", err)
	}

	// 3. 严格比对 SHA-256 校验和 (防止第三方镜像篡改/投毒)
	if err := verifyFileChecksum(tmpFile, expectedHash); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("SHA-256 完整性校验未通过: %w", err)
	}
	log.Printf("[更新] SHA-256 完整性校验通过 (%s)，二进制格式合法", expectedHash)

	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("修改权限失败: %w", err)
	}
	if err := os.Rename(tmpFile, execPath); err != nil {
		return fmt.Errorf("替换二进制失败: %w", err)
	}

	log.Printf("[更新] 已升级到 %s，正在执行平滑自重启...", targetTag)
	restartProcess(execPath)
	return nil
}

func fetchChecksums(rel *githubRelease, targetTag string) (map[string]string, error) {
	downloadURL := ""
	for _, asset := range rel.Assets {
		if asset.Name == "sha256sums.txt" {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		downloadURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/sha256sums.txt", repoOwner, repoName, targetTag)
	}

	urls := []string{
		downloadURL,
		"https://gh-proxy.com/" + downloadURL,
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error

	for _, u := range urls {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "NetRadar-Agent-Updater")

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			lastErr = fmt.Errorf("请求 %s 失败", u)
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		checksums := make(map[string]string)
		lines := strings.Split(string(body), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				hash := strings.TrimSpace(fields[0])
				name := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
				checksums[name] = hash
			}
		}

		if len(checksums) > 0 {
			return checksums, nil
		}
	}

	return nil, fmt.Errorf("未能获取或解析 sha256sums.txt: %v", lastErr)
}

func verifyFileChecksum(filePath, expectedHex string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("计算哈希失败: %w", err)
	}

	actualHex := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualHex, strings.TrimSpace(expectedHex)) {
		return fmt.Errorf("校验和不匹配 (预期: %s, 实际: %s)", expectedHex, actualHex)
	}
	return nil
}

func validateExecutableHeader(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	header := make([]byte, 16)
	n, err := f.Read(header)
	if err != nil || n < 4 {
		return fmt.Errorf("读取二进制头部失败或文件长度过短")
	}

	switch runtime.GOOS {
	case "linux":
		if header[0] != 0x7f || header[1] != 'E' || header[2] != 'L' || header[3] != 'F' {
			return fmt.Errorf("非合法 Linux ELF 二进制头部")
		}
	case "darwin":
		if !(header[0] == 0xfe && header[1] == 0xed && header[2] == 0xfa && (header[3] == 0xce || header[3] == 0xcf)) &&
			!(header[0] == 0xcf && header[1] == 0xfa && header[2] == 0xed && header[3] == 0xfe) &&
			!(header[0] == 0xca && header[1] == 0xfe && header[2] == 0xba && header[3] == 0xbe) {
			return fmt.Errorf("非合法 macOS Mach-O 二进制头部")
		}
	case "windows":
		if header[0] != 'M' || header[1] != 'Z' {
			return fmt.Errorf("非合法 Windows PE 二进制头部")
		}
	}

	return nil
}

func restartProcess(execPath string) {
	// 1. 在 Linux/Unix 上优先使用 syscall.Exec 直接用新可执行文件替换当前进程镜像
	// 保持原 PID、文件描述符与后台终端环境，无需依赖外部 systemd 或 cron 即可实现零间断即时重启
	if runtime.GOOS != "windows" {
		err := syscall.Exec(execPath, os.Args, os.Environ())
		if err != nil {
			log.Printf("[更新] syscall.Exec 重启失败: %v，尝试拉起新进程...", err)
		}
	}

	// 2. Windows 或 syscall.Exec 不可用时的回退机制：拉起新子进程后安全退出
	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		log.Printf("[更新] 启动新进程失败: %v", err)
	}
	os.Exit(0)
}

func getTargetAssetName() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	if arch == "arm" {
		return "agent-linux-armv7"
	}

	return fmt.Sprintf("agent-%s-%s", osName, arch)
}
