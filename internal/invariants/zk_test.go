package invariants

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckZeroKnowledge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frag"), []byte("ciphertext-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	needle := []byte("PLAINTEXT_SECRET_MARKER")
	if err := CheckZeroKnowledge([]string{dir}, nil, [][]byte{needle}); err != nil {
		t.Fatal(err)
	}
	if err := CheckZeroKnowledge([]string{dir}, [][]byte{[]byte("wrap " + string(needle))}, [][]byte{needle}); err == nil {
		t.Fatal("expected dump hit")
	}
}
