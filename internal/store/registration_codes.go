package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MintRegistrationCode inserts a unused code.
func (s *Store) MintRegistrationCode(ctx context.Context, ownerID uuid.UUID, codeHash, endpoint string, expiresAt time.Time) (RegistrationCode, error) {
	var c RegistrationCode
	err := s.pool.QueryRow(ctx, `
		INSERT INTO node_registration_codes (owner_id, code_hash, endpoint, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, owner_id, code_hash, endpoint, expires_at, used_at, node_id, created_at
	`, ownerID, codeHash, endpoint, expiresAt).Scan(
		&c.ID, &c.OwnerID, &c.CodeHash, &c.Endpoint, &c.ExpiresAt, &c.UsedAt, &c.NodeID, &c.CreatedAt,
	)
	if err != nil {
		return RegistrationCode{}, mapQueryErr("store.mintRegistrationCode", err)
	}
	return c, nil
}

// ConsumeRegistrationCode marks a unused, unexpired code as used and binds nodeID.
// The node row must already exist (node_id is a FK to nodes).
func (s *Store) ConsumeRegistrationCode(ctx context.Context, codeHash string, nodeID uuid.UUID) (RegistrationCode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RegistrationCode{}, mapQueryErr("store.consumeRegistrationCode", err)
	}
	defer tx.Rollback(ctx)

	var c RegistrationCode
	err = tx.QueryRow(ctx, `
		SELECT id, owner_id, code_hash, endpoint, expires_at, used_at, node_id, created_at
		FROM node_registration_codes
		WHERE code_hash = $1
		FOR UPDATE
	`, codeHash).Scan(&c.ID, &c.OwnerID, &c.CodeHash, &c.Endpoint, &c.ExpiresAt, &c.UsedAt, &c.NodeID, &c.CreatedAt)
	if err != nil {
		return RegistrationCode{}, mapQueryErr("store.consumeRegistrationCode", err)
	}
	if c.UsedAt != nil {
		return RegistrationCode{}, ErrConflict
	}
	if time.Now().After(c.ExpiresAt) {
		return RegistrationCode{}, ErrNotFound
	}
	_, err = tx.Exec(ctx, `
		UPDATE node_registration_codes
		SET used_at = now(), node_id = $2
		WHERE id = $1
	`, c.ID, nodeID)
	if err != nil {
		return RegistrationCode{}, mapQueryErr("store.consumeRegistrationCode", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return RegistrationCode{}, mapQueryErr("store.consumeRegistrationCode", err)
	}
	now := time.Now().UTC()
	c.UsedAt = &now
	c.NodeID = &nodeID
	return c, nil
}

// RegistrationCodeByHash loads a code.
func (s *Store) RegistrationCodeByHash(ctx context.Context, codeHash string) (RegistrationCode, error) {
	var c RegistrationCode
	err := s.pool.QueryRow(ctx, `
		SELECT id, owner_id, code_hash, endpoint, expires_at, used_at, node_id, created_at
		FROM node_registration_codes WHERE code_hash = $1
	`, codeHash).Scan(&c.ID, &c.OwnerID, &c.CodeHash, &c.Endpoint, &c.ExpiresAt, &c.UsedAt, &c.NodeID, &c.CreatedAt)
	if err != nil {
		return RegistrationCode{}, mapQueryErr("store.registrationCodeByHash", err)
	}
	return c, nil
}
