package invariants

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
)

func TestCheckAllEmptyFleet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres integration test")
	}
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://dsp:dsp@127.0.0.1:5433/dsp?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s, err := store.Open(ctx, url)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(s.Close)
	if err := CheckAll(context.Background(), s); err != nil {
		t.Fatal(err)
	}
}
