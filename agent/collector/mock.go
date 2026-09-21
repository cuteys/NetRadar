package collector

import (
	"math/rand"
	"time"
)

type MockTarget struct {
	IP       string
	Port     int
	Proto    string
	Weight   int
}

var mockTargets = []MockTarget{
	{"104.244.42.1", 443, "TCP", 15},
	{"140.82.114.4", 443, "TCP", 10},
	{"116.207.245.38", 443, "TCP", 30},
	{"183.2.172.185", 443, "TCP", 25},
	{"223.5.5.5", 53, "UDP", 20},
	{"119.29.29.29", 53, "UDP", 15},
	{"162.254.197.36", 27015, "UDP", 20},
	{"13.107.4.52", 443, "TCP", 10},
	{"151.101.1.69", 443, "TCP", 12},
	{"185.199.108.153", 443, "TCP", 10},
	{"1.1.1.1", 443, "UDP", 15},
	{"180.101.50.242", 443, "TCP", 20},
	{"123.125.114.144", 443, "TCP", 18},
	{"125.71.229.130", 443, "TCP", 15},
}

var mockLANDevices = []struct {
	IP   string
	Base int64
}{
	{"192.168.31.55", 150000},
	{"192.168.31.88", 450000},
	{"192.168.31.166", 80000},
	{"192.168.31.199", 600000},
	{"192.168.31.10", 250000},
}

type MockCollector struct {
	accumEntries []*RawConntrackEntry
	r            *rand.Rand
}

func NewMockCollector() *MockCollector {
	src := rand.NewSource(time.Now().UnixNano())
	mc := &MockCollector{
		r: rand.New(src),
	}
	mc.initEntries()
	return mc
}

func (m *MockCollector) initEntries() {
	sport := 49152
	for _, dev := range mockLANDevices {
		numConns := 3 + m.r.Intn(4)
		for i := 0; i < numConns; i++ {
			target := mockTargets[m.r.Intn(len(mockTargets))]
			sport++
			e := &RawConntrackEntry{
				Proto:    target.Proto,
				Src1:     dev.IP,
				Dst1:     target.IP,
				Sport1:   sport,
				Dport1:   target.Port,
				Packets1: 10,
				Bytes1:   1000,
				Src2:     target.IP,
				Dst2:     dev.IP,
				Sport2:   target.Port,
				Dport2:   sport,
				Packets2: 15,
				Bytes2:   2500,
				State:    "ESTABLISHED",
			}
			m.accumEntries = append(m.accumEntries, e)
		}
	}
}

func (m *MockCollector) GenerateNextTick() []*RawConntrackEntry {
	for _, e := range m.accumEntries {
		if m.r.Float32() < 0.85 {
			var baseIn int64 = 5000
			var baseOut int64 = 1000

			if e.Src1 == "192.168.31.199" {
				baseIn = 80000 + int64(m.r.Intn(120000))
			} else if e.Src1 == "192.168.31.88" {
				baseIn = 120000 + int64(m.r.Intn(200000))
				baseOut = 20000 + int64(m.r.Intn(30000))
			} else if e.Src1 == "192.168.31.55" {
				baseIn = 30000 + int64(m.r.Intn(50000))
				baseOut = 10000 + int64(m.r.Intn(20000))
			} else {
				baseIn = 5000 + int64(m.r.Intn(15000))
				baseOut = 2000 + int64(m.r.Intn(6000))
			}

			e.Bytes1 += baseOut
			e.Packets1 += int64(m.r.Intn(10) + 1)
			e.Bytes2 += baseIn
			e.Packets2 += int64(m.r.Intn(25) + 2)
		}
	}
	return m.accumEntries
}
