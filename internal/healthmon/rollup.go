package healthmon

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const rollupTick = time.Hour

// ExpectedHeartbeats is how many heartbeats a live node sends in one rollup window.
func ExpectedHeartbeats(window time.Duration) float64 {
	if window <= 0 {
		window = rollupTick
	}
	return float64(window / HeartbeatInterval)
}

// RunRollup writes hourly node_stats from Redis heartbeat metrics until ctx is cancelled.
func RunRollup(ctx context.Context, st *store.Store, rdb *redis.Client) {
	RunRollupBus(ctx, st, rdb, nil)
}

// RunRollupBus is RunRollup plus optional usage-event emission.
func RunRollupBus(ctx context.Context, st *store.Store, rdb *redis.Client, bus *events.Bus) {
	t := time.NewTicker(rollupTick)
	defer t.Stop()
	rollupOnce(ctx, st, rdb, bus)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rollupOnce(ctx, st, rdb, bus)
		}
	}
}

func rollupOnce(ctx context.Context, st *store.Store, rdb *redis.Client, bus *events.Bus) {
	window := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	RollupWindow(ctx, st, rdb, bus, window)
}

// RollupWindow writes node_stats for every node for the given UTC hour.
func RollupWindow(ctx context.Context, st *store.Store, rdb *redis.Client, bus *events.Bus, window time.Time) {
	if st == nil {
		return
	}
	window = window.UTC().Truncate(time.Hour)
	nodes, err := st.ListAllNodes(ctx)
	if err != nil {
		slog.Error("healthmon rollup list", "err", err)
		return
	}
	expected := ExpectedHeartbeats(rollupTick)
	for _, n := range nodes {
		row := store.NodeStats{
			NodeID:      n.ID,
			WindowStart: window,
			UptimeRatio: 0,
			FreeBytes:   n.CapacityBytes - n.UsedBytes,
		}
		var received int64
		if rdb != nil {
			if v, err := rdb.Get(ctx, HeartbeatCountKey(n.ID, window)).Int64(); err == nil {
				received = v
			}
			m, err := rdb.HGetAll(ctx, "node:metrics:"+n.ID.String()).Result()
			if err == nil && len(m) > 0 {
				row.CPULoad = parseF32(m["cpu_load"])
				row.MemUsedRatio = parseF32(m["mem_used_ratio"])
				row.AvgLatencyMS = parseF32(m["disk_read_latency_ms"])
				if fb, err := strconv.ParseInt(m["free_bytes"], 10, 64); err == nil {
					row.FreeBytes = fb
				}
			}
		}
		if expected > 0 {
			ratio := float32(float64(received) / expected)
			if ratio > 1 {
				ratio = 1
			}
			if ratio < 0 {
				ratio = 0
			}
			row.UptimeRatio = ratio
		}
		if err := st.UpsertNodeStats(ctx, row); err != nil {
			slog.Error("healthmon rollup", "err", err, "node_id", n.ID)
			continue
		}
		if bus != nil {
			if err := bus.PublishUsage(ctx, events.UsageEvent{
				Kind:      events.UsageUptime,
				SubjectID: n.ID,
				Window:    window,
				Payload:   map[string]any{"uptime_ratio": row.UptimeRatio, "heartbeats": received},
			}); err != nil {
				slog.Error("healthmon usage uptime", "err", err, "node_id", n.ID)
			}
		}
	}
}

func parseF32(s string) float32 {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0
	}
	return float32(f)
}

// MetricsForNodes reads Redis heartbeat hashes for scoring.
func MetricsForNodes(ctx context.Context, rdb *redis.Client, ids []uuid.UUID) map[uuid.UUID]store.NodeStats {
	out := map[uuid.UUID]store.NodeStats{}
	if rdb == nil {
		return out
	}
	for _, id := range ids {
		m, err := rdb.HGetAll(ctx, "node:metrics:"+id.String()).Result()
		if err != nil || len(m) == 0 {
			continue
		}
		row := store.NodeStats{NodeID: id, CPULoad: parseF32(m["cpu_load"]), MemUsedRatio: parseF32(m["mem_used_ratio"]), AvgLatencyMS: parseF32(m["disk_read_latency_ms"])}
		if fb, err := strconv.ParseInt(m["free_bytes"], 10, 64); err == nil {
			row.FreeBytes = fb
		}
		out[id] = row
	}
	return out
}
