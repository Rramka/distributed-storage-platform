package receipts

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEgressReceiptNonceRoundTrip(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 16)
	for i := range nonce {
		nonce[i] = byte(i + 1)
	}
	node, frag := uuid.New(), uuid.New()
	sha := Digest([]byte("frag"))
	r, err := SignEgress(priv, node, frag, sha, 99, time.Unix(1_700_000_000, 0), nonce)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(r.Raw, pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nonce) != 16 || got.Nonce[0] != 1 || got.Size != 99 {
		t.Fatalf("%+v", got)
	}
	put, err := Sign(priv, node, frag, sha, 99, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Parse(put.Raw, pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Nonce) != 0 {
		t.Fatalf("put nonce %v", p2.Nonce)
	}
}
