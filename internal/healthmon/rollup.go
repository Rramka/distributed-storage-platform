package healthmon

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const rollupTick = time.Hour

// RunRollup writes hourly node_stats from Redis heartbeat metrics until ctx is cancelled.
func RunRollup(ctx context.Context, st *store.Store, rdb *redis.Client) {
	t := time.NewTicker(rollupTick)
	defer t.Stop()
	rollupOnce(ctx, st, rdb)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rollupOnce(ctx, st, rdb)
		}
	}
}

func rollupOnce(ctx context.Context, st *store.Store, rdb *redis.Client) {
	if st == nil {
		return
	}
	nodes, err := st.ListOnlineNodes(ctx)
	if err != nil {
		slog.Error("healthmon rollup list", "err", err)
		return
	}
	window := time.Now().UTC().Truncate(time.Hour)
	for _, n := range nodes {
		row := store.NodeStats{
			NodeID:      n.ID,
			WindowStart: window,
			UptimeRatio: 1,
			FreeBytes:   n.CapacityBytes - n.UsedBytes,
		}
		if rdb != nil {
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
		if err := st.UpsertNodeStats(ctx, row); err != nil {
			slog.Error("healthmon rollup", "err", err, "node_id", n.ID)
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
