package main

import (
	"context"
	"crypto/x509"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/repair"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

func main() {
	go func() {
		if err := httpserver.ListenAndServe("repair", httpserver.AddrFromEnv(":8084"), nil); err != nil {
			slog.Error("repair healthz", "err", err)
		}
	}()

	pg := os.Getenv("POSTGRES_URL")
	natsURL := os.Getenv("NATS_URL")
	metaURL := os.Getenv("METADATA_URL")
	caDir := os.Getenv("CA_DIR")
	if pg == "" || natsURL == "" || metaURL == "" || caDir == "" {
		slog.Error("POSTGRES_URL, NATS_URL, METADATA_URL, CA_DIR required")
		os.Exit(1)
	}
	st, err := store.Open(context.Background(), pg)
	if err != nil {
		slog.Error("repair store", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	bus, err := events.Connect(context.Background(), natsURL)
	if err != nil {
		slog.Error("repair nats", "err", err)
		os.Exit(1)
	}
	defer bus.Close()

	c, err := ca.EnsureCA(caDir)
	if err != nil {
		slog.Error("repair ca", "err", err)
		os.Exit(1)
	}
	var leaf *x509.Certificate
	if len(c.Cert.Raw) > 0 {
		leaf = c.Cert
	}

	maxC := 4
	if v := os.Getenv("REPAIR_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxC = n
		}
	}
	var budget int64
	if v := os.Getenv("REPAIR_BUDGET_MBPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			budget = int64(n) * 1024 * 1024
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	w := &repair.Worker{
		Store:         st,
		Bus:           bus,
		Meta:          repair.NewMeta(metaURL),
		CA:            leaf,
		MaxConcurrent: maxC,
		BudgetBps:     budget,
	}
	slog.Info("repair worker running")
	if err := w.Run(ctx); err != nil && !errorsIsCanceled(err) {
		slog.Error("repair exited", "err", err)
		os.Exit(1)
	}
}

func errorsIsCanceled(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}
