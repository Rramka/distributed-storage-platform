package metadata

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

const codeTTL = 24 * time.Hour

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func mintRegSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "reg_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// MintRegistrationCode creates a one-time agent bind code.
func (s *StoreService) MintRegistrationCode(ctx context.Context, userID uuid.UUID, endpoint, country, region string, asn *int) (store.RegistrationCode, string, error) {
	if endpoint == "" || country == "" || region == "" || asn == nil {
		return store.RegistrationCode{}, "", ErrInvalid
	}
	secret, err := mintRegSecret()
	if err != nil {
		return store.RegistrationCode{}, "", err
	}
	c, err := s.Store.MintRegistrationCode(ctx, userID, hashSecret(secret), endpoint, country, region, asn, time.Now().Add(codeTTL))
	if err != nil {
		return store.RegistrationCode{}, "", err
	}
	return c, secret, nil
}

// ListNodes returns the provider's nodes.
func (s *StoreService) ListNodes(ctx context.Context, userID uuid.UUID) ([]store.Node, error) {
	return s.Store.ListNodesByOwner(ctx, userID)
}

// RegisterNode consumes a code, issues a cert, and inserts the node atomically.
func (s *StoreService) RegisterNode(ctx context.Context, code string, csr []byte, endpoint, osName, version, label string, capacity int64) (store.Node, string, error) {
	if s.IssueNode == nil {
		return store.Node{}, "", fmt.Errorf("metadata.registerNode: CA not configured")
	}
	if code == "" || len(csr) == 0 {
		return store.Node{}, "", ErrInvalid
	}
	hashed := hashSecret(code)
	rc, err := s.Store.RegistrationCodeByHash(ctx, hashed)
	if err != nil {
		return store.Node{}, "", err
	}
	if rc.UsedAt != nil {
		return store.Node{}, "", ErrConflict
	}
	if time.Now().After(rc.ExpiresAt) {
		return store.Node{}, "", ErrNotFound
	}
	_ = endpoint
	nodeID := uuid.New()
	certPEM, pub, fp, expires, err := s.IssueNode(csr, nodeID, rc.Endpoint)
	if err != nil {
		return store.Node{}, "", err
	}
	if osName == "" {
		osName = "linux"
	}
	if version == "" {
		version = "m2"
	}
	if capacity <= 0 {
		capacity = 10 << 30
	}
	n, err := s.Store.RegisterNode(ctx, hashed, store.CreateNodeParams{
		ID:              nodeID,
		CertFingerprint: fp,
		PublicKey:       pub,
		CertPEM:         string(certPEM),
		CertExpiresAt:   expires,
		HostnameLabel:   label,
		OS:              osName,
		AgentVersion:    version,
		CapacityBytes:   capacity,
	})
	if err != nil {
		if err == store.ErrConflict {
			return store.Node{}, "", ErrConflict
		}
		return store.Node{}, "", err
	}
	_ = s.Store.InsertAudit(ctx, "node", &n.ID, "node.register", "node", &n.ID, map[string]any{"owner_id": n.OwnerID.String()})
	return n, string(certPEM), nil
}

// RenewCertificate issues a new leaf for the same public key.
// peerFP must match the registry fingerprint so a superseded leaf cannot rotate the node.
func (s *StoreService) RenewCertificate(ctx context.Context, nodeID uuid.UUID, csr, peerFP []byte) (string, error) {
	if s.IssueNode == nil || len(csr) == 0 {
		return "", ErrInvalid
	}
	n, err := s.Store.NodeByID(ctx, nodeID)
	if err != nil {
		return "", err
	}
	if err := store.AdmitNodeRenew(n); err != nil {
		return "", ErrForbidden
	}
	if !bytesEqual(peerFP, n.CertFingerprint) {
		return "", ErrForbidden
	}
	endpoint := n.Endpoint
	certPEM, pub, fp, expires, err := s.IssueNode(csr, nodeID, endpoint)
	if err != nil {
		return "", err
	}
	if !bytesEqual(pub, n.PublicKey) {
		return "", ErrInvalid
	}
	if err := s.Store.RotateNodeCert(ctx, nodeID, fp, string(certPEM), expires); err != nil {
		return "", err
	}
	_ = s.Store.InsertAudit(ctx, "node", &nodeID, "node.renew", "node", &nodeID, nil)
	return string(certPEM), nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// HeartbeatNode updates durable liveness fields.
func (s *StoreService) HeartbeatNode(ctx context.Context, nodeID uuid.UUID, used, capacity int64) error {
	return s.Store.TouchNode(ctx, nodeID, used, capacity)
}

// IssueNodeFromCA is the default IssueNode using a CA.
func IssueNodeFromCA(c *ca.CA) func(csr []byte, nodeID uuid.UUID, endpoint string) ([]byte, []byte, []byte, time.Time, error) {
	return func(csr []byte, nodeID uuid.UUID, endpoint string) ([]byte, []byte, []byte, time.Time, error) {
		pem, cert, err := c.IssueNode(csr, nodeID, endpoint, ca.NodeTTL)
		if err != nil {
			return nil, nil, nil, time.Time{}, err
		}
		pub, ok := cert.PublicKey.(ed25519.PublicKey)
		if !ok {
			return nil, nil, nil, time.Time{}, fmt.Errorf("metadata: cert public key is not ed25519")
		}
		return pem, []byte(pub), ca.Fingerprint(cert), cert.NotAfter, nil
	}
}
