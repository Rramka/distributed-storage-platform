package agent

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestChunkStorePutGet(t *testing.T) {
	t.Parallel()
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	id := uuid.New()
	payload := []byte("ciphertext-not-plaintext")
	sum := sha256.Sum256(payload)
	meta, err := st.Put(id, bytes.NewReader(payload), sum[:], uint64(len(payload)+10), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Size != int64(len(payload)) {
		t.Fatalf("size %d", meta.Size)
	}
	f, got, err := st.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, _ := io.ReadAll(f)
	if !bytes.Equal(raw, payload) {
		t.Fatal("bytes")
	}
	if got.Size != meta.Size {
		t.Fatal("meta")
	}
	if _, err := st.Put(uuid.New(), bytes.NewReader(payload), bytes.Repeat([]byte{1}, 32), 100, time.Now().Add(time.Hour)); err != ErrHash {
		t.Fatalf("hash: %v", err)
	}
	if err := st.ConsumeNonce([]byte("0123456789abcdef"), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.ConsumeNonce([]byte("0123456789abcdef"), time.Now().Add(time.Minute)); err != ErrReplay {
		t.Fatalf("replay: %v", err)
	}
}

func TestPutRejectsOverMax(t *testing.T) {
	t.Parallel()
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	payload := []byte("abcd")
	sum := sha256.Sum256(payload)
	if _, err := st.Put(uuid.New(), bytes.NewReader(payload), sum[:], 3, time.Now().Add(time.Hour)); err != ErrTooLarge {
		t.Fatalf("%v", err)
	}
}

func TestDataDirHasNoPlaintextMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	marker := []byte("PLAINTEXT_SECRET_MARKER")
	sum := sha256.Sum256([]byte("cipher"))
	if _, err := st.Put(uuid.New(), bytes.NewReader([]byte("cipher")), sum[:], 100, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	err = filepathWalkContains(dir, marker)
	if err != nil {
		t.Fatal(err)
	}
}

func filepathWalkContains(root string, needle []byte) error {
	return walk(root, needle)
}

func walk(root string, needle []byte) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(b, needle) {
			return fmtContains(path)
		}
		return nil
	})
}

func fmtContains(path string) error {
	return errContains{path}
}

type errContains struct{ path string }

func (e errContains) Error() string { return "plaintext in " + e.path }
