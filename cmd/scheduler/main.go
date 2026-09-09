package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
)

func main() {
	if err := httpserver.ListenAndServe("scheduler", httpserver.AddrFromEnv(":8082"), nil); err != nil {
		slog.Error("scheduler exited", "err", err)
		os.Exit(1)
	}
}
