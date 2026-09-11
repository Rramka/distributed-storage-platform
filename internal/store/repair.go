package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LostPlacement is a fragment that was on a failed node.
type LostPlacement struct {
	PlacementID            int64
	FragmentID             uuid.UUID
	ChunkID                uuid.UUID
	ShardIndex             int16
	SizeBytes              int
	SHA256                 []byte
	NodeID                 uuid.UUID
	Endpoint               string
	PublicKey              []byte
	ReconstructedElsewhere bool
}

// MarkPlacementsLostByNode flips stored placements on nodeID to lost.
func (s *Store) MarkPlacementsLostByNode(ctx context.Context, nodeID uuid.UUID) ([]LostPlacement, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE fragment_placements p
		SET status = 'lost', lost_at = now()
		FROM fragments fr
		WHERE p.fragment_id = fr.id
		  AND p.node_id = $1
		  AND p.status = 'stored'
		RETURNING p.id, p.fragment_id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256, p.node_id
	`, nodeID)
	if err != nil {
		return nil, mapQueryErr("store.markPlacementsLostByNode", err)
	}
	defer rows.Close()
	var out []LostPlacement
	for rows.Next() {
		var lp LostPlacement
		if err := rows.Scan(&lp.PlacementID, &lp.FragmentID, &lp.ChunkID, &lp.ShardIndex, &lp.SizeBytes, &lp.SHA256, &lp.NodeID); err != nil {
			return nil, mapQueryErr("store.markPlacementsLostByNode", err)
		}
		out = append(out, lp)
	}
	return out, rows.Err()
}

// ChunkHealth is the count of stored placements for a chunk.
func (s *Store) ChunkHealth(ctx context.Context, chunkID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		WHERE fr.chunk_id = $1 AND p.status = 'stored'
	`, chunkID).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.chunkHealth", err)
	}
	return n, nil
}

// Occupant is a node that already holds a fragment of a chunk.
type Occupant struct {
	ChunkID uuid.UUID
	Node    Node
}

// ListOccupants returns pending/stored holders for the given chunks.
func (s *Store) ListOccupants(ctx context.Context, chunkIDs []uuid.UUID) ([]Occupant, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT fr.chunk_id, n.id, n.owner_id, n.cert_fingerprint, n.public_key, n.cert_pem, n.cert_expires_at,
			n.hostname_label, n.os, n.agent_version, n.country, n.region, n.asn, n.endpoint,
			n.capacity_bytes, n.used_bytes, n.status, n.reputation, n.registered_at, n.last_seen_at,
			n.probation_until, n.reputation_updated_at
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN nodes n ON n.id = p.node_id
		WHERE fr.chunk_id = ANY($1) AND p.status IN ('pending', 'stored')
	`, chunkIDs)
	if err != nil {
		return nil, mapQueryErr("store.listOccupants", err)
	}
	defer rows.Close()
	var out []Occupant
	for rows.Next() {
		var o Occupant
		var label, country, region *string
		err := rows.Scan(
			&o.ChunkID,
			&o.Node.ID, &o.Node.OwnerID, &o.Node.CertFingerprint, &o.Node.PublicKey, &o.Node.CertPEM, &o.Node.CertExpiresAt,
			&label, &o.Node.OS, &o.Node.AgentVersion, &country, &region, &o.Node.ASN, &o.Node.Endpoint,
			&o.Node.CapacityBytes, &o.Node.UsedBytes, &o.Node.Status, &o.Node.Reputation, &o.Node.RegisteredAt, &o.Node.LastSeenAt,
			&o.Node.ProbationUntil, &o.Node.ReputationUpdatedAt,
		)
		if err != nil {
			return nil, mapQueryErr("store.listOccupants", err)
		}
		if label != nil {
			o.Node.HostnameLabel = *label
		}
		if country != nil {
			o.Node.Country = *country
		}
		if region != nil {
			o.Node.Region = *region
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// RepairFragment is one shard of a committed chunk plus optional live placement.
type RepairFragment struct {
	Fragment  Fragment
	Placement *Placement
	Node      *Node
}

// LoadRepairChunk loads a committed chunk's fragments and live placements.
func (s *Store) LoadRepairChunk(ctx context.Context, chunkID uuid.UUID) (Chunk, []RepairFragment, error) {
	var ch Chunk
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.version_id, c.seq, c.size_bytes, c.sha256
		FROM chunks c
		JOIN file_versions v ON v.id = c.version_id
		WHERE c.id = $1 AND v.status = 'committed'
	`, chunkID).Scan(&ch.ID, &ch.VersionID, &ch.Seq, &ch.SizeBytes, &ch.SHA256)
	if err != nil {
		return Chunk{}, nil, mapQueryErr("store.loadRepairChunk", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (fr.id)
		       fr.id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256,
		       p.id, p.node_id, p.status, p.stored_at,
		       n.id, n.endpoint, n.public_key, n.status
		FROM fragments fr
		LEFT JOIN fragment_placements p ON p.fragment_id = fr.id AND p.status IN ('pending', 'stored')
		LEFT JOIN nodes n ON n.id = p.node_id
		WHERE fr.chunk_id = $1
		ORDER BY fr.id,
		         CASE p.status WHEN 'stored' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END,
		         p.id
	`, chunkID)
	if err != nil {
		return Chunk{}, nil, mapQueryErr("store.loadRepairChunk", err)
	}
	defer rows.Close()
	var out []RepairFragment
	for rows.Next() {
		var rf RepairFragment
		var plID *int64
		var plNode *uuid.UUID
		var plStatus *string
		var plStored *time.Time
		var nID *uuid.UUID
		var endpoint *string
		var pub []byte
		var nStatus *string
		if err := rows.Scan(
			&rf.Fragment.ID, &rf.Fragment.ChunkID, &rf.Fragment.ShardIndex, &rf.Fragment.SizeBytes, &rf.Fragment.SHA256,
			&plID, &plNode, &plStatus, &plStored,
			&nID, &endpoint, &pub, &nStatus,
		); err != nil {
			return Chunk{}, nil, mapQueryErr("store.loadRepairChunk", err)
		}
		if plID != nil && plNode != nil && plStatus != nil {
			pl := Placement{ID: *plID, FragmentID: rf.Fragment.ID, NodeID: *plNode, Status: *plStatus, StoredAt: plStored}
			rf.Placement = &pl
			if nID != nil {
				n := Node{ID: *nID, PublicKey: pub}
				if endpoint != nil {
					n.Endpoint = *endpoint
				}
				if nStatus != nil {
					n.Status = *nStatus
				}
				rf.Node = &n
			}
		}
		out = append(out, rf)
	}
	return ch, out, rows.Err()
}

// PendingTargetsForChunk lists pending placements of a chunk for receipt verify.
func (s *Store) PendingTargetsForChunk(ctx context.Context, chunkID uuid.UUID) ([]PlacementTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.fragment_id, p.node_id, p.status, p.stored_at,
		       fr.id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256,
		       n.id, n.public_key, n.endpoint
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN nodes n ON n.id = p.node_id
		WHERE fr.chunk_id = $1 AND p.status = 'pending'
	`, chunkID)
	if err != nil {
		return nil, mapQueryErr("store.pendingTargetsForChunk", err)
	}
	defer rows.Close()
	var out []PlacementTarget
	for rows.Next() {
		var t PlacementTarget
		if err := rows.Scan(
			&t.Placement.ID, &t.Placement.FragmentID, &t.Placement.NodeID, &t.Placement.Status, &t.Placement.StoredAt,
			&t.Fragment.ID, &t.Fragment.ChunkID, &t.Fragment.ShardIndex, &t.Fragment.SizeBytes, &t.Fragment.SHA256,
			&t.Node.ID, &t.Node.PublicKey, &t.Node.Endpoint,
		); err != nil {
			return nil, mapQueryErr("store.pendingTargetsForChunk", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CommitRepairPlacements marks listed pending placements stored.
func (s *Store) CommitRepairPlacements(ctx context.Context, fragmentIDs []uuid.UUID) error {
	if len(fragmentIDs) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapQueryErr("store.commitRepairPlacements", err)
	}
	defer tx.Rollback(ctx)
	for _, fid := range fragmentIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE fragment_placements
			SET status = 'stored', stored_at = now()
			WHERE fragment_id = $1 AND status = 'pending'
		`, fid); err != nil {
			return mapQueryErr("store.commitRepairPlacements", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return mapQueryErr("store.commitRepairPlacements", err)
	}
	return nil
}

// ListLostOnNode returns lost placements still attributed to nodeID.
func (s *Store) ListLostOnNode(ctx context.Context, nodeID uuid.UUID) ([]LostPlacement, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.fragment_id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256,
		       p.node_id, n.endpoint, n.public_key,
		       EXISTS (
		           SELECT 1 FROM fragment_placements p2
		           WHERE p2.fragment_id = p.fragment_id
		             AND p2.status = 'stored'
		             AND p2.node_id <> p.node_id
		       )
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN nodes n ON n.id = p.node_id
		WHERE p.node_id = $1 AND p.status = 'lost'
	`, nodeID)
	if err != nil {
		return nil, mapQueryErr("store.listLostOnNode", err)
	}
	defer rows.Close()
	var out []LostPlacement
	for rows.Next() {
		var lp LostPlacement
		if err := rows.Scan(
			&lp.PlacementID, &lp.FragmentID, &lp.ChunkID, &lp.ShardIndex, &lp.SizeBytes, &lp.SHA256,
			&lp.NodeID, &lp.Endpoint, &lp.PublicKey, &lp.ReconstructedElsewhere,
		); err != nil {
			return nil, mapQueryErr("store.listLostOnNode", err)
		}
		out = append(out, lp)
	}
	return out, rows.Err()
}

// RestorePlacements flips lost rows back to stored after flap re-validation.
func (s *Store) RestorePlacements(ctx context.Context, ids []int64) error {
	return s.updatePlacementIDs(ctx, ids, `
		UPDATE fragment_placements
		SET status = 'stored', stored_at = now(), lost_at = NULL
		WHERE id = ANY($1) AND status = 'lost'
	`, "store.restorePlacements")
}

// ExpirePlacements marks surplus copies expiring after reconstruction elsewhere.
func (s *Store) ExpirePlacements(ctx context.Context, ids []int64) error {
	return s.updatePlacementIDs(ctx, ids, `
		UPDATE fragment_placements
		SET status = 'expiring'
		WHERE id = ANY($1) AND status = 'lost'
	`, "store.expirePlacements")
}

func (s *Store) updatePlacementIDs(ctx context.Context, ids []int64, q, op string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, q, ids)
	if err != nil {
		return mapQueryErr(op, err)
	}
	return nil
}

// FileHolders returns distinct node endpoints storing a committed file.
func (s *Store) FileHolders(ctx context.Context, fileID uuid.UUID) ([]Node, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT n.id, n.owner_id, n.cert_fingerprint, n.public_key, n.cert_pem, n.cert_expires_at,
			n.hostname_label, n.os, n.agent_version, n.country, n.region, n.asn, n.endpoint,
			n.capacity_bytes, n.used_bytes, n.status, n.reputation, n.registered_at, n.last_seen_at,
			n.probation_until, n.reputation_updated_at
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN chunks c ON c.id = fr.chunk_id
		JOIN file_versions v ON v.id = c.version_id
		JOIN files f ON f.id = v.file_id
		JOIN nodes n ON n.id = p.node_id
		WHERE f.id = $1 AND v.status = 'committed' AND p.status = 'stored'
	`, fileID)
	if err != nil {
		return nil, mapQueryErr("store.fileHolders", err)
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, mapQueryErr("store.fileHolders", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ChunkHealthByFile returns stored-placement counts per chunk of a committed file.
func (s *Store) ChunkHealthByFile(ctx context.Context, fileID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, count(p.id) FILTER (WHERE p.status = 'stored')
		FROM chunks c
		JOIN file_versions v ON v.id = c.version_id
		JOIN files f ON f.id = v.file_id
		JOIN fragments fr ON fr.chunk_id = c.id
		LEFT JOIN fragment_placements p ON p.fragment_id = fr.id
		WHERE f.id = $1 AND v.status = 'committed'
		GROUP BY c.id
	`, fileID)
	if err != nil {
		return nil, mapQueryErr("store.chunkHealthByFile", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, mapQueryErr("store.chunkHealthByFile", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}

// UnderHealthyChunks returns committed chunks with fewer than min stored placements
// on nodes that are not offline or quarantined.
func (s *Store) UnderHealthyChunks(ctx context.Context, min int) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id
		FROM chunks c
		JOIN file_versions v ON v.id = c.version_id AND v.status = 'committed'
		JOIN fragments fr ON fr.chunk_id = c.id
		JOIN fragment_placements p ON p.fragment_id = fr.id AND p.status = 'stored'
		JOIN nodes n ON n.id = p.node_id AND n.status NOT IN ('offline', 'quarantined')
		GROUP BY c.id
		HAVING count(*) < $1
	`, min)
	if err != nil {
		return nil, mapQueryErr("store.underHealthyChunks", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapQueryErr("store.underHealthyChunks", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ChunkCapViolations reports chunks whose stored placements break hard caps.
func (s *Store) ChunkCapViolations(ctx context.Context, regionCap, asnCap, ownerCap int) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		WITH live AS (
			SELECT fr.chunk_id, p.node_id, n.region, n.asn, n.owner_id
			FROM fragment_placements p
			JOIN fragments fr ON fr.id = p.fragment_id
			JOIN nodes n ON n.id = p.node_id
			JOIN chunks c ON c.id = fr.chunk_id
			JOIN file_versions v ON v.id = c.version_id AND v.status = 'committed'
			WHERE p.status = 'stored'
		),
		wide AS (
			SELECT chunk_id FROM live GROUP BY chunk_id HAVING count(DISTINCT node_id) >= 10
		)
		SELECT chunk_id FROM (
			SELECT l.chunk_id FROM live l JOIN wide w USING (chunk_id) GROUP BY l.chunk_id, l.node_id HAVING count(*) > 1
			UNION
			SELECT l.chunk_id FROM live l JOIN wide w USING (chunk_id) WHERE l.region <> '' GROUP BY l.chunk_id, l.region HAVING count(*) > $1
			UNION
			SELECT l.chunk_id FROM live l JOIN wide w USING (chunk_id) WHERE l.asn IS NOT NULL GROUP BY l.chunk_id, l.asn HAVING count(*) > $2
			UNION
			SELECT l.chunk_id FROM live l JOIN wide w USING (chunk_id) GROUP BY l.chunk_id, l.owner_id HAVING count(*) > $3
		) v
	`, regionCap, asnCap, ownerCap)
	if err != nil {
		return nil, mapQueryErr("store.chunkCapViolations", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapQueryErr("store.chunkCapViolations", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// AnyCommittedSize returns the logical size of one committed file, if any.
func (s *Store) AnyCommittedSize(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `
		SELECT v.size_bytes FROM files f
		JOIN file_versions v ON v.id = f.current_version
		WHERE v.status = 'committed' AND f.deleted_at IS NULL
		LIMIT 1
	`).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.anyCommittedSize", err)
	}
	return n, nil
}

// AnyCommittedFileID returns one committed file, if any.
func (s *Store) AnyCommittedFileID(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT f.id FROM files f
		JOIN file_versions v ON v.id = f.current_version
		WHERE v.status = 'committed' AND f.deleted_at IS NULL
		LIMIT 1
	`).Scan(&id)
	if err != nil {
		return uuid.Nil, mapQueryErr("store.anyCommittedFileID", err)
	}
	return id, nil
}

// AnyCommittedHolders returns holders of any committed file.
func (s *Store) AnyCommittedHolders(ctx context.Context) ([]Node, error) {
	id, err := s.AnyCommittedFileID(ctx)
	if err != nil {
		return nil, err
	}
	return s.FileHolders(ctx, id)
}

// AnyChunkHealth returns stored counts for every committed chunk.
func (s *Store) AnyChunkHealth(ctx context.Context) (map[uuid.UUID]int, error) {
	id, err := s.AnyCommittedFileID(ctx)
	if err != nil {
		return nil, err
	}
	return s.ChunkHealthByFile(ctx, id)
}
func (s *Store) ForceNodeLastSeen(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE nodes SET last_seen_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return mapQueryErr("store.forceNodeLastSeen", err)
	}
	return nil
}

// ForceNodeStatus is a test helper.
func (s *Store) ForceNodeStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE nodes SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return mapQueryErr("store.forceNodeStatus", err)
	}
	return nil
}
