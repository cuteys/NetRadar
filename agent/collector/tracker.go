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

// ProcessConntrack 处理连接跟踪原始条目并计算流量增量
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

	// 重置终端当前周期速率
	for _, dev := range dt.deviceTotals {
		dev.RateInBps = 0
		dev.RateOutBps = 0
	}

	seenKeys := make(map[string]bool)

	for _, e := range entries {
		srcIsPrivate := IsPrivateIP(e.Src1)
		dstIsPrivate := IsPrivateIP(e.Dst1)

		// 忽略纯局域网互访
		if srcIsPrivate && dstIsPrivate {
			continue
		}

		var lanIP, wanIP string
		var lanPort, wanPort int
		var bytesOut, bytesIn int64 // 发送与接收字节数

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
			// 网关本身对外通信
			lanIP = "Gateway"
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
			// 新连接首次记录基准值，不计算全量历史差额
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

		state.LastBytesIn = bytesIn
		state.LastBytesOut = bytesOut
		state.LastSeen = now

		// 仅在当前周期有流量增量时记录
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

			// 统计局域网终端流量
			if lanIP != "Gateway" {
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

	// 清理超过60秒无活动的流
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
