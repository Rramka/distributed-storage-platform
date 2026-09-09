package e2e

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/agent"
	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/pipeline"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

const marker = "PLAINTEXT_SECRET_MARKER_vacation"

func TestEncryptedRoundTripNoPlaintextOnDisk(t *testing.T) {
	t.Parallel()
	platformCA, err := ca.EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	seed := bytes.Repeat([]byte{0x42}, 32)
	signer, err := tickets.NewSignerFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	a, srv := startTestAgent(t, platformCA, signer, dir)
	defer srv.Close()
	defer a.Close()

	plain := []byte(marker + strings.Repeat("!", 4000))
	fk, err := pipeline.GenerateFileKey()
	if err != nil {
		t.Fatal(err)
	}
	var ct bytes.Buffer
	prefix, _, _, contentSHA, err := pipeline.Encrypt(&ct, bytes.NewReader(plain), fk)
	if err != nil {
		t.Fatal(err)
	}
	cipher := ct.Bytes()
	sum := sha256.Sum256(cipher)
	if !bytes.Equal(sum[:], contentSHA) {
		t.Fatal("content hash")
	}

	fid := uuid.New()
	tk, err := signer.Sign(tickets.Ticket{
		Op: tickets.OpPut, FragmentID: fid, NodeID: a.ID(),
		SHA256: sum[:], MaxBytes: uint64(len(cipher) + 16),
	})
	if err != nil {
		t.Fatal(err)
	}
	cl := tlsClient(platformCA.Cert, a.ID())
	putReq, _ := http.NewRequest(http.MethodPut, srv.URL+"/fragments/"+fid.String(), bytes.NewReader(cipher))
	putReq.Header.Set("X-DSP-Ticket", tk.Raw)
	putResp, err := cl.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(putResp.Body)
	putResp.Body.Close()
	if putResp.StatusCode != 200 {
		t.Fatalf("put %d %s", putResp.StatusCode, body)
	}
	var rec struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal(body, &rec); err != nil || rec.Receipt == "" {
		t.Fatalf("receipt %s", body)
	}

	getTk, err := signer.Sign(tickets.Ticket{
		Op: tickets.OpGet, FragmentID: fid, NodeID: a.ID(),
		SHA256: sum[:], MaxBytes: uint64(len(cipher) + 16),
	})
	if err != nil {
		t.Fatal(err)
	}
	getReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/fragments/"+fid.String(), nil)
	getReq.Header.Set("X-DSP-Ticket", getTk.Raw)
	getResp, err := cl.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if getResp.StatusCode != 200 {
		t.Fatalf("get %d %s", getResp.StatusCode, got)
	}
	if !bytes.Equal(got, cipher) {
		t.Fatal("cipher mismatch")
	}

	var out bytes.Buffer
	if err := pipeline.Decrypt(&out, bytes.NewReader(got), fk, prefix); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), plain) {
		t.Fatal("plaintext mismatch")
	}

	if err := scanPlaintext(dir, marker); err != nil {
		t.Fatal(err)
	}
}

func startTestAgent(t *testing.T, platformCA *ca.CA, signer *tickets.Signer, dir string) (*agent.Agent, *httptest.Server) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	csr, err := ca.CreateCSR(priv)
	if err != nil {
		t.Fatal(err)
	}
	leafPEM, _, err := platformCA.IssueNode(csr, id, "127.0.0.1:7443", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := ca.EncodeKeyPEM(priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "identity"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "identity", "node.key"), keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "identity", "node.crt"), leafPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "identity", "node.id"), []byte(id.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: platformCA.DER}), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := agent.Config{
		DataDir:         dir,
		MaxStorageGB:    1,
		Endpoint:        "127.0.0.1:7443",
		RegisterURL:     "http://127.0.0.1:1",
		TicketPublicKey: signer.PublicKeyHex(),
		CAFile:          caPath,
		HealthAddr:      ":0",
		FragmentAddr:    ":0",
		AgentVersion:    "test",
		OS:              "linux",
	}
	a, err := agent.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(a.Handler())
	srv.TLS = a.TLSConfig()
	srv.StartTLS()
	return a, srv
}

func tlsClient(caCert *x509.Certificate, nodeID uuid.UUID) *http.Client {
	cfg := agent.ClientTLS(caCert, nodeID, "127.0.0.1")
	return &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}, Timeout: 10 * time.Second}
}

func scanPlaintext(root, marker string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte(marker)) {
			return &plainErr{path: path}
		}
		return nil
	})
}

type plainErr struct{ path string }

func (e *plainErr) Error() string { return "plaintext marker in " + e.path }
