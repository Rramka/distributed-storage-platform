package auth

import (
	"strings"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		password string
		attempt  string
		wantErr  error
	}{
		{name: "match", password: "correct horse", attempt: "correct horse"},
		{name: "mismatch", password: "correct horse", attempt: "wrong", wantErr: ErrPasswordMismatch},
		{name: "empty attempt", password: "secret", attempt: "", wantErr: ErrPasswordMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hash, err := HashPassword(tt.password)
			if err != nil {
				t.Fatalf("HashPassword: %v", err)
			}
			if !strings.HasPrefix(hash, "$argon2id$") {
				t.Fatalf("encoding: %q", hash)
			}
			err = VerifyPassword(hash, tt.attempt)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("VerifyPassword: %v", err)
				}
				return
			}
			if err == nil || err != tt.wantErr {
				t.Fatalf("VerifyPassword err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestHashPasswordUniqueSalt(t *testing.T) {
	t.Parallel()
	a, err := HashPassword("same")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("same")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected distinct salts")
	}
}

func TestVerifyPasswordInvalidHash(t *testing.T) {
	t.Parallel()
	tests := []string{"", "not-argon", "$argon2id$v=19$bad", "$argon2i$v=19$m=1,t=1,p=1$YQ$YQ"}
	for _, h := range tests {
		if err := VerifyPassword(h, "x"); err != ErrInvalidHash {
			t.Fatalf("hash %q: err = %v, want ErrInvalidHash", h, err)
		}
	}
}

func TestMintAPIKey(t *testing.T) {
	t.Parallel()
	k, err := MintAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidSecretFormat(k.Secret) {
		t.Fatalf("secret format: %q", k.Secret)
	}
	if len(k.Hash) != 64 {
		t.Fatalf("hash len %d", len(k.Hash))
	}
	if HashAPIKey(k.Secret) != k.Hash {
		t.Fatal("hash mismatch")
	}
	other, err := MintAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if k.Secret == other.Secret {
		t.Fatal("expected unique secrets")
	}
}

func TestValidSecretFormat(t *testing.T) {
	t.Parallel()
	good, err := MintAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		secret string
		want   bool
	}{
		{good.Secret, true},
		{"", false},
		{"dsp_short", false},
		{"nope", false},
		{"DSP_" + strings.TrimPrefix(good.Secret, "dsp_"), false},
	}
	for _, tt := range tests {
		if got := ValidSecretFormat(tt.secret); got != tt.want {
			t.Fatalf("ValidSecretFormat(%q) = %v, want %v", tt.secret, got, tt.want)
		}
	}
}

func TestHashAPIKeyDeterministic(t *testing.T) {
	t.Parallel()
	secret := "dsp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if HashAPIKey(secret) != HashAPIKey(secret) {
		t.Fatal("hash not deterministic")
	}
}
