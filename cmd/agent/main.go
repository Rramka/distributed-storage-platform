package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rramka/distributed-storage-platform/internal/agent"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
)

func main() {
	yamlPath := os.Getenv("AGENT_CONFIG")
	cfg, err := agent.LoadConfig(yamlPath)
	if err != nil {
		slog.Error("agent config", "err", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a, err := agent.Open(ctx, cfg)
	if err != nil {
		slog.Error("agent open", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	go func() {
		if err := httpserver.ListenAndServe("agent", cfg.HealthAddr, nil); err != nil {
			slog.Error("agent health", "err", err)
		}
	}()

	slog.Info("agent serving fragments", "node_id", a.ID().String(), "addr", cfg.FragmentAddr)
	if err := a.ServeFragments(ctx); err != nil && ctx.Err() == nil {
		slog.Error("agent exited", "err", err)
		os.Exit(1)
	}
}
