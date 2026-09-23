package store

import (
	"testing"
	"time"
)

func TestAdmitNode(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	exp := now.Add(time.Hour)
	if err := AdmitNode(Node{Status: "online", CertExpiresAt: &exp}, now); err != nil {
		t.Fatal(err)
	}
	rev := now
	if err := AdmitNode(Node{Status: "online", CertExpiresAt: &exp, RevokedAt: &rev}, now); err == nil {
		t.Fatal("revoked")
	}
	if err := AdmitNode(Node{Status: "quarantined", CertExpiresAt: &exp}, now); err == nil {
		t.Fatal("quarantined")
	}
	past := now.Add(-time.Hour)
	if err := AdmitNode(Node{Status: "online", CertExpiresAt: &past}, now); err == nil {
		t.Fatal("expired")
	}
	if err := AdmitNodeRenew(Node{Status: "online", CertExpiresAt: &past}); err != nil {
		t.Fatal("renew should allow expired")
	}
}
