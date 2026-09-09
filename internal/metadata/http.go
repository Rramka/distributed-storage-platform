package metadata

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/apierr"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

// Mount registers internal HTTP routes on mux. GET /healthz should already be present.
func Mount(mux *http.ServeMux, svc Service) {
	mux.HandleFunc("POST /internal/users", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if !decode(w, r, &req) {
			return
		}
		u, err := svc.CreateUser(r.Context(), req.Email, req.Password)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusCreated, userJSON(u))
	})

	mux.HandleFunc("POST /internal/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if !decode(w, r, &req) {
			return
		}
		u, err := svc.VerifyPassword(r.Context(), req.Email, req.Password)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusOK, userJSON(u))
	})

	mux.HandleFunc("GET /internal/auth/api-keys/{hash}", func(w http.ResponseWriter, r *http.Request) {
		k, u, err := svc.LookupAPIKey(r.Context(), r.PathValue("hash"))
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"key":  apiKeyJSON(k),
			"user": userJSON(u),
		})
	})

	mux.HandleFunc("POST /internal/users/{userID}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		var req struct {
			KeyHash   string     `json:"key_hash"`
			Label     string     `json:"label"`
			Scopes    []string   `json:"scopes"`
			ExpiresAt *time.Time `json:"expires_at"`
		}
		if !decode(w, r, &req) {
			return
		}
		k, err := svc.CreateAPIKey(r.Context(), userID, req.KeyHash, req.Label, req.Scopes, req.ExpiresAt)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusCreated, apiKeyJSON(k))
	})

	mux.HandleFunc("GET /internal/users/{userID}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		keys, err := svc.ListAPIKeys(r.Context(), userID)
		if writeErr(w, r, err) {
			return
		}
		out := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, apiKeyJSON(k))
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_keys": out})
	})

	mux.HandleFunc("DELETE /internal/users/{userID}/api-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		keyID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		if writeErr(w, r, svc.RevokeAPIKey(r.Context(), userID, keyID)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /internal/users/{userID}/buckets", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &req) {
			return
		}
		b, err := svc.CreateBucket(r.Context(), userID, req.Name)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusCreated, bucketJSON(b))
	})

	mux.HandleFunc("GET /internal/users/{userID}/buckets", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		buckets, err := svc.ListBuckets(r.Context(), userID)
		if writeErr(w, r, err) {
			return
		}
		out := make([]map[string]any, 0, len(buckets))
		for _, b := range buckets {
			out = append(out, bucketJSON(b))
		}
		writeJSON(w, http.StatusOK, map[string]any{"buckets": out})
	})

	mux.HandleFunc("DELETE /internal/users/{userID}/buckets/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		if writeErr(w, r, svc.DeleteBucket(r.Context(), userID, id)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /internal/users/{userID}/folders", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		var req struct {
			BucketID string `json:"bucket_id"`
			Path     string `json:"path"`
		}
		if !decode(w, r, &req) {
			return
		}
		bid, err := uuid.Parse(req.BucketID)
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		f, err := svc.CreateFolder(r.Context(), userID, bid, req.Path)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusCreated, fileJSON(f))
	})

	mux.HandleFunc("GET /internal/users/{userID}/files", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		bid, err := uuid.Parse(r.URL.Query().Get("bucket_id"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		files, next, err := svc.ListFiles(r.Context(), userID, bid, r.URL.Query().Get("prefix"), r.URL.Query().Get("cursor"), limit)
		if writeErr(w, r, err) {
			return
		}
		out := make([]map[string]any, 0, len(files))
		for _, f := range files {
			out = append(out, fileJSON(f))
		}
		body := map[string]any{"files": out}
		if next != "" {
			body["next_cursor"] = next
		}
		writeJSON(w, http.StatusOK, body)
	})

	mux.HandleFunc("GET /internal/users/{userID}/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		f, err := svc.GetFile(r.Context(), userID, id)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusOK, fileJSON(f))
	})

	mux.HandleFunc("PUT /internal/users/{userID}/rename", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		var req struct {
			FileID  string `json:"file_id"`
			NewPath string `json:"new_path"`
		}
		if !decode(w, r, &req) {
			return
		}
		fid, err := uuid.Parse(req.FileID)
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		f, err := svc.RenameFile(r.Context(), userID, fid, req.NewPath)
		if writeErr(w, r, err) {
			return
		}
		writeJSON(w, http.StatusOK, fileJSON(f))
	})

	mux.HandleFunc("DELETE /internal/users/{userID}/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseUser(w, r)
		if !ok {
			return
		}
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeErr(w, r, ErrInvalid)
			return
		}
		if writeErr(w, r, svc.DeleteFile(r.Context(), userID, id)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func parseUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		writeErr(w, r, ErrInvalid)
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeErr(w, r, ErrInvalid)
		return false
	}
	return true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	rid := requestID(r)
	switch {
	case errors.Is(err, ErrInvalid):
		apierr.Write(w, apierr.CodeInvalidRequest, "invalid request", rid)
	case errors.Is(err, ErrUnauthenticated):
		apierr.Write(w, apierr.CodeUnauthenticated, "unauthenticated", rid)
	case errors.Is(err, ErrForbidden):
		apierr.Write(w, apierr.CodeForbidden, "forbidden", rid)
	case errors.Is(err, ErrNotFound), errors.Is(err, store.ErrNotFound):
		apierr.Write(w, apierr.CodeNotFound, "not found", rid)
	case errors.Is(err, ErrConflict), errors.Is(err, store.ErrConflict):
		apierr.Write(w, apierr.CodeAlreadyExists, "already exists", rid)
	case errors.Is(err, ErrNotEmpty), errors.Is(err, store.ErrNotEmpty):
		apierr.Write(w, apierr.CodeConflict, "bucket is not empty", rid)
	default:
		slog.Error("metadata handler", "err", err, "request_id", rid)
		apierr.Write(w, apierr.CodeInternal, "internal error", rid)
	}
	return true
}

func requestID(r *http.Request) string {
	if v := r.Header.Get("X-Request-ID"); v != "" {
		return v
	}
	return apierr.NewRequestID()
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func userJSON(u store.User) map[string]any {
	return map[string]any{
		"id":         u.ID.String(),
		"email":      u.Email,
		"role":       u.Role,
		"status":     u.Status,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func apiKeyJSON(k store.APIKey) map[string]any {
	m := map[string]any{
		"id":         k.ID.String(),
		"user_id":    k.UserID.String(),
		"label":      k.Label,
		"scopes":     k.Scopes,
		"created_at": k.CreatedAt.UTC().Format(time.RFC3339),
	}
	if k.ExpiresAt != nil {
		m["expires_at"] = k.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return m
}

func bucketJSON(b store.Bucket) map[string]any {
	return map[string]any{
		"id":         b.ID.String(),
		"owner_id":   b.OwnerID.String(),
		"name":       b.Name,
		"created_at": b.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func fileJSON(f store.File) map[string]any {
	return map[string]any{
		"id":         f.ID.String(),
		"bucket_id":  f.BucketID.String(),
		"path":       f.Path,
		"is_folder":  f.IsFolder,
		"status":     f.Status,
		"created_at": f.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": f.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
