package receipts

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
)

func TestReceiptRoundTrip(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	node := uuid.New()
	frag := uuid.New()
	sha := Digest([]byte("bytes"))
	r, err := Sign(priv, node, frag, sha, 5, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify(r.Raw, pub, node, frag, sha, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.NodeID != node || got.FragmentID != frag || got.Size != 5 {
		t.Fatalf("%+v", got)
	}
}

func TestReceiptForgeryAndMismatch(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	node := uuid.New()
	frag := uuid.New()
	sha := Digest([]byte("bytes"))
	r, err := Sign(priv, node, frag, sha, 5, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(r.Raw, other.Public().(ed25519.PublicKey), node, frag, sha, 5); err != ErrSignature {
		t.Fatalf("other key: %v", err)
	}
	if _, err := Verify(r.Raw, pub, uuid.New(), frag, sha, 5); err != ErrMismatch {
		t.Fatalf("node: %v", err)
	}
	if _, err := Verify(r.Raw, pub, node, uuid.New(), sha, 5); err != ErrMismatch {
		t.Fatalf("frag: %v", err)
	}
	if _, err := Verify(r.Raw, pub, node, frag, Digest([]byte("x")), 5); err != ErrMismatch {
		t.Fatalf("sha: %v", err)
	}
	if _, err := Verify(r.Raw, pub, node, frag, sha, 6); err != ErrMismatch {
		t.Fatalf("size: %v", err)
	}
	if _, err := Parse("not-a-receipt", pub); err != ErrInvalid {
		t.Fatalf("junk: %v", err)
	}
}

func TestPropertyMutatedFieldRejected(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fn := func(n uint8) bool {
		sha := Digest([]byte("bytes"))
		r, err := Sign(priv, uuid.New(), uuid.New(), sha, 5, time.Unix(1_700_000_000, 0))
		if err != nil {
			return false
		}
		parts := strings.Split(r.Raw, ".")
		if len(parts) != 2 {
			return false
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil || len(payload) == 0 {
			return false
		}
		payload[int(n)%len(payload)] ^= 0x01
		wire := base64.RawURLEncoding.EncodeToString(payload) + "." + parts[1]
		_, err = Parse(wire, pub)
		return err != nil
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 64}); err != nil {
		t.Fatal(err)
	}
}

func FuzzReceiptParse(f *testing.F) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		f.Fatal(err)
	}
	sha := Digest([]byte("bytes"))
	r, err := Sign(priv, uuid.New(), uuid.New(), sha, 5, time.Unix(1_700_000_000, 0))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(r.Raw)
	f.Add("not-a-receipt")
	f.Add("")
	f.Add(".")
	f.Add("abc.def")
	f.Fuzz(func(t *testing.T, wire string) {
		_, _ = Parse(wire, pub)
	})
}
