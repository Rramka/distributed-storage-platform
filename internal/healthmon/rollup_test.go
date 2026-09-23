package healthmon

import (
	"context"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestExpectedHeartbeats(t *testing.T) {
	t.Parallel()
	if ExpectedHeartbeats(time.Hour) != 360 {
		t.Fatalf("got %v", ExpectedHeartbeats(time.Hour))
	}
}

func TestRollupWindowUptimeFromHeartbeats(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "hb-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.CreateNode(ctx, store.CreateNodeParams{
		OwnerID:         u.ID,
		CertFingerprint: []byte(uuid.NewString()),
		PublicKey:       make([]byte, 32),
		CertPEM:         "pem",
		CertExpiresAt:   time.Now().Add(time.Hour),
		OS:              "linux",
		AgentVersion:    "test",
		Endpoint:        "127.0.0.1:1",
		CapacityBytes:   1 << 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	window := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	if err := rdb.Set(ctx, HeartbeatCountKey(n.ID, window), 180, 0).Err(); err != nil {
		t.Fatal(err)
	}
	RollupWindow(ctx, s, rdb, nil, window)
	got, err := s.ListNodeStats(ctx, n.ID, window, window.Add(time.Hour))
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	if got[0].UptimeRatio < 0.4 || got[0].UptimeRatio > 0.6 {
		t.Fatalf("uptime %v want ~0.5", got[0].UptimeRatio)
	}
}
