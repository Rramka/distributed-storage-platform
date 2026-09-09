// Package gateway is the public REST API (docs/08-api.md).
package gateway

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/auth"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/ratelimit"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxIdentity
)

type identity struct {
	User store.User
	Key  store.APIKey
}

// Server is the public HTTP handler.
type Server struct {
	meta  metadata.Service
	limit *ratelimit.Limiter
	mux   *http.ServeMux
}

// New returns the public gateway handler. GET /healthz is registered on mux.
func New(mux *http.ServeMux, meta metadata.Service, limit *ratelimit.Limiter) *Server {
	s := &Server{meta: meta, limit: limit, mux: mux}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	s.mux.HandleFunc("POST /v1/auth/register", s.with(authNone, s.handleRegister))
	s.mux.HandleFunc("POST /v1/auth/api-keys", s.with(authBasic, s.handleCreateAPIKey))
	s.mux.HandleFunc("GET /v1/auth/api-keys", s.with(authKey, s.handleListAPIKeys))
	s.mux.HandleFunc("DELETE /v1/auth/api-keys/{id}", s.with(authKeyWrite, s.handleRevokeAPIKey))

	s.mux.HandleFunc("POST /v1/buckets", s.with(authKeyWrite, s.handleCreateBucket))
	s.mux.HandleFunc("GET /v1/buckets", s.with(authKey, s.handleListBuckets))
	s.mux.HandleFunc("DELETE /v1/buckets/{id}", s.with(authKeyWrite, s.handleDeleteBucket))

	s.mux.HandleFunc("POST /v1/folders", s.with(authKeyWrite, s.handleCreateFolder))
	s.mux.HandleFunc("GET /v1/files", s.with(authKey, s.handleListFiles))
	s.mux.HandleFunc("GET /v1/files/{id}", s.with(authKey, s.handleGetFile))
	s.mux.HandleFunc("PUT /v1/rename", s.with(authKeyWrite, s.handleRename))
	s.mux.HandleFunc("DELETE /v1/file/{id}", s.with(authKeyWrite, s.handleDeleteFile))

	s.mux.HandleFunc("POST /v1/nodes/registration-codes", s.with(authKeyWrite, s.handleMintCode))
	s.mux.HandleFunc("GET /v1/nodes", s.with(authKey, s.handleListNodes))
	s.mux.HandleFunc("POST /v1/upload", s.with(authKeyWrite, s.handlePlanUpload))
	s.mux.HandleFunc("POST /v1/upload/{id}/commit", s.with(authKeyWrite, s.handleCommitUpload))
	s.mux.HandleFunc("GET /v1/download/{id}", s.with(authKey, s.handlePlanDownload))

	s.mux.HandleFunc("/{path...}", s.handleNotFound)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rid := r.Header.Get("X-Request-ID")
	if rid == "" {
		rid = apierr.NewRequestID()
	}
	w.Header().Set("X-Request-ID", rid)
	r = r.WithContext(context.WithValue(r.Context(), ctxRequestID, rid))
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	apierr.Write(w, apierr.CodeNotFound, "no such endpoint", requestID(r))
}

type authKind int

const (
	authNone authKind = iota
	authBasic
	authKey
	authKeyWrite
)

func (s *Server) with(kind authKind, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rid := requestID(r)
		ctx := r.Context()

		switch kind {
		case authNone, authBasic:
			if s.limit != nil {
				res, err := s.limit.AllowAuth(ctx, clientIP(r))
				if err != nil {
					apierr.Write(w, apierr.CodeInternal, "internal error", rid)
					return
				}
				if !res.Allowed {
					w.Header().Set("Retry-After", formatRetry(res.RetryAfter))
					apierr.Write(w, apierr.CodeRateLimited, "rate limited", rid)
					return
				}
			}
			if kind == authBasic {
				email, password, ok := r.BasicAuth()
				if !ok {
					w.Header().Set("WWW-Authenticate", `Basic realm="dsp"`)
					apierr.Write(w, apierr.CodeUnauthenticated, "unauthenticated", rid)
					return
				}
				u, err := s.meta.VerifyPassword(ctx, email, password)
				if err != nil {
					s.writeMeta(w, r, err)
					return
				}
				r = r.WithContext(context.WithValue(ctx, ctxIdentity, identity{User: u}))
			}
		case authKey, authKeyWrite:
			id, err := s.authenticateKey(ctx, r)
			if err != nil {
				s.writeMeta(w, r, err)
				return
			}
			need := "read"
			if kind == authKeyWrite {
				need = "write"
			}
			if !hasScope(id.Key.Scopes, need) {
				apierr.Write(w, apierr.CodeForbidden, "forbidden", rid)
				return
			}
			if s.limit != nil && r.Method == http.MethodGet {
				res, err := s.limit.AllowRead(ctx, id.User.ID.String())
				if err != nil {
					apierr.Write(w, apierr.CodeInternal, "internal error", rid)
					return
				}
				if !res.Allowed {
					w.Header().Set("Retry-After", formatRetry(res.RetryAfter))
					apierr.Write(w, apierr.CodeRateLimited, "rate limited", rid)
					return
				}
			}
			if s.limit != nil && (r.URL.Path == "/v1/upload" || strings.HasSuffix(r.URL.Path, "/commit") || strings.HasPrefix(r.URL.Path, "/v1/download/")) {
				res, err := s.limit.AllowPlan(ctx, id.User.ID.String())
				if err != nil {
					apierr.Write(w, apierr.CodeInternal, "internal error", rid)
					return
				}
				if !res.Allowed {
					w.Header().Set("Retry-After", formatRetry(res.RetryAfter))
					apierr.Write(w, apierr.CodeRateLimited, "rate limited", rid)
					return
				}
			}
			r = r.WithContext(context.WithValue(ctx, ctxIdentity, id))
		}
		next(w, r)
	}
}

func (s *Server) authenticateKey(ctx context.Context, r *http.Request) (identity, error) {
	secret := r.Header.Get("X-Api-Key")
	if !auth.ValidSecretFormat(secret) {
		return identity{}, metadata.ErrUnauthenticated
	}
	k, u, err := s.meta.LookupAPIKey(ctx, auth.HashAPIKey(secret))
	if err != nil {
		return identity{}, err
	}
	return identity{User: u, Key: k}, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	version := os.Getenv("DSP_VERSION")
	if version == "" {
		version = "dev"
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": version,
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.meta.CreateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	body := userPublic(u)
	body["hint"] = "Generate your master key client-side; the platform never sees it."
	writeJSON(w, http.StatusCreated, body)
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	var req struct {
		Label     string     `json:"label"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	minted, err := auth.MintAPIKey()
	if err != nil {
		apierr.Write(w, apierr.CodeInternal, "internal error", requestID(r))
		return
	}
	k, err := s.meta.CreateAPIKey(r.Context(), id.User.ID, minted.Hash, req.Label, req.Scopes, req.ExpiresAt)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	body := apiKeyPublic(k)
	body["secret"] = minted.Secret
	writeJSON(w, http.StatusCreated, body)
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	keys, err := s.meta.ListAPIKeys(r.Context(), id.User.ID)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, apiKeyPublic(k))
	}
	writeJSON(w, http.StatusOK, map[string]any{"api_keys": out})
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	keyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	if err := s.meta.RevokeAPIKey(r.Context(), id.User.ID, keyID); err != nil {
		s.writeMeta(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := s.meta.CreateBucket(r.Context(), id.User.ID, req.Name)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	buckets, err := s.meta.ListBuckets(r.Context(), id.User.ID)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	if buckets == nil {
		buckets = []store.Bucket{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"buckets": buckets})
}

func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	bid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	if err := s.meta.DeleteBucket(r.Context(), id.User.ID, bid); err != nil {
		s.writeMeta(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	var req struct {
		BucketID string `json:"bucket_id"`
		Path     string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	bid, err := uuid.Parse(req.BucketID)
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	f, err := s.meta.CreateFolder(r.Context(), id.User.ID, bid, req.Path)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	bid, err := uuid.Parse(r.URL.Query().Get("bucket_id"))
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	files, next, err := s.meta.ListFiles(r.Context(), id.User.ID, bid, r.URL.Query().Get("prefix"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	if files == nil {
		files = []store.File{}
	}
	body := map[string]any{"files": files}
	if next != "" {
		body["next_cursor"] = next
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	fid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	f, err := s.meta.GetFile(r.Context(), id.User.ID, fid)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	var req struct {
		FileID  string `json:"file_id"`
		NewPath string `json:"new_path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	fid, err := uuid.Parse(req.FileID)
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	f, err := s.meta.RenameFile(r.Context(), id.User.ID, fid, req.NewPath)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	fid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	if err := s.meta.DeleteFile(r.Context(), id.User.ID, fid); err != nil {
		s.writeMeta(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMintCode(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	c, secret, err := s.meta.MintRegistrationCode(r.Context(), id.User.ID, req.Endpoint)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         c.ID.String(),
		"endpoint":   c.Endpoint,
		"expires_at": c.ExpiresAt.UTC().Format(time.RFC3339),
		"secret":     secret,
	})
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	nodes, err := s.meta.ListNodes(r.Context(), id.User.ID)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	if nodes == nil {
		nodes = []store.Node{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes})
}

func (s *Server) handlePlanUpload(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	m, err := decodeUploadManifest(r)
	if err != nil {
		apierr.Write(w, apierr.CodeManifestInvalid, "manifest invalid", requestID(r))
		return
	}
	res, err := s.meta.PlanUpload(r.Context(), id.User.ID, m)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"upload_id":  res.UploadID.String(),
		"file_id":    res.FileID.String(),
		"expires_at": res.ExpiresAt.UTC().Format(time.RFC3339),
		"placements": res.Placements,
	})
}

func (s *Server) handleCommitUpload(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	uid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	var req struct {
		Receipts []string `json:"receipts"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	f, v, err := s.meta.CommitUpload(r.Context(), id.User.ID, uid, req.Receipts)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"file_id":    f.ID.String(),
		"version_no": v.VersionNo,
		"status":     v.Status,
	})
}

func (s *Server) handlePlanDownload(w http.ResponseWriter, r *http.Request) {
	id := mustIdentity(r)
	fid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return
	}
	res, err := s.meta.PlanDownload(r.Context(), id.User.ID, fid)
	if err != nil {
		s.writeMeta(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func decodeUploadManifest(r *http.Request) (store.UploadManifest, error) {
	var jm struct {
		BucketID       string          `json:"bucket_id"`
		Path           string          `json:"path"`
		SizeBytes      int64           `json:"size_bytes"`
		ContentSHA256  string          `json:"content_sha256"`
		EncryptionMeta json.RawMessage `json:"encryption_meta"`
		ChunkSize      int             `json:"chunk_size"`
		EC             struct {
			Data   int `json:"data"`
			Parity int `json:"parity"`
		} `json:"ec"`
		Chunks []struct {
			Seq       int    `json:"seq"`
			SHA256    string `json:"sha256"`
			SizeBytes int    `json:"size_bytes"`
			Fragments []struct {
				ShardIndex int    `json:"shard_index"`
				SHA256     string `json:"sha256"`
				SizeBytes  int    `json:"size_bytes"`
			} `json:"fragments"`
		} `json:"chunks"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&jm); err != nil {
		return store.UploadManifest{}, err
	}
	bid, err := uuid.Parse(jm.BucketID)
	if err != nil {
		return store.UploadManifest{}, err
	}
	sum, err := hex.DecodeString(jm.ContentSHA256)
	if err != nil {
		return store.UploadManifest{}, err
	}
	m := store.UploadManifest{
		BucketID: bid, Path: jm.Path, SizeBytes: jm.SizeBytes, ContentSHA256: sum,
		EncryptionMeta: jm.EncryptionMeta, ChunkSize: jm.ChunkSize,
		ECData: int16(jm.EC.Data), ECParity: int16(jm.EC.Parity),
	}
	if m.ECData == 0 {
		m.ECData = 1
	}
	for _, ch := range jm.Chunks {
		cs, err := hex.DecodeString(ch.SHA256)
		if err != nil {
			return store.UploadManifest{}, err
		}
		mc := store.ManifestChunk{Seq: ch.Seq, SizeBytes: ch.SizeBytes, SHA256: cs}
		for _, fr := range ch.Fragments {
			fs, err := hex.DecodeString(fr.SHA256)
			if err != nil {
				return store.UploadManifest{}, err
			}
			mc.Fragments = append(mc.Fragments, store.ManifestFragment{ShardIndex: int16(fr.ShardIndex), SizeBytes: fr.SizeBytes, SHA256: fs})
		}
		m.Chunks = append(m.Chunks, mc)
	}
	return m, nil
}

func (s *Server) writeMeta(w http.ResponseWriter, r *http.Request, err error) {
	rid := requestID(r)
	var ce *metadata.CallError
	if errors.As(err, &ce) {
		apierr.WriteStatus(w, ce.Status, ce.Code, ce.Message, rid)
		return
	}
	switch {
	case errors.Is(err, metadata.ErrInvalid):
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", rid)
	case errors.Is(err, metadata.ErrUnauthenticated):
		apierr.Write(w, apierr.CodeUnauthenticated, "unauthenticated", rid)
	case errors.Is(err, metadata.ErrForbidden):
		apierr.Write(w, apierr.CodeForbidden, "forbidden", rid)
	case errors.Is(err, metadata.ErrNotFound), errors.Is(err, store.ErrNotFound):
		apierr.Write(w, apierr.CodeNotFound, "not found", rid)
	case errors.Is(err, metadata.ErrConflict), errors.Is(err, store.ErrConflict):
		apierr.Write(w, apierr.CodeAlreadyExists, "already exists", rid)
	case errors.Is(err, metadata.ErrNotEmpty), errors.Is(err, store.ErrNotEmpty):
		apierr.Write(w, apierr.CodeConflict, "bucket is not empty", rid)
	case errors.Is(err, metadata.ErrIncomplete), errors.Is(err, store.ErrIncomplete):
		apierr.Write(w, apierr.CodeConflict, "too few fragments confirmed", rid)
	case errors.Is(err, metadata.ErrUnavailable), errors.Is(err, store.ErrUnavailable):
		apierr.Write(w, apierr.CodePlacementUnavailable, "placement unavailable", rid)
	default:
		slog.Error("metadata call failed", "err", err, "request_id", rid)
		apierr.Write(w, apierr.CodeInternal, "internal error", rid)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", requestID(r))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func requestID(r *http.Request) string {
	if v, ok := r.Context().Value(ctxRequestID).(string); ok && v != "" {
		return v
	}
	return apierr.NewRequestID()
}

func mustIdentity(r *http.Request) identity {
	id, _ := r.Context().Value(ctxIdentity).(identity)
	return id
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

func formatRetry(d time.Duration) string {
	sec := int(d.Seconds())
	if sec < 1 {
		sec = 1
	}
	return strconv.Itoa(sec)
}

func userPublic(u store.User) map[string]any {
	return map[string]any{
		"id":         u.ID.String(),
		"email":      u.Email,
		"role":       u.Role,
		"status":     u.Status,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func apiKeyPublic(k store.APIKey) map[string]any {
	m := map[string]any{
		"id":         k.ID.String(),
		"label":      k.Label,
		"scopes":     k.Scopes,
		"created_at": k.CreatedAt.UTC().Format(time.RFC3339),
	}
	if k.ExpiresAt != nil {
		m["expires_at"] = k.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return m
}
