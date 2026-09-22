package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// OpenForTest opens Postgres for integration tests.
// Skips when the database is unreachable unless DSP_REQUIRE_PG is set, in which case it fails.
func OpenForTest(t *testing.T) *Store {
	t.Helper()
	if testing.Short() && os.Getenv("DSP_REQUIRE_PG") == "" {
		t.Skip("skipping postgres integration test")
	}
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://dsp:dsp@127.0.0.1:5433/dsp?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := Open(ctx, url)
	if err != nil {
		if os.Getenv("DSP_REQUIRE_PG") != "" {
			t.Fatalf("postgres required: %v", err)
		}
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// Exec runs a one-off statement (test fixtures and red-team plants).
func (s *Store) Exec(ctx context.Context, q string, args ...any) error {
	_, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return mapQueryErr("store.exec", err)
	}
	return nil
}

// InsertStoredPlacement inserts a stored placement row (test helper).
func (s *Store) InsertStoredPlacement(ctx context.Context, fragmentID, nodeID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO fragment_placements (fragment_id, node_id, status, stored_at)
		VALUES ($1, $2, 'stored', now())
	`, fragmentID, nodeID)
	if err != nil {
		return mapQueryErr("store.insertStoredPlacement", err)
	}
	return nil
}
