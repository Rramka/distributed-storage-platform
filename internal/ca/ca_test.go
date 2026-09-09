package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEnsureCAIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := EnsureCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EnsureCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Cert.Equal(b.Cert) {
		t.Fatal("expected same CA on reload")
	}
}

func TestIssueNodeSANClamp(t *testing.T) {
	t.Parallel()
	c, err := EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		endpoint string
		wantDNS  string
		wantIP   net.IP
		wantErr  error
	}{
		{name: "dns hostport", endpoint: "agent1:7443", wantDNS: "agent1"},
		{name: "localhost", endpoint: "localhost:7443", wantDNS: "localhost"},
		{name: "ipv4", endpoint: "127.0.0.1:7443", wantIP: net.ParseIP("127.0.0.1")},
		{name: "bad host", endpoint: "not a host!:1", wantErr: ErrSANClamp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			csr, err := CreateCSR(priv)
			if err != nil {
				t.Fatal(err)
			}
			id := uuid.New()
			pem, cert, err := c.IssueNode(csr, id, tt.endpoint, time.Hour)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(pem) == 0 {
				t.Fatal("empty pem")
			}
			if cert.Subject.CommonName != id.String() {
				t.Fatalf("cn %q", cert.Subject.CommonName)
			}
			if tt.wantDNS != "" {
				if len(cert.DNSNames) != 1 || cert.DNSNames[0] != tt.wantDNS {
					t.Fatalf("dns %v", cert.DNSNames)
				}
			}
			if tt.wantIP != nil {
				if len(cert.IPAddresses) != 1 || !cert.IPAddresses[0].Equal(tt.wantIP) {
					t.Fatalf("ips %v", cert.IPAddresses)
				}
			}
			if err := cert.CheckSignatureFrom(c.Cert); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIssueNodeIgnoresCSRSans(t *testing.T) {
	t.Parallel()
	c, err := EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := CreateCSR(priv)
	if err != nil {
		t.Fatal(err)
	}
	_, cert, err := c.IssueNode(csr, uuid.New(), "127.0.0.1:7443", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range cert.DNSNames {
		if d == "evil.example" {
			t.Fatal("csr SAN leaked")
		}
	}
	roots := x509.NewCertPool()
	roots.AddCert(c.Cert)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintStable(t *testing.T) {
	t.Parallel()
	c, err := EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := Fingerprint(c.Cert)
	b := Fingerprint(c.Cert)
	if len(a) != 32 || string(a) != string(b) {
		t.Fatal("fingerprint")
	}
}

func TestBadCSR(t *testing.T) {
	t.Parallel()
	c, err := EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.IssueNode([]byte("nope"), uuid.New(), "host:1", time.Hour); err == nil {
		t.Fatal("expected invalid csr")
	}
}
