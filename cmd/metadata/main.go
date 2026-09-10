package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Rramka/distributed-storage-platform/internal/ca"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/metadata"
	"github.com/Rramka/distributed-storage-platform/internal/scheduler"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/Rramka/distributed-storage-platform/internal/tickets"
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

	svc := &metadata.StoreService{Store: st}
	if dir := os.Getenv("CA_DIR"); dir != "" {
		c, err := ca.EnsureCA(dir)
		if err != nil {
			slog.Error("ca", "err", err)
			os.Exit(1)
		}
		svc.IssueNode = metadata.IssueNodeFromCA(c)
	}
	if seed := os.Getenv("TICKET_SIGNING_SEED"); seed != "" {
		raw, err := tickets.ParseSeed(seed)
		if err != nil {
			slog.Error("ticket seed", "err", err)
			os.Exit(1)
		}
		signer, err := tickets.NewSignerFromSeed(raw)
		if err != nil {
			slog.Error("ticket signer", "err", err)
			os.Exit(1)
		}
		svc.SignTicket = metadata.SignTicketFromSigner(signer)
		slog.Info("ticket public key", "hex", signer.PublicKeyHex())
	}
	if su := os.Getenv("SCHEDULER_URL"); su != "" {
		sc := scheduler.NewClient(su)
		svc.Place = func(ctx context.Context, needs []metadata.PlaceNeed) ([]metadata.PlaceAssign, error) {
			var sn []scheduler.Need
			for _, n := range needs {
				sn = append(sn, scheduler.Need{FragmentID: n.FragmentID, ChunkID: n.ChunkID, SizeBytes: n.SizeBytes})
			}
			as, err := sc.Place(ctx, sn)
			if err != nil {
				return nil, err
			}
			out := make([]metadata.PlaceAssign, 0, len(as))
			for _, a := range as {
				out = append(out, metadata.PlaceAssign{FragmentID: a.FragmentID, Node: a.Node})
			}
			return out, nil
		}
	}

	mux := httpserver.NewMux("metadata")
	metadata.Mount(mux, svc)
	metadata.MountRepair(mux, svc)
	if err := httpserver.ListenAndServe("metadata", httpserver.AddrFromEnv(":8081"), mux); err != nil {
		slog.Error("metadata exited", "err", err)
		os.Exit(1)
	}
}
