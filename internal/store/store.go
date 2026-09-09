// Package store is the Postgres access layer for control-plane services.
// Schema: docs/03-data-model.md.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("store: not found")
	ErrConflict = errors.New("store: already exists")
	ErrNotEmpty = errors.New("store: not empty")
	ErrInvalid  = errors.New("store: invalid request")
)

// Store wraps a pgx pool.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects and pings Postgres.
func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("store.open: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store.open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store.open: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() {
	s.pool.Close()
}

// Ping checks connectivity.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("store.ping: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func mapQueryErr(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return fmt.Errorf("%s: %w", op, err)
}

// User is a row in users.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// APIKey is a row in api_keys (hash stored; secret never persisted).
type APIKey struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	KeyHash   string     `json:"-"`
	Label     string     `json:"label"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
}

// Bucket is a row in buckets.
type Bucket struct {
	ID        uuid.UUID `json:"id"`
	OwnerID   uuid.UUID `json:"owner_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// File is a row in files (folders in M1).
type File struct {
	ID        uuid.UUID  `json:"id"`
	BucketID  uuid.UUID  `json:"bucket_id"`
	Path      string     `json:"path"`
	IsFolder  bool       `json:"is_folder"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}
