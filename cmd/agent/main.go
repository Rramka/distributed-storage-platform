package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
)

func main() {
	if err := httpserver.ListenAndServe("agent", httpserver.AddrFromEnv(":9000"), nil); err != nil {
		slog.Error("agent exited", "err", err)
		os.Exit(1)
	}
}
