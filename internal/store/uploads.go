package store

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ManifestFragment is one shard in an upload plan.
type ManifestFragment struct {
	ShardIndex int16
	SizeBytes  int
	SHA256     []byte
}

// ManifestChunk is one encrypted chunk in an upload plan.
type ManifestChunk struct {
	Seq       int
	SizeBytes int
	SHA256    []byte
	Fragments []ManifestFragment
}

// UploadManifest is the client-computed plan body.
type UploadManifest struct {
	BucketID       uuid.UUID
	Path           string
	SizeBytes      int64
	ContentSHA256  []byte
	EncryptionMeta json.RawMessage
	ChunkSize      int
	ECData         int16
	ECParity       int16
	Chunks         []ManifestChunk
}

// PlannedFragment is a fragment row plus optional existing placement.
type PlannedFragment struct {
	ChunkSeq   int
	ShardIndex int16
	Fragment   Fragment
	Chunk      Chunk
	Placement  *Placement
}

// PlannedUpload is the result of inserting or resuming a pending version.
type PlannedUpload struct {
	File    File
	Version FileVersion
	Resume  bool
	Pending []PlannedFragment
	Stored  []PlannedFragment
}

// BeginUpload inserts or resumes a pending version.
func (s *Store) BeginUpload(ctx context.Context, ownerID uuid.UUID, m UploadManifest) (PlannedUpload, error) {
	path, err := NormalizePath(m.Path)
	if err != nil {
		return PlannedUpload{}, err
	}
	if _, err := s.BucketByID(ctx, ownerID, m.BucketID); err != nil {
		return PlannedUpload{}, err
	}
	if m.ChunkSize <= 0 || len(m.Chunks) == 0 || len(m.ContentSHA256) != 32 {
		return PlannedUpload{}, ErrInvalid
	}
	if m.ECData <= 0 {
		m.ECData = 1
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
	}
	defer tx.Rollback(ctx)

	var f File
	err = tx.QueryRow(ctx, `
		SELECT f.id, f.bucket_id, f.path, f.is_folder, f.current_version, f.status, f.created_at, f.updated_at, f.deleted_at
		FROM files f
		WHERE f.bucket_id = $1 AND f.path = $2 AND f.deleted_at IS NULL
	`, m.BucketID, path).Scan(&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.CurrentVersion, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt)
	exists := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
	}
	if exists && (f.Status == "active" || f.IsFolder) {
		return PlannedUpload{}, ErrConflict
	}

	if exists && f.Status == "uploading" {
		var v FileVersion
		err = tx.QueryRow(ctx, `
			SELECT id, file_id, version_no, size_bytes, content_sha256, encryption_meta, chunk_size,
			       ec_data_shards, ec_parity_shards, status, created_at, committed_at
			FROM file_versions
			WHERE file_id = $1 AND status = 'pending'
			ORDER BY version_no DESC
			LIMIT 1
		`, f.ID).Scan(&v.ID, &v.FileID, &v.VersionNo, &v.SizeBytes, &v.ContentSHA256, &v.EncryptionMeta,
			&v.ChunkSize, &v.ECDataShards, &v.ECParityShards, &v.Status, &v.CreatedAt, &v.CommittedAt)
		if err == nil && bytes.Equal(v.ContentSHA256, m.ContentSHA256) {
			out, err := loadPlanned(ctx, tx, f, v, true)
			if err != nil {
				return PlannedUpload{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
			}
			return out, nil
		}
		if err != nil && err != pgx.ErrNoRows {
			return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
		}
		return PlannedUpload{}, ErrConflict
	}

	if !exists {
		err = tx.QueryRow(ctx, `
			INSERT INTO files (bucket_id, path, is_folder, status)
			VALUES ($1, $2, false, 'uploading')
			RETURNING id, bucket_id, path, is_folder, current_version, status, created_at, updated_at, deleted_at
		`, m.BucketID, path).Scan(&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.CurrentVersion, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt)
		if err != nil {
			return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
		}
	}

	var v FileVersion
	err = tx.QueryRow(ctx, `
		INSERT INTO file_versions (
			file_id, version_no, size_bytes, content_sha256, encryption_meta,
			chunk_size, ec_data_shards, ec_parity_shards, status
		) VALUES ($1, 1, $2, $3, $4, $5, $6, $7, 'pending')
		RETURNING id, file_id, version_no, size_bytes, content_sha256, encryption_meta, chunk_size,
		          ec_data_shards, ec_parity_shards, status, created_at, committed_at
	`, f.ID, m.SizeBytes, m.ContentSHA256, []byte(m.EncryptionMeta), m.ChunkSize, m.ECData, m.ECParity).Scan(
		&v.ID, &v.FileID, &v.VersionNo, &v.SizeBytes, &v.ContentSHA256, &v.EncryptionMeta,
		&v.ChunkSize, &v.ECDataShards, &v.ECParityShards, &v.Status, &v.CreatedAt, &v.CommittedAt,
	)
	if err != nil {
		return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
	}

	for _, ch := range m.Chunks {
		var chunk Chunk
		err = tx.QueryRow(ctx, `
			INSERT INTO chunks (version_id, seq, size_bytes, sha256)
			VALUES ($1, $2, $3, $4)
			RETURNING id, version_id, seq, size_bytes, sha256
		`, v.ID, ch.Seq, ch.SizeBytes, ch.SHA256).Scan(&chunk.ID, &chunk.VersionID, &chunk.Seq, &chunk.SizeBytes, &chunk.SHA256)
		if err != nil {
			return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
		}
		if len(ch.Fragments) == 0 {
			return PlannedUpload{}, ErrInvalid
		}
		for _, fr := range ch.Fragments {
			if _, err = tx.Exec(ctx, `
				INSERT INTO fragments (chunk_id, shard_index, size_bytes, sha256)
				VALUES ($1, $2, $3, $4)
			`, chunk.ID, fr.ShardIndex, fr.SizeBytes, fr.SHA256); err != nil {
				return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
			}
		}
	}

	out, err := loadPlanned(ctx, tx, f, v, false)
	if err != nil {
		return PlannedUpload{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PlannedUpload{}, mapQueryErr("store.beginUpload", err)
	}
	return out, nil
}

func loadPlanned(ctx context.Context, q queryer, f File, v FileVersion, resume bool) (PlannedUpload, error) {
	rows, err := q.Query(ctx, `
		SELECT c.id, c.version_id, c.seq, c.size_bytes, c.sha256,
		       fr.id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256,
		       p.id, p.node_id, p.status, p.stored_at
		FROM chunks c
		JOIN fragments fr ON fr.chunk_id = c.id
		LEFT JOIN fragment_placements p ON p.fragment_id = fr.id AND p.status IN ('pending','stored')
		WHERE c.version_id = $1
		ORDER BY c.seq, fr.shard_index
	`, v.ID)
	if err != nil {
		return PlannedUpload{}, mapQueryErr("store.loadPlanned", err)
	}
	defer rows.Close()
	out := PlannedUpload{File: f, Version: v, Resume: resume}
	for rows.Next() {
		var pf PlannedFragment
		var plID *int64
		var plNode *uuid.UUID
		var plStatus *string
		var plStored *time.Time
		if err := rows.Scan(
			&pf.Chunk.ID, &pf.Chunk.VersionID, &pf.Chunk.Seq, &pf.Chunk.SizeBytes, &pf.Chunk.SHA256,
			&pf.Fragment.ID, &pf.Fragment.ChunkID, &pf.Fragment.ShardIndex, &pf.Fragment.SizeBytes, &pf.Fragment.SHA256,
			&plID, &plNode, &plStatus, &plStored,
		); err != nil {
			return PlannedUpload{}, mapQueryErr("store.loadPlanned", err)
		}
		pf.ChunkSeq = pf.Chunk.Seq
		pf.ShardIndex = pf.Fragment.ShardIndex
		if plID != nil && plNode != nil && plStatus != nil {
			pl := Placement{ID: *plID, FragmentID: pf.Fragment.ID, NodeID: *plNode, Status: *plStatus, StoredAt: plStored}
			pf.Placement = &pl
			if pl.Status == "stored" {
				out.Stored = append(out.Stored, pf)
				continue
			}
		}
		out.Pending = append(out.Pending, pf)
	}
	if err := rows.Err(); err != nil {
		return PlannedUpload{}, mapQueryErr("store.loadPlanned", err)
	}
	return out, nil
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// InsertPlacements writes pending placement rows.
func (s *Store) InsertPlacements(ctx context.Context, pairs []Placement) error {
	for _, p := range pairs {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO fragment_placements (fragment_id, node_id, status)
			VALUES ($1, $2, 'pending')
			ON CONFLICT (fragment_id, node_id) DO NOTHING
		`, p.FragmentID, p.NodeID)
		if err != nil {
			return mapQueryErr("store.insertPlacements", err)
		}
	}
	return nil
}

// VersionByID loads a version if the file belongs to ownerID.
func (s *Store) VersionByID(ctx context.Context, ownerID, versionID uuid.UUID) (File, FileVersion, error) {
	var f File
	var v FileVersion
	err := s.pool.QueryRow(ctx, `
		SELECT f.id, f.bucket_id, f.path, f.is_folder, f.current_version, f.status, f.created_at, f.updated_at, f.deleted_at,
		       v.id, v.file_id, v.version_no, v.size_bytes, v.content_sha256, v.encryption_meta, v.chunk_size,
		       v.ec_data_shards, v.ec_parity_shards, v.status, v.created_at, v.committed_at
		FROM file_versions v
		JOIN files f ON f.id = v.file_id
		JOIN buckets b ON b.id = f.bucket_id
		WHERE v.id = $1 AND b.owner_id = $2 AND f.deleted_at IS NULL
	`, versionID, ownerID).Scan(
		&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.CurrentVersion, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
		&v.ID, &v.FileID, &v.VersionNo, &v.SizeBytes, &v.ContentSHA256, &v.EncryptionMeta,
		&v.ChunkSize, &v.ECDataShards, &v.ECParityShards, &v.Status, &v.CreatedAt, &v.CommittedAt,
	)
	if err != nil {
		return File{}, FileVersion{}, mapQueryErr("store.versionByID", err)
	}
	return f, v, nil
}

// PlacementTargets are fragment+node rows that still need receipts.
type PlacementTarget struct {
	Placement Placement
	Fragment  Fragment
	Node      Node
}

// PendingTargetsForVersion lists placements with node public keys for receipt verify.
func (s *Store) PendingTargetsForVersion(ctx context.Context, versionID uuid.UUID) ([]PlacementTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.fragment_id, p.node_id, p.status, p.stored_at,
		       fr.id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256,
		       n.id, n.public_key, n.endpoint
		FROM fragment_placements p
		JOIN fragments fr ON fr.id = p.fragment_id
		JOIN chunks c ON c.id = fr.chunk_id
		JOIN nodes n ON n.id = p.node_id
		WHERE c.version_id = $1 AND p.status IN ('pending','stored')
	`, versionID)
	if err != nil {
		return nil, mapQueryErr("store.pendingTargets", err)
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
			return nil, mapQueryErr("store.pendingTargets", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CommitUpload marks listed placements stored and commits the version if every fragment is stored.
func (s *Store) CommitUpload(ctx context.Context, ownerID, versionID uuid.UUID, stored []uuid.UUID) (File, FileVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return File{}, FileVersion{}, mapQueryErr("store.commitUpload", err)
	}
	defer tx.Rollback(ctx)

	f, v, err := s.scanVersionTx(ctx, tx, ownerID, versionID)
	if err != nil {
		return File{}, FileVersion{}, err
	}
	if v.Status == "committed" {
		if err := tx.Commit(ctx); err != nil {
			return File{}, FileVersion{}, mapQueryErr("store.commitUpload", err)
		}
		return f, v, nil
	}
	if v.Status != "pending" {
		return File{}, FileVersion{}, ErrConflict
	}

	for _, fid := range stored {
		_, err = tx.Exec(ctx, `
			UPDATE fragment_placements
			SET status = 'stored', stored_at = now()
			WHERE fragment_id = $1 AND status = 'pending'
		`, fid)
		if err != nil {
			return File{}, FileVersion{}, mapQueryErr("store.commitUpload", err)
		}
	}

	var short int
	err = tx.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT c.id
			FROM chunks c
			JOIN fragments fr ON fr.chunk_id = c.id
			LEFT JOIN fragment_placements p ON p.fragment_id = fr.id AND p.status = 'stored'
			WHERE c.version_id = $1
			GROUP BY c.id
			HAVING count(p.id) < $2
		) short_chunks
	`, versionID, CommitThreshold).Scan(&short)
	if err != nil {
		return File{}, FileVersion{}, mapQueryErr("store.commitUpload", err)
	}
	if short > 0 {
		return File{}, FileVersion{}, ErrIncomplete
	}

	err = tx.QueryRow(ctx, `
		UPDATE file_versions SET status = 'committed', committed_at = now()
		WHERE id = $1
		RETURNING id, file_id, version_no, size_bytes, content_sha256, encryption_meta, chunk_size,
		          ec_data_shards, ec_parity_shards, status, created_at, committed_at
	`, versionID).Scan(&v.ID, &v.FileID, &v.VersionNo, &v.SizeBytes, &v.ContentSHA256, &v.EncryptionMeta,
		&v.ChunkSize, &v.ECDataShards, &v.ECParityShards, &v.Status, &v.CreatedAt, &v.CommittedAt)
	if err != nil {
		return File{}, FileVersion{}, mapQueryErr("store.commitUpload", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE files SET status = 'active', current_version = $2, updated_at = now()
		WHERE id = $1
	`, f.ID, v.ID)
	if err != nil {
		return File{}, FileVersion{}, mapQueryErr("store.commitUpload", err)
	}
	f.Status = "active"
	f.CurrentVersion = &v.ID
	if err := tx.Commit(ctx); err != nil {
		return File{}, FileVersion{}, mapQueryErr("store.commitUpload", err)
	}
	return f, v, nil
}

func (s *Store) scanVersionTx(ctx context.Context, tx pgx.Tx, ownerID, versionID uuid.UUID) (File, FileVersion, error) {
	var f File
	var v FileVersion
	err := tx.QueryRow(ctx, `
		SELECT f.id, f.bucket_id, f.path, f.is_folder, f.current_version, f.status, f.created_at, f.updated_at, f.deleted_at,
		       v.id, v.file_id, v.version_no, v.size_bytes, v.content_sha256, v.encryption_meta, v.chunk_size,
		       v.ec_data_shards, v.ec_parity_shards, v.status, v.created_at, v.committed_at
		FROM file_versions v
		JOIN files f ON f.id = v.file_id
		JOIN buckets b ON b.id = f.bucket_id
		WHERE v.id = $1 AND b.owner_id = $2 AND f.deleted_at IS NULL
		FOR UPDATE OF v
	`, versionID, ownerID).Scan(
		&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.CurrentVersion, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
		&v.ID, &v.FileID, &v.VersionNo, &v.SizeBytes, &v.ContentSHA256, &v.EncryptionMeta,
		&v.ChunkSize, &v.ECDataShards, &v.ECParityShards, &v.Status, &v.CreatedAt, &v.CommittedAt,
	)
	if err != nil {
		return File{}, FileVersion{}, mapQueryErr("store.scanVersion", err)
	}
	return f, v, nil
}

// DownloadChunk is a committed chunk plus stored placements.
type DownloadChunk struct {
	Chunk      Chunk
	Placements []DownloadPlacement
}

// DownloadPlacement is a healthy stored placement.
type DownloadPlacement struct {
	Fragment Fragment
	Node     Node
}

// LoadDownload returns the current committed version map.
func (s *Store) LoadDownload(ctx context.Context, ownerID, fileID uuid.UUID) (File, FileVersion, []DownloadChunk, error) {
	f, err := s.FileByID(ctx, ownerID, fileID)
	if err != nil {
		return File{}, FileVersion{}, nil, err
	}
	if f.CurrentVersion == nil {
		return File{}, FileVersion{}, nil, ErrNotFound
	}
	var v FileVersion
	err = s.pool.QueryRow(ctx, `
		SELECT id, file_id, version_no, size_bytes, content_sha256, encryption_meta, chunk_size,
		       ec_data_shards, ec_parity_shards, status, created_at, committed_at
		FROM file_versions WHERE id = $1 AND status = 'committed'
	`, *f.CurrentVersion).Scan(
		&v.ID, &v.FileID, &v.VersionNo, &v.SizeBytes, &v.ContentSHA256, &v.EncryptionMeta,
		&v.ChunkSize, &v.ECDataShards, &v.ECParityShards, &v.Status, &v.CreatedAt, &v.CommittedAt,
	)
	if err != nil {
		return File{}, FileVersion{}, nil, mapQueryErr("store.loadDownload", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.version_id, c.seq, c.size_bytes, c.sha256,
		       fr.id, fr.chunk_id, fr.shard_index, fr.size_bytes, fr.sha256,
		       n.id, n.endpoint, n.status
		FROM chunks c
		JOIN fragments fr ON fr.chunk_id = c.id
		JOIN fragment_placements p ON p.fragment_id = fr.id AND p.status = 'stored'
		JOIN nodes n ON n.id = p.node_id
		WHERE c.version_id = $1
		ORDER BY c.seq, fr.shard_index
	`, v.ID)
	if err != nil {
		return File{}, FileVersion{}, nil, mapQueryErr("store.loadDownload", err)
	}
	defer rows.Close()
	bySeq := map[int]*DownloadChunk{}
	var order []int
	for rows.Next() {
		var ch Chunk
		var fr Fragment
		var node Node
		if err := rows.Scan(
			&ch.ID, &ch.VersionID, &ch.Seq, &ch.SizeBytes, &ch.SHA256,
			&fr.ID, &fr.ChunkID, &fr.ShardIndex, &fr.SizeBytes, &fr.SHA256,
			&node.ID, &node.Endpoint, &node.Status,
		); err != nil {
			return File{}, FileVersion{}, nil, mapQueryErr("store.loadDownload", err)
		}
		dc, ok := bySeq[ch.Seq]
		if !ok {
			dc = &DownloadChunk{Chunk: ch}
			bySeq[ch.Seq] = dc
			order = append(order, ch.Seq)
		}
		dc.Placements = append(dc.Placements, DownloadPlacement{Fragment: fr, Node: node})
	}
	if err := rows.Err(); err != nil {
		return File{}, FileVersion{}, nil, mapQueryErr("store.loadDownload", err)
	}
	out := make([]DownloadChunk, 0, len(order))
	for _, seq := range order {
		out = append(out, *bySeq[seq])
	}
	return f, v, out, nil
}

// FileByPath loads a non-deleted file.
func (s *Store) FileByPath(ctx context.Context, ownerID, bucketID uuid.UUID, path string) (File, error) {
	p, err := NormalizePath(path)
	if err != nil {
		return File{}, err
	}
	var f File
	err = s.pool.QueryRow(ctx, `
		SELECT f.id, f.bucket_id, f.path, f.is_folder, f.current_version, f.status, f.created_at, f.updated_at, f.deleted_at
		FROM files f
		JOIN buckets b ON b.id = f.bucket_id
		WHERE f.bucket_id = $1 AND b.owner_id = $2 AND f.path = $3 AND f.deleted_at IS NULL
	`, bucketID, ownerID, p).Scan(&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.CurrentVersion, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt)
	if err != nil {
		return File{}, mapQueryErr("store.fileByPath", err)
	}
	return f, nil
}
