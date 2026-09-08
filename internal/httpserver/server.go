package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/health"
)

// ListenAndServe runs a process with GET /healthz until SIGINT/SIGTERM.
func ListenAndServe(service, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health.Handler(service))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return context.Background()
		},
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "service", service, "addr", addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		slog.Info("shutting down", "service", service, "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return err
		}
		return <-errCh
	case err := <-errCh:
		return err
	}
}

// AddrFromEnv reads HTTP_ADDR or returns fallback (":8080" style).
func AddrFromEnv(fallback string) string {
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		return v
	}
	return fallback
}
