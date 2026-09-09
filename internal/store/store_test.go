package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres integration test")
	}
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://dsp:dsp@127.0.0.1:5433/dsp?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s, err := Open(ctx, url)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"/photos/2026", "/photos/2026", false},
		{"/photos/2026/", "/photos/2026", false},
		{"/", "", true},
		{"photos", "", true},
		{"/photos/../etc", "", true},
		{"//photos", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizePath(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q err %v, want %q", got, err, tt.want)
			}
		})
	}
}

func TestUserBucketFolderCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	email := "m1-" + uuid.NewString() + "@example.com"

	u, err := s.CreateUser(ctx, email, "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Email != email {
		t.Fatalf("email %q", u.Email)
	}
	_, err = s.CreateUser(ctx, email, "hash")
	if err != ErrConflict {
		t.Fatalf("duplicate user: %v", err)
	}

	got, err := s.UserByEmail(ctx, stringsUpper(email))
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserByEmail: %+v %v", got, err)
	}

	b, err := s.CreateBucket(ctx, u.ID, "videos")
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	_, err = s.CreateBucket(ctx, u.ID, "videos")
	if err != ErrConflict {
		t.Fatalf("duplicate bucket: %v", err)
	}

	folder, err := s.CreateFolder(ctx, u.ID, b.ID, "/photos/2026")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	child, err := s.CreateFolder(ctx, u.ID, b.ID, "/photos/2026/july")
	if err != nil {
		t.Fatalf("child folder: %v", err)
	}

	listed, next, err := s.ListFiles(ctx, u.ID, b.ID, "/photos", "", 10)
	if err != nil || next != "" || len(listed) != 2 {
		t.Fatalf("list: n=%d next=%q err=%v", len(listed), next, err)
	}

	renamed, err := s.RenameFile(ctx, u.ID, folder.ID, "/pics")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Path != "/pics" {
		t.Fatalf("renamed path %q", renamed.Path)
	}
	childGot, err := s.FileByID(ctx, u.ID, child.ID)
	if err != nil || childGot.Path != "/pics/july" {
		t.Fatalf("child after rename: %+v %v", childGot, err)
	}

	if err := s.DeleteBucket(ctx, u.ID, b.ID); err != ErrNotEmpty {
		t.Fatalf("delete non-empty: %v", err)
	}
	if err := s.SoftDeleteFile(ctx, u.ID, folder.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := s.FileByID(ctx, u.ID, child.ID); err != ErrNotFound {
		t.Fatalf("child still visible: %v", err)
	}
	if err := s.DeleteBucket(ctx, u.ID, b.ID); err != nil {
		t.Fatalf("delete empty: %v", err)
	}
}

func TestAPIKeyLookupAndRevoke(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "key-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	hash := "deadbeef" + uuid.NewString()
	k, err := s.CreateAPIKey(ctx, u.ID, hash, "cli", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	got, err := s.APIKeyByHash(ctx, hash)
	if err != nil || got.ID != k.ID {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	if err := s.RevokeAPIKey(ctx, u.ID, k.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.APIKeyByHash(ctx, hash); err != ErrNotFound {
		t.Fatalf("revoked still active: %v", err)
	}
	if err := s.RevokeAPIKey(ctx, u.ID, k.ID); err != ErrNotFound {
		t.Fatalf("double revoke: %v", err)
	}
}

func TestOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, err := s.CreateUser(ctx, "a-"+uuid.NewString()+"@example.com", "h")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateUser(ctx, "b-"+uuid.NewString()+"@example.com", "h")
	if err != nil {
		t.Fatal(err)
	}
	bucket, err := s.CreateBucket(ctx, a.ID, "private")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BucketByID(ctx, b.ID, bucket.ID); err != ErrNotFound {
		t.Fatalf("cross-owner bucket: %v", err)
	}
	folder, err := s.CreateFolder(ctx, a.ID, bucket.ID, "/secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileByID(ctx, b.ID, folder.ID); err != ErrNotFound {
		t.Fatalf("cross-owner file: %v", err)
	}
}

func stringsUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
