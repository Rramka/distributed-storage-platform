package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const nodeCols = `
	id, owner_id, cert_fingerprint, public_key, cert_pem, cert_expires_at,
	hostname_label, os, agent_version, country, region, asn, endpoint,
	capacity_bytes, used_bytes, status, reputation, registered_at, last_seen_at`

func scanNode(row interface{ Scan(dest ...any) error }) (Node, error) {
	var n Node
	var label, country, region *string
	err := row.Scan(
		&n.ID, &n.OwnerID, &n.CertFingerprint, &n.PublicKey, &n.CertPEM, &n.CertExpiresAt,
		&label, &n.OS, &n.AgentVersion, &country, &region, &n.ASN, &n.Endpoint,
		&n.CapacityBytes, &n.UsedBytes, &n.Status, &n.Reputation, &n.RegisteredAt, &n.LastSeenAt,
	)
	if label != nil {
		n.HostnameLabel = *label
	}
	if country != nil {
		n.Country = *country
	}
	if region != nil {
		n.Region = *region
	}
	return n, err
}

// CreateNodeParams is the insert set for a newly issued node.
type CreateNodeParams struct {
	ID              uuid.UUID
	OwnerID         uuid.UUID
	CertFingerprint []byte
	PublicKey       []byte
	CertPEM         string
	CertExpiresAt   time.Time
	HostnameLabel   string
	OS              string
	AgentVersion    string
	Country         string
	Region          string
	ASN             *int
	Endpoint        string
	CapacityBytes   int64
}

// CreateNode inserts a pending node.
func (s *Store) CreateNode(ctx context.Context, p CreateNodeParams) (Node, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	n, err := scanNode(s.pool.QueryRow(ctx, `
		INSERT INTO nodes (
			id, owner_id, cert_fingerprint, public_key, cert_pem, cert_expires_at,
			hostname_label, os, agent_version, country, region, asn, endpoint, capacity_bytes, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'pending')
		RETURNING `+nodeCols, p.ID, p.OwnerID, p.CertFingerprint, p.PublicKey, p.CertPEM, p.CertExpiresAt,
		nullIfEmpty(p.HostnameLabel), p.OS, p.AgentVersion, nullIfEmpty(p.Country), nullIfEmpty(p.Region), p.ASN, p.Endpoint, p.CapacityBytes,
	))
	if err != nil {
		return Node{}, mapQueryErr("store.createNode", err)
	}
	return n, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// NodeByID loads a node.
func (s *Store) NodeByID(ctx context.Context, id uuid.UUID) (Node, error) {
	n, err := scanNode(s.pool.QueryRow(ctx, `SELECT `+nodeCols+` FROM nodes WHERE id = $1`, id))
	if err != nil {
		return Node{}, mapQueryErr("store.nodeByID", err)
	}
	return n, nil
}

// NodeByFingerprint loads a node by cert fingerprint.
func (s *Store) NodeByFingerprint(ctx context.Context, fp []byte) (Node, error) {
	n, err := scanNode(s.pool.QueryRow(ctx, `SELECT `+nodeCols+` FROM nodes WHERE cert_fingerprint = $1`, fp))
	if err != nil {
		return Node{}, mapQueryErr("store.nodeByFingerprint", err)
	}
	return n, nil
}

// ListNodesByOwner returns the provider's nodes.
func (s *Store) ListNodesByOwner(ctx context.Context, ownerID uuid.UUID) ([]Node, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+nodeCols+` FROM nodes WHERE owner_id = $1 ORDER BY registered_at`, ownerID)
	if err != nil {
		return nil, mapQueryErr("store.listNodesByOwner", err)
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, mapQueryErr("store.listNodesByOwner", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ListOnlineNodes returns nodes with status=online.
func (s *Store) ListOnlineNodes(ctx context.Context) ([]Node, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+nodeCols+` FROM nodes WHERE status = 'online' ORDER BY id`)
	if err != nil {
		return nil, mapQueryErr("store.listOnlineNodes", err)
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, mapQueryErr("store.listOnlineNodes", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// TouchNode updates liveness fields and flips pending/suspect/offline → online.
func (s *Store) TouchNode(ctx context.Context, id uuid.UUID, usedBytes, capacityBytes int64) error {
	_, _, err := s.TouchNodeStatus(ctx, id, usedBytes, capacityBytes)
	return err
}

// TouchNodeStatus is TouchNode plus the previous and new status (for NATS events).
func (s *Store) TouchNodeStatus(ctx context.Context, id uuid.UUID, usedBytes, capacityBytes int64) (from, to string, err error) {
	err = s.pool.QueryRow(ctx, `
		WITH old AS (
			SELECT status FROM nodes WHERE id = $1
		)
		UPDATE nodes
		SET last_seen_at = now(),
		    used_bytes = $2::bigint,
		    capacity_bytes = CASE WHEN $3::bigint > 0 THEN $3::bigint ELSE capacity_bytes END,
		    status = CASE
				WHEN status IN ('pending', 'suspect', 'offline') THEN 'online'
				ELSE status
			END
		WHERE id = $1
		RETURNING (SELECT status FROM old), status
	`, id, usedBytes, capacityBytes).Scan(&from, &to)
	if err != nil {
		return "", "", mapQueryErr("store.touchNode", err)
	}
	return from, to, nil
}

// NodeTransition is one status change from TransitionStaleNodes.
type NodeTransition struct {
	ID   uuid.UUID
	From string
	To   string
}

// TransitionStaleNodes flips online→suspect after suspectAfter and
// online/suspect→offline after offlineAfter. Only those two live states.
func (s *Store) TransitionStaleNodes(ctx context.Context, suspectAfter, offlineAfter time.Duration) ([]NodeTransition, error) {
	suspectSecs := int(suspectAfter.Seconds())
	offlineSecs := int(offlineAfter.Seconds())
	rows, err := s.pool.Query(ctx, `
		WITH prev AS (
			SELECT id, status AS old_status, last_seen_at
			FROM nodes
			WHERE status IN ('online', 'suspect')
			  AND last_seen_at IS NOT NULL
		)
		UPDATE nodes n
		SET status = CASE
			WHEN p.last_seen_at < now() - ($2::int * interval '1 second') THEN 'offline'
			WHEN p.old_status = 'online'
			     AND p.last_seen_at < now() - ($1::int * interval '1 second') THEN 'suspect'
			ELSE n.status
		END
		FROM prev p
		WHERE n.id = p.id
		  AND (
			p.last_seen_at < now() - ($2::int * interval '1 second')
			OR (p.old_status = 'online' AND p.last_seen_at < now() - ($1::int * interval '1 second'))
		  )
		RETURNING n.id, p.old_status, n.status
	`, suspectSecs, offlineSecs)
	if err != nil {
		return nil, mapQueryErr("store.transitionStaleNodes", err)
	}
	defer rows.Close()
	var out []NodeTransition
	for rows.Next() {
		var t NodeTransition
		if err := rows.Scan(&t.ID, &t.From, &t.To); err != nil {
			return nil, mapQueryErr("store.transitionStaleNodes", err)
		}
		if t.From != t.To {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}
