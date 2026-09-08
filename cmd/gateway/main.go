package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
)

func main() {
	if err := httpserver.ListenAndServe("gateway", httpserver.AddrFromEnv(":8080")); err != nil {
		slog.Error("gateway exited", "err", err)
		os.Exit(1)
	}
}
