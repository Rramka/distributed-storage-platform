package metadata

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func TestRenewCertificateKeepsPublicKey(t *testing.T) {
	s := store.OpenForTest(t)
	platform, err := ca.EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := &StoreService{Store: s, IssueNode: IssueNodeFromCA(platform)}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := ca.CreateCSR(priv)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.CreateUser(context.Background(), "renew-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	nodeID := uuid.New()
	pem1, pub, fp, exp, err := svc.IssueNode(csr, nodeID, "127.0.0.1:7443")
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.CreateNode(context.Background(), store.CreateNodeParams{
		ID: nodeID, OwnerID: owner.ID, CertFingerprint: fp, PublicKey: pub,
		CertPEM: string(pem1), CertExpiresAt: exp, OS: "linux", AgentVersion: "t",
		Endpoint: "127.0.0.1:7443", CapacityBytes: 1 << 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	csr2, err := ca.CreateCSR(priv)
	if err != nil {
		t.Fatal(err)
	}
	pem2, err := svc.RenewCertificate(context.Background(), n.ID, csr2, fp)
	if err != nil {
		t.Fatal(err)
	}
	if pem2 == "" || pem2 == string(pem1) {
		t.Fatal("expected a new certificate pem")
	}
	got, err := s.NodeByID(context.Background(), n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.PublicKey) != string(n.PublicKey) {
		t.Fatal("public key changed")
	}
	if string(got.CertFingerprint) == string(n.CertFingerprint) {
		t.Fatal("fingerprint should rotate")
	}
	if _, err := svc.RenewCertificate(context.Background(), n.ID, csr2, fp); err == nil {
		t.Fatal("superseded fingerprint must not renew")
	}
	if err := s.RevokeNode(context.Background(), n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RenewCertificate(context.Background(), n.ID, csr2, got.CertFingerprint); err == nil {
		t.Fatal("revoked renew")
	}
}

func TestQuotaCheckBlocksPlanUpload(t *testing.T) {
	t.Parallel()
	svc := &StoreService{
		QuotaCheck: func(context.Context, uuid.UUID) error { return ErrQuota },
		Place:      func(context.Context, []PlaceNeed) ([]PlaceAssign, error) { return nil, nil },
		SignTicket: func(string, uuid.UUID, uuid.UUID, []byte, uint64) (string, time.Time, error) {
			return "t", time.Now(), nil
		},
	}
	_, err := svc.PlanUpload(context.Background(), uuid.New(), store.UploadManifest{})
	if err != ErrQuota {
		t.Fatalf("got %v", err)
	}
}
