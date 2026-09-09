package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/health"
)

func TestNewMuxHealthz(t *testing.T) {
	t.Parallel()
	mux := NewMux("gateway")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body health.Response
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Service != "gateway" || body.Status != "ok" {
		t.Fatalf("body %+v", body)
	}
}
