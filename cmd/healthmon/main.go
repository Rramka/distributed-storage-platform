package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
)

func main() {
	if err := httpserver.ListenAndServe("healthmon", httpserver.AddrFromEnv(":8083")); err != nil {
		slog.Error("healthmon exited", "err", err)
		os.Exit(1)
	}
}
