package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/gateway"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

func main() {
	metaURL := os.Getenv("METADATA_URL")
	if metaURL == "" {
		slog.Error("METADATA_URL is required")
		os.Exit(1)
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		slog.Error("REDIS_URL is required")
		os.Exit(1)
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("REDIS_URL", "err", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	mux := httpserver.NewMux("gateway")
	h := gateway.New(mux, metadata.NewClient(metaURL), ratelimit.New(rdb))
	if err := httpserver.ListenAndServe("gateway", httpserver.AddrFromEnv(":8080"), h); err != nil {
		slog.Error("gateway exited", "err", err)
		os.Exit(1)
	}
}
