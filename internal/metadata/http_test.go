package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

type mem struct {
	users   map[string]store.User
	pass    map[uuid.UUID]string
	keys    map[string]store.APIKey
	buckets map[uuid.UUID][]store.Bucket
	files   map[uuid.UUID]store.File
}

func newMem() *mem {
	return &mem{
		users:   map[string]store.User{},
		pass:    map[uuid.UUID]string{},
		keys:    map[string]store.APIKey{},
		buckets: map[uuid.UUID][]store.Bucket{},
		files:   map[uuid.UUID]store.File{},
	}
}

func (m *mem) CreateUser(_ context.Context, email, password string) (store.User, error) {
	if _, ok := m.users[email]; ok {
		return store.User{}, ErrConflict
	}
	if email == "" || len(password) < 8 {
		return store.User{}, ErrInvalid
	}
	u := store.User{ID: uuid.New(), Email: email, Role: "customer", Status: "active", CreatedAt: time.Now().UTC()}
	m.users[email] = u
	m.pass[u.ID] = password
	return u, nil
}

func (m *mem) VerifyPassword(_ context.Context, email, password string) (store.User, error) {
	u, ok := m.users[email]
	if !ok || m.pass[u.ID] != password {
		return store.User{}, ErrUnauthenticated
	}
	return u, nil
}

func (m *mem) LookupAPIKey(_ context.Context, keyHash string) (store.APIKey, store.User, error) {
	k, ok := m.keys[keyHash]
	if !ok {
		return store.APIKey{}, store.User{}, ErrUnauthenticated
	}
	for _, u := range m.users {
		if u.ID == k.UserID {
			return k, u, nil
		}
	}
	return store.APIKey{}, store.User{}, ErrNotFound
}

func (m *mem) CreateAPIKey(_ context.Context, userID uuid.UUID, keyHash, label string, scopes []string, expiresAt *time.Time) (store.APIKey, error) {
	if label == "" {
		return store.APIKey{}, ErrInvalid
	}
	k := store.APIKey{ID: uuid.New(), UserID: userID, KeyHash: keyHash, Label: label, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: time.Now().UTC()}
	m.keys[keyHash] = k
	return k, nil
}

func (m *mem) ListAPIKeys(_ context.Context, userID uuid.UUID) ([]store.APIKey, error) {
	var out []store.APIKey
	for _, k := range m.keys {
		if k.UserID == userID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (m *mem) RevokeAPIKey(_ context.Context, userID, keyID uuid.UUID) error {
	for hash, k := range m.keys {
		if k.ID == keyID && k.UserID == userID {
			delete(m.keys, hash)
			return nil
		}
	}
	return ErrNotFound
}

func (m *mem) CreateBucket(_ context.Context, userID uuid.UUID, name string) (store.Bucket, error) {
	b := store.Bucket{ID: uuid.New(), OwnerID: userID, Name: name, CreatedAt: time.Now().UTC()}
	m.buckets[userID] = append(m.buckets[userID], b)
	return b, nil
}

func (m *mem) ListBuckets(_ context.Context, userID uuid.UUID) ([]store.Bucket, error) {
	return m.buckets[userID], nil
}

func (m *mem) DeleteBucket(_ context.Context, userID, bucketID uuid.UUID) error {
	list := m.buckets[userID]
	for i, b := range list {
		if b.ID == bucketID {
			for _, f := range m.files {
				if f.BucketID == bucketID {
					return ErrNotEmpty
				}
			}
			m.buckets[userID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *mem) CreateFolder(_ context.Context, userID, bucketID uuid.UUID, path string) (store.File, error) {
	f := store.File{ID: uuid.New(), BucketID: bucketID, Path: path, IsFolder: true, Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	m.files[f.ID] = f
	return f, nil
}

func (m *mem) ListFiles(_ context.Context, userID, bucketID uuid.UUID, prefix, cursor string, limit int) ([]store.File, string, error) {
	var out []store.File
	for _, f := range m.files {
		if f.BucketID == bucketID {
			out = append(out, f)
		}
	}
	return out, "", nil
}

func (m *mem) GetFile(_ context.Context, userID, fileID uuid.UUID) (store.File, error) {
	f, ok := m.files[fileID]
	if !ok {
		return store.File{}, ErrNotFound
	}
	return f, nil
}

func (m *mem) RenameFile(_ context.Context, userID, fileID uuid.UUID, newPath string) (store.File, error) {
	f, ok := m.files[fileID]
	if !ok {
		return store.File{}, ErrNotFound
	}
	f.Path = newPath
	m.files[fileID] = f
	return f, nil
}

func (m *mem) DeleteFile(_ context.Context, userID, fileID uuid.UUID) error {
	if _, ok := m.files[fileID]; !ok {
		return ErrNotFound
	}
	delete(m.files, fileID)
	return nil
}

func TestInternalHTTPRegisterAndLookup(t *testing.T) {
	t.Parallel()
	mux := httpserver.NewMux("metadata")
	Mount(mux, newMem())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL)

	ctx := context.Background()
	u, err := c.CreateUser(ctx, "ada@example.com", "password1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Email != "ada@example.com" {
		t.Fatalf("email %q", u.Email)
	}
	_, err = c.CreateUser(ctx, "ada@example.com", "password1")
	var ce *CallError
	if err == nil || !asCall(err, &ce) || ce.Code != apierr.CodeAlreadyExists {
		t.Fatalf("dup: %v", err)
	}

	got, err := c.VerifyPassword(ctx, "ada@example.com", "password1")
	if err != nil || got.ID != u.ID {
		t.Fatalf("verify: %+v %v", got, err)
	}
	_, err = c.VerifyPassword(ctx, "ada@example.com", "nope----")
	if err == nil || !asCall(err, &ce) || ce.Code != apierr.CodeUnauthenticated {
		t.Fatalf("bad password: %v", err)
	}

	hash := "abc123"
	k, err := c.CreateAPIKey(ctx, u.ID, hash, "cli", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lk, lu, err := c.LookupAPIKey(ctx, hash)
	if err != nil || lk.ID != k.ID || lu.ID != u.ID {
		t.Fatalf("lookup %+v %+v %v", lk, lu, err)
	}

	b, err := c.CreateBucket(ctx, u.ID, "videos")
	if err != nil {
		t.Fatal(err)
	}
	f, err := c.CreateFolder(ctx, u.ID, b.ID, "/photos")
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := c.RenameFile(ctx, u.ID, f.ID, "/pics")
	if err != nil || renamed.Path != "/pics" {
		t.Fatalf("rename %+v %v", renamed, err)
	}
	if err := c.DeleteFile(ctx, u.ID, f.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteBucket(ctx, u.ID, b.ID); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidJSON(t *testing.T) {
	t.Parallel()
	mux := httpserver.NewMux("metadata")
	Mount(mux, newMem())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/users", bytes.NewBufferString("{"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	var env apierr.Envelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != apierr.CodeInvalidRequest {
		t.Fatalf("%+v", env)
	}
}

func asCall(err error, dest **CallError) bool {
	ce, ok := err.(*CallError)
	if ok {
		*dest = ce
	}
	return ok
}
