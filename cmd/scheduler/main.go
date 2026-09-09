package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/scheduler"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/redis/go-redis/v9"
)

func main() {
	pg := os.Getenv("POSTGRES_URL")
	if pg == "" {
		slog.Error("POSTGRES_URL is required")
		os.Exit(1)
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		slog.Error("REDIS_URL is required")
		os.Exit(1)
	}
	st, err := store.Open(context.Background(), pg)
	if err != nil {
		slog.Error("scheduler store", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("REDIS_URL", "err", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	mux := httpserver.NewMux("scheduler")
	scheduler.Mount(mux, &scheduler.Service{Store: st, Redis: rdb})
	if err := httpserver.ListenAndServe("scheduler", httpserver.AddrFromEnv(":8082"), mux); err != nil {
		slog.Error("scheduler exited", "err", err)
		os.Exit(1)
	}
}
