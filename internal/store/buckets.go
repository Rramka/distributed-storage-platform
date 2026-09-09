package store

import (
	"context"

	"github.com/google/uuid"
)

// CreateBucket inserts a bucket owned by ownerID.
func (s *Store) CreateBucket(ctx context.Context, ownerID uuid.UUID, name string) (Bucket, error) {
	var b Bucket
	err := s.pool.QueryRow(ctx, `
		INSERT INTO buckets (owner_id, name)
		VALUES ($1, $2)
		RETURNING id, owner_id, name, created_at
	`, ownerID, name).Scan(&b.ID, &b.OwnerID, &b.Name, &b.CreatedAt)
	if err != nil {
		return Bucket{}, mapQueryErr("store.createBucket", err)
	}
	return b, nil
}

// ListBuckets returns buckets for an owner.
func (s *Store) ListBuckets(ctx context.Context, ownerID uuid.UUID) ([]Bucket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_id, name, created_at
		FROM buckets WHERE owner_id = $1
		ORDER BY name
	`, ownerID)
	if err != nil {
		return nil, mapQueryErr("store.listBuckets", err)
	}
	defer rows.Close()
	var out []Bucket
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.ID, &b.OwnerID, &b.Name, &b.CreatedAt); err != nil {
			return nil, mapQueryErr("store.listBuckets", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BucketByID loads a bucket if it belongs to ownerID.
func (s *Store) BucketByID(ctx context.Context, ownerID, id uuid.UUID) (Bucket, error) {
	var b Bucket
	err := s.pool.QueryRow(ctx, `
		SELECT id, owner_id, name, created_at
		FROM buckets WHERE id = $1 AND owner_id = $2
	`, id, ownerID).Scan(&b.ID, &b.OwnerID, &b.Name, &b.CreatedAt)
	if err != nil {
		return Bucket{}, mapQueryErr("store.bucketByID", err)
	}
	return b, nil
}

// DeleteBucket removes an empty bucket. Non-deleted files block the delete.
func (s *Store) DeleteBucket(ctx context.Context, ownerID, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapQueryErr("store.deleteBucket", err)
	}
	defer tx.Rollback(ctx)

	var exists uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM buckets WHERE id = $1 AND owner_id = $2`, id, ownerID).Scan(&exists)
	if err != nil {
		return mapQueryErr("store.deleteBucket", err)
	}

	var n int
	err = tx.QueryRow(ctx, `
		SELECT count(*) FROM files WHERE bucket_id = $1 AND deleted_at IS NULL
	`, id).Scan(&n)
	if err != nil {
		return mapQueryErr("store.deleteBucket", err)
	}
	if n > 0 {
		return ErrNotEmpty
	}

	// Soft-deleted rows still reference the bucket; M1 has no fragment GC yet.
	if _, err = tx.Exec(ctx, `DELETE FROM files WHERE bucket_id = $1 AND deleted_at IS NOT NULL`, id); err != nil {
		return mapQueryErr("store.deleteBucket", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM buckets WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return mapQueryErr("store.deleteBucket", err)
	}
	return tx.Commit(ctx)
}
