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
	lastTickTime time.Time
}

func NewDeltaTracker() *DeltaTracker {
	return &DeltaTracker{
		flows:        make(map[string]*FlowState),
		lastTickTime: time.Now(),
	}
}

// ProcessConntrack 计算增量流指标，直接生成上报载荷
func (dt *DeltaTracker) ProcessConntrack(nodeID string, entries []*RawConntrackEntry) *model.NodeMetricsPayload {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(dt.lastTickTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1.0
	}
	dt.lastTickTime = now

	activeFlows := make([]*model.FlowRecord, 0, 32)
	var totalDeltaIn int64
	var totalDeltaOut int64

	seenKeys := make(map[string]bool, len(entries))

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

		// 针对刚开启统计的长连接基线校准
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
		}
	}

	// 清理超过 60 秒无活跃的连接状态
	for k, v := range dt.flows {
		if !seenKeys[k] && now.Sub(v.LastSeen) > 60*time.Second {
			delete(dt.flows, k)
		}
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

	return payload
}
