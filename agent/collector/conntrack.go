package collector

import (
	"bufio"
	"fmt"
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

func EnsureConntrackAcct() {
	if _, err := os.Stat(ConntrackAcctPath); err == nil {
		data, err := os.ReadFile(ConntrackAcctPath)
		if err == nil && strings.TrimSpace(string(data)) != "1" {
			_ = os.WriteFile(ConntrackAcctPath, []byte("1\n"), 0644)
		}
	}
}

func CheckConntrackAvailable() (string, bool) {
	EnsureConntrackAcct()
	for _, p := range ConntrackPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	return "", false
}

func ReadConntrackEntries(path string) ([]*RawConntrackEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []*RawConntrackEntry
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

func parseConntrackLine(line string) *RawConntrackEntry {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil
	}

	entry := &RawConntrackEntry{
		Proto: "OTHER",
		State: "ESTABLISHED",
	}

	if len(fields) >= 3 && (fields[0] == "ipv4" || fields[0] == "ipv6") && !strings.Contains(fields[2], "=") {
		entry.Proto = strings.ToUpper(fields[2])
	} else {
		for _, f := range fields {
			if f == "tcp" || f == "udp" || f == "icmp" || f == "sctp" || f == "gre" || f == "esp" {
				entry.Proto = strings.ToUpper(f)
				break
			}
		}
	}

	var dir int
	for _, f := range fields {
		if f == "ESTABLISHED" || f == "TIME_WAIT" || f == "CLOSE_WAIT" || f == "SYN_SENT" {
			entry.State = f
			continue
		}

		if idx := strings.IndexByte(f, '='); idx > 0 {
			key := f[:idx]
			val := f[idx+1:]

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
	}

	if entry.Src1 == "" || entry.Dst1 == "" {
		return nil
	}
	return entry
}

func GenerateFlowKey(proto, src string, sport int, dst string, dport int) string {
	if src > dst {
		src, dst = dst, src
		sport, dport = dport, sport
	}
	return fmt.Sprintf("%s:%s:%d-%s:%d", proto, src, sport, dst, dport)
}
