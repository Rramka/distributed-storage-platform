package main

import (
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
)

func main() {
	dir := os.Getenv("CA_DIR")
	if dir == "" {
		dir = "/ca"
	}
	c, err := ca.EnsureCA(dir)
	if err != nil {
		slog.Error("ca-init", "err", err)
		os.Exit(1)
	}
	if _, err := c.EnsureServiceCert(dir, "healthmon"); err != nil {
		slog.Error("ca-init healthmon cert", "err", err)
		os.Exit(1)
	}
	if seed := os.Getenv("TICKET_SIGNING_SEED"); seed != "" {
		raw, err := tickets.ParseSeed(seed)
		if err != nil {
			slog.Error("ticket seed", "err", err)
			os.Exit(1)
		}
		s, err := tickets.NewSignerFromSeed(raw)
		if err != nil {
			slog.Error("ticket signer", "err", err)
			os.Exit(1)
		}
		if err := os.WriteFile(dir+"/ticket.pub", []byte(s.PublicKeyHex()+"\n"), 0o644); err != nil {
			slog.Error("ticket.pub", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("ca ready", "dir", dir)
}
