package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

// Agent is a running storage node.
type Agent struct {
	cfg       Config
	id        identity
	store     *ChunkStore
	tickets   *tickets.Verifier
	caCert    *x509.Certificate
	tlsCert   tls.Certificate
	mtls      *http.Client
	healthmon string
}

// Open loads identity (registering if needed) and the chunk store.
func Open(ctx context.Context, cfg Config) (*Agent, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("agent.open: %w", err)
	}
	if cfg.RegistrationCode == "" {
		if p := os.Getenv("REGISTRATION_CODE_FILE"); p != "" {
			code, err := waitForCode(p, 2*time.Minute)
			if err != nil {
				return nil, err
			}
			cfg.RegistrationCode = code
		}
	}
	ver, err := loadVerifier(cfg.TicketPublicKey)
	if err != nil {
		return nil, err
	}
	id, ready, err := loadIdentity(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if !ready {
		id, err = register(ctx, cfg, id.Priv)
		if err != nil {
			return nil, err
		}
	}
	st, err := OpenStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	tlsC, err := tlsCert(id.Priv, id.Cert)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	a := &Agent{
		cfg:       cfg,
		id:        id,
		store:     st,
		tickets:   ver,
		tlsCert:   tlsC,
		healthmon: cfg.HealthmonURL,
	}
	if cfg.CAFile != "" {
		cacert, err := loadCACert(cfg.CAFile)
		if err != nil {
			_ = st.Close()
			return nil, err
		}
		a.caCert = cacert
		pool := x509.NewCertPool()
		pool.AddCert(cacert)
		a.mtls = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion:   tls.VersionTLS13,
					RootCAs:      pool,
					Certificates: []tls.Certificate{tlsC},
					ServerName:   "healthmon",
				},
			},
		}
	}
	return a, nil
}

// ID is the registered node UUID.
func (a *Agent) ID() uuid.UUID { return a.id.ID }

// Close releases the chunk store.
func (a *Agent) Close() error { return a.store.Close() }

// Handler is the fragment API (no TLS). Tests wrap it with httptest TLS.
func (a *Agent) Handler() http.Handler { return a.fragmentMux() }

// TLSConfig serves the fragment API with the node certificate.
func (a *Agent) TLSConfig() *tls.Config {
	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{a.tlsCert},
	}
	if a.caCert != nil {
		cfg.ClientCAs = x509.NewCertPool()
		cfg.ClientCAs.AddCert(a.caCert)
	}
	return cfg
}

// ServeFragments listens on cfg.FragmentAddr with TLS and runs heartbeats.
func (a *Agent) ServeFragments(ctx context.Context) error {
	go a.heartbeatLoop(ctx)
	srv := &http.Server{
		Addr:              a.cfg.FragmentAddr,
		Handler:           a.Handler(),
		TLSConfig:         a.TLSConfig(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	ln, err := tls.Listen("tcp", a.cfg.FragmentAddr, a.TLSConfig())
	if err != nil {
		return fmt.Errorf("agent.serve: %w", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// ClientTLS returns a client config that trusts the platform CA and expects this node's CN.
func ClientTLS(caCert *x509.Certificate, nodeID uuid.UUID, serverName string) *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    pool,
		ServerName: serverName,
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return fmt.Errorf("agent: missing peer cert")
			}
			cert, err := x509.ParseCertificate(raw[0])
			if err != nil {
				return err
			}
			id, err := ca.ParseNodeID(cert)
			if err != nil || id != nodeID {
				return fmt.Errorf("agent: node identity mismatch")
			}
			return nil
		},
	}
}

// PutTicket is a helper used by tests and the CLI.
func PutHeader(ticket string) string { return ticket }
