package store

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

// NormalizePath requires a leading slash, no "..", and is not "/".
func NormalizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: path must start with /", ErrInvalid)
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("%w: invalid path", ErrInvalid)
	}
	trimmed := strings.TrimRight(p, "/")
	if trimmed == "" {
		return "", fmt.Errorf("%w: root is not a valid folder", ErrInvalid)
	}
	cleaned := path.Clean(trimmed)
	if cleaned != trimmed {
		return "", fmt.Errorf("%w: invalid path", ErrInvalid)
	}
	return cleaned, nil
}

// CreateFolder inserts an active folder row, owner-scoped via the bucket.
func (s *Store) CreateFolder(ctx context.Context, ownerID, bucketID uuid.UUID, rawPath string) (File, error) {
	p, err := NormalizePath(rawPath)
	if err != nil {
		return File{}, err
	}
	if _, err := s.BucketByID(ctx, ownerID, bucketID); err != nil {
		return File{}, err
	}
	var f File
	err = s.pool.QueryRow(ctx, `
		INSERT INTO files (bucket_id, path, is_folder, status)
		VALUES ($1, $2, true, 'active')
		RETURNING id, bucket_id, path, is_folder, status, created_at, updated_at, deleted_at
	`, bucketID, p).Scan(
		&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	if err != nil {
		return File{}, mapQueryErr("store.createFolder", err)
	}
	return f, nil
}

// FileByID returns a non-deleted file owned by ownerID.
func (s *Store) FileByID(ctx context.Context, ownerID, id uuid.UUID) (File, error) {
	var f File
	err := s.pool.QueryRow(ctx, `
		SELECT f.id, f.bucket_id, f.path, f.is_folder, f.current_version, f.status, f.created_at, f.updated_at, f.deleted_at
		FROM files f
		JOIN buckets b ON b.id = f.bucket_id
		WHERE f.id = $1 AND b.owner_id = $2 AND f.deleted_at IS NULL
	`, id, ownerID).Scan(
		&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.CurrentVersion, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	if err != nil {
		return File{}, mapQueryErr("store.fileByID", err)
	}
	return f, nil
}

// ListFiles returns non-deleted files/folders under prefix, after cursor, up to limit+1.
func (s *Store) ListFiles(ctx context.Context, ownerID, bucketID uuid.UUID, prefix, cursor string, limit int) ([]File, string, error) {
	if _, err := s.BucketByID(ctx, ownerID, bucketID); err != nil {
		return nil, "", err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	likePrefix := prefix
	pathEq := prefix
	if prefix != "" {
		likePrefix = prefix + "/%"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT f.id, f.bucket_id, f.path, f.is_folder, f.status, f.created_at, f.updated_at, f.deleted_at
		FROM files f
		JOIN buckets b ON b.id = f.bucket_id
		WHERE f.bucket_id = $1 AND b.owner_id = $2
		  AND f.deleted_at IS NULL
		  AND ($3 = '' OR f.path = $4 OR f.path LIKE $5)
		  AND f.path > $6
		ORDER BY f.path
		LIMIT $7
	`, bucketID, ownerID, prefix, pathEq, likePrefix, cursor, limit+1)
	if err != nil {
		return nil, "", mapQueryErr("store.listFiles", err)
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt); err != nil {
			return nil, "", mapQueryErr("store.listFiles", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, "", mapQueryErr("store.listFiles", err)
	}
	var next string
	if len(out) > limit {
		out = out[:limit]
		next = out[len(out)-1].Path
	}
	return out, next, nil
}

// RenameFile updates path. Folder renames rewrite the subtree prefix.
func (s *Store) RenameFile(ctx context.Context, ownerID, fileID uuid.UUID, newPath string) (File, error) {
	p, err := NormalizePath(newPath)
	if err != nil {
		return File{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return File{}, mapQueryErr("store.renameFile", err)
	}
	defer tx.Rollback(ctx)

	var f File
	err = tx.QueryRow(ctx, `
		SELECT f.id, f.bucket_id, f.path, f.is_folder, f.status, f.created_at, f.updated_at, f.deleted_at
		FROM files f
		JOIN buckets b ON b.id = f.bucket_id
		WHERE f.id = $1 AND b.owner_id = $2 AND f.deleted_at IS NULL
		FOR UPDATE OF f
	`, fileID, ownerID).Scan(
		&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	if err != nil {
		return File{}, mapQueryErr("store.renameFile", err)
	}
	if f.Path == p {
		return f, nil
	}

	old := f.Path
	if f.IsFolder {
		var collision int
		err = tx.QueryRow(ctx, `
			SELECT count(*) FROM files
			WHERE bucket_id = $1 AND deleted_at IS NULL
			  AND (path = $2 OR path LIKE $2 || '/%')
			  AND NOT (path = $3 OR path LIKE $3 || '/%')
		`, f.BucketID, p, old).Scan(&collision)
		if err != nil {
			return File{}, mapQueryErr("store.renameFile", err)
		}
		if collision > 0 {
			return File{}, ErrConflict
		}
		_, err = tx.Exec(ctx, `
			UPDATE files
			SET path = $1 || substr(path, length($2) + 1),
			    updated_at = now()
			WHERE bucket_id = $3
			  AND deleted_at IS NULL
			  AND (path = $2 OR path LIKE $2 || '/%')
		`, p, old, f.BucketID)
		if err != nil {
			return File{}, mapQueryErr("store.renameFile", err)
		}
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE files SET path = $1, updated_at = now()
			WHERE id = $2
		`, p, f.ID)
		if err != nil {
			return File{}, mapQueryErr("store.renameFile", err)
		}
	}

	err = tx.QueryRow(ctx, `
		SELECT id, bucket_id, path, is_folder, status, created_at, updated_at, deleted_at
		FROM files WHERE id = $1
	`, f.ID).Scan(
		&f.ID, &f.BucketID, &f.Path, &f.IsFolder, &f.Status, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	if err != nil {
		return File{}, mapQueryErr("store.renameFile", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return File{}, mapQueryErr("store.renameFile", err)
	}
	return f, nil
}

// SoftDeleteFile sets deleted_at. Folders also soft-delete descendants.
func (s *Store) SoftDeleteFile(ctx context.Context, ownerID, fileID uuid.UUID) error {
	f, err := s.FileByID(ctx, ownerID, fileID)
	if err != nil {
		return err
	}
	if f.IsFolder {
		_, err = s.pool.Exec(ctx, `
			UPDATE files
			SET status = 'deleted', deleted_at = now(), updated_at = now()
			WHERE bucket_id = $1
			  AND deleted_at IS NULL
			  AND (path = $2 OR path LIKE $2 || '/%')
		`, f.BucketID, f.Path)
	} else {
		_, err = s.pool.Exec(ctx, `
			UPDATE files
			SET status = 'deleted', deleted_at = now(), updated_at = now()
			WHERE id = $1
		`, f.ID)
	}
	if err != nil {
		return mapQueryErr("store.softDeleteFile", err)
	}
	return nil
}
