package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
)

func main() {
	if err := httpserver.ListenAndServe("metadata", httpserver.AddrFromEnv(":8081")); err != nil {
		slog.Error("metadata exited", "err", err)
		os.Exit(1)
	}
}
