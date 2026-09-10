package metadata

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

// CallError is an error returned by the metadata HTTP API.
type CallError struct {
	Status  int
	Code    string
	Message string
}

func (e *CallError) Error() string {
	return fmt.Sprintf("metadata: %s: %s", e.Code, e.Message)
}

// Client is an HTTP client for the internal metadata API.
type Client struct {
	base   string
	client *http.Client
}

// NewClient talks to metadata at baseURL (e.g. http://metadata:8081).
func NewClient(baseURL string) *Client {
	return &Client{
		base: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) CreateUser(ctx context.Context, email, password string) (store.User, error) {
	var u store.User
	err := c.do(ctx, http.MethodPost, "/internal/users", map[string]string{
		"email":    email,
		"password": password,
	}, http.StatusCreated, &u)
	return u, err
}

func (c *Client) VerifyPassword(ctx context.Context, email, password string) (store.User, error) {
	var u store.User
	err := c.do(ctx, http.MethodPost, "/internal/auth/verify", map[string]string{
		"email":    email,
		"password": password,
	}, http.StatusOK, &u)
	return u, err
}

func (c *Client) LookupAPIKey(ctx context.Context, keyHash string) (store.APIKey, store.User, error) {
	var out struct {
		Key  store.APIKey `json:"key"`
		User store.User   `json:"user"`
	}
	err := c.do(ctx, http.MethodGet, "/internal/auth/api-keys/"+url.PathEscape(keyHash), nil, http.StatusOK, &out)
	return out.Key, out.User, err
}

func (c *Client) CreateAPIKey(ctx context.Context, userID uuid.UUID, keyHash, label string, scopes []string, expiresAt *time.Time) (store.APIKey, error) {
	body := map[string]any{
		"key_hash": keyHash,
		"label":    label,
		"scopes":   scopes,
	}
	if expiresAt != nil {
		body["expires_at"] = expiresAt.UTC().Format(time.RFC3339)
	}
	var k store.APIKey
	err := c.do(ctx, http.MethodPost, "/internal/users/"+userID.String()+"/api-keys", body, http.StatusCreated, &k)
	return k, err
}

func (c *Client) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error) {
	var out struct {
		APIKeys []store.APIKey `json:"api_keys"`
	}
	err := c.do(ctx, http.MethodGet, "/internal/users/"+userID.String()+"/api-keys", nil, http.StatusOK, &out)
	return out.APIKeys, err
}

func (c *Client) RevokeAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, "/internal/users/"+userID.String()+"/api-keys/"+keyID.String(), nil, http.StatusNoContent, nil)
}

func (c *Client) CreateBucket(ctx context.Context, userID uuid.UUID, name string) (store.Bucket, error) {
	var b store.Bucket
	err := c.do(ctx, http.MethodPost, "/internal/users/"+userID.String()+"/buckets", map[string]string{"name": name}, http.StatusCreated, &b)
	return b, err
}

func (c *Client) ListBuckets(ctx context.Context, userID uuid.UUID) ([]store.Bucket, error) {
	var out struct {
		Buckets []store.Bucket `json:"buckets"`
	}
	err := c.do(ctx, http.MethodGet, "/internal/users/"+userID.String()+"/buckets", nil, http.StatusOK, &out)
	return out.Buckets, err
}

func (c *Client) DeleteBucket(ctx context.Context, userID, bucketID uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, "/internal/users/"+userID.String()+"/buckets/"+bucketID.String(), nil, http.StatusNoContent, nil)
}

func (c *Client) CreateFolder(ctx context.Context, userID, bucketID uuid.UUID, path string) (store.File, error) {
	var f store.File
	err := c.do(ctx, http.MethodPost, "/internal/users/"+userID.String()+"/folders", map[string]string{
		"bucket_id": bucketID.String(),
		"path":      path,
	}, http.StatusCreated, &f)
	return f, err
}

func (c *Client) ListFiles(ctx context.Context, userID, bucketID uuid.UUID, prefix, cursor string, limit int) ([]store.File, string, error) {
	q := url.Values{}
	q.Set("bucket_id", bucketID.String())
	if prefix != "" {
		q.Set("prefix", prefix)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out struct {
		Files      []store.File `json:"files"`
		NextCursor string       `json:"next_cursor"`
	}
	err := c.do(ctx, http.MethodGet, "/internal/users/"+userID.String()+"/files?"+q.Encode(), nil, http.StatusOK, &out)
	return out.Files, out.NextCursor, err
}

func (c *Client) GetFile(ctx context.Context, userID, fileID uuid.UUID) (store.File, error) {
	var f store.File
	err := c.do(ctx, http.MethodGet, "/internal/users/"+userID.String()+"/files/"+fileID.String(), nil, http.StatusOK, &f)
	return f, err
}

func (c *Client) RenameFile(ctx context.Context, userID, fileID uuid.UUID, newPath string) (store.File, error) {
	var f store.File
	err := c.do(ctx, http.MethodPut, "/internal/users/"+userID.String()+"/rename", map[string]string{
		"file_id":  fileID.String(),
		"new_path": newPath,
	}, http.StatusOK, &f)
	return f, err
}

func (c *Client) DeleteFile(ctx context.Context, userID, fileID uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, "/internal/users/"+userID.String()+"/files/"+fileID.String(), nil, http.StatusNoContent, nil)
}

func (c *Client) MintRegistrationCode(ctx context.Context, userID uuid.UUID, endpoint, country, region string, asn *int) (store.RegistrationCode, string, error) {
	var out struct {
		ID        uuid.UUID `json:"id"`
		Endpoint  string    `json:"endpoint"`
		Country   string    `json:"country"`
		Region    string    `json:"region"`
		ASN       *int      `json:"asn"`
		ExpiresAt time.Time `json:"expires_at"`
		Secret    string    `json:"secret"`
	}
	err := c.do(ctx, http.MethodPost, "/internal/users/"+userID.String()+"/registration-codes", map[string]any{
		"endpoint": endpoint,
		"country":  country,
		"region":   region,
		"asn":      asn,
	}, http.StatusCreated, &out)
	return store.RegistrationCode{
		ID: out.ID, OwnerID: userID, Endpoint: out.Endpoint,
		Country: out.Country, Region: out.Region, ASN: out.ASN, ExpiresAt: out.ExpiresAt,
	}, out.Secret, err
}

func (c *Client) ListNodes(ctx context.Context, userID uuid.UUID) ([]store.Node, error) {
	var out struct {
		Nodes []store.Node `json:"nodes"`
	}
	err := c.do(ctx, http.MethodGet, "/internal/users/"+userID.String()+"/nodes", nil, http.StatusOK, &out)
	return out.Nodes, err
}

func (c *Client) RegisterNode(ctx context.Context, code string, csr []byte, endpoint, osName, version, label string, capacity int64) (store.Node, string, error) {
	var out struct {
		NodeID   string `json:"node_id"`
		CertPEM  string `json:"cert_pem"`
		Endpoint string `json:"endpoint"`
	}
	err := c.do(ctx, http.MethodPost, "/internal/nodes/register", map[string]any{
		"registration_code": code,
		"csr":               encodeB64(csr),
		"endpoint":          endpoint,
		"os":                osName,
		"agent_version":     version,
		"hostname_label":    label,
		"capacity_bytes":    capacity,
	}, http.StatusCreated, &out)
	id, _ := uuid.Parse(out.NodeID)
	return store.Node{ID: id, Endpoint: out.Endpoint}, out.CertPEM, err
}

func encodeB64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func manifestJSON(m store.UploadManifest) map[string]any {
	chunks := make([]map[string]any, 0, len(m.Chunks))
	for _, ch := range m.Chunks {
		frags := make([]map[string]any, 0, len(ch.Fragments))
		for _, fr := range ch.Fragments {
			frags = append(frags, map[string]any{
				"shard_index": fr.ShardIndex,
				"sha256":      hex.EncodeToString(fr.SHA256),
				"size_bytes":  fr.SizeBytes,
			})
		}
		chunks = append(chunks, map[string]any{
			"seq":        ch.Seq,
			"sha256":     hex.EncodeToString(ch.SHA256),
			"size_bytes": ch.SizeBytes,
			"fragments":  frags,
		})
	}
	return map[string]any{
		"bucket_id":       m.BucketID.String(),
		"path":            m.Path,
		"size_bytes":      m.SizeBytes,
		"content_sha256":  hex.EncodeToString(m.ContentSHA256),
		"encryption_meta": json.RawMessage(m.EncryptionMeta),
		"chunk_size":      m.ChunkSize,
		"ec":              map[string]int{"data": int(m.ECData), "parity": int(m.ECParity)},
		"chunks":          chunks,
	}
}

type ctxKeyRequestID struct{}

func (c *Client) HeartbeatNode(ctx context.Context, nodeID uuid.UUID, used, capacity int64) error {
	return c.do(ctx, http.MethodPost, "/internal/nodes/"+nodeID.String()+"/heartbeat", map[string]int64{
		"used_bytes":     used,
		"capacity_bytes": capacity,
	}, http.StatusOK, nil)
}

func (c *Client) PlanUpload(ctx context.Context, userID uuid.UUID, m store.UploadManifest) (PlanResult, error) {
	body := manifestJSON(m)
	var res PlanResult
	err := c.do(ctx, http.MethodPost, "/internal/users/"+userID.String()+"/upload", body, http.StatusCreated, &res)
	return res, err
}

func (c *Client) CommitUpload(ctx context.Context, userID, uploadID uuid.UUID, recs []string) (store.File, store.FileVersion, error) {
	var out struct {
		FileID    uuid.UUID `json:"file_id"`
		VersionNo int       `json:"version_no"`
		Status    string    `json:"status"`
	}
	err := c.do(ctx, http.MethodPost, "/internal/users/"+userID.String()+"/upload/"+uploadID.String()+"/commit", map[string]any{"receipts": recs}, http.StatusOK, &out)
	return store.File{ID: out.FileID}, store.FileVersion{VersionNo: out.VersionNo, Status: out.Status}, err
}

func (c *Client) PlanDownload(ctx context.Context, userID, fileID uuid.UUID) (DownloadResult, error) {
	var res DownloadResult
	err := c.do(ctx, http.MethodGet, "/internal/users/"+userID.String()+"/download/"+fileID.String(), nil, http.StatusOK, &res)
	return res, err
}

func (c *Client) do(ctx context.Context, method, path string, body any, want int, dst any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if rid, _ := ctx.Value(ctxKeyRequestID{}).(string); rid != "" {
		req.Header.Set("X-Request-ID", rid)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("metadata.do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		var env apierr.Envelope
		_ = json.NewDecoder(resp.Body).Decode(&env)
		code := env.Error.Code
		if code == "" {
			code = apierr.CodeInternal
		}
		msg := env.Error.Message
		if msg == "" {
			msg = resp.Status
		}
		return &CallError{Status: resp.StatusCode, Code: code, Message: msg}
	}
	if dst == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("metadata.decode: %w", err)
	}
	return nil
}

// WithRequestID stores the public request id on ctx for outbound calls.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID{}, id)
}
