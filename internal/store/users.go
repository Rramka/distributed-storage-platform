package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// CreateUser inserts a customer. email is stored lowercased.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, role, status, created_at, updated_at
	`, email, passwordHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return User{}, mapQueryErr("store.createUser", err)
	}
	return u, nil
}

// UserByEmail loads a user by email (case-insensitive).
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	return s.scanUser(ctx, `
		SELECT id, email, password_hash, role, status, created_at, updated_at
		FROM users WHERE email = $1
	`, email)
}

// UserByID loads a user by id.
func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.scanUser(ctx, `
		SELECT id, email, password_hash, role, status, created_at, updated_at
		FROM users WHERE id = $1
	`, id)
}

func (s *Store) scanUser(ctx context.Context, q string, args ...any) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, q, args...).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return User{}, mapQueryErr("store.user", err)
	}
	return u, nil
}
