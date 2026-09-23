package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NodeStats is one hourly rollup row (node_stats).
type NodeStats struct {
	NodeID        uuid.UUID `json:"node_id"`
	WindowStart   time.Time `json:"window_start"`
	UptimeRatio   float32   `json:"uptime_ratio"`
	AvgLatencyMS  float32   `json:"avg_latency_ms"`
	FreeBytes     int64     `json:"free_bytes"`
	CPULoad       float32   `json:"cpu_load"`
	MemUsedRatio  float32   `json:"mem_used_ratio"`
	AuditsPassed  int       `json:"audits_passed"`
	AuditsFailed  int       `json:"audits_failed"`
	BytesServed   int64     `json:"bytes_served"`
	BytesIngested int64     `json:"bytes_ingested"`
}

// UpsertNodeStats writes non-audit fields for the hour; audit counters accumulate.
func (s *Store) UpsertNodeStats(ctx context.Context, row NodeStats) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO node_stats (
			node_id, window_start, uptime_ratio, avg_latency_ms, free_bytes,
			cpu_load, mem_used_ratio, audits_passed, audits_failed, bytes_served, bytes_ingested
		) VALUES ($1,$2,$3,$4,$5,$6,$7,0,0,0,0)
		ON CONFLICT (node_id, window_start) DO UPDATE SET
			uptime_ratio = EXCLUDED.uptime_ratio,
			avg_latency_ms = EXCLUDED.avg_latency_ms,
			free_bytes = EXCLUDED.free_bytes,
			cpu_load = EXCLUDED.cpu_load,
			mem_used_ratio = EXCLUDED.mem_used_ratio
	`, row.NodeID, row.WindowStart, row.UptimeRatio, row.AvgLatencyMS, row.FreeBytes, row.CPULoad, row.MemUsedRatio)
	if err != nil {
		return mapQueryErr("store.upsertNodeStats", err)
	}
	return nil
}

// BumpNodeBytes increments ingest and/or egress counters for the current hour.
func (s *Store) BumpNodeBytes(ctx context.Context, nodeID uuid.UUID, ingested, served int64) error {
	if ingested == 0 && served == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO node_stats (
			node_id, window_start, uptime_ratio, bytes_ingested, bytes_served
		) VALUES ($1, date_trunc('hour', now()), 0, $2, $3)
		ON CONFLICT (node_id, window_start) DO UPDATE SET
			bytes_ingested = node_stats.bytes_ingested + EXCLUDED.bytes_ingested,
			bytes_served = node_stats.bytes_served + EXCLUDED.bytes_served
	`, nodeID, ingested, served)
	if err != nil {
		return mapQueryErr("store.bumpNodeBytes", err)
	}
	return nil
}

// NodeReliability is the 30-day rolling uptime and audit pass rate.
func (s *Store) NodeReliability(ctx context.Context, nodeID uuid.UUID) (uptime30d, auditPass30d float32, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(uptime_ratio), 0),
			CASE WHEN COALESCE(SUM(audits_passed + audits_failed), 0) = 0 THEN 1
			     ELSE SUM(audits_passed)::real / SUM(audits_passed + audits_failed)::real
			END
		FROM node_stats
		WHERE node_id = $1 AND window_start > now() - interval '30 days'
	`, nodeID).Scan(&uptime30d, &auditPass30d)
	if err != nil {
		return 0, 0, mapQueryErr("store.nodeReliability", err)
	}
	return uptime30d, auditPass30d, nil
}

// ListNodeStats returns hourly rollups for a node in [from, to).
func (s *Store) ListNodeStats(ctx context.Context, nodeID uuid.UUID, from, to time.Time) ([]NodeStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT node_id, window_start, uptime_ratio,
			COALESCE(avg_latency_ms, 0), COALESCE(free_bytes, 0),
			COALESCE(cpu_load, 0), COALESCE(mem_used_ratio, 0),
			audits_passed, audits_failed, bytes_served, bytes_ingested
		FROM node_stats
		WHERE node_id = $1 AND window_start >= $2 AND window_start < $3
		ORDER BY window_start
	`, nodeID, from, to)
	if err != nil {
		return nil, mapQueryErr("store.listNodeStats", err)
	}
	defer rows.Close()
	var out []NodeStats
	for rows.Next() {
		var r NodeStats
		if err := rows.Scan(
			&r.NodeID, &r.WindowStart, &r.UptimeRatio, &r.AvgLatencyMS, &r.FreeBytes,
			&r.CPULoad, &r.MemUsedRatio, &r.AuditsPassed, &r.AuditsFailed, &r.BytesServed, &r.BytesIngested,
		); err != nil {
			return nil, mapQueryErr("store.listNodeStats", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BumpNodeAudit increments the current hour's pass or fail counter.
func (s *Store) BumpNodeAudit(ctx context.Context, nodeID uuid.UUID, passed bool) error {
	var p, f int
	if passed {
		p = 1
	} else {
		f = 1
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO node_stats (
			node_id, window_start, uptime_ratio, audits_passed, audits_failed
		) VALUES ($1, date_trunc('hour', now()), 1, $2, $3)
		ON CONFLICT (node_id, window_start) DO UPDATE SET
			audits_passed = node_stats.audits_passed + EXCLUDED.audits_passed,
			audits_failed = node_stats.audits_failed + EXCLUDED.audits_failed
	`, nodeID, p, f)
	if err != nil {
		return mapQueryErr("store.bumpNodeAudit", err)
	}
	return nil
}

// LatestNodeStats returns the most recent rollup per node.
func (s *Store) LatestNodeStats(ctx context.Context) (map[uuid.UUID]NodeStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (node_id)
			node_id, window_start, uptime_ratio,
			COALESCE(avg_latency_ms, 0), COALESCE(free_bytes, 0),
			COALESCE(cpu_load, 0), COALESCE(mem_used_ratio, 0),
			audits_passed, audits_failed, bytes_served, bytes_ingested
		FROM node_stats
		ORDER BY node_id, window_start DESC
	`)
	if err != nil {
		return nil, mapQueryErr("store.latestNodeStats", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]NodeStats{}
	for rows.Next() {
		var r NodeStats
		if err := rows.Scan(
			&r.NodeID, &r.WindowStart, &r.UptimeRatio, &r.AvgLatencyMS, &r.FreeBytes,
			&r.CPULoad, &r.MemUsedRatio, &r.AuditsPassed, &r.AuditsFailed, &r.BytesServed, &r.BytesIngested,
		); err != nil {
			return nil, mapQueryErr("store.latestNodeStats", err)
		}
		out[r.NodeID] = r
	}
	return out, rows.Err()
}

// CountLivePlacements returns pending+stored placements on a node (probation quota).
func (s *Store) CountLivePlacements(ctx context.Context, nodeID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM fragment_placements
		WHERE node_id = $1 AND status IN ('pending', 'stored')
	`, nodeID).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.countLivePlacements", err)
	}
	return n, nil
}

// CountLivePlacementsAll returns pending+stored counts keyed by node.
func (s *Store) CountLivePlacementsAll(ctx context.Context) (map[uuid.UUID]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT node_id, count(*) FROM fragment_placements
		WHERE status IN ('pending', 'stored')
		GROUP BY node_id
	`)
	if err != nil {
		return nil, mapQueryErr("store.countLivePlacementsAll", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, mapQueryErr("store.countLivePlacementsAll", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}
