package healthmon

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
)

func TestComputeChallengeSetRoundTrip(t *testing.T) {
	t.Parallel()
	data := bytes.Repeat([]byte("cipher"), 200)
	fid, nid := uuid.New(), uuid.New()
	set := ComputeChallengeSet(data, fid, nid, 8, 64)
	if len(set) != 8 {
		t.Fatalf("len %d", len(set))
	}
	for _, c := range set {
		if c.Offset < 0 || c.Length <= 0 || c.Offset+c.Length > len(data) {
			t.Fatalf("range %d+%d of %d", c.Offset, c.Length, len(data))
		}
		h := sha256.New()
		_, _ = h.Write(c.Nonce)
		_, _ = h.Write(data[c.Offset : c.Offset+c.Length])
		if !bytesEqual(h.Sum(nil), c.Expected) {
			t.Fatal("expected mismatch")
		}
		if c.FragmentID != fid || c.NodeID != nid {
			t.Fatal("ids")
		}
	}
}

func TestComputeChallengeSetTinyFragment(t *testing.T) {
	t.Parallel()
	data := []byte("ab")
	set := ComputeChallengeSet(data, uuid.New(), uuid.New(), 3, 4096)
	if len(set) != 3 {
		t.Fatalf("len %d", len(set))
	}
	for _, c := range set {
		if c.Offset+c.Length > len(data) {
			t.Fatalf("oob %d+%d", c.Offset, c.Length)
		}
	}
}

func TestChallengerAuditPassFail(t *testing.T) {
	t.Parallel()
	data := bytes.Repeat([]byte("z"), 128)
	fid, nid := uuid.New(), uuid.New()
	set := ComputeChallengeSet(data, fid, nid, 1, 16)
	ch := set[0]

	gotPass := sha256.Sum256(append(append([]byte{}, ch.Nonce...), data[ch.Offset:ch.Offset+ch.Length]...))
	if !bytesEqual(gotPass[:], ch.Expected) {
		t.Fatal("self-check")
	}
	wrong := sha256.Sum256([]byte("nope"))
	if bytesEqual(wrong[:], ch.Expected) {
		t.Fatal("wrong should not match")
	}
}
