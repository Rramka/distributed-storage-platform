package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestScrubDeletesCorruptFragment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	id := uuid.New()
	payload := []byte("good-ciphertext")
	sum := sha256.Sum256(payload)
	if _, err := st.Put(id, bytes.NewReader(payload), sum[:], 64, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.pathFor(id), []byte("tampered-bytes!!!!"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &Agent{store: st}
	if err := a.scrubOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.Get(id); err != ErrNotStored {
		t.Fatalf("corrupt fragment still present: %v", err)
	}
}
