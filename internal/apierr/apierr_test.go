package apierr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code string
		want int
	}{
		{CodeInvalidRequest, http.StatusBadRequest},
		{CodeUnauthenticated, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeNotFound, http.StatusNotFound},
		{CodeConflict, http.StatusConflict},
		{CodeAlreadyExists, http.StatusConflict},
		{CodeRateLimited, http.StatusTooManyRequests},
		{CodeInternal, http.StatusInternalServerError},
		{"unknown", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			if got := Status(tt.code); got != tt.want {
				t.Fatalf("Status(%q) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func TestWriteEnvelope(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	Write(rec, CodeNotFound, "file missing", "req_abc")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %q", ct)
	}
	var env Envelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != CodeNotFound || env.Error.Message != "file missing" || env.Error.RequestID != "req_abc" {
		t.Fatalf("body %+v", env.Error)
	}
}

func TestNewRequestID(t *testing.T) {
	t.Parallel()
	id := NewRequestID()
	if !strings.HasPrefix(id, "req_") {
		t.Fatalf("prefix: %q", id)
	}
	if len(id) != 4+24 {
		t.Fatalf("len %d want 28: %q", len(id), id)
	}
	other := NewRequestID()
	if id == other {
		t.Fatal("expected unique request ids")
	}
}
