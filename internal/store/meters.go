package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CustomerStored is logical committed bytes for one owner.
type CustomerStored struct {
	OwnerID   uuid.UUID
	SizeBytes int64
}

// NodeHeld is physical stored fragment bytes on one node.
type NodeHeld struct {
	NodeID     uuid.UUID
	OwnerID    uuid.UUID
	SizeBytes  int64
	Reputation float32
}

// ListCustomerStoredBytes sums committed version sizes for live files.
func (s *Store) ListCustomerStoredBytes(ctx context.Context) ([]CustomerStored, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.owner_id, COALESCE(SUM(v.size_bytes), 0)
		FROM file_versions v
		JOIN files f ON f.id = v.file_id AND f.deleted_at IS NULL
		JOIN buckets b ON b.id = f.bucket_id
		WHERE v.status = 'committed'
		GROUP BY b.owner_id
	`)
	if err != nil {
		return nil, mapQueryErr("store.listCustomerStoredBytes", err)
	}
	defer rows.Close()
	var out []CustomerStored
	for rows.Next() {
		var r CustomerStored
		if err := rows.Scan(&r.OwnerID, &r.SizeBytes); err != nil {
			return nil, mapQueryErr("store.listCustomerStoredBytes", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListNodeHeldBytes sums stored fragment sizes per node (billing meter).
func (s *Store) ListNodeHeldBytes(ctx context.Context) ([]NodeHeld, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.owner_id, COALESCE(SUM(fr.size_bytes), 0), n.reputation
		FROM nodes n
		LEFT JOIN fragment_placements p ON p.node_id = n.id AND p.status = 'stored'
		LEFT JOIN fragments fr ON fr.id = p.fragment_id
		GROUP BY n.id, n.owner_id, n.reputation
	`)
	if err != nil {
		return nil, mapQueryErr("store.listNodeHeldBytes", err)
	}
	defer rows.Close()
	var out []NodeHeld
	for rows.Next() {
		var r NodeHeld
		if err := rows.Scan(&r.NodeID, &r.OwnerID, &r.SizeBytes, &r.Reputation); err != nil {
			return nil, mapQueryErr("store.listNodeHeldBytes", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CustomerStoredBytes is the live logical footprint for one user.
func (s *Store) CustomerStoredBytes(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(v.size_bytes), 0)
		FROM file_versions v
		JOIN files f ON f.id = v.file_id AND f.deleted_at IS NULL
		JOIN buckets b ON b.id = f.bucket_id
		WHERE v.status = 'committed' AND b.owner_id = $1
	`, ownerID).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.customerStoredBytes", err)
	}
	return n, nil
}

// CustomerEgress is owner-keyed bytes from download receipts in a window.
type CustomerEgress struct {
	OwnerID uuid.UUID
	Bytes   int64
}

// ListCustomerEgressBytes sums download_receipts in [window, window+1h).
func (s *Store) ListCustomerEgressBytes(ctx context.Context, window time.Time) ([]CustomerEgress, error) {
	start := window.UTC().Truncate(time.Hour)
	end := start.Add(time.Hour)
	rows, err := s.pool.Query(ctx, `
		SELECT b.owner_id, COALESCE(SUM(r.bytes), 0)
		FROM download_receipts r
		JOIN files f ON f.id = r.file_id
		JOIN buckets b ON b.id = f.bucket_id
		WHERE r.created_at >= $1 AND r.created_at < $2
		GROUP BY b.owner_id
	`, start, end)
	if err != nil {
		return nil, mapQueryErr("store.listCustomerEgressBytes", err)
	}
	defer rows.Close()
	var out []CustomerEgress
	for rows.Next() {
		var r CustomerEgress
		if err := rows.Scan(&r.OwnerID, &r.Bytes); err != nil {
			return nil, mapQueryErr("store.listCustomerEgressBytes", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NodeWindowServed is bytes_served from node_stats for one hour.
type NodeWindowServed struct {
	NodeID  uuid.UUID
	OwnerID uuid.UUID
	Bytes   int64
}

// ListNodeWindowServed returns egress meters for the hour.
func (s *Store) ListNodeWindowServed(ctx context.Context, window time.Time) ([]NodeWindowServed, error) {
	start := window.UTC().Truncate(time.Hour)
	rows, err := s.pool.Query(ctx, `
		SELECT s.node_id, n.owner_id, s.bytes_served
		FROM node_stats s
		JOIN nodes n ON n.id = s.node_id
		WHERE s.window_start = $1 AND s.bytes_served > 0
	`, start)
	if err != nil {
		return nil, mapQueryErr("store.listNodeWindowServed", err)
	}
	defer rows.Close()
	var out []NodeWindowServed
	for rows.Next() {
		var r NodeWindowServed
		if err := rows.Scan(&r.NodeID, &r.OwnerID, &r.Bytes); err != nil {
			return nil, mapQueryErr("store.listNodeWindowServed", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
