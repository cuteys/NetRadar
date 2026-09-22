package collector

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type RawConntrackEntry struct {
	Proto    string
	Src1     string
	Dst1     string
	Sport1   int
	Dport1   int
	Packets1 int64
	Bytes1   int64

	Src2     string
	Dst2     string
	Sport2   int
	Dport2   int
	Packets2 int64
	Bytes2   int64

	State string
}

var ConntrackPaths = []string{
	"/proc/net/nf_conntrack",
	"/proc/net/ip_conntrack",
}

const ConntrackAcctPath = "/proc/sys/net/netfilter/nf_conntrack_acct"

// EnsureConntrackAcct 确保内核开启了连接跟踪流量统计计数
func EnsureConntrackAcct() {
	if _, err := os.Stat(ConntrackAcctPath); err == nil {
		data, err := os.ReadFile(ConntrackAcctPath)
		if err == nil && strings.TrimSpace(string(data)) != "1" {
			_ = os.WriteFile(ConntrackAcctPath, []byte("1\n"), 0644)
		}
	}
}

// CheckConntrackAvailable 检测当前系统是否可用 nf_conntrack
func CheckConntrackAvailable() (string, bool) {
	EnsureConntrackAcct()
	for _, p := range ConntrackPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	return "", false
}

// ReadConntrackEntries 高效流式读取并解析连接跟踪表
func ReadConntrackEntries(path string) ([]*RawConntrackEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make([]*RawConntrackEntry, 0, 128)
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		entry := parseConntrackLine(line)
		if entry != nil {
			entries = append(entries, entry)
		}
	}

	return entries, scanner.Err()
}

// parseConntrackLine 高性能单次扫描解析单行连接跟踪记录，避免频繁的堆内存分配
func parseConntrackLine(line string) *RawConntrackEntry {
	if len(line) < 20 {
		return nil
	}

	entry := &RawConntrackEntry{
		Proto: "OTHER",
		State: "ESTABLISHED",
	}

	var tokenCount int
	var dir int

	for len(line) > 0 {
		// 跳过前导空白
		i := 0
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		line = line[i:]
		if len(line) == 0 {
			break
		}

		// 截取当前 token
		j := 0
		for j < len(line) && line[j] != ' ' && line[j] != '\t' {
			j++
		}
		f := line[:j]
		line = line[j:]
		tokenCount++

		// 前几个 token 识别协议类型
		if tokenCount <= 5 {
			if f == "tcp" || f == "udp" || f == "icmp" || f == "sctp" || f == "gre" || f == "esp" {
				entry.Proto = strings.ToUpper(f)
				continue
			}
		}

		// 常见连接状态
		if f == "ESTABLISHED" || f == "TIME_WAIT" || f == "CLOSE_WAIT" || f == "SYN_SENT" {
			entry.State = f
			continue
		}

		eqIdx := strings.IndexByte(f, '=')
		if eqIdx <= 0 {
			continue
		}

		key := f[:eqIdx]
		val := f[eqIdx+1:]

		switch key {
		case "src":
			if entry.Src1 == "" {
				entry.Src1 = val
				dir = 0
			} else {
				entry.Src2 = val
				dir = 1
			}
		case "dst":
			if dir == 0 {
				entry.Dst1 = val
			} else {
				entry.Dst2 = val
			}
		case "sport":
			p, _ := strconv.Atoi(val)
			if dir == 0 {
				entry.Sport1 = p
			} else {
				entry.Sport2 = p
			}
		case "dport":
			p, _ := strconv.Atoi(val)
			if dir == 0 {
				entry.Dport1 = p
			} else {
				entry.Dport2 = p
			}
		case "packets":
			pkts, _ := strconv.ParseInt(val, 10, 64)
			if dir == 0 {
				entry.Packets1 = pkts
			} else {
				entry.Packets2 = pkts
			}
		case "bytes":
			b, _ := strconv.ParseInt(val, 10, 64)
			if dir == 0 {
				entry.Bytes1 = b
			} else {
				entry.Bytes2 = b
			}
		}
	}

	if entry.Src1 == "" || entry.Dst1 == "" {
		return nil
	}
	return entry
}

// GenerateFlowKey 预分配内存构造双向唯一流标识
func GenerateFlowKey(proto, src string, sport int, dst string, dport int) string {
	if src > dst {
		src, dst = dst, src
		sport, dport = dport, sport
	}

	var b strings.Builder
	b.Grow(len(proto) + len(src) + len(dst) + 16)
	b.WriteString(proto)
	b.WriteByte(':')
	b.WriteString(src)
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(sport))
	b.WriteByte('-')
	b.WriteString(dst)
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(dport))
	return b.String()
}
