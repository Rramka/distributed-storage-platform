package agent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/google/uuid"
)

type identity struct {
	ID   uuid.UUID
	Priv ed25519.PrivateKey
	Cert []byte
}

func loadOrCreateKey(dataDir string) (ed25519.PrivateKey, error) {
	if err := os.MkdirAll(identityDir(dataDir), 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(identityDir(dataDir), "node.key")
	if b, err := os.ReadFile(path); err == nil {
		return ca.ParseKeyPEM(b)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("agent.key: %w", err)
	}
	pem, err := ca.EncodeKeyPEM(priv)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem, 0o600); err != nil {
		return nil, fmt.Errorf("agent.key: %w", err)
	}
	return priv, nil
}

func loadIdentity(dataDir string) (identity, bool, error) {
	priv, err := loadOrCreateKey(dataDir)
	if err != nil {
		return identity{}, false, err
	}
	certPEM, err := os.ReadFile(filepath.Join(identityDir(dataDir), "node.crt"))
	if err != nil {
		return identity{Priv: priv}, false, nil
	}
	id, err := parseNodeIDFile(dataDir)
	if err != nil {
		return identity{}, false, err
	}
	return identity{ID: id, Priv: priv, Cert: certPEM}, true, nil
}

func persistIdentity(dataDir string, id uuid.UUID, certPEM []byte) error {
	if err := os.WriteFile(filepath.Join(identityDir(dataDir), "node.crt"), certPEM, 0o644); err != nil {
		return err
	}
	return writeNodeID(dataDir, id)
}

type registerResponse struct {
	NodeID  string `json:"node_id"`
	CertPEM string `json:"cert_pem"`
}

func register(ctx context.Context, cfg Config, priv ed25519.PrivateKey) (identity, error) {
	csr, err := ca.CreateCSR(priv)
	if err != nil {
		return identity{}, err
	}
	body, _ := json.Marshal(map[string]any{
		"registration_code": cfg.RegistrationCode,
		"csr":               base64.StdEncoding.EncodeToString(csr),
		"endpoint":          cfg.Endpoint,
		"os":                cfg.OS,
		"agent_version":     cfg.AgentVersion,
		"hostname_label":    cfg.HostnameLabel,
		"capacity_bytes":    capacityBytes(cfg),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.RegisterURL+"/internal/nodes/register", bytes.NewReader(body))
	if err != nil {
		return identity{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return identity{}, fmt.Errorf("agent.register: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return identity{}, fmt.Errorf("agent.register: http %d %s", resp.StatusCode, raw)
	}
	var out registerResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return identity{}, fmt.Errorf("agent.register: %w", err)
	}
	id, err := uuid.Parse(out.NodeID)
	if err != nil {
		return identity{}, fmt.Errorf("agent.register: %w", err)
	}
	certPEM := []byte(out.CertPEM)
	if err := persistIdentity(cfg.DataDir, id, certPEM); err != nil {
		return identity{}, err
	}
	return identity{ID: id, Priv: priv, Cert: certPEM}, nil
}

func waitForCode(path string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		b, err := os.ReadFile(path)
		if err == nil && len(bytes.TrimSpace(b)) > 0 {
			return string(bytes.TrimSpace(b)), nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("agent: registration code file %s not ready", path)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
