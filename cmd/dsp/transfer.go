package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/agent"
	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/pipeline"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func cmdNodes(args []string, stdout, stderr io.Writer, c *client) error {
	_, raw, err := c.do(http.MethodGet, "/v1/nodes", nil, false)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func cmdProvider(args []string, stdout, stderr io.Writer, c *client) error {
	if len(args) < 2 || args[0] != "codes" || args[1] != "create" {
		return fmt.Errorf("usage: dsp provider codes create -endpoint host:port -country US -region us-east -asn 64501")
	}
	f := splitFlags(args[2:])
	ep := f["endpoint"]
	country := f["country"]
	region := f["region"]
	asnStr := f["asn"]
	if ep == "" || country == "" || region == "" || asnStr == "" {
		return fmt.Errorf("provider codes create requires -endpoint -country -region -asn")
	}
	asn, err := strconv.Atoi(asnStr)
	if err != nil {
		return fmt.Errorf("provider codes create: invalid -asn")
	}
	_, raw, err := c.do(http.MethodPost, "/v1/nodes/registration-codes", map[string]any{
		"endpoint": ep,
		"country":  country,
		"region":   region,
		"asn":      asn,
	}, false)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func cmdPut(args []string, stdout, stderr io.Writer, c *client, getenv getenvFunc) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: dsp put <local> <bucket>:<path>")
	}
	local := args[0]
	bucket, path, err := splitRemote(args[1])
	if err != nil {
		return err
	}
	pass := envOr(getenv, "DSP_PASSPHRASE", "")
	if pass == "" {
		return fmt.Errorf("dsp put requires DSP_PASSPHRASE")
	}
	bid, err := resolveBucket(c, bucket)
	if err != nil {
		return err
	}
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}

	mk, kdf, err := pipeline.DeriveMasterKey(pass, pipeline.DefaultKDF())
	if err != nil {
		return err
	}
	fk, err := pipeline.GenerateFileKey()
	if err != nil {
		return err
	}
	wrapped, wrapNonce, err := pipeline.WrapFileKey(mk, fk)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "dsp-ct-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	prefix, _, _, contentSHA, err := pipeline.Encrypt(tmp, f, fk)
	if err != nil {
		return err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		return err
	}
	meta := pipeline.NewMeta(kdf, wrapped, wrapNonce, prefix)
	metaJSON, err := pipeline.MarshalMeta(meta)
	if err != nil {
		return err
	}

	type chunkBuf struct {
		info   pipeline.ChunkInfo
		shards [][]byte
	}
	var chunks []chunkBuf
	if err := pipeline.SplitChunks(tmp, pipeline.DefaultChunk, func(ci pipeline.ChunkInfo, b []byte) error {
		shards, err := pipeline.EncodeChunk(b)
		if err != nil {
			return err
		}
		chunks = append(chunks, chunkBuf{info: ci, shards: shards})
		return nil
	}); err != nil {
		return err
	}

	manChunks := make([]map[string]any, 0, len(chunks))
	for _, ch := range chunks {
		frags := make([]map[string]any, 0, len(ch.shards))
		for i, sh := range ch.shards {
			sum := sha256.Sum256(sh)
			frags = append(frags, map[string]any{
				"shard_index": i,
				"sha256":      hex.EncodeToString(sum[:]),
				"size_bytes":  len(sh),
			})
		}
		manChunks = append(manChunks, map[string]any{
			"seq":        ch.info.Seq,
			"sha256":     hex.EncodeToString(ch.info.SHA256),
			"size_bytes": ch.info.SizeBytes,
			"fragments":  frags,
		})
	}
	body := map[string]any{
		"bucket_id":       bid.String(),
		"path":            path,
		"size_bytes":      st.Size(),
		"content_sha256":  hex.EncodeToString(contentSHA),
		"encryption_meta": json.RawMessage(metaJSON),
		"chunk_size":      pipeline.DefaultChunk,
		"ec":              map[string]int{"data": pipeline.ECData, "parity": pipeline.ECParity},
		"chunks":          manChunks,
	}
	_, raw, err := c.do(http.MethodPost, "/v1/upload", body, false)
	if err != nil {
		return err
	}
	var plan struct {
		UploadID   string `json:"upload_id"`
		Placements []struct {
			ChunkSeq   int    `json:"chunk_seq"`
			ShardIndex int16  `json:"shard_index"`
			FragmentID string `json:"fragment_id"`
			NodeID     string `json:"node_id"`
			Endpoint   string `json:"endpoint"`
			Ticket     string `json:"ticket"`
		} `json:"placements"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		return err
	}
	type shardKey struct {
		seq   int
		index int16
	}
	byKey := map[shardKey][]byte{}
	for _, ch := range chunks {
		for i, sh := range ch.shards {
			byKey[shardKey{seq: ch.info.Seq, index: int16(i)}] = sh
		}
	}
	caCert, err := loadCA(getenv)
	if err != nil {
		return err
	}
	var receipts []string
	okPerChunk := map[int]int{}
	var mu sync.Mutex
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	for _, p := range plan.Placements {
		p := p
		data := byKey[shardKey{seq: p.ChunkSeq, index: p.ShardIndex}]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rec, err := putFragment(caCert, p.Endpoint, p.NodeID, p.FragmentID, p.Ticket, data)
			if err != nil {
				return
			}
			mu.Lock()
			receipts = append(receipts, rec)
			okPerChunk[p.ChunkSeq]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	for _, ch := range chunks {
		if okPerChunk[ch.info.Seq] < store.CommitThreshold {
			return fmt.Errorf("chunk %d: %d fragments stored, need %d", ch.info.Seq, okPerChunk[ch.info.Seq], store.CommitThreshold)
		}
	}
	_, raw, err = c.do(http.MethodPost, "/v1/upload/"+plan.UploadID+"/commit", map[string]any{"receipts": receipts}, false)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func cmdGet(args []string, stdout, stderr io.Writer, c *client, getenv getenvFunc) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: dsp get <bucket>:<path> <local>")
	}
	bucket, path, err := splitRemote(args[0])
	if err != nil {
		return err
	}
	dest := args[1]
	pass := envOr(getenv, "DSP_PASSPHRASE", "")
	if pass == "" {
		return fmt.Errorf("dsp get requires DSP_PASSPHRASE")
	}
	bid, err := resolveBucket(c, bucket)
	if err != nil {
		return err
	}
	q := "bucket_id=" + bid.String() + "&prefix=" + path
	_, raw, err := c.do(http.MethodGet, "/v1/files?"+q, nil, false)
	if err != nil {
		return err
	}
	var listed struct {
		Files []store.File `json:"files"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return err
	}
	var fileID uuid.UUID
	for _, f := range listed.Files {
		if f.Path == path && !f.IsFolder {
			fileID = f.ID
			break
		}
	}
	if fileID == uuid.Nil {
		return fmt.Errorf("file not found: %s", path)
	}
	_, raw, err = c.do(http.MethodGet, "/v1/download/"+fileID.String(), nil, false)
	if err != nil {
		return err
	}
	var dl struct {
		ContentSHA256  string          `json:"content_sha256"`
		EncryptionMeta json.RawMessage `json:"encryption_meta"`
		Chunks         []struct {
			Seq       int    `json:"seq"`
			SHA256    string `json:"sha256"`
			SizeBytes int    `json:"size_bytes"`
			Fragments []struct {
				ShardIndex int16  `json:"shard_index"`
				FragmentID string `json:"fragment_id"`
				SHA256     string `json:"sha256"`
				NodeID     string `json:"node_id"`
				Endpoint   string `json:"endpoint"`
				Ticket     string `json:"ticket"`
			} `json:"fragments"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(raw, &dl); err != nil {
		return err
	}
	caCert, err := loadCA(getenv)
	if err != nil {
		return err
	}
	h := sha256.New()
	var cipherBuf bytes.Buffer
	w := io.MultiWriter(&cipherBuf, h)
	for _, ch := range dl.Chunks {
		frags := make([]downloadFrag, 0, len(ch.Fragments))
		for _, fr := range ch.Fragments {
			frags = append(frags, downloadFrag{
				ShardIndex: fr.ShardIndex,
				FragmentID: fr.FragmentID,
				SHA256:     fr.SHA256,
				NodeID:     fr.NodeID,
				Endpoint:   fr.Endpoint,
				Ticket:     fr.Ticket,
			})
		}
		chunk, err := fetchChunk(caCert, ch.Seq, ch.SHA256, ch.SizeBytes, frags)
		if err != nil {
			return err
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	wantAll, _ := hex.DecodeString(dl.ContentSHA256)
	if !bytes.Equal(h.Sum(nil), wantAll) {
		return fmt.Errorf("content hash mismatch")
	}
	em, err := pipeline.UnmarshalMeta(dl.EncryptionMeta)
	if err != nil {
		return err
	}
	fk, err := pipeline.UnlockFK(pass, em)
	if err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := pipeline.Decrypt(out, bytes.NewReader(cipherBuf.Bytes()), fk, em.NoncePrefix); err != nil {
		return err
	}
	fmt.Fprintln(stdout, dest)
	return nil
}

func splitRemote(s string) (bucket, path string, err error) {
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return "", "", fmt.Errorf("remote must be bucket:/path")
	}
	bucket, path = s[:i], s[i+1:]
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return bucket, path, nil
}

func resolveBucket(c *client, name string) (uuid.UUID, error) {
	if id, err := uuid.Parse(name); err == nil {
		return id, nil
	}
	_, raw, err := c.do(http.MethodGet, "/v1/buckets", nil, false)
	if err != nil {
		return uuid.Nil, err
	}
	var out struct {
		Buckets []store.Bucket `json:"buckets"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return uuid.Nil, err
	}
	for _, b := range out.Buckets {
		if b.Name == name {
			return b.ID, nil
		}
	}
	return uuid.Nil, fmt.Errorf("bucket %q not found", name)
}

func loadCA(getenv getenvFunc) (*x509.Certificate, error) {
	p := envOr(getenv, "DSP_CA_FILE", "")
	if p == "" {
		return nil, fmt.Errorf("DSP_CA_FILE is required")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return ca.ParseCertificatePEM(b)
}

type downloadFrag struct {
	ShardIndex int16
	FragmentID string
	SHA256     string
	NodeID     string
	Endpoint   string
	Ticket     string
}

func fetchChunk(caCert *x509.Certificate, seq int, chunkSHA string, sizeBytes int, frags []downloadFrag) ([]byte, error) {
	if len(frags) == 0 {
		return nil, fmt.Errorf("no fragments for chunk %d", seq)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type hit struct {
		idx  int16
		data []byte
	}
	hits := make(chan hit, len(frags))
	var wg sync.WaitGroup
	for _, fr := range frags {
		fr := fr
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := getFragment(ctx, caCert, fr.Endpoint, fr.NodeID, fr.FragmentID, fr.Ticket)
			if err != nil {
				return
			}
			sum := sha256.Sum256(data)
			want, err := hex.DecodeString(fr.SHA256)
			if err != nil || !bytes.Equal(sum[:], want) {
				return
			}
			select {
			case hits <- hit{idx: fr.ShardIndex, data: data}:
			case <-ctx.Done():
			}
		}()
	}
	go func() {
		wg.Wait()
		close(hits)
	}()
	shards := make([][]byte, pipeline.ECTotal)
	got := 0
	for h := range hits {
		if int(h.idx) < 0 || int(h.idx) >= pipeline.ECTotal || shards[h.idx] != nil {
			continue
		}
		shards[h.idx] = h.data
		got++
		if got >= pipeline.ECData {
			cancel()
			break
		}
	}
	if got < pipeline.ECData {
		return nil, fmt.Errorf("chunk %d: only %d of %d shards", seq, got, pipeline.ECData)
	}
	chunk, err := pipeline.ReconstructChunk(shards, sizeBytes)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(chunk)
	want, err := hex.DecodeString(chunkSHA)
	if err != nil || !bytes.Equal(sum[:], want) {
		return nil, fmt.Errorf("chunk %d hash mismatch", seq)
	}
	return chunk, nil
}

func putFragment(caCert *x509.Certificate, endpoint, nodeID, fragID, ticket string, data []byte) (string, error) {
	nid, err := uuid.Parse(nodeID)
	if err != nil {
		return "", err
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
	}
	cl := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: &http.Transport{TLSClientConfig: agent.ClientTLS(caCert, nid, host)},
	}
	req, err := http.NewRequest(http.MethodPut, "https://"+endpoint+"/fragments/"+fragID, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("put fragment: http %d %s", resp.StatusCode, raw)
	}
	var out struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.Receipt, nil
}

func getFragment(ctx context.Context, caCert *x509.Certificate, endpoint, nodeID, fragID, ticket string) ([]byte, error) {
	nid, err := uuid.Parse(nodeID)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
	}
	cl := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: &http.Transport{TLSClientConfig: agent.ClientTLS(caCert, nid, host)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+endpoint+"/fragments/"+fragID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-DSP-Ticket", ticket)
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get fragment: http %d %s", resp.StatusCode, raw)
	}
	return raw, nil
}
