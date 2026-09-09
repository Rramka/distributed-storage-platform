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
	ErrNotFound    = errors.New("store: not found")
	ErrConflict    = errors.New("store: already exists")
	ErrNotEmpty    = errors.New("store: not empty")
	ErrInvalid     = errors.New("store: invalid request")
	ErrIncomplete  = errors.New("store: incomplete placements")
	ErrUnavailable = errors.New("store: placement unavailable")
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

// File is a row in files.
type File struct {
	ID             uuid.UUID  `json:"id"`
	BucketID       uuid.UUID  `json:"bucket_id"`
	Path           string     `json:"path"`
	IsFolder       bool       `json:"is_folder"`
	CurrentVersion *uuid.UUID `json:"current_version,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"-"`
}

// Node is a row in nodes.
type Node struct {
	ID              uuid.UUID  `json:"id"`
	OwnerID         uuid.UUID  `json:"owner_id"`
	CertFingerprint []byte     `json:"cert_fingerprint"`
	PublicKey       []byte     `json:"-"`
	CertPEM         string     `json:"-"`
	CertExpiresAt   *time.Time `json:"cert_expires_at,omitempty"`
	HostnameLabel   string     `json:"hostname_label,omitempty"`
	OS              string     `json:"os"`
	AgentVersion    string     `json:"agent_version"`
	Country         string     `json:"country,omitempty"`
	Region          string     `json:"region,omitempty"`
	ASN             *int       `json:"asn,omitempty"`
	Endpoint        string     `json:"endpoint"`
	CapacityBytes   int64      `json:"capacity_bytes"`
	UsedBytes       int64      `json:"used_bytes"`
	Status          string     `json:"status"`
	Reputation      float32    `json:"reputation"`
	RegisteredAt    time.Time  `json:"registered_at"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
}

// RegistrationCode is a row in node_registration_codes (hash stored).
type RegistrationCode struct {
	ID        uuid.UUID  `json:"id"`
	OwnerID   uuid.UUID  `json:"owner_id"`
	CodeHash  string     `json:"-"`
	Endpoint  string     `json:"endpoint"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	NodeID    *uuid.UUID `json:"node_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// FileVersion is a row in file_versions.
type FileVersion struct {
	ID             uuid.UUID  `json:"id"`
	FileID         uuid.UUID  `json:"file_id"`
	VersionNo      int        `json:"version_no"`
	SizeBytes      int64      `json:"size_bytes"`
	ContentSHA256  []byte     `json:"content_sha256"`
	EncryptionMeta []byte     `json:"encryption_meta"`
	ChunkSize      int        `json:"chunk_size"`
	ECDataShards   int16      `json:"ec_data_shards"`
	ECParityShards int16      `json:"ec_parity_shards"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	CommittedAt    *time.Time `json:"committed_at,omitempty"`
}

// Chunk is a row in chunks.
type Chunk struct {
	ID        uuid.UUID `json:"id"`
	VersionID uuid.UUID `json:"version_id"`
	Seq       int       `json:"seq"`
	SizeBytes int       `json:"size_bytes"`
	SHA256    []byte    `json:"sha256"`
}

// Fragment is a row in fragments.
type Fragment struct {
	ID         uuid.UUID `json:"id"`
	ChunkID    uuid.UUID `json:"chunk_id"`
	ShardIndex int16     `json:"shard_index"`
	SizeBytes  int       `json:"size_bytes"`
	SHA256     []byte    `json:"sha256"`
}

// Placement is a row in fragment_placements.
type Placement struct {
	ID         int64      `json:"id"`
	FragmentID uuid.UUID  `json:"fragment_id"`
	NodeID     uuid.UUID  `json:"node_id"`
	Status     string     `json:"status"`
	StoredAt   *time.Time `json:"stored_at,omitempty"`
}
