package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

func main() {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		slog.Error("POSTGRES_URL is required")
		os.Exit(1)
	}
	st, err := store.Open(context.Background(), url)
	if err != nil {
		slog.Error("metadata store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	mux := httpserver.NewMux("metadata")
	metadata.Mount(mux, &metadata.StoreService{Store: st})
	if err := httpserver.ListenAndServe("metadata", httpserver.AddrFromEnv(":8081"), mux); err != nil {
		slog.Error("metadata exited", "err", err)
		os.Exit(1)
	}
}
