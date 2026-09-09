package tickets

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
)

func testSigner(t *testing.T) *Signer {
	t.Helper()
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	s, err := NewSignerFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validTicket() Ticket {
	sum := sha256.Sum256([]byte("frag"))
	return Ticket{
		Op:         OpPut,
		FragmentID: uuid.New(),
		NodeID:     uuid.New(),
		SHA256:     sum[:],
		MaxBytes:   100,
		ExpiresAt:  time.Now().Add(time.Hour),
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	s := testSigner(t)
	v := NewVerifier(s.PublicKey())
	want := validTicket()
	got, err := s.Sign(want)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := v.Verify(got.Raw, OpPut, want.NodeID, want.FragmentID)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.FragmentID != want.FragmentID || parsed.MaxBytes != want.MaxBytes {
		t.Fatalf("%+v", parsed)
	}
	if !EqualSHA(parsed.SHA256, want.SHA256) {
		t.Fatal("sha")
	}
}

func TestRejectExpiredWrongNodeWrongFragmentTamper(t *testing.T) {
	t.Parallel()
	s := testSigner(t)
	v := NewVerifier(s.PublicKey())
	base := validTicket()

	expired := base
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	et, err := s.Sign(expired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Parse(et.Raw); err != ErrExpired {
		t.Fatalf("expired: %v", err)
	}

	good, err := s.Sign(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(good.Raw, OpPut, uuid.New(), base.FragmentID); err != ErrBound {
		t.Fatalf("wrong node: %v", err)
	}
	if _, err := v.Verify(good.Raw, OpPut, base.NodeID, uuid.New()); err != ErrBound {
		t.Fatalf("wrong frag: %v", err)
	}
	if _, err := v.Verify(good.Raw, OpGet, base.NodeID, base.FragmentID); err != ErrOp {
		t.Fatalf("wrong op: %v", err)
	}

	parts := strings.Split(good.Raw, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[0])
	payload[0] ^= 0xff
	tampered := base64.RawURLEncoding.EncodeToString(payload) + "." + parts[1]
	if _, err := v.Parse(tampered); err != ErrSignature {
		t.Fatalf("tamper: %v", err)
	}
}

func TestPropertyMutatedFieldRejected(t *testing.T) {
	t.Parallel()
	s := testSigner(t)
	v := NewVerifier(s.PublicKey())
	fn := func(n uint8) bool {
		tk := validTicket()
		signed, err := s.Sign(tk)
		if err != nil {
			return false
		}
		parts := strings.Split(signed.Raw, ".")
		payload, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil || len(payload) == 0 {
			return false
		}
		payload[int(n)%len(payload)] ^= 0x01
		wire := base64.RawURLEncoding.EncodeToString(payload) + "." + parts[1]
		_, err = v.Parse(wire)
		return err != nil
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 64}); err != nil {
		t.Fatal(err)
	}
}

func TestParseSeed(t *testing.T) {
	t.Parallel()
	hexSeed := strings.Repeat("ab", 32)
	a, err := ParseSeed(hexSeed)
	if err != nil || len(a) != 32 {
		t.Fatalf("%v %d", err, len(a))
	}
	b, err := ParseSeed(hexSeed)
	if err != nil || string(a) != string(b) {
		t.Fatal("hex seed not stable")
	}
	c, err := ParseSeed("passphrase-not-hex")
	if err != nil || len(c) != 32 {
		t.Fatal(err)
	}
}

func TestNewSignerRejectsBadSeed(t *testing.T) {
	t.Parallel()
	if _, err := NewSignerFromSeed([]byte("short")); err != ErrSeed {
		t.Fatalf("%v", err)
	}
}
