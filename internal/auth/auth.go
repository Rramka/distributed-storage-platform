// Package auth hashes passwords (argon2id) and mints API keys (docs/07-security.md).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	keyPrefix = "dsp_"
	keyBytes  = 32

	argonTime    = 1
	argonMemory  = 32 * 1024
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

var (
	ErrPasswordMismatch = errors.New("auth: password mismatch")
	ErrInvalidHash      = errors.New("auth: invalid password hash")
)

// HashPassword returns a PHC-encoded argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth.hashPassword: %w", err)
	}
	sum := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

// VerifyPassword checks password against a PHC-encoded argon2id hash.
func VerifyPassword(encoded, password string) error {
	salt, want, time, memory, threads, keyLen, err := parseArgon2id(encoded)
	if err != nil {
		return err
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	if subtle.ConstantTimeCompare(want, got) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

func parseArgon2id(encoded string) (salt, hash []byte, time, memory uint32, threads uint8, keyLen uint32, err error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}
	if len(hash) == 0 || len(salt) == 0 {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}
	return salt, hash, time, memory, threads, uint32(len(hash)), nil
}

// APIKey is a freshly minted secret plus the hash stored in Postgres.
type APIKey struct {
	Secret string
	Hash   string
}

// MintAPIKey returns a 256-bit random secret with a dsp_ prefix.
func MintAPIKey() (APIKey, error) {
	raw := make([]byte, keyBytes)
	if _, err := rand.Read(raw); err != nil {
		return APIKey{}, fmt.Errorf("auth.mintAPIKey: %w", err)
	}
	secret := keyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return APIKey{Secret: secret, Hash: HashAPIKey(secret)}, nil
}

// HashAPIKey is the SHA-256 hex digest stored in api_keys.key_hash.
func HashAPIKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ValidSecretFormat reports whether secret looks like a minted key.
func ValidSecretFormat(secret string) bool {
	if !strings.HasPrefix(secret, keyPrefix) {
		return false
	}
	payload := strings.TrimPrefix(secret, keyPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	return err == nil && len(decoded) == keyBytes
}
