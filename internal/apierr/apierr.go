// Package apierr implements the public error envelope from docs/08-api.md.
package apierr

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

const (
	CodeInvalidRequest       = "invalid_request"
	CodeUnauthenticated      = "unauthenticated"
	CodeForbidden            = "forbidden"
	CodeNotFound             = "not_found"
	CodeConflict             = "conflict"
	CodeAlreadyExists        = "already_exists"
	CodeRateLimited          = "rate_limited"
	CodeInternal             = "internal"
	CodeManifestInvalid      = "manifest_invalid"
	CodePlacementUnavailable = "placement_unavailable"
)

// Envelope is the JSON body for every non-2xx response.
type Envelope struct {
	Error Body `json:"error"`
}

// Body is the inner error object.
type Body struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// Status maps an error code to its HTTP status.
func Status(code string) int {
	switch code {
	case CodeInvalidRequest, CodeManifestInvalid:
		return http.StatusBadRequest
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict, CodeAlreadyExists:
		return http.StatusConflict
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodePlacementUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// NewRequestID returns a req_… identifier.
func NewRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req_000000000000000000000000"
	}
	return "req_" + hex.EncodeToString(b[:])
}

// Write emits the standard error envelope and status derived from code.
func Write(w http.ResponseWriter, code, message, requestID string) {
	WriteStatus(w, Status(code), code, message, requestID)
}

// WriteStatus emits the envelope with an explicit HTTP status.
func WriteStatus(w http.ResponseWriter, status int, code, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Error: Body{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}})
}
