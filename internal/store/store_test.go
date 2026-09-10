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

func TestCommitThreshold(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		stored  int
		wantErr error
	}{
		{"13 rejected", 13, ErrIncomplete},
		{"14 commits", 14, nil},
		{"15 commits", 15, nil},
		{"16 commits", 16, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := s.CreateUser(ctx, "ct-"+uuid.NewString()+"@example.com", "hash")
			if err != nil {
				t.Fatal(err)
			}
			b, err := s.CreateBucket(ctx, u.ID, "b")
			if err != nil {
				t.Fatal(err)
			}
			sha := bytes32(1)
			frags := make([]ManifestFragment, 16)
			for i := range frags {
				frags[i] = ManifestFragment{ShardIndex: int16(i), SizeBytes: 32, SHA256: bytes32(byte(i + 2))}
			}
			planned, err := s.BeginUpload(ctx, u.ID, UploadManifest{
				BucketID:       b.ID,
				Path:           "/f-" + uuid.NewString(),
				SizeBytes:      100,
				ContentSHA256:  sha,
				EncryptionMeta: []byte(`{"algo":"aes-256-gcm"}`),
				ChunkSize:      16 * 1024 * 1024,
				ECData:         10,
				ECParity:       6,
				Chunks:         []ManifestChunk{{Seq: 0, SizeBytes: 100, SHA256: sha, Fragments: frags}},
			})
			if err != nil {
				t.Fatal(err)
			}
			node, err := s.CreateNode(ctx, CreateNodeParams{
				OwnerID:         u.ID,
				CertFingerprint: []byte(uuid.NewString()),
				PublicKey:       bytes32(3),
				CertPEM:         "pem",
				CertExpiresAt:   time.Now().Add(time.Hour),
				OS:              "linux",
				AgentVersion:    "test",
				Endpoint:        "127.0.0.1:9",
				CapacityBytes:   1 << 30,
			})
			if err != nil {
				t.Fatal(err)
			}
			var rows []Placement
			ids := make([]uuid.UUID, 0, len(planned.Pending))
			for _, pf := range planned.Pending {
				rows = append(rows, Placement{FragmentID: pf.Fragment.ID, NodeID: node.ID})
				ids = append(ids, pf.Fragment.ID)
			}
			if err := s.InsertPlacements(ctx, rows); err != nil {
				t.Fatal(err)
			}
			_, _, err = s.CommitUpload(ctx, u.ID, planned.Version.ID, ids[:tt.stored])
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("got %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTouchNodeLargeCapacity(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "cap-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.CreateNode(ctx, CreateNodeParams{
		OwnerID:         u.ID,
		CertFingerprint: []byte(uuid.NewString()),
		PublicKey:       bytes32(9),
		CertPEM:         "pem",
		CertExpiresAt:   time.Now().Add(time.Hour),
		OS:              "linux",
		AgentVersion:    "test",
		Endpoint:        "127.0.0.1:1",
		CapacityBytes:   5 << 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TouchNode(ctx, n.ID, 0, 5<<30); err != nil {
		t.Fatalf("TouchNode 5GiB: %v", err)
	}
	got, err := s.NodeByID(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "online" {
		t.Fatalf("status %q", got.Status)
	}
}

func TestTransitionStaleNodes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "stale-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.CreateNode(ctx, CreateNodeParams{
		OwnerID:         u.ID,
		CertFingerprint: []byte(uuid.NewString()),
		PublicKey:       bytes32(1),
		CertPEM:         "pem",
		CertExpiresAt:   time.Now().Add(time.Hour),
		OS:              "linux",
		AgentVersion:    "test",
		Endpoint:        "127.0.0.1:9",
		CapacityBytes:   1 << 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TouchNode(ctx, n.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.ForceNodeLastSeen(ctx, n.ID, time.Now().Add(-45*time.Second)); err != nil {
		t.Fatal(err)
	}
	ts, err := s.TransitionStaleNodes(ctx, 30*time.Second, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tr := range ts {
		if tr.ID == n.ID {
			found = true
			if tr.From != "online" || tr.To != "suspect" {
				t.Fatalf("got %+v", tr)
			}
		}
	}
	if !found {
		t.Fatal("expected suspect transition")
	}
	if err := s.ForceNodeLastSeen(ctx, n.ID, time.Now().Add(-6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	ts, err = s.TransitionStaleNodes(ctx, 30*time.Second, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, tr := range ts {
		if tr.ID == n.ID {
			found = true
			if tr.To != "offline" {
				t.Fatalf("got %+v", tr)
			}
		}
	}
	if !found {
		t.Fatal("expected offline transition")
	}
	from, to, err := s.TouchNodeStatus(ctx, n.ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if from != "offline" || to != "online" {
		t.Fatalf("recovery %s -> %s", from, to)
	}
}

func bytes32(seed byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed
	}
	return b
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
