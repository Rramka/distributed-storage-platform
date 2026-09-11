// Package tickets issues and verifies signed, time-limited placement and retrieval tickets.
// docs/07-security.md.
package tickets

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	OpPut       = "put"
	OpGet       = "get"
	OpDelete    = "delete"
	OpChallenge = "challenge"

	NonceSize  = 16
	SHA256Size = 32
	DefaultTTL = 15 * time.Minute
	seedHexLen = 64
)

var (
	ErrInvalid   = errors.New("tickets: invalid")
	ErrSignature = errors.New("tickets: bad signature")
	ErrExpired   = errors.New("tickets: expired")
	ErrBound     = errors.New("tickets: binding mismatch")
	ErrOp        = errors.New("tickets: wrong op")
	ErrSeed      = errors.New("tickets: invalid signing seed")
)

// Ticket is a signed capability for one fragment on one node.
type Ticket struct {
	Op         string
	FragmentID uuid.UUID
	NodeID     uuid.UUID
	SHA256     []byte
	MaxBytes   uint64
	ExpiresAt  time.Time
	Nonce      []byte
	// Raw is the wire form after Sign.
	Raw string
}

// Signer issues tickets with an Ed25519 seed.
type Signer struct {
	priv ed25519.PrivateKey
	now  func() time.Time
}

// Verifier checks tickets against a pinned public key.
type Verifier struct {
	pub ed25519.PublicKey
	now func() time.Time
}

// NewSignerFromSeed accepts a 32-byte seed.
func NewSignerFromSeed(seed []byte) (*Signer, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, ErrSeed
	}
	return &Signer{priv: ed25519.NewKeyFromSeed(seed), now: time.Now}, nil
}

// ParseSeed decodes a 64-char hex seed, or SHA-256s a longer secret.
func ParseSeed(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ErrSeed
	}
	if len(s) == seedHexLen {
		b, err := hex.DecodeString(s)
		if err == nil && len(b) == ed25519.SeedSize {
			return b, nil
		}
	}
	sum := sha256.Sum256([]byte(s))
	return sum[:], nil
}

// PublicKey is the 32-byte Ed25519 public key.
func (s *Signer) PublicKey() ed25519.PublicKey {
	return s.priv.Public().(ed25519.PublicKey)
}

// PublicKeyHex is the hex form stored in agent config / TICKET_PUBLIC_KEY.
func (s *Signer) PublicKeyHex() string {
	return hex.EncodeToString(s.PublicKey())
}

// NewVerifierFromHex parses a 32-byte public key.
func NewVerifierFromHex(h string) (*Verifier, error) {
	b, err := hex.DecodeString(strings.TrimSpace(h))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, ErrInvalid
	}
	return &Verifier{pub: b, now: time.Now}, nil
}

// NewVerifier uses a raw public key.
func NewVerifier(pub ed25519.PublicKey) *Verifier {
	return &Verifier{pub: pub, now: time.Now}
}

// Sign encodes and signs t. Nonce and ExpiresAt are filled if empty.
func (s *Signer) Sign(t Ticket) (Ticket, error) {
	if t.Op != OpPut && t.Op != OpGet && t.Op != OpDelete && t.Op != OpChallenge {
		return Ticket{}, ErrOp
	}
	if t.FragmentID == uuid.Nil || t.NodeID == uuid.Nil {
		return Ticket{}, ErrInvalid
	}
	if len(t.SHA256) != SHA256Size {
		return Ticket{}, ErrInvalid
	}
	if len(t.Nonce) == 0 {
		t.Nonce = make([]byte, NonceSize)
		if _, err := rand.Read(t.Nonce); err != nil {
			return Ticket{}, fmt.Errorf("tickets.sign: %w", err)
		}
	}
	if len(t.Nonce) != NonceSize {
		return Ticket{}, ErrInvalid
	}
	if t.ExpiresAt.IsZero() {
		t.ExpiresAt = s.now().Add(DefaultTTL)
	}
	payload := encode(t)
	sig := ed25519.Sign(s.priv, payload)
	t.Raw = encodeWire(payload, sig)
	return t, nil
}

// Parse verifies the signature and decodes fields. Binding checks are Verify.
func (v *Verifier) Parse(wire string) (Ticket, error) {
	payload, sig, err := decodeWire(wire)
	if err != nil {
		return Ticket{}, err
	}
	if !ed25519.Verify(v.pub, payload, sig) {
		return Ticket{}, ErrSignature
	}
	t, err := decode(payload)
	if err != nil {
		return Ticket{}, err
	}
	t.Raw = wire
	if !v.now().Before(t.ExpiresAt) {
		return Ticket{}, ErrExpired
	}
	return t, nil
}

// Verify parses and checks op, node, and fragment bindings.
func (v *Verifier) Verify(wire, op string, nodeID, fragmentID uuid.UUID) (Ticket, error) {
	t, err := v.Parse(wire)
	if err != nil {
		return Ticket{}, err
	}
	if t.Op != op {
		return Ticket{}, ErrOp
	}
	if t.NodeID != nodeID || t.FragmentID != fragmentID {
		return Ticket{}, ErrBound
	}
	return t, nil
}

// EqualSHA reports whether got matches the ticket hash.
func EqualSHA(ticketSHA, got []byte) bool {
	if len(ticketSHA) != SHA256Size || len(got) != SHA256Size {
		return false
	}
	return subtle.ConstantTimeCompare(ticketSHA, got) == 1
}

func encode(t Ticket) []byte {
	var buf []byte
	buf = appendU16String(buf, t.Op)
	buf = append(buf, t.FragmentID[:]...)
	buf = append(buf, t.NodeID[:]...)
	buf = append(buf, t.SHA256...)
	buf = appendU64(buf, t.MaxBytes)
	buf = appendI64(buf, t.ExpiresAt.UTC().Unix())
	buf = append(buf, t.Nonce...)
	return buf
}

func decode(p []byte) (Ticket, error) {
	op, rest, err := takeU16String(p)
	if err != nil {
		return Ticket{}, err
	}
	if len(rest) < 16+16+SHA256Size+8+8+NonceSize {
		return Ticket{}, ErrInvalid
	}
	var frag, node uuid.UUID
	copy(frag[:], rest[:16])
	copy(node[:], rest[16:32])
	sha := append([]byte(nil), rest[32:32+SHA256Size]...)
	rest = rest[32+SHA256Size:]
	max := binary.BigEndian.Uint64(rest[:8])
	exp := int64(binary.BigEndian.Uint64(rest[8:16]))
	nonce := append([]byte(nil), rest[16:16+NonceSize]...)
	if len(rest) != 16+NonceSize {
		return Ticket{}, ErrInvalid
	}
	return Ticket{
		Op:         op,
		FragmentID: frag,
		NodeID:     node,
		SHA256:     sha,
		MaxBytes:   max,
		ExpiresAt:  time.Unix(exp, 0).UTC(),
		Nonce:      nonce,
	}, nil
}

func encodeWire(payload, sig []byte) string {
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func decodeWire(wire string) (payload, sig []byte, err error) {
	i := strings.IndexByte(wire, '.')
	if i <= 0 || i == len(wire)-1 {
		return nil, nil, ErrInvalid
	}
	payload, err = base64.RawURLEncoding.DecodeString(wire[:i])
	if err != nil {
		return nil, nil, ErrInvalid
	}
	sig, err = base64.RawURLEncoding.DecodeString(wire[i+1:])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, nil, ErrInvalid
	}
	return payload, sig, nil
}

func appendU16String(buf []byte, s string) []byte {
	if len(s) > 0xffff {
		s = s[:0xffff]
	}
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(s)))
	return append(buf, s...)
}

func takeU16String(p []byte) (string, []byte, error) {
	if len(p) < 2 {
		return "", nil, ErrInvalid
	}
	n := int(binary.BigEndian.Uint16(p[:2]))
	p = p[2:]
	if len(p) < n {
		return "", nil, ErrInvalid
	}
	return string(p[:n]), p[n:], nil
}

func appendU64(buf []byte, n uint64) []byte {
	return binary.BigEndian.AppendUint64(buf, n)
}

func appendI64(buf []byte, n int64) []byte {
	return binary.BigEndian.AppendUint64(buf, uint64(n))
}
