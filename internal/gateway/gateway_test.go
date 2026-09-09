package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/auth"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/ratelimit"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type fakeMeta struct {
	mu      sync.Mutex
	users   map[string]store.User
	pass    map[uuid.UUID]string
	keys    map[string]store.APIKey
	buckets map[uuid.UUID][]store.Bucket
	files   map[uuid.UUID]store.File
}

func newFake() *fakeMeta {
	return &fakeMeta{
		users:   map[string]store.User{},
		pass:    map[uuid.UUID]string{},
		keys:    map[string]store.APIKey{},
		buckets: map[uuid.UUID][]store.Bucket{},
		files:   map[uuid.UUID]store.File{},
	}
}

func (m *fakeMeta) CreateUser(_ context.Context, email, password string) (store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[email]; ok {
		return store.User{}, metadata.ErrConflict
	}
	if email == "" || len(password) < 8 {
		return store.User{}, metadata.ErrInvalid
	}
	u := store.User{ID: uuid.New(), Email: email, Role: "customer", Status: "active", CreatedAt: time.Now().UTC()}
	m.users[email] = u
	m.pass[u.ID] = password
	return u, nil
}

func (m *fakeMeta) VerifyPassword(_ context.Context, email, password string) (store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[email]
	if !ok || m.pass[u.ID] != password {
		return store.User{}, metadata.ErrUnauthenticated
	}
	return u, nil
}

func (m *fakeMeta) LookupAPIKey(_ context.Context, keyHash string) (store.APIKey, store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[keyHash]
	if !ok {
		return store.APIKey{}, store.User{}, metadata.ErrUnauthenticated
	}
	for _, u := range m.users {
		if u.ID == k.UserID {
			return k, u, nil
		}
	}
	return store.APIKey{}, store.User{}, metadata.ErrNotFound
}

func (m *fakeMeta) CreateAPIKey(_ context.Context, userID uuid.UUID, keyHash, label string, scopes []string, expiresAt *time.Time) (store.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if label == "" {
		return store.APIKey{}, metadata.ErrInvalid
	}
	if len(scopes) == 0 {
		scopes = []string{"read", "write"}
	}
	k := store.APIKey{ID: uuid.New(), UserID: userID, KeyHash: keyHash, Label: label, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: time.Now().UTC()}
	m.keys[keyHash] = k
	return k, nil
}

func (m *fakeMeta) ListAPIKeys(_ context.Context, userID uuid.UUID) ([]store.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.APIKey
	for _, k := range m.keys {
		if k.UserID == userID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (m *fakeMeta) RevokeAPIKey(_ context.Context, userID, keyID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for h, k := range m.keys {
		if k.ID == keyID && k.UserID == userID {
			delete(m.keys, h)
			return nil
		}
	}
	return metadata.ErrNotFound
}

func (m *fakeMeta) CreateBucket(_ context.Context, userID uuid.UUID, name string) (store.Bucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := store.Bucket{ID: uuid.New(), OwnerID: userID, Name: name, CreatedAt: time.Now().UTC()}
	m.buckets[userID] = append(m.buckets[userID], b)
	return b, nil
}

func (m *fakeMeta) ListBuckets(_ context.Context, userID uuid.UUID) ([]store.Bucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buckets[userID], nil
}

func (m *fakeMeta) DeleteBucket(_ context.Context, userID, bucketID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.buckets[userID]
	for i, b := range list {
		if b.ID == bucketID {
			for _, f := range m.files {
				if f.BucketID == bucketID {
					return metadata.ErrNotEmpty
				}
			}
			m.buckets[userID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return metadata.ErrNotFound
}

func (m *fakeMeta) CreateFolder(_ context.Context, userID, bucketID uuid.UUID, path string) (store.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := store.File{ID: uuid.New(), BucketID: bucketID, Path: path, IsFolder: true, Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	m.files[f.ID] = f
	return f, nil
}

func (m *fakeMeta) ListFiles(_ context.Context, userID, bucketID uuid.UUID, prefix, cursor string, limit int) ([]store.File, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.File
	for _, f := range m.files {
		if f.BucketID == bucketID {
			out = append(out, f)
		}
	}
	return out, "", nil
}

func (m *fakeMeta) GetFile(_ context.Context, userID, fileID uuid.UUID) (store.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[fileID]
	if !ok {
		return store.File{}, metadata.ErrNotFound
	}
	return f, nil
}

func (m *fakeMeta) RenameFile(_ context.Context, userID, fileID uuid.UUID, newPath string) (store.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[fileID]
	if !ok {
		return store.File{}, metadata.ErrNotFound
	}
	f.Path = newPath
	m.files[fileID] = f
	return f, nil
}

func (m *fakeMeta) DeleteFile(_ context.Context, userID, fileID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[fileID]; !ok {
		return metadata.ErrNotFound
	}
	delete(m.files, fileID)
	return nil
}

func testServer(t *testing.T, meta metadata.Service) http.Handler {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	mux := httpserver.NewMux("gateway")
	return New(mux, meta, ratelimit.New(rdb))
}

func TestRegisterKeyAndFolderCRUD(t *testing.T) {
	t.Parallel()
	h := testServer(t, newFake())

	reg := doJSON(t, h, http.MethodPost, "/v1/auth/register", `{"email":"ada@example.com","password":"password1"}`, "", "")
	if reg.Code != http.StatusCreated {
		t.Fatalf("register %d %s", reg.Code, reg.Body.String())
	}
	var user map[string]any
	decodeBody(t, reg, &user)
	if user["email"] != "ada@example.com" || user["hint"] == nil {
		t.Fatalf("user %+v", user)
	}

	createKey := httptest.NewRequest(http.MethodPost, "/v1/auth/api-keys", bytes.NewBufferString(`{"label":"cli"}`))
	createKey.SetBasicAuth("ada@example.com", "password1")
	createKey.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, createKey)
	if rec.Code != http.StatusCreated {
		t.Fatalf("api-keys %d %s", rec.Code, rec.Body.String())
	}
	var key map[string]any
	decodeBody(t, rec, &key)
	secret, _ := key["secret"].(string)
	if !auth.ValidSecretFormat(secret) {
		t.Fatalf("secret %v", key["secret"])
	}

	bkt := doJSON(t, h, http.MethodPost, "/v1/buckets", `{"name":"videos"}`, secret, "")
	if bkt.Code != http.StatusCreated {
		t.Fatalf("bucket %d %s", bkt.Code, bkt.Body.String())
	}
	var bucket store.Bucket
	decodeBody(t, bkt, &bucket)

	folderBody := `{"bucket_id":"` + bucket.ID.String() + `","path":"/photos/2026"}`
	fold := doJSON(t, h, http.MethodPost, "/v1/folders", folderBody, secret, "")
	if fold.Code != http.StatusCreated {
		t.Fatalf("folder %d %s", fold.Code, fold.Body.String())
	}
	var folder store.File
	decodeBody(t, fold, &folder)

	listed := doJSON(t, h, http.MethodGet, "/v1/files?bucket_id="+bucket.ID.String(), "", secret, "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list %d %s", listed.Code, listed.Body.String())
	}

	ren := doJSON(t, h, http.MethodPut, "/v1/rename", `{"file_id":"`+folder.ID.String()+`","new_path":"/pics"}`, secret, "")
	if ren.Code != http.StatusOK {
		t.Fatalf("rename %d %s", ren.Code, ren.Body.String())
	}

	del := doJSON(t, h, http.MethodDelete, "/v1/file/"+folder.ID.String(), "", secret, "")
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete file %d %s", del.Code, del.Body.String())
	}
	delB := doJSON(t, h, http.MethodDelete, "/v1/buckets/"+bucket.ID.String(), "", secret, "")
	if delB.Code != http.StatusNoContent {
		t.Fatalf("delete bucket %d %s", delB.Code, delB.Body.String())
	}
}

func TestUnauthenticated(t *testing.T) {
	t.Parallel()
	h := testServer(t, newFake())
	rec := doJSON(t, h, http.MethodGet, "/v1/buckets", "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	var env apierr.Envelope
	decodeBody(t, rec, &env)
	if env.Error.Code != apierr.CodeUnauthenticated || env.Error.RequestID == "" {
		t.Fatalf("%+v", env.Error)
	}
}

func TestUnknownRouteEnvelope(t *testing.T) {
	t.Parallel()
	h := testServer(t, newFake())
	rec := doJSON(t, h, http.MethodGet, "/v1/nope", "", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	var env apierr.Envelope
	decodeBody(t, rec, &env)
	if env.Error.Code != apierr.CodeNotFound {
		t.Fatalf("%+v", env.Error)
	}
}

func TestAuthRateLimit(t *testing.T) {
	t.Parallel()
	h := testServer(t, newFake())
	var last *httptest.ResponseRecorder
	for i := 0; i < ratelimit.AuthLimit+1; i++ {
		last = doJSON(t, h, http.MethodPost, "/v1/auth/register", `{"email":"x@example.com","password":"password1"}`, "", "203.0.113.9")
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d body %s", last.Code, last.Body.String())
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()
	h := testServer(t, newFake())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	hz := httptest.NewRecorder()
	h.ServeHTTP(hz, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if hz.Code != http.StatusOK {
		t.Fatalf("healthz %d", hz.Code)
	}
}

func doJSON(t *testing.T, h http.Handler, method, path, body, apiKey, ip string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(dst); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
}
