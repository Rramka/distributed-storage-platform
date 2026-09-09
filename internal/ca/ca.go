// Package ca is the private CA used to mint node mTLS certificates at registration.
// docs/05-node-agent.md, docs/07-security.md.
package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	caCertName = "ca.crt"
	caKeyName  = "ca.key"
	caKeyPerm  = 0o600
	caTTL      = 10 * 365 * 24 * time.Hour
	NodeTTL    = 30 * 24 * time.Hour
)

var (
	ErrInvalidCSR = errors.New("ca: invalid csr")
	ErrSANClamp   = errors.New("ca: endpoint host is not a valid SAN")
)

// CA holds the platform private CA.
type CA struct {
	Cert *x509.Certificate
	Key  ed25519.PrivateKey
	DER  []byte
}

// EnsureCA loads an existing CA from dir or creates a self-signed Ed25519 CA.
func EnsureCA(dir string) (*CA, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("ca.ensureCA: %w", err)
	}
	certPath := filepath.Join(dir, caCertName)
	keyPath := filepath.Join(dir, caKeyName)
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return Load(certPath, keyPath)
		}
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ca.ensureCA: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	now := time.Now().Add(-time.Minute)
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "DSP Node CA", Organization: []string{"dsp"}},
		NotBefore:             now,
		NotAfter:              now.Add(caTTL),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		return nil, fmt.Errorf("ca.ensureCA: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("ca.ensureCA: %w", err)
	}
	if err := writeCertPEM(certPath, der); err != nil {
		return nil, err
	}
	if err := writeKeyPEM(keyPath, priv); err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: priv, DER: der}, nil
}

// Load reads a CA certificate and PKCS#8 key from disk.
func Load(certPath, keyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("ca.load: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("ca.load: %w", err)
	}
	cert, der, err := parseCertPEM(certPEM)
	if err != nil {
		return nil, err
	}
	key, err := parseKeyPEM(keyPEM)
	if err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: key, DER: der}, nil
}

// IssueNode signs a node leaf. SANs are taken from endpoint, not the CSR.
func (c *CA) IssueNode(csrDER []byte, nodeID uuid.UUID, endpoint string, ttl time.Duration) (certPEM []byte, cert *x509.Certificate, err error) {
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidCSR, err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidCSR, err)
	}
	pub, ok := csr.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("%w: not ed25519", ErrInvalidCSR)
	}
	dns, ips, err := sansFromEndpoint(endpoint)
	if err != nil {
		return nil, nil, err
	}
	if ttl <= 0 {
		ttl = NodeTTL
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().Add(-time.Minute)
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: nodeID.String()},
		NotBefore:             now,
		NotAfter:              now.Add(ttl),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dns,
		IPAddresses:           ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.Cert, pub, c.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("ca.issueNode: %w", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("ca.issueNode: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), parsed, nil
}

// EnsureServiceCert writes dir/<name>.crt and dir/<name>.key for a control-plane service.
func (c *CA) EnsureServiceCert(dir, name string) (tls.Certificate, error) {
	certPath := filepath.Join(dir, name+".crt")
	keyPath := filepath.Join(dir, name+".key")
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return tls.LoadX509KeyPair(certPath, keyPath)
		}
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("ca.ensureServiceCert: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now().Add(-time.Minute)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    now,
		NotAfter:     now.Add(NodeTTL),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{name, "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.Cert, pub, c.Key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("ca.ensureServiceCert: %w", err)
	}
	if err := writeCertPEM(certPath, der); err != nil {
		return tls.Certificate{}, err
	}
	if err := writeKeyPEM(keyPath, priv); err != nil {
		return tls.Certificate{}, err
	}
	return tls.LoadX509KeyPair(certPath, keyPath)
}

// Fingerprint is SHA-256 of the certificate DER.
func Fingerprint(cert *x509.Certificate) []byte {
	sum := sha256.Sum256(cert.Raw)
	return sum[:]
}

// Pool returns a cert pool containing the CA.
func (c *CA) Pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(c.Cert)
	return p
}

// ClientTLSConfig is TLS 1.3 with optional client certificate.
func (c *CA) ClientTLSConfig(cert *tls.Certificate) *tls.Config {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    c.Pool(),
	}
	if cert != nil {
		cfg.Certificates = []tls.Certificate{*cert}
	}
	return cfg
}

// ServerTLSConfig is TLS 1.3. requireClient enables mTLS.
func (c *CA) ServerTLSConfig(cert tls.Certificate, requireClient bool) *tls.Config {
	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
	}
	if requireClient {
		cfg.ClientCAs = c.Pool()
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg
}

// CreateCSR builds a node CSR from an Ed25519 key.
func CreateCSR(priv ed25519.PrivateKey) ([]byte, error) {
	tmpl := &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "pending"},
		SignatureAlgorithm: x509.PureEd25519,
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, priv)
	if err != nil {
		return nil, fmt.Errorf("ca.createCSR: %w", err)
	}
	return der, nil
}

// ParseNodeID returns the UUID in the certificate subject CN.
func ParseNodeID(cert *x509.Certificate) (uuid.UUID, error) {
	id, err := uuid.Parse(cert.Subject.CommonName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ca.parseNodeID: %w", err)
	}
	return id, nil
}

func sansFromEndpoint(endpoint string) (dns []string, ips []net.IP, err error) {
	host := endpoint
	if h, _, splitErr := net.SplitHostPort(endpoint); splitErr == nil {
		host = h
	}
	if host == "" {
		return nil, nil, fmt.Errorf("%w: empty host", ErrSANClamp)
	}
	if ip := net.ParseIP(host); ip != nil {
		return nil, []net.IP{ip}, nil
	}
	if !validDNS(host) {
		return nil, nil, fmt.Errorf("%w: %q", ErrSANClamp, host)
	}
	return []string{host}, nil, nil
}

func validDNS(host string) bool {
	if host == "localhost" {
		return true
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, c := range host {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("ca.serial: %w", err)
	}
	return serial, nil
}

func writeCertPEM(path string, der []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func writeKeyPEM(path string, priv ed25519.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("ca.writeKey: %w", err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), caKeyPerm); err != nil {
		return fmt.Errorf("ca.writeKey: %w", err)
	}
	return nil
}

func parseCertPEM(raw []byte) (*x509.Certificate, []byte, error) {
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, nil, errors.New("ca: invalid certificate pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("ca.parseCert: %w", err)
	}
	return cert, block.Bytes, nil
}

func parseKeyPEM(raw []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("ca: invalid key pem")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("ca.parseKey: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("ca: key is not ed25519")
	}
	return priv, nil
}

// ParseCertificatePEM parses the first certificate in a PEM blob.
func ParseCertificatePEM(raw []byte) (*x509.Certificate, error) {
	cert, _, err := parseCertPEM(raw)
	return cert, err
}

// ParseKeyPEM parses a PKCS#8 Ed25519 key.
func ParseKeyPEM(raw []byte) (ed25519.PrivateKey, error) {
	return parseKeyPEM(raw)
}

// EncodeKeyPEM encodes an Ed25519 key as PKCS#8 PEM.
func EncodeKeyPEM(priv ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// TLSCertFromPEM builds a tls.Certificate from PEM cert + key.
func TLSCertFromPEM(certPEM, keyPEM []byte) (tls.Certificate, error) {
	return tls.X509KeyPair(certPEM, keyPEM)
}
