package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestRegisterRefusesPlaintextHTTP(t *testing.T) {
	t.Parallel()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, err = register(context.Background(), Config{
		RegisterURL: "http://metadata:8081",
		CAFile:      "/ca/ca.crt",
	}, priv)
	if err == nil {
		t.Fatal("expected plaintext registration to fail")
	}
}

func TestRegisterClientRequiresHTTPSAndCA(t *testing.T) {
	t.Parallel()
	if _, err := registerClient(Config{RegisterURL: "http://127.0.0.1:8081", CAFile: "x"}); err == nil {
		t.Fatal("http should fail")
	}
	if _, err := registerClient(Config{RegisterURL: "https://metadata:8444"}); err == nil {
		t.Fatal("missing CA should fail")
	}
}
