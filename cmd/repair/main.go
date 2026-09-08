package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
)

func main() {
	if err := httpserver.ListenAndServe("repair", httpserver.AddrFromEnv(":8084")); err != nil {
		slog.Error("repair exited", "err", err)
		os.Exit(1)
	}
}
