package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
)

const renewLead = 7 * 24 * time.Hour

func (a *Agent) maybeRenew(ctx context.Context) error {
	if len(a.id.Cert) == 0 || a.cfg.RegisterURL == "" {
		return nil
	}
	cert, err := ca.ParseCertificatePEM(a.id.Cert)
	if err != nil {
		return err
	}
	if time.Until(cert.NotAfter) > renewLead {
		return nil
	}
	return a.renewCert(ctx)
}

func (a *Agent) renewCert(ctx context.Context) error {
	csr, err := ca.CreateCSR(a.id.Priv)
	if err != nil {
		return err
	}
	cl, err := a.renewClient()
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"csr": base64.StdEncoding.EncodeToString(csr)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.RegisterURL+"/internal/nodes/renew", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.Do(req)
	if err != nil {
		return fmt.Errorf("agent.renew: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("agent.renew: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("agent.renew: http %d %s", resp.StatusCode, raw)
	}
	var out registerResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("agent.renew: %w", err)
	}
	certPEM := []byte(out.CertPEM)
	if err := persistIdentity(a.cfg.DataDir, a.id.ID, certPEM); err != nil {
		return err
	}
	tlsC, err := tlsCert(a.id.Priv, certPEM)
	if err != nil {
		return err
	}
	a.id.Cert = certPEM
	a.tlsCert = tlsC
	if a.mtls != nil && a.caCert != nil {
		pool := x509.NewCertPool()
		pool.AddCert(a.caCert)
		a.mtls.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:   tls.VersionTLS13,
				RootCAs:      pool,
				Certificates: []tls.Certificate{tlsC},
				ServerName:   "healthmon",
			},
		}
	}
	return nil
}

func (a *Agent) renewClient() (*http.Client, error) {
	u, err := url.Parse(a.cfg.RegisterURL)
	if err != nil {
		return nil, fmt.Errorf("agent.renew: %w", err)
	}
	if a.caCert == nil {
		return nil, fmt.Errorf("agent.renew: CA required")
	}
	pool := x509.NewCertPool()
	pool.AddCert(a.caCert)
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:   tls.VersionTLS13,
				RootCAs:      pool,
				Certificates: []tls.Certificate{a.tlsCert},
				ServerName:   u.Hostname(),
			},
		},
	}, nil
}
