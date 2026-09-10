package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/pipeline"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
	"github.com/google/uuid"
)

func TestReedSolomonRoundTripSixOfSixteenDown(t *testing.T) {
	t.Parallel()
	platformCA, err := ca.EnsureCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	seed := bytes.Repeat([]byte{0x43}, 32)
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
		srv *httptest.Server
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
			srv: srv,
		}
	}

	plain := []byte(marker + strings.Repeat("!", 8000))
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
	if !bytes.Equal(sha256Sum(cipher), contentSHA) {
		t.Fatal("content hash")
	}

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
		var rec struct {
			Receipt string `json:"receipt"`
		}
		if err := json.Unmarshal(body, &rec); err != nil || rec.Receipt == "" {
			t.Fatalf("receipt %d %s", i, body)
		}
	}

	for i := pipeline.ECData; i < nAgents; i++ {
		nodes[i].srv.Close()
	}

	partial := make([][]byte, pipeline.ECTotal)
	for i := 0; i < pipeline.ECData; i++ {
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
		got, err := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if getResp.StatusCode != 200 {
			t.Fatalf("get %d: %d %s", i, getResp.StatusCode, got)
		}
		if !bytes.Equal(sha256Sum(got), sums[i]) {
			t.Fatalf("fragment hash %d", i)
		}
		partial[i] = got
	}

	work := make([][]byte, pipeline.ECTotal)
	for i, s := range partial {
		if s != nil {
			work[i] = append([]byte(nil), s...)
		}
	}
	if err := pipeline.ReconstructShards(work); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < pipeline.ECTotal; i++ {
		if !bytes.Equal(work[i], shards[i]) {
			t.Fatalf("rebuilt shard %d mismatch", i)
		}
	}

	rebuilt, err := pipeline.ReconstructChunk(partial, len(cipher))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rebuilt, cipher) {
		t.Fatal("reconstructed ciphertext mismatch")
	}

	var out bytes.Buffer
	if err := pipeline.Decrypt(&out, bytes.NewReader(rebuilt), fk, prefix); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), plain) {
		t.Fatal("plaintext mismatch")
	}

	for i, n := range nodes {
		if err := scanPlaintext(n.dir, marker); err != nil {
			t.Fatalf("node %d: %v", i, err)
		}
	}
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}
