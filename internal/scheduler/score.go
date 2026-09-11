package scheduler

import "github.com/Rramka/distributed-storage-platform/internal/store"

// Scoring weights from docs/06-scheduler-and-repair.md.
const (
	WCap = 0.20
	WUp  = 0.30
	WRep = 0.30
	WLat = 0.10
	WBw  = 0.10
	// BandwidthFactor is a documented neutral constant until throughput is measured.
	BandwidthFactor = 1.0
	ProbationQuota  = 2
)

func nodeScore(n store.Node) float64 {
	var cap float64
	if n.CapacityBytes > 0 {
		cap = float64(n.CapacityBytes-n.UsedBytes) / float64(n.CapacityBytes)
		if cap < 0 {
			cap = 0
		}
		if cap > 1 {
			cap = 1
		}
	}
	up := float64(n.UptimeRatio)
	if up <= 0 {
		up = 0.5
	}
	if up > 1 {
		up = 1
	}
	rep := float64(n.Reputation)
	if rep < 0 {
		rep = 0
	}
	if rep > 1 {
		rep = 1
	}
	lat := latencyFactor(n.LatencyMS)
	pressure := float64(n.CPULoad)*0.5 + float64(n.MemUsedRatio)*0.5
	if pressure < 0 {
		pressure = 0
	}
	if pressure > 1 {
		pressure = 1
	}
	s := WCap*cap + WUp*up + WRep*rep + WLat*lat + WBw*BandwidthFactor - pressure
	if s < 1e-6 {
		s = 1e-6
	}
	return s
}

func latencyFactor(ms float32) float64 {
	if ms <= 0 {
		return 1
	}
	return 1 / (1 + float64(ms)/50)
}
