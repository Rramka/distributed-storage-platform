// Package pipeline is the client-side encryption, wrapping, and chunking path.
// docs/04-storage-pipeline.md. Reed-Solomon is M3; M2 stores one fragment per chunk.
package pipeline

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	AlgoGCM        = "aes-256-gcm"
	WrapGCM        = "aes-256-gcm"
	KDFName        = "argon2id"
	FileKeySize    = 32
	MasterKeySize  = 32
	SegmentSize    = 64 * 1024
	GCMTagSize     = 16
	NoncePrefixLen = 4
	WrapNonceLen   = 12
	DefaultChunk   = 16 * 1024 * 1024

	DefaultMemory  = 64 * 1024
	DefaultTime    = 3
	DefaultThreads = 4
	SaltLen        = 16
)

var (
	ErrAuth       = errors.New("pipeline: authentication failed")
	ErrPassphrase = errors.New("pipeline: wrong passphrase")
	ErrMeta       = errors.New("pipeline: invalid encryption_meta")
	ErrKey        = errors.New("pipeline: invalid key")
	ErrTruncated  = errors.New("pipeline: truncated ciphertext")
)

// KDFParams are stored in encryption_meta so they can be strengthened later.
type KDFParams struct {
	Name    string `json:"name"`
	Memory  uint32 `json:"memory"`
	Time    uint32 `json:"time"`
	Threads uint8  `json:"threads"`
	Salt    []byte `json:"salt"`
}

// EncryptionMeta is persisted on file_versions.encryption_meta.
type EncryptionMeta struct {
	Algo        string    `json:"algo"`
	Wrap        string    `json:"wrap"`
	WrappedFK   []byte    `json:"wrapped_fk"`
	WrapNonce   []byte    `json:"wrap_nonce"`
	KDF         KDFParams `json:"kdf"`
	NoncePrefix []byte    `json:"nonce_prefix"`
}

// DefaultKDF returns the production Argon2id parameters from the spec.
func DefaultKDF() KDFParams {
	return KDFParams{Name: KDFName, Memory: DefaultMemory, Time: DefaultTime, Threads: DefaultThreads}
}

// TestKDF is a cheap parameter set for unit tests.
func TestKDF() KDFParams {
	return KDFParams{Name: KDFName, Memory: 8 * 1024, Time: 1, Threads: 1}
}

// DeriveMasterKey runs Argon2id. Salt is generated if empty.
func DeriveMasterKey(passphrase string, p KDFParams) ([]byte, KDFParams, error) {
	if p.Name == "" {
		p.Name = KDFName
	}
	if p.Name != KDFName {
		return nil, p, ErrMeta
	}
	if p.Memory == 0 {
		p.Memory = DefaultMemory
	}
	if p.Time == 0 {
		p.Time = DefaultTime
	}
	if p.Threads == 0 {
		p.Threads = DefaultThreads
	}
	if len(p.Salt) == 0 {
		p.Salt = make([]byte, SaltLen)
		if _, err := rand.Read(p.Salt); err != nil {
			return nil, p, fmt.Errorf("pipeline.derive: %w", err)
		}
	}
	mk := argon2.IDKey([]byte(passphrase), p.Salt, p.Time, p.Memory, p.Threads, MasterKeySize)
	return mk, p, nil
}

// GenerateFileKey returns 32 random bytes.
func GenerateFileKey() ([]byte, error) {
	fk := make([]byte, FileKeySize)
	if _, err := rand.Read(fk); err != nil {
		return nil, fmt.Errorf("pipeline.fileKey: %w", err)
	}
	return fk, nil
}

// WrapFileKey seals FK under MK with AES-256-GCM (solo-track wrap; not AES-KW).
func WrapFileKey(mk, fk []byte) (wrapped, nonce []byte, err error) {
	if len(mk) != MasterKeySize || len(fk) != FileKeySize {
		return nil, nil, ErrKey
	}
	nonce = make([]byte, WrapNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("pipeline.wrap: %w", err)
	}
	aead, err := gcm(mk)
	if err != nil {
		return nil, nil, err
	}
	return aead.Seal(nil, nonce, fk, []byte("fk")), nonce, nil
}

// UnwrapFileKey opens a wrapped FK.
func UnwrapFileKey(mk, wrapped, nonce []byte) ([]byte, error) {
	if len(mk) != MasterKeySize {
		return nil, ErrKey
	}
	aead, err := gcm(mk)
	if err != nil {
		return nil, err
	}
	fk, err := aead.Open(nil, nonce, wrapped, []byte("fk"))
	if err != nil {
		return nil, ErrPassphrase
	}
	return fk, nil
}

// NewMeta builds encryption_meta around a wrapped FK.
func NewMeta(p KDFParams, wrapped, wrapNonce, noncePrefix []byte) EncryptionMeta {
	return EncryptionMeta{
		Algo:        AlgoGCM,
		Wrap:        WrapGCM,
		WrappedFK:   wrapped,
		WrapNonce:   wrapNonce,
		KDF:         p,
		NoncePrefix: noncePrefix,
	}
}

// MarshalMeta encodes encryption_meta as JSON.
func MarshalMeta(m EncryptionMeta) (json.RawMessage, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// UnmarshalMeta decodes encryption_meta.
func UnmarshalMeta(raw json.RawMessage) (EncryptionMeta, error) {
	var m EncryptionMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return EncryptionMeta{}, ErrMeta
	}
	if m.Algo != AlgoGCM || m.Wrap != WrapGCM || len(m.NoncePrefix) != NoncePrefixLen {
		return EncryptionMeta{}, ErrMeta
	}
	return m, nil
}

// UnlockFK derives MK from the passphrase and unwraps the file key.
func UnlockFK(passphrase string, m EncryptionMeta) ([]byte, error) {
	mk, _, err := DeriveMasterKey(passphrase, m.KDF)
	if err != nil {
		return nil, err
	}
	return UnwrapFileKey(mk, m.WrappedFK, m.WrapNonce)
}

// Encrypt streams plaintext from src to dst as GCM segments.
// Ciphertext layout: [4-byte nonce prefix] then sealed 64 KiB segments (last may be short).
func Encrypt(dst io.Writer, src io.Reader, fk []byte) (noncePrefix []byte, plainSize, cipherSize int64, contentSHA []byte, err error) {
	if len(fk) != FileKeySize {
		return nil, 0, 0, nil, ErrKey
	}
	aead, err := gcm(fk)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	noncePrefix = make([]byte, NoncePrefixLen)
	if _, err := rand.Read(noncePrefix); err != nil {
		return nil, 0, 0, nil, fmt.Errorf("pipeline.encrypt: %w", err)
	}
	h := sha256.New()
	w := io.MultiWriter(dst, h)
	if _, err := w.Write(noncePrefix); err != nil {
		return nil, 0, 0, nil, err
	}
	cipherSize = int64(NoncePrefixLen)

	buf := make([]byte, SegmentSize)
	var seq uint64
	var pending []byte
	for {
		n, readErr := io.ReadFull(src, buf)
		if n > 0 {
			if pending != nil {
				if err := writeSeg(w, aead, noncePrefix, seq, pending, true, &cipherSize); err != nil {
					return nil, 0, 0, nil, err
				}
				seq++
			}
			pending = append([]byte(nil), buf[:n]...)
			plainSize += int64(n)
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			if pending == nil {
				if err := writeSeg(w, aead, noncePrefix, 0, nil, false, &cipherSize); err != nil {
					return nil, 0, 0, nil, err
				}
			} else {
				if err := writeSeg(w, aead, noncePrefix, seq, pending, false, &cipherSize); err != nil {
					return nil, 0, 0, nil, err
				}
			}
			return noncePrefix, plainSize, cipherSize, h.Sum(nil), nil
		}
		if readErr != nil {
			return nil, 0, 0, nil, readErr
		}
	}
}

func writeSeg(w io.Writer, aead cipher.AEAD, prefix []byte, seq uint64, plain []byte, more bool, cipherSize *int64) error {
	sealed := aead.Seal(nil, segmentNonce(prefix, seq), plain, aad(seq, more))
	if _, err := w.Write(sealed); err != nil {
		return err
	}
	*cipherSize += int64(len(sealed))
	return nil
}

// Decrypt streams ciphertext from src to dst. Fails closed on any tag mismatch or truncation.
func Decrypt(dst io.Writer, src io.Reader, fk, noncePrefix []byte) error {
	if len(fk) != FileKeySize || len(noncePrefix) != NoncePrefixLen {
		return ErrKey
	}
	aead, err := gcm(fk)
	if err != nil {
		return err
	}
	gotPrefix := make([]byte, NoncePrefixLen)
	if _, err := io.ReadFull(src, gotPrefix); err != nil {
		return ErrTruncated
	}
	if subtle.ConstantTimeCompare(gotPrefix, noncePrefix) != 1 {
		return ErrAuth
	}

	segCipher := SegmentSize + GCMTagSize
	buf := make([]byte, segCipher)
	var seq uint64
	var pending []byte
	for {
		n, readErr := io.ReadFull(src, buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			if readErr == nil {
				if pending != nil {
					if err := openSegment(aead, dst, noncePrefix, seq, pending, true); err != nil {
						return err
					}
					seq++
				}
				pending = chunk
				continue
			}
			if readErr == io.ErrUnexpectedEOF || readErr == io.EOF {
				if pending != nil {
					if err := openSegment(aead, dst, noncePrefix, seq, pending, true); err != nil {
						return err
					}
					seq++
				}
				if err := openSegment(aead, dst, noncePrefix, seq, chunk, false); err != nil {
					return err
				}
				return nil
			}
			return readErr
		}
		if readErr == io.EOF {
			if pending != nil {
				return openSegment(aead, dst, noncePrefix, seq, pending, false)
			}
			return ErrTruncated
		}
		if readErr != nil {
			return readErr
		}
	}
}

func openSegment(aead cipher.AEAD, dst io.Writer, prefix []byte, seq uint64, sealed []byte, more bool) error {
	if len(sealed) < GCMTagSize {
		return ErrTruncated
	}
	plain, err := aead.Open(nil, segmentNonce(prefix, seq), sealed, aad(seq, more))
	if err != nil {
		return ErrAuth
	}
	_, err = dst.Write(plain)
	return err
}

// ChunkInfo is one ciphertext chunk (M2: also the single fragment).
type ChunkInfo struct {
	Seq       int
	SizeBytes int
	SHA256    []byte
}

// SplitChunks reads ciphertext and invokes emit for each chunk.
func SplitChunks(r io.Reader, chunkSize int, emit func(ChunkInfo, []byte) error) error {
	if chunkSize <= 0 {
		chunkSize = DefaultChunk
	}
	buf := make([]byte, chunkSize)
	seq := 0
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			sum := sha256.Sum256(buf[:n])
			cp := append([]byte(nil), buf[:n]...)
			if e := emit(ChunkInfo{Seq: seq, SizeBytes: n, SHA256: sum[:]}, cp); e != nil {
				return e
			}
			seq++
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			if seq == 0 {
				sum := sha256.Sum256(nil)
				return emit(ChunkInfo{Seq: 0, SizeBytes: 0, SHA256: sum[:]}, nil)
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func gcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("pipeline.aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("pipeline.gcm: %w", err)
	}
	return aead, nil
}

func segmentNonce(prefix []byte, seq uint64) []byte {
	n := make([]byte, 12)
	copy(n, prefix)
	binary.BigEndian.PutUint64(n[4:], seq)
	return n
}

func aad(seq uint64, more bool) []byte {
	var b [9]byte
	binary.BigEndian.PutUint64(b[:8], seq)
	if more {
		b[8] = 1
	}
	return b[:]
}
