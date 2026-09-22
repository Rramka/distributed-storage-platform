package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/pipeline"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

func TestCorruptFragmentDetectedAndRepaired(t *testing.T) {
	t.Parallel()
	platformCA, err := ca.EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	seed := bytes.Repeat([]byte{0x44}, 32)
	signer, err := tickets.NewSignerFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}

	const nAgents = 16
	type node struct {
		dir string
		id  uuid.UUID
		url string
		cl  *http.Client
	}
	nodes := make([]node, nAgents)
	for i := 0; i < nAgents; i++ {
		dir := t.TempDir()
		a, srv := startTestAgent(t, platformCA, signer, dir)
		t.Cleanup(func() { _ = a.Close() })
		nodes[i] = node{
			dir: dir,
			id:  a.ID(),
			url: srv.URL,
			cl:  tlsClient(platformCA.Cert, a.ID()),
		}
	}

	plain := []byte(marker + strings.Repeat("!", 8000))
	fk, err := pipeline.GenerateFileKey()
	if err != nil {
		t.Fatal(err)
	}
	var ct bytes.Buffer
	prefix, _, _, _, err := pipeline.Encrypt(&ct, bytes.NewReader(plain), fk)
	if err != nil {
		t.Fatal(err)
	}
	cipher := ct.Bytes()
	shards, err := pipeline.EncodeChunk(cipher)
	if err != nil {
		t.Fatal(err)
	}
	fragIDs := make([]uuid.UUID, pipeline.ECTotal)
	sums := make([][]byte, pipeline.ECTotal)
	for i, sh := range shards {
		fragIDs[i] = uuid.New()
		sum := sha256.Sum256(sh)
		sums[i] = sum[:]
		tk, err := signer.Sign(tickets.Ticket{
			Op: tickets.OpPut, FragmentID: fragIDs[i], NodeID: nodes[i].id,
			SHA256: sums[i], MaxBytes: uint64(len(sh) + 16),
		})
		if err != nil {
			t.Fatal(err)
		}
		putReq, _ := http.NewRequest(http.MethodPut, nodes[i].url+"/fragments/"+fragIDs[i].String(), bytes.NewReader(sh))
		putReq.Header.Set("X-DSP-Ticket", tk.Raw)
		putResp, err := nodes[i].cl.Do(putReq)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(putResp.Body)
		putResp.Body.Close()
		if putResp.StatusCode != 200 {
			t.Fatalf("put %d: %d %s", i, putResp.StatusCode, body)
		}
	}

	offset, length := 0, 64
	if len(shards[0]) < length {
		length = len(shards[0])
	}
	nonce := bytes.Repeat([]byte{0xab}, 16)
	h := sha256.New()
	_, _ = h.Write(nonce)
	_, _ = h.Write(shards[0][offset : offset+length])
	expected := h.Sum(nil)

	path, err := findFragFile(nodes[0].dir, fragIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, flipEveryBlock(raw, 4096), 0o600); err != nil {
		t.Fatal(err)
	}

	chalTk, err := signer.Sign(tickets.Ticket{
		Op: tickets.OpChallenge, FragmentID: fragIDs[0], NodeID: nodes[0].id,
		SHA256: sums[0], MaxBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	u := nodes[0].url + "/challenge?fragment_id=" + fragIDs[0].String() +
		"&offset=0&length=" + itoa(length) + "&nonce=" + hex.EncodeToString(nonce)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("X-DSP-Ticket", chalTk.Raw)
	resp, err := nodes[0].cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("challenge %d %s", resp.StatusCode, body)
	}
	var out struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	got, err := hex.DecodeString(out.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, expected) {
		t.Fatal("corrupted fragment still matched challenge")
	}

	partial := make([][]byte, pipeline.ECTotal)
	skipped := 0
	for i := 0; i < nAgents; i++ {
		tk, err := signer.Sign(tickets.Ticket{
			Op: tickets.OpGet, FragmentID: fragIDs[i], NodeID: nodes[i].id,
			SHA256: sums[i], MaxBytes: uint64(len(shards[i]) + 16),
		})
		if err != nil {
			t.Fatal(err)
		}
		getReq, _ := http.NewRequest(http.MethodGet, nodes[i].url+"/fragments/"+fragIDs[i].String(), nil)
		getReq.Header.Set("X-DSP-Ticket", tk.Raw)
		getResp, err := nodes[i].cl.Do(getReq)
		if err != nil {
			t.Fatal(err)
		}
		gotBody, err := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if getResp.StatusCode != 200 {
			skipped++
			continue
		}
		sum := sha256.Sum256(gotBody)
		if !bytes.Equal(sum[:], sums[i]) {
			skipped++
			continue
		}
		partial[i] = gotBody
	}
	if skipped == 0 {
		t.Fatal("expected hash mismatch on corrupted shard")
	}
	rebuilt, err := pipeline.ReconstructChunk(partial, len(cipher))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rebuilt, cipher) {
		t.Fatal("reconstructed ciphertext mismatch")
	}
	var plainOut bytes.Buffer
	if err := pipeline.Decrypt(&plainOut, bytes.NewReader(rebuilt), fk, prefix); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plainOut.Bytes(), plain) {
		t.Fatal("plaintext mismatch")
	}
}

func findFragFile(root string, id uuid.UUID) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, id.String()+".frag") {
			found = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", os.ErrNotExist
	}
	return found, nil
}

func flipEveryBlock(data []byte, block int) []byte {
	out := append([]byte(nil), data...)
	if block <= 0 {
		block = 4096
	}
	for i := 0; i < len(out); i += block {
		out[i] ^= 0xff
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
