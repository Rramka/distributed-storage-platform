// Package agent is the storage node: chunk store, fragment API, registration, heartbeats.
// docs/05-node-agent.md. Scrub, GC, bandwidth limiter, and self-update are deferred to M4.
package agent

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Config is agent.yaml plus env overrides.
type Config struct {
	DataDir          string `yaml:"data_dir"`
	MaxStorageGB     int64  `yaml:"max_storage_gb"`
	MinFreeDiskGB    int64  `yaml:"min_free_disk_gb"`
	Endpoint         string `yaml:"endpoint"`
	RegisterURL      string `yaml:"register_url"`
	HealthmonURL     string `yaml:"healthmon_url"`
	TicketPublicKey  string `yaml:"ticket_public_key"`
	RegistrationCode string `yaml:"registration_code"`
	HealthAddr       string `yaml:"health_addr"`
	FragmentAddr     string `yaml:"fragment_addr"`
	CAFile           string `yaml:"ca_file"`
	OS               string `yaml:"os"`
	AgentVersion     string `yaml:"agent_version"`
	HostnameLabel    string `yaml:"hostname_label"`
}

// LoadConfig reads yamlPath (optional) then env.
func LoadConfig(yamlPath string) (Config, error) {
	cfg := Config{
		DataDir:       "/var/lib/storage-agent",
		MaxStorageGB:  10,
		MinFreeDiskGB: 1,
		HealthAddr:    ":9000",
		FragmentAddr:  ":7443",
		AgentVersion:  "m2",
		OS:            detectOS(),
	}
	if yamlPath != "" {
		b, err := os.ReadFile(yamlPath)
		if err != nil && !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("agent.loadConfig: %w", err)
		}
		if err == nil {
			if err := yaml.Unmarshal(b, &cfg); err != nil {
				return Config{}, fmt.Errorf("agent.loadConfig: %w", err)
			}
		}
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv("REGISTER_URL"); v != "" {
		cfg.RegisterURL = v
	}
	if v := os.Getenv("HEALTHMON_URL"); v != "" {
		cfg.HealthmonURL = v
	}
	if v := os.Getenv("TICKET_PUBLIC_KEY"); v != "" {
		cfg.TicketPublicKey = v
	}
	if v := os.Getenv("TICKET_PUBLIC_KEY_FILE"); v != "" {
		b, err := os.ReadFile(v)
		if err != nil {
			return Config{}, fmt.Errorf("agent.loadConfig: %w", err)
		}
		cfg.TicketPublicKey = strings.TrimSpace(string(b))
	}
	if v := os.Getenv("REGISTRATION_CODE"); v != "" {
		cfg.RegistrationCode = v
	}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		cfg.HealthAddr = v
	}
	if v := os.Getenv("FRAGMENT_ADDR"); v != "" {
		cfg.FragmentAddr = v
	}
	if v := os.Getenv("CA_FILE"); v != "" {
		cfg.CAFile = v
	}
	if v := os.Getenv("MAX_STORAGE_GB"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		if n > 0 {
			cfg.MaxStorageGB = n
		}
	}
	if cfg.Endpoint == "" || cfg.RegisterURL == "" || cfg.TicketPublicKey == "" {
		return Config{}, fmt.Errorf("agent.loadConfig: endpoint, register_url, ticket_public_key required")
	}
	return cfg, nil
}

func detectOS() string {
	return runtime.GOOS
}

func identityDir(dataDir string) string {
	return filepath.Join(dataDir, "identity")
}

func fragmentsDir(dataDir string) string {
	return filepath.Join(dataDir, "fragments")
}

func metaDBPath(dataDir string) string {
	return filepath.Join(dataDir, "meta.db")
}

func loadVerifier(hexKey string) (*tickets.Verifier, error) {
	return tickets.NewVerifierFromHex(hexKey)
}

func loadCACert(path string) (*x509.Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("agent.ca: %w", err)
	}
	return ca.ParseCertificatePEM(b)
}

func tlsCert(priv ed25519.PrivateKey, certPEM []byte) (tls.Certificate, error) {
	keyPEM, err := ca.EncodeKeyPEM(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	return ca.TLSCertFromPEM(certPEM, keyPEM)
}

func parseNodeIDFile(dataDir string) (uuid.UUID, error) {
	b, err := os.ReadFile(filepath.Join(identityDir(dataDir), "node.id"))
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(string(b))
}

func writeNodeID(dataDir string, id uuid.UUID) error {
	return os.WriteFile(filepath.Join(identityDir(dataDir), "node.id"), []byte(id.String()), 0o644)
}

func capacityBytes(cfg Config) int64 {
	return cfg.MaxStorageGB * 1024 * 1024 * 1024
}

func heartbeatEvery() time.Duration { return 10 * time.Second }
