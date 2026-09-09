package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CreateAPIKey stores the hash of a secret. The secret itself is never written.
func (s *Store) CreateAPIKey(ctx context.Context, userID uuid.UUID, keyHash, label string, scopes []string, expiresAt *time.Time) (APIKey, error) {
	if len(scopes) == 0 {
		scopes = []string{"read", "write"}
	}
	var k APIKey
	err := s.pool.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, key_hash, label, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, key_hash, label, scopes, expires_at, revoked_at, created_at
	`, userID, keyHash, label, scopes, expiresAt).Scan(
		&k.ID, &k.UserID, &k.KeyHash, &k.Label, &k.Scopes, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt,
	)
	if err != nil {
		return APIKey{}, mapQueryErr("store.createAPIKey", err)
	}
	return k, nil
}

// APIKeyByHash returns an active (unrevoked, unexpired) key.
func (s *Store) APIKeyByHash(ctx context.Context, keyHash string) (APIKey, error) {
	var k APIKey
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, key_hash, label, scopes, expires_at, revoked_at, created_at
		FROM api_keys
		WHERE key_hash = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
	`, keyHash).Scan(
		&k.ID, &k.UserID, &k.KeyHash, &k.Label, &k.Scopes, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt,
	)
	if err != nil {
		return APIKey{}, mapQueryErr("store.apiKeyByHash", err)
	}
	return k, nil
}

// ListAPIKeys returns non-revoked keys for a user.
func (s *Store) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, key_hash, label, scopes, expires_at, revoked_at, created_at
		FROM api_keys
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, mapQueryErr("store.listAPIKeys", err)
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.Label, &k.Scopes, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, mapQueryErr("store.listAPIKeys", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeAPIKey sets revoked_at. Owner-scoped.
func (s *Store) RevokeAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_keys
		SET revoked_at = now()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, keyID, userID)
	if err != nil {
		return mapQueryErr("store.revokeAPIKey", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
