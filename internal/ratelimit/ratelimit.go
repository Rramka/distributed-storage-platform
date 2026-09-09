// Package ratelimit implements Redis token buckets from docs/08-api.md.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// AuthLimit is 10 requests per minute per IP.
	AuthLimit = 10
	// ReadLimit is 600 metadata reads per minute per account.
	ReadLimit = 600
	// PlanLimit is 60 upload/download plans per minute per account (unused until M2).
	PlanLimit = 60
)

var tokenBucketLua = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_per_sec = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local data = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil then
  tokens = capacity
  ts = now
end
tokens = math.min(capacity, tokens + (now - ts) * refill_per_sec)
if tokens < requested then
  redis.call('HSET', key, 'tokens', tokens, 'ts', now)
  redis.call('EXPIRE', key, 120)
  local wait = 0
  if refill_per_sec > 0 then
    wait = math.ceil((requested - tokens) / refill_per_sec)
  end
  return {0, wait}
end
tokens = tokens - requested
redis.call('HSET', key, 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', key, 120)
return {1, 0}
`)

// Limiter is a Redis-backed token bucket.
type Limiter struct {
	rdb *redis.Client
	now func() time.Time
}

// New wraps a Redis client.
func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb, now: time.Now}
}

// Result is the outcome of Allow.
type Result struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Allow consumes one token from the named bucket.
func (l *Limiter) Allow(ctx context.Context, key string, capacity int, window time.Duration) (Result, error) {
	refill := float64(capacity) / window.Seconds()
	now := float64(l.now().Unix())
	vals, err := tokenBucketLua.Run(ctx, l.rdb, []string{key}, capacity, refill, now, 1).Int64Slice()
	if err != nil {
		return Result{}, fmt.Errorf("ratelimit.allow: %w", err)
	}
	if len(vals) < 2 {
		return Result{}, fmt.Errorf("ratelimit.allow: unexpected script result")
	}
	if vals[0] == 1 {
		return Result{Allowed: true}, nil
	}
	return Result{Allowed: false, RetryAfter: time.Duration(vals[1]) * time.Second}, nil
}

// AllowAuth is 10/min per IP.
func (l *Limiter) AllowAuth(ctx context.Context, ip string) (Result, error) {
	return l.Allow(ctx, "rl:auth:"+ip, AuthLimit, time.Minute)
}

// AllowRead is 600/min per account.
func (l *Limiter) AllowRead(ctx context.Context, userID string) (Result, error) {
	return l.Allow(ctx, "rl:read:"+userID, ReadLimit, time.Minute)
}

// AllowPlan is 60/min per account for upload/download planning.
func (l *Limiter) AllowPlan(ctx context.Context, userID string) (Result, error) {
	return l.Allow(ctx, "rl:plan:"+userID, PlanLimit, time.Minute)
}
