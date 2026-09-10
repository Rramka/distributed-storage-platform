package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const codeCols = `id, owner_id, code_hash, endpoint, country, region, asn, expires_at, used_at, node_id, created_at`

func scanCode(row interface{ Scan(dest ...any) error }) (RegistrationCode, error) {
	var c RegistrationCode
	var country, region *string
	err := row.Scan(
		&c.ID, &c.OwnerID, &c.CodeHash, &c.Endpoint, &country, &region, &c.ASN,
		&c.ExpiresAt, &c.UsedAt, &c.NodeID, &c.CreatedAt,
	)
	if country != nil {
		c.Country = *country
	}
	if region != nil {
		c.Region = *region
	}
	return c, err
}

// MintRegistrationCode inserts an unused code with placement attributes.
func (s *Store) MintRegistrationCode(ctx context.Context, ownerID uuid.UUID, codeHash, endpoint, country, region string, asn *int, expiresAt time.Time) (RegistrationCode, error) {
	c, err := scanCode(s.pool.QueryRow(ctx, `
		INSERT INTO node_registration_codes (owner_id, code_hash, endpoint, country, region, asn, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+codeCols, ownerID, codeHash, endpoint, nullIfEmpty(country), nullIfEmpty(region), asn, expiresAt,
	))
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

	c, err := scanCode(tx.QueryRow(ctx, `
		SELECT `+codeCols+`
		FROM node_registration_codes
		WHERE code_hash = $1
		FOR UPDATE
	`, codeHash))
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
	c, err := scanCode(s.pool.QueryRow(ctx, `
		SELECT `+codeCols+` FROM node_registration_codes WHERE code_hash = $1
	`, codeHash))
	if err != nil {
		return RegistrationCode{}, mapQueryErr("store.registrationCodeByHash", err)
	}
	return c, nil
}
