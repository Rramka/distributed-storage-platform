package healthmon

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

const (
	// DefaultSuspectAfter is 3 missed 10s heartbeats (docs/05-node-agent.md).
	DefaultSuspectAfter = 30 * time.Second
	// DefaultOfflineAfter is 5 minutes silent before repair evaluation.
	DefaultOfflineAfter = 5 * time.Minute
	defaultTick         = 5 * time.Second
)

// NextStatus applies the health state machine to one node.
// pending/draining/quarantined/retired are left unchanged.
func NextStatus(current string, lastSeen, now time.Time, suspectAfter, offlineAfter time.Duration) string {
	if lastSeen.IsZero() {
		return current
	}
	silent := now.Sub(lastSeen)
	switch current {
	case "online":
		if silent >= offlineAfter {
			return "offline"
		}
		if silent >= suspectAfter {
			return "suspect"
		}
	case "suspect":
		if silent >= offlineAfter {
			return "offline"
		}
	}
	return current
}

// Monitor scans for stale nodes and publishes NATS transitions.
type Monitor struct {
	Store        *store.Store
	Bus          *events.Bus
	SuspectAfter time.Duration
	OfflineAfter time.Duration
	Tick         time.Duration
}

// DurationsFromEnv reads HEALTH_SUSPECT_AFTER / HEALTH_OFFLINE_AFTER.
func DurationsFromEnv() (suspect, offline time.Duration) {
	suspect = DefaultSuspectAfter
	offline = DefaultOfflineAfter
	if v := os.Getenv("HEALTH_SUSPECT_AFTER"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			suspect = d
		}
	}
	if v := os.Getenv("HEALTH_OFFLINE_AFTER"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			offline = d
		}
	}
	return suspect, offline
}

// Run ticks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	if m.SuspectAfter <= 0 {
		m.SuspectAfter = DefaultSuspectAfter
	}
	if m.OfflineAfter <= 0 {
		m.OfflineAfter = DefaultOfflineAfter
	}
	tick := m.Tick
	if tick <= 0 {
		tick = defaultTick
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	m.scan(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.scan(ctx)
		}
	}
}

func (m *Monitor) scan(ctx context.Context) {
	ts, err := m.Store.TransitionStaleNodes(ctx, m.SuspectAfter, m.OfflineAfter)
	if err != nil {
		slog.Error("healthmon scan", "err", err)
		return
	}
	for _, tr := range ts {
		slog.Info("node status", "node_id", tr.ID, "from", tr.From, "to", tr.To)
		if m.Bus == nil {
			continue
		}
		if err := m.Bus.PublishNode(ctx, events.NodeEvent{
			NodeID: tr.ID,
			From:   tr.From,
			To:     tr.To,
			At:     time.Now().UTC(),
		}); err != nil {
			slog.Error("healthmon publish", "err", err, "node_id", tr.ID)
		}
		if tr.To == "offline" {
			_, _, _ = m.Store.ApplyReputationEvent(ctx, tr.ID, store.SignalUnplannedOffline)
		}
	}
}
