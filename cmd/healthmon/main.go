package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/healthmon"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pg := os.Getenv("POSTGRES_URL")
	redisURL := os.Getenv("REDIS_URL")
	caDir := os.Getenv("CA_DIR")
	natsURL := os.Getenv("NATS_URL")
	if pg == "" || redisURL == "" || caDir == "" || natsURL == "" {
		slog.Error("POSTGRES_URL, REDIS_URL, CA_DIR, NATS_URL required")
		os.Exit(1)
	}
	st, err := store.Open(ctx, pg)
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

	bus, err := events.Connect(ctx, natsURL)
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

	healthz := httpserver.NewServer(httpserver.AddrFromEnv(":8083"), httpserver.NewMux("healthmon"))
	go func() {
		if err := healthz.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("healthmon healthz", "err", err)
		}
	}()

	mux := http.NewServeMux()
	healthmon.Mount(mux, &healthmon.Server{Store: st, Redis: rdb, Bus: bus})
	suspect, offline := healthmon.DurationsFromEnv()
	mon := &healthmon.Monitor{
		Store:        st,
		Bus:          bus,
		SuspectAfter: suspect,
		OfflineAfter: offline,
	}
	go mon.Run(ctx)
	if mu := os.Getenv("METADATA_URL"); mu != "" {
		tick, interval := healthmon.ChallengeDurationsFromEnv()
		ch := &healthmon.Challenger{
			Store:    st,
			Bus:      bus,
			Meta:     healthmon.NewChallengeMeta(mu),
			CA:       c.Cert,
			Tick:     tick,
			Interval: interval,
		}
		go ch.Run(ctx)
		go healthmon.RunRollup(ctx, st, rdb)
	}
	addr := os.Getenv("HEARTBEAT_ADDR")
	if addr == "" {
		addr = ":8443"
	}
	srv := httpserver.NewTLSServer(addr, mux, c.ServerTLSConfig(cert, true))
	go func() {
		slog.Info("healthmon mTLS listening", "addr", addr)
		if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("healthmon exited", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("healthmon shutting down")
	shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
	_ = healthz.Shutdown(shut)
}
