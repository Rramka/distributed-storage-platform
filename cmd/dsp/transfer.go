package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
		return fmt.Errorf("usage: dsp provider codes create -endpoint host:port")
	}
	f := splitFlags(args[2:])
	ep := f["endpoint"]
	if ep == "" {
		return fmt.Errorf("provider codes create requires -endpoint")
	}
	_, raw, err := c.do(http.MethodPost, "/v1/nodes/registration-codes", map[string]string{"endpoint": ep}, false)
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
		info pipeline.ChunkInfo
		data []byte
	}
	var chunks []chunkBuf
	if err := pipeline.SplitChunks(tmp, pipeline.DefaultChunk, func(ci pipeline.ChunkInfo, b []byte) error {
		chunks = append(chunks, chunkBuf{info: ci, data: b})
		return nil
	}); err != nil {
		return err
	}

	manChunks := make([]map[string]any, 0, len(chunks))
	for _, ch := range chunks {
		manChunks = append(manChunks, map[string]any{
			"seq":        ch.info.Seq,
			"sha256":     hex.EncodeToString(ch.info.SHA256),
			"size_bytes": ch.info.SizeBytes,
			"fragments": []map[string]any{{
				"shard_index": 0,
				"sha256":      hex.EncodeToString(ch.info.SHA256),
				"size_bytes":  ch.info.SizeBytes,
			}},
		})
	}
	body := map[string]any{
		"bucket_id":       bid.String(),
		"path":            path,
		"size_bytes":      st.Size(),
		"content_sha256":  hex.EncodeToString(contentSHA),
		"encryption_meta": json.RawMessage(metaJSON),
		"chunk_size":      pipeline.DefaultChunk,
		"ec":              map[string]int{"data": 1, "parity": 0},
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
	bySeq := map[int][]byte{}
	for _, ch := range chunks {
		bySeq[ch.info.Seq] = ch.data
	}
	caCert, err := loadCA(getenv)
	if err != nil {
		return err
	}
	var receipts []string
	var mu sync.Mutex
	errCh := make(chan error, len(plan.Placements))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, p := range plan.Placements {
		p := p
		data := bySeq[p.ChunkSeq]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rec, err := putFragment(caCert, p.Endpoint, p.NodeID, p.FragmentID, p.Ticket, data)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			receipts = append(receipts, rec)
			mu.Unlock()
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil {
			return e
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
			Seq       int `json:"seq"`
			Fragments []struct {
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
		if len(ch.Fragments) == 0 {
			return fmt.Errorf("no fragments for chunk %d", ch.Seq)
		}
		fr := ch.Fragments[0]
		data, err := getFragment(caCert, fr.Endpoint, fr.NodeID, fr.FragmentID, fr.Ticket)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		want, _ := hex.DecodeString(fr.SHA256)
		if !bytes.Equal(sum[:], want) {
			return fmt.Errorf("fragment hash mismatch")
		}
		if _, err := w.Write(data); err != nil {
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

func getFragment(caCert *x509.Certificate, endpoint, nodeID, fragID, ticket string) ([]byte, error) {
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
	req, err := http.NewRequest(http.MethodGet, "https://"+endpoint+"/fragments/"+fragID, nil)
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
