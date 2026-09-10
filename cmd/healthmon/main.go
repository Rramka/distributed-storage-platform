package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/healthmon"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/redis/go-redis/v9"
)

func main() {
	go func() {
		if err := httpserver.ListenAndServe("healthmon", httpserver.AddrFromEnv(":8083"), nil); err != nil {
			slog.Error("healthmon healthz", "err", err)
		}
	}()

	pg := os.Getenv("POSTGRES_URL")
	redisURL := os.Getenv("REDIS_URL")
	caDir := os.Getenv("CA_DIR")
	natsURL := os.Getenv("NATS_URL")
	if pg == "" || redisURL == "" || caDir == "" || natsURL == "" {
		slog.Error("POSTGRES_URL, REDIS_URL, CA_DIR, NATS_URL required")
		os.Exit(1)
	}
	st, err := store.Open(context.Background(), pg)
	if err != nil {
		slog.Error("healthmon store", "err", err)
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

	bus, err := events.Connect(context.Background(), natsURL)
	if err != nil {
		slog.Error("healthmon nats", "err", err)
		os.Exit(1)
	}
	defer bus.Close()

	c, err := ca.EnsureCA(caDir)
	if err != nil {
		slog.Error("ca", "err", err)
		os.Exit(1)
	}
	cert, err := c.EnsureServiceCert(caDir, "healthmon")
	if err != nil {
		slog.Error("healthmon cert", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	healthmon.Mount(mux, &healthmon.Server{Store: st, Redis: rdb, Bus: bus})
	suspect, offline := healthmon.DurationsFromEnv()
	mon := &healthmon.Monitor{
		Store:        st,
		Bus:          bus,
		SuspectAfter: suspect,
		OfflineAfter: offline,
	}
	go mon.Run(context.Background())
	addr := os.Getenv("HEARTBEAT_ADDR")
	if addr == "" {
		addr = ":8443"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		TLSConfig:         c.ServerTLSConfig(cert, true),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("healthmon mTLS listening", "addr", addr)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		slog.Error("healthmon exited", "err", err)
		os.Exit(1)
	}
}
