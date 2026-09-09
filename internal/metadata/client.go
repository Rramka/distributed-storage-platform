package metadata

import (
	"bytes"
	"context"
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

type ctxKeyRequestID struct{}

// WithRequestID stores the public request id on ctx for outbound calls.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID{}, id)
}
