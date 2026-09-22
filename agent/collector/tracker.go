package collector

import (
	"sync"
	"time"

	"netradar/pkg/model"
)

type FlowState struct {
	LastBytesIn  int64
	LastBytesOut int64
	LastSeen     time.Time
}

type DeltaTracker struct {
	mu           sync.Mutex
	flows        map[string]*FlowState
	deviceTotals map[string]*model.DeviceStats
	lastTickTime time.Time
}

func NewDeltaTracker() *DeltaTracker {
	return &DeltaTracker{
		flows:        make(map[string]*FlowState),
		deviceTotals: make(map[string]*model.DeviceStats),
		lastTickTime: time.Now(),
	}
}

func (dt *DeltaTracker) ProcessConntrack(nodeID string, entries []*RawConntrackEntry) (*model.NodeMetricsPayload, []*model.DeviceStats) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(dt.lastTickTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1.0
	}
	dt.lastTickTime = now

	var activeFlows []*model.FlowRecord
	var totalDeltaIn int64
	var totalDeltaOut int64

	for _, dev := range dt.deviceTotals {
		dev.RateInBps = 0
		dev.RateOutBps = 0
	}

	seenKeys := make(map[string]bool)

	for _, e := range entries {
		srcIsPrivate := IsPrivateIP(e.Src1)
		dstIsPrivate := IsPrivateIP(e.Dst1)

		if srcIsPrivate && dstIsPrivate {
			continue
		}

		var lanIP, wanIP string
		var lanPort, wanPort int
		var bytesOut, bytesIn int64

		if srcIsPrivate && !dstIsPrivate {
			lanIP = e.Src1
			lanPort = e.Sport1
			wanIP = e.Dst1
			wanPort = e.Dport1
			bytesOut = e.Bytes1
			bytesIn = e.Bytes2
		} else if !srcIsPrivate && dstIsPrivate {
			wanIP = e.Src1
			wanPort = e.Sport1
			lanIP = e.Dst1
			lanPort = e.Dport1
			bytesIn = e.Bytes1
			bytesOut = e.Bytes2
		} else {
			lanIP = e.Src1
			lanPort = e.Sport1
			wanIP = e.Dst1
			wanPort = e.Dport1
			bytesOut = e.Bytes1
			bytesIn = e.Bytes2
		}

		key := GenerateFlowKey(e.Proto, lanIP, lanPort, wanIP, wanPort)
		seenKeys[key] = true

		state, exists := dt.flows[key]
		if !exists {
			state = &FlowState{
				LastBytesIn:  bytesIn,
				LastBytesOut: bytesOut,
				LastSeen:     now,
			}
			dt.flows[key] = state
			continue
		}

		deltaIn := bytesIn - state.LastBytesIn
		deltaOut := bytesOut - state.LastBytesOut
		if deltaIn < 0 {
			deltaIn = 0
		}
		if deltaOut < 0 {
			deltaOut = 0
		}

		// 若之前记录为 0 且当前值较大（如刚开启内核流量统计的已有长连接），重新对齐基线，避免产生假峰值
		if (state.LastBytesIn == 0 && bytesIn > 2*1024*1024) || (state.LastBytesOut == 0 && bytesOut > 2*1024*1024) {
			state.LastBytesIn = bytesIn
			state.LastBytesOut = bytesOut
			state.LastSeen = now
			continue
		}

		state.LastBytesIn = bytesIn
		state.LastBytesOut = bytesOut
		state.LastSeen = now

		if deltaIn > 0 || deltaOut > 0 {
			totalDeltaIn += deltaIn
			totalDeltaOut += deltaOut

			flowRecord := &model.FlowRecord{
				ID:        key,
				NodeID:    nodeID,
				Timestamp: now,
				Protocol:  e.Proto,
				SrcIP:     lanIP,
				SrcPort:   lanPort,
				DstIP:     wanIP,
				DstPort:   wanPort,
				BytesIn:   deltaIn,
				BytesOut:  deltaOut,
				Packets:   (e.Packets1 + e.Packets2),
				State:     e.State,
			}
			activeFlows = append(activeFlows, flowRecord)

			if lanIP != "" {
				dev, ok := dt.deviceTotals[lanIP]
				if !ok {
					dev = &model.DeviceStats{
						IP:         lanIP,
						Name:       GuessDeviceName(lanIP),
						Category:   GuessDeviceCategory(lanIP),
						LastActive: now.Unix(),
					}
					dt.deviceTotals[lanIP] = dev
				}
				dev.TotalIn += deltaIn
				dev.TotalOut += deltaOut
				dev.RateInBps += float64(deltaIn) / elapsed
				dev.RateOutBps += float64(deltaOut) / elapsed
				dev.LastActive = now.Unix()
				dev.ConnCount++
			}
		}
	}

	for k, v := range dt.flows {
		if !seenKeys[k] && now.Sub(v.LastSeen) > 60*time.Second {
			delete(dt.flows, k)
		}
	}

	var devList []*model.DeviceStats
	for _, dev := range dt.deviceTotals {
		devList = append(devList, dev)
	}

	payload := &model.NodeMetricsPayload{
		NodeID:        nodeID,
		Timestamp:     now.Unix(),
		Interval:      elapsed,
		Flows:         activeFlows,
		TotalBytesIn:  totalDeltaIn,
		TotalBytesOut: totalDeltaOut,
		RateInBps:     float64(totalDeltaIn) / elapsed,
		RateOutBps:    float64(totalDeltaOut) / elapsed,
		ActiveConns:   len(entries),
	}

	return payload, devList
}
