package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAllowThenBlock(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	lim := New(rdb)
	fixed := time.Unix(1_700_000_000, 0)
	lim.now = func() time.Time { return fixed }

	ctx := context.Background()
	for i := 0; i < AuthLimit; i++ {
		res, err := lim.AllowAuth(ctx, "1.2.3.4")
		if err != nil || !res.Allowed {
			t.Fatalf("iter %d: %+v %v", i, res, err)
		}
	}
	res, err := lim.AllowAuth(ctx, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed {
		t.Fatal("expected rate limit")
	}
	if res.RetryAfter < time.Second {
		t.Fatalf("retry-after %s", res.RetryAfter)
	}

	res, err = lim.AllowAuth(ctx, "9.9.9.9")
	if err != nil || !res.Allowed {
		t.Fatalf("other ip: %+v %v", res, err)
	}
}

func TestAllowReadIndependent(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	lim := New(rdb)
	ctx := context.Background()
	res, err := lim.AllowRead(ctx, "user-1")
	if err != nil || !res.Allowed {
		t.Fatalf("%+v %v", res, err)
	}
}
