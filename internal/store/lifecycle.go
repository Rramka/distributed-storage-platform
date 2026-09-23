package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ExpiringPlacement is a fragment the lifecycle reaper should delete on a node.
type ExpiringPlacement struct {
	ID         int64
	FragmentID uuid.UUID
	NodeID     uuid.UUID
	Endpoint   string
	SHA256     []byte
	SizeBytes  int
}

// ListExpiringPlacements returns placements waiting for agent DELETE confirmation.
func (s *Store) ListExpiringPlacements(ctx context.Context, limit int) ([]ExpiringPlacement, error) {
	if limit <= 0 {
		limit = 64
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.fragment_id, p.node_id, n.endpoint, fr.sha256, fr.size_bytes
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN nodes n ON n.id = p.node_id
		WHERE p.status = 'expiring'
		ORDER BY p.id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, mapQueryErr("store.listExpiringPlacements", err)
	}
	defer rows.Close()
	var out []ExpiringPlacement
	for rows.Next() {
		var r ExpiringPlacement
		if err := rows.Scan(&r.ID, &r.FragmentID, &r.NodeID, &r.Endpoint, &r.SHA256, &r.SizeBytes); err != nil {
			return nil, mapQueryErr("store.listExpiringPlacements", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExpiringPlacementByID loads one expiring placement.
func (s *Store) ExpiringPlacementByID(ctx context.Context, id int64) (ExpiringPlacement, error) {
	var r ExpiringPlacement
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, p.fragment_id, p.node_id, n.endpoint, fr.sha256, fr.size_bytes
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN nodes n ON n.id = p.node_id
		WHERE p.id = $1 AND p.status = 'expiring'
	`, id).Scan(&r.ID, &r.FragmentID, &r.NodeID, &r.Endpoint, &r.SHA256, &r.SizeBytes)
	if err != nil {
		return ExpiringPlacement{}, mapQueryErr("store.expiringPlacementByID", err)
	}
	return r, nil
}

// MarkPlacementDeleted records that the node confirmed fragment removal.
func (s *Store) MarkPlacementDeleted(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE fragment_placements SET status = 'deleted'
		WHERE id = $1 AND status = 'expiring'
	`, id)
	if err != nil {
		return mapQueryErr("store.markPlacementDeleted", err)
	}
	return nil
}

// InsertDownloadTicket records a GET ticket nonce issued for a file.
func (s *Store) InsertDownloadTicket(ctx context.Context, nonce []byte, fileID, fragmentID, nodeID uuid.UUID, sizeBytes int64, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO download_tickets (nonce, file_id, fragment_id, node_id, size_bytes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (nonce) DO NOTHING
	`, nonce, fileID, fragmentID, nodeID, sizeBytes, expiresAt)
	if err != nil {
		return mapQueryErr("store.insertDownloadTicket", err)
	}
	return nil
}

// DownloadTicket is a GET ticket waiting for an egress receipt.
type DownloadTicket struct {
	Nonce      []byte
	FileID     uuid.UUID
	FragmentID uuid.UUID
	NodeID     uuid.UUID
	SizeBytes  int64
	ExpiresAt  time.Time
	PublicKey  []byte
}

// DownloadTicketByNonce loads an unused GET ticket for this file.
func (s *Store) DownloadTicketByNonce(ctx context.Context, ownerID, fileID uuid.UUID, nonce []byte) (DownloadTicket, error) {
	var t DownloadTicket
	err := s.pool.QueryRow(ctx, `
		SELECT t.nonce, t.file_id, t.fragment_id, t.node_id, t.size_bytes, t.expires_at, n.public_key
		FROM download_tickets t
		JOIN files f ON f.id = t.file_id
		JOIN buckets b ON b.id = f.bucket_id
		JOIN nodes n ON n.id = t.node_id
		WHERE t.nonce = $1 AND t.file_id = $2 AND b.owner_id = $3
	`, nonce, fileID, ownerID).Scan(&t.Nonce, &t.FileID, &t.FragmentID, &t.NodeID, &t.SizeBytes, &t.ExpiresAt, &t.PublicKey)
	if err != nil {
		return DownloadTicket{}, mapQueryErr("store.downloadTicketByNonce", err)
	}
	return t, nil
}

// ConsumeDownloadReceipt records a verified egress receipt. Duplicate nonce is ErrConflict.
func (s *Store) ConsumeDownloadReceipt(ctx context.Context, nonce []byte, fileID, fragmentID, nodeID uuid.UUID, bytes int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO download_receipts (nonce, file_id, fragment_id, node_id, bytes)
		VALUES ($1, $2, $3, $4, $5)
	`, nonce, fileID, fragmentID, nodeID, bytes)
	if err != nil {
		return mapQueryErr("store.consumeDownloadReceipt", err)
	}
	return nil
}
