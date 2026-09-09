// Package metadata is the internal HTTP API over Postgres metadata (docs/03-data-model.md).
package metadata

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/auth"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

var (
	ErrNotFound        = store.ErrNotFound
	ErrConflict        = store.ErrConflict
	ErrNotEmpty        = store.ErrNotEmpty
	ErrUnauthenticated = errors.New("metadata: unauthenticated")
	ErrInvalid         = errors.New("metadata: invalid request")
	ErrForbidden       = errors.New("metadata: forbidden")
)

// Service is the metadata operations the gateway calls.
type Service interface {
	CreateUser(ctx context.Context, email, password string) (store.User, error)
	VerifyPassword(ctx context.Context, email, password string) (store.User, error)
	LookupAPIKey(ctx context.Context, keyHash string) (store.APIKey, store.User, error)
	CreateAPIKey(ctx context.Context, userID uuid.UUID, keyHash, label string, scopes []string, expiresAt *time.Time) (store.APIKey, error)
	ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error)
	RevokeAPIKey(ctx context.Context, userID, keyID uuid.UUID) error
	CreateBucket(ctx context.Context, userID uuid.UUID, name string) (store.Bucket, error)
	ListBuckets(ctx context.Context, userID uuid.UUID) ([]store.Bucket, error)
	DeleteBucket(ctx context.Context, userID, bucketID uuid.UUID) error
	CreateFolder(ctx context.Context, userID, bucketID uuid.UUID, path string) (store.File, error)
	ListFiles(ctx context.Context, userID, bucketID uuid.UUID, prefix, cursor string, limit int) ([]store.File, string, error)
	GetFile(ctx context.Context, userID, fileID uuid.UUID) (store.File, error)
	RenameFile(ctx context.Context, userID, fileID uuid.UUID, newPath string) (store.File, error)
	DeleteFile(ctx context.Context, userID, fileID uuid.UUID) error
}

// StoreService implements Service against Postgres.
type StoreService struct {
	Store *store.Store
}

func (s *StoreService) CreateUser(ctx context.Context, email, password string) (store.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return store.User{}, ErrInvalid
	}
	if len(password) < 8 {
		return store.User{}, ErrInvalid
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return store.User{}, err
	}
	u, err := s.Store.CreateUser(ctx, email, hash)
	if errors.Is(err, store.ErrConflict) {
		return store.User{}, ErrConflict
	}
	return u, err
}

func (s *StoreService) VerifyPassword(ctx context.Context, email, password string) (store.User, error) {
	u, err := s.Store.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.User{}, ErrUnauthenticated
		}
		return store.User{}, err
	}
	if u.Status != "active" {
		return store.User{}, ErrUnauthenticated
	}
	if err := auth.VerifyPassword(u.PasswordHash, password); err != nil {
		return store.User{}, ErrUnauthenticated
	}
	return u, nil
}

func (s *StoreService) LookupAPIKey(ctx context.Context, keyHash string) (store.APIKey, store.User, error) {
	k, err := s.Store.APIKeyByHash(ctx, keyHash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.APIKey{}, store.User{}, ErrUnauthenticated
		}
		return store.APIKey{}, store.User{}, err
	}
	u, err := s.Store.UserByID(ctx, k.UserID)
	if err != nil {
		return store.APIKey{}, store.User{}, err
	}
	if u.Status != "active" {
		return store.APIKey{}, store.User{}, ErrUnauthenticated
	}
	return k, u, nil
}

func (s *StoreService) CreateAPIKey(ctx context.Context, userID uuid.UUID, keyHash, label string, scopes []string, expiresAt *time.Time) (store.APIKey, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return store.APIKey{}, ErrInvalid
	}
	return s.Store.CreateAPIKey(ctx, userID, keyHash, label, scopes, expiresAt)
}

func (s *StoreService) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error) {
	return s.Store.ListAPIKeys(ctx, userID)
}

func (s *StoreService) RevokeAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	return s.Store.RevokeAPIKey(ctx, userID, keyID)
}

func (s *StoreService) CreateBucket(ctx context.Context, userID uuid.UUID, name string) (store.Bucket, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Bucket{}, ErrInvalid
	}
	return s.Store.CreateBucket(ctx, userID, name)
}

func (s *StoreService) ListBuckets(ctx context.Context, userID uuid.UUID) ([]store.Bucket, error) {
	return s.Store.ListBuckets(ctx, userID)
}

func (s *StoreService) DeleteBucket(ctx context.Context, userID, bucketID uuid.UUID) error {
	return s.Store.DeleteBucket(ctx, userID, bucketID)
}

func (s *StoreService) CreateFolder(ctx context.Context, userID, bucketID uuid.UUID, path string) (store.File, error) {
	f, err := s.Store.CreateFolder(ctx, userID, bucketID, path)
	if errors.Is(err, store.ErrInvalid) {
		return store.File{}, ErrInvalid
	}
	return f, err
}

func (s *StoreService) ListFiles(ctx context.Context, userID, bucketID uuid.UUID, prefix, cursor string, limit int) ([]store.File, string, error) {
	return s.Store.ListFiles(ctx, userID, bucketID, prefix, cursor, limit)
}

func (s *StoreService) GetFile(ctx context.Context, userID, fileID uuid.UUID) (store.File, error) {
	return s.Store.FileByID(ctx, userID, fileID)
}

func (s *StoreService) RenameFile(ctx context.Context, userID, fileID uuid.UUID, newPath string) (store.File, error) {
	f, err := s.Store.RenameFile(ctx, userID, fileID, newPath)
	if errors.Is(err, store.ErrInvalid) {
		return store.File{}, ErrInvalid
	}
	return f, err
}

func (s *StoreService) DeleteFile(ctx context.Context, userID, fileID uuid.UUID) error {
	return s.Store.SoftDeleteFile(ctx, userID, fileID)
}

func validEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at < 1 || at == len(email)-1 {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}
