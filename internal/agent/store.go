package agent

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

var (
	ErrNotStored = errors.New("agent: fragment not stored")
	ErrReplay    = errors.New("agent: ticket nonce replayed")
	ErrHash      = errors.New("agent: fragment hash mismatch")
	ErrTooLarge  = errors.New("agent: fragment exceeds max_bytes")
)

var (
	bktMeta  = []byte("fragments")
	bktNonce = []byte("nonces")
)

// FragMeta is the local index record.
type FragMeta struct {
	Size         int64     `json:"size"`
	SHA256       []byte    `json:"sha256"`
	ExpiresAt    time.Time `json:"expires_at"`
	StoredAt     time.Time `json:"stored_at"`
	LastVerified time.Time `json:"last_verified"`
}

// ChunkStore is a crash-safe fragment directory plus bbolt index.
type ChunkStore struct {
	dir string
	db  *bbolt.DB
}

// OpenStore creates data_dir/fragments and opens meta.db.
func OpenStore(dataDir string) (*ChunkStore, error) {
	if err := os.MkdirAll(fragmentsDir(dataDir), 0o755); err != nil {
		return nil, fmt.Errorf("agent.openStore: %w", err)
	}
	db, err := bbolt.Open(metaDBPath(dataDir), 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("agent.openStore: %w", err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bktMeta); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bktNonce); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("agent.openStore: %w", err)
	}
	return &ChunkStore{dir: fragmentsDir(dataDir), db: db}, nil
}

// Close releases the index.
func (s *ChunkStore) Close() error { return s.db.Close() }

func (s *ChunkStore) pathFor(id uuid.UUID) string {
	h := strings.ReplaceAll(id.String(), "-", "")
	if len(h) < 4 {
		h = h + "0000"
	}
	return filepath.Join(s.dir, h[:2], h[2:4], id.String()+".frag")
}

// Put streams r to a temp file, fsyncs, verifies hash, renames, then indexes.
func (s *ChunkStore) Put(id uuid.UUID, r io.Reader, wantSHA []byte, maxBytes uint64, expires time.Time) (FragMeta, error) {
	final := s.pathFor(id)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return FragMeta{}, fmt.Errorf("agent.put: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(final), ".put-*")
	if err != nil {
		return FragMeta{}, fmt.Errorf("agent.put: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	h := sha256.New()
	limit := int64(maxBytes) + 1
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, limit))
	if err != nil {
		_ = tmp.Close()
		return FragMeta{}, fmt.Errorf("agent.put: %w", err)
	}
	if uint64(n) > maxBytes {
		_ = tmp.Close()
		return FragMeta{}, ErrTooLarge
	}
	sum := h.Sum(nil)
	if !equalSHA(wantSHA, sum) {
		_ = tmp.Close()
		return FragMeta{}, ErrHash
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return FragMeta{}, fmt.Errorf("agent.put: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return FragMeta{}, fmt.Errorf("agent.put: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		return FragMeta{}, fmt.Errorf("agent.put: %w", err)
	}
	now := time.Now().UTC()
	meta := FragMeta{Size: n, SHA256: sum, ExpiresAt: expires.UTC(), StoredAt: now, LastVerified: now}
	raw, _ := json.Marshal(meta)
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bktMeta).Put(id[:], raw)
	})
	if err != nil {
		_ = os.Remove(final)
		return FragMeta{}, fmt.Errorf("agent.put: %w", err)
	}
	return meta, nil
}

// Get opens a stored fragment.
func (s *ChunkStore) Get(id uuid.UUID) (*os.File, FragMeta, error) {
	var meta FragMeta
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bktMeta).Get(id[:])
		if v == nil {
			return ErrNotStored
		}
		return json.Unmarshal(v, &meta)
	})
	if err != nil {
		return nil, FragMeta{}, err
	}
	f, err := os.Open(s.pathFor(id))
	if err != nil {
		return nil, FragMeta{}, fmt.Errorf("agent.get: %w", err)
	}
	return f, meta, nil
}

// Delete removes a fragment and its index row.
func (s *ChunkStore) Delete(id uuid.UUID) error {
	_ = os.Remove(s.pathFor(id))
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bktMeta).Delete(id[:])
	})
}

// UsedBytes sums indexed sizes.
func (s *ChunkStore) UsedBytes() (int64, int) {
	var used int64
	var n int
	_ = s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bktMeta).ForEach(func(_, v []byte) error {
			var m FragMeta
			if json.Unmarshal(v, &m) == nil {
				used += m.Size
				n++
			}
			return nil
		})
	})
	return used, n
}

// ConsumeNonce rejects replays until expiry.
func (s *ChunkStore) ConsumeNonce(nonce []byte, exp time.Time) error {
	now := time.Now()
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bktNonce)
		if v := b.Get(nonce); v != nil {
			until := int64(binary.BigEndian.Uint64(v))
			if now.Unix() < until {
				return ErrReplay
			}
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(exp.Unix()))
		return b.Put(append([]byte(nil), nonce...), buf)
	})
}

func equalSHA(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
