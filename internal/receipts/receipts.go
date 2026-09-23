// Package receipts verifies node-signed storage receipts at upload commit.
// docs/07-security.md.
package receipts

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const shaSize = 32

var (
	ErrInvalid   = errors.New("receipts: invalid")
	ErrSignature = errors.New("receipts: bad signature")
	ErrMismatch  = errors.New("receipts: field mismatch")
)

// Receipt is a node-signed acknowledgement that a fragment was stored or served.
type Receipt struct {
	NodeID     uuid.UUID
	FragmentID uuid.UUID
	SHA256     []byte
	Size       uint64
	StoredAt   time.Time
	Nonce      []byte
	Raw        string
}

// Sign encodes and signs a receipt with the node's identity key.
func Sign(priv ed25519.PrivateKey, nodeID, fragmentID uuid.UUID, sha []byte, size uint64, storedAt time.Time) (Receipt, error) {
	return sign(priv, nodeID, fragmentID, sha, size, storedAt, nil)
}

// SignEgress signs a GET receipt bound to the retrieval ticket nonce.
func SignEgress(priv ed25519.PrivateKey, nodeID, fragmentID uuid.UUID, sha []byte, size uint64, storedAt time.Time, nonce []byte) (Receipt, error) {
	if len(nonce) != 16 {
		return Receipt{}, ErrInvalid
	}
	return sign(priv, nodeID, fragmentID, sha, size, storedAt, nonce)
}

func sign(priv ed25519.PrivateKey, nodeID, fragmentID uuid.UUID, sha []byte, size uint64, storedAt time.Time, nonce []byte) (Receipt, error) {
	if len(priv) != ed25519.PrivateKeySize || len(sha) != shaSize {
		return Receipt{}, ErrInvalid
	}
	if nodeID == uuid.Nil || fragmentID == uuid.Nil {
		return Receipt{}, ErrInvalid
	}
	r := Receipt{
		NodeID:     nodeID,
		FragmentID: fragmentID,
		SHA256:     append([]byte(nil), sha...),
		Size:       size,
		StoredAt:   storedAt.UTC(),
		Nonce:      append([]byte(nil), nonce...),
	}
	payload := encode(r)
	sig := ed25519.Sign(priv, payload)
	r.Raw = encodeWire(payload, sig)
	return r, nil
}

// Parse verifies the signature against nodePub and decodes fields.
func Parse(wire string, nodePub ed25519.PublicKey) (Receipt, error) {
	payload, sig, err := decodeWire(wire)
	if err != nil {
		return Receipt{}, err
	}
	if len(nodePub) != ed25519.PublicKeySize || !ed25519.Verify(nodePub, payload, sig) {
		return Receipt{}, ErrSignature
	}
	r, err := decode(payload)
	if err != nil {
		return Receipt{}, err
	}
	r.Raw = wire
	return r, nil
}

// Verify checks the receipt matches the expected fragment, hash, and size.
func Verify(wire string, nodePub ed25519.PublicKey, nodeID, fragmentID uuid.UUID, wantSHA []byte, wantSize uint64) (Receipt, error) {
	r, err := Parse(wire, nodePub)
	if err != nil {
		return Receipt{}, err
	}
	if r.NodeID != nodeID || r.FragmentID != fragmentID {
		return Receipt{}, ErrMismatch
	}
	if r.Size != wantSize {
		return Receipt{}, ErrMismatch
	}
	if len(wantSHA) != shaSize || subtle.ConstantTimeCompare(r.SHA256, wantSHA) != 1 {
		return Receipt{}, ErrMismatch
	}
	return r, nil
}

func encode(r Receipt) []byte {
	var buf []byte
	buf = append(buf, r.NodeID[:]...)
	buf = append(buf, r.FragmentID[:]...)
	buf = append(buf, r.SHA256...)
	buf = binary.BigEndian.AppendUint64(buf, r.Size)
	buf = binary.BigEndian.AppendUint64(buf, uint64(r.StoredAt.UTC().Unix()))
	if len(r.Nonce) == 16 {
		buf = append(buf, r.Nonce...)
	}
	return buf
}

func decode(p []byte) (Receipt, error) {
	base := 16 + 16 + shaSize + 8 + 8
	if len(p) != base && len(p) != base+16 {
		return Receipt{}, ErrInvalid
	}
	var node, frag uuid.UUID
	copy(node[:], p[:16])
	copy(frag[:], p[16:32])
	sha := append([]byte(nil), p[32:32+shaSize]...)
	size := binary.BigEndian.Uint64(p[32+shaSize : 40+shaSize])
	exp := int64(binary.BigEndian.Uint64(p[40+shaSize : 48+shaSize]))
	r := Receipt{
		NodeID:     node,
		FragmentID: frag,
		SHA256:     sha,
		Size:       size,
		StoredAt:   time.Unix(exp, 0).UTC(),
	}
	if len(p) == base+16 {
		r.Nonce = append([]byte(nil), p[base:]...)
	}
	return r, nil
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

// ParseUnsigned decodes receipt fields without verifying the signature.
func ParseUnsigned(wire string) (Receipt, error) {
	payload, _, err := decodeWire(wire)
	if err != nil {
		return Receipt{}, err
	}
	r, err := decode(payload)
	if err != nil {
		return Receipt{}, err
	}
	r.Raw = wire
	return r, nil
}

// Digest is a helper for tests.
func Digest(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
