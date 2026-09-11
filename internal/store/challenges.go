package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Challenge is one pre-computed storage-proof tuple. Bytes are never stored.
type Challenge struct {
	FragmentID uuid.UUID
	NodeID     uuid.UUID
	Seq        int
	Offset     int
	Length     int
	Nonce      []byte
	Expected   []byte
}

// AuditTarget is a stored placement due for a challenge.
type AuditTarget struct {
	PlacementID int64
	FragmentID  uuid.UUID
	NodeID      uuid.UUID
	ChunkID     uuid.UUID
	SizeBytes   int
	SHA256      []byte
	Endpoint    string
	Reputation  float32
	Remaining   int
}

const challengeLowWater = 2

// InsertChallengeSet replaces unspent tuples for (fragment, node) with set.
func (s *Store) InsertChallengeSet(ctx context.Context, set []Challenge) error {
	if len(set) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapQueryErr("store.insertChallengeSet", err)
	}
	defer tx.Rollback(ctx)
	fid, nid := set[0].FragmentID, set[0].NodeID
	if _, err := tx.Exec(ctx, `
		DELETE FROM fragment_challenges
		WHERE fragment_id = $1 AND node_id = $2 AND spent_at IS NULL
	`, fid, nid); err != nil {
		return mapQueryErr("store.insertChallengeSet", err)
	}
	for _, c := range set {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fragment_challenges
				(fragment_id, node_id, seq, offset_bytes, length_bytes, nonce, expected)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (fragment_id, node_id, seq) DO UPDATE
			SET offset_bytes = EXCLUDED.offset_bytes,
			    length_bytes = EXCLUDED.length_bytes,
			    nonce = EXCLUDED.nonce,
			    expected = EXCLUDED.expected,
			    spent_at = NULL
		`, c.FragmentID, c.NodeID, c.Seq, c.Offset, c.Length, c.Nonce, c.Expected); err != nil {
			return mapQueryErr("store.insertChallengeSet", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return mapQueryErr("store.insertChallengeSet", err)
	}
	return nil
}

// CountRemainingChallenges returns unspent tuples for a placement.
func (s *Store) CountRemainingChallenges(ctx context.Context, fragmentID, nodeID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM fragment_challenges
		WHERE fragment_id = $1 AND node_id = $2 AND spent_at IS NULL
	`, fragmentID, nodeID).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.countRemainingChallenges", err)
	}
	return n, nil
}

// TakeNextUnspent returns the next unspent challenge without spending it.
func (s *Store) TakeNextUnspent(ctx context.Context, fragmentID, nodeID uuid.UUID) (Challenge, error) {
	var c Challenge
	c.FragmentID = fragmentID
	c.NodeID = nodeID
	err := s.pool.QueryRow(ctx, `
		SELECT seq, offset_bytes, length_bytes, nonce, expected
		FROM fragment_challenges
		WHERE fragment_id = $1 AND node_id = $2 AND spent_at IS NULL
		ORDER BY seq
		LIMIT 1
	`, fragmentID, nodeID).Scan(&c.Seq, &c.Offset, &c.Length, &c.Nonce, &c.Expected)
	if err != nil {
		return Challenge{}, mapQueryErr("store.takeNextUnspent", err)
	}
	return c, nil
}

// MarkChallengeSpent stamps spent_at on one tuple.
func (s *Store) MarkChallengeSpent(ctx context.Context, fragmentID, nodeID uuid.UUID, seq int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE fragment_challenges
		SET spent_at = now()
		WHERE fragment_id = $1 AND node_id = $2 AND seq = $3 AND spent_at IS NULL
	`, fragmentID, nodeID, seq)
	if err != nil {
		return mapQueryErr("store.markChallengeSpent", err)
	}
	return nil
}

// TouchLastAudit records that a placement was challenged.
func (s *Store) TouchLastAudit(ctx context.Context, fragmentID, nodeID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE fragment_placements
		SET last_audit_at = now()
		WHERE fragment_id = $1 AND node_id = $2
	`, fragmentID, nodeID)
	if err != nil {
		return mapQueryErr("store.touchLastAudit", err)
	}
	return nil
}

// MarkPlacementLost flips one stored placement to lost and returns the row.
func (s *Store) MarkPlacementLost(ctx context.Context, fragmentID, nodeID uuid.UUID) (LostPlacement, error) {
	var lp LostPlacement
	err := s.pool.QueryRow(ctx, `
		UPDATE fragment_placements p
		SET status = 'lost', lost_at = now()
		FROM fragments fr
		WHERE p.fragment_id = fr.id
		  AND p.fragment_id = $1
		  AND p.node_id = $2
		  AND p.status = 'stored'
		RETURNING p.id, p.fragment_id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256, p.node_id
	`, fragmentID, nodeID).Scan(
		&lp.PlacementID, &lp.FragmentID, &lp.ChunkID, &lp.ShardIndex, &lp.SizeBytes, &lp.SHA256, &lp.NodeID,
	)
	if err != nil {
		return LostPlacement{}, mapQueryErr("store.markPlacementLost", err)
	}
	return lp, nil
}

// ListDueAudits returns stored placements on online nodes whose last audit is older than minAge.
// Low-reputation nodes are listed first. Remaining is the unspent challenge count.
func (s *Store) ListDueAudits(ctx context.Context, minAge time.Duration, limit int) ([]AuditTarget, error) {
	if limit <= 0 {
		limit = 16
	}
	secs := int(minAge.Seconds())
	if secs < 0 {
		secs = 0
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.fragment_id, p.node_id, fr.chunk_id, fr.size_bytes, fr.sha256,
		       n.endpoint, n.reputation,
		       (SELECT count(*) FROM fragment_challenges c
		        WHERE c.fragment_id = p.fragment_id AND c.node_id = p.node_id AND c.spent_at IS NULL)
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN nodes n ON n.id = p.node_id
		WHERE p.status = 'stored'
		  AND n.status = 'online'
		  AND (p.last_audit_at IS NULL OR p.last_audit_at < now() - ($1::int * interval '1 second'))
		ORDER BY n.reputation ASC, p.last_audit_at NULLS FIRST, p.id
		LIMIT $2
	`, secs, limit)
	if err != nil {
		return nil, mapQueryErr("store.listDueAudits", err)
	}
	defer rows.Close()
	var out []AuditTarget
	for rows.Next() {
		var t AuditTarget
		if err := rows.Scan(
			&t.PlacementID, &t.FragmentID, &t.NodeID, &t.ChunkID, &t.SizeBytes, &t.SHA256,
			&t.Endpoint, &t.Reputation, &t.Remaining,
		); err != nil {
			return nil, mapQueryErr("store.listDueAudits", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ChallengeLowWater is the refill threshold.
func ChallengeLowWater() int { return challengeLowWater }

// LoadPlacementFragment returns fragment bytes-metadata for ticket minting.
func (s *Store) LoadPlacementFragment(ctx context.Context, fragmentID, nodeID uuid.UUID) (Fragment, Node, error) {
	var fr Fragment
	var n Node
	err := s.pool.QueryRow(ctx, `
		SELECT fr.id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256,
		       n.id, n.endpoint, n.public_key, n.status, n.reputation
		FROM fragments fr
		JOIN fragment_placements p ON p.fragment_id = fr.id AND p.node_id = $2
		JOIN nodes n ON n.id = p.node_id
		WHERE fr.id = $1
	`, fragmentID, nodeID).Scan(
		&fr.ID, &fr.ChunkID, &fr.ShardIndex, &fr.SizeBytes, &fr.SHA256,
		&n.ID, &n.Endpoint, &n.PublicKey, &n.Status, &n.Reputation,
	)
	if err != nil {
		return Fragment{}, Node{}, mapQueryErr("store.loadPlacementFragment", err)
	}
	return fr, n, nil
}

// ListAllNodes returns every node (fleet-wide; demo / visualizer).
func (s *Store) ListAllNodes(ctx context.Context) ([]Node, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+nodeCols+` FROM nodes ORDER BY hostname_label NULLS LAST, registered_at`)
	if err != nil {
		return nil, mapQueryErr("store.listAllNodes", err)
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, mapQueryErr("store.listAllNodes", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CommittedChunkHealth returns healthy stored-placement counts for every committed chunk.
func (s *Store) CommittedChunkHealth(ctx context.Context) (map[uuid.UUID]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, count(p.id) FILTER (
			WHERE p.status = 'stored' AND n.status IS NOT NULL AND n.status NOT IN ('offline', 'quarantined')
		)
		FROM chunks c
		JOIN file_versions v ON v.id = c.version_id AND v.status = 'committed'
		JOIN fragments fr ON fr.chunk_id = c.id
		LEFT JOIN fragment_placements p ON p.fragment_id = fr.id
		LEFT JOIN nodes n ON n.id = p.node_id
		GROUP BY c.id
		ORDER BY c.seq, c.id
	`)
	if err != nil {
		return nil, mapQueryErr("store.committedChunkHealth", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, mapQueryErr("store.committedChunkHealth", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}
