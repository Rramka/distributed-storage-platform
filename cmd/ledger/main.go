package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/events"
	"github.com/Rramka/distributed-storage-platform/internal/httpserver"
	"github.com/Rramka/distributed-storage-platform/internal/ledger"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

func main() {
	pg := os.Getenv("POSTGRES_URL")
	natsURL := os.Getenv("NATS_URL")
	if pg == "" {
		slog.Error("POSTGRES_URL required")
		os.Exit(1)
	}
	st, err := store.Open(context.Background(), pg)
	if err != nil {
		slog.Error("ledger store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	led := &ledger.Ledger{Store: st, Rates: ledger.DefaultRates()}
	accrue := &ledger.Accruer{Ledger: led, Meters: ledger.StoreMeters{Store: st}}

	mux := httpserver.NewMux("ledger")
	ledger.Mount(mux, led)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go accrue.Run(ctx)
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				now := time.Now().UTC()
				start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
				end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
				if err := led.CloseCycle(ctx, start, end); err != nil {
					slog.Error("ledger.closeCycle", "err", err)
				}
			}
		}
	}()

	if natsURL != "" {
		bus, err := events.Connect(ctx, natsURL)
		if err != nil {
			slog.Error("ledger nats", "err", err)
			os.Exit(1)
		}
		defer bus.Close()
		go func() {
			_ = bus.Consume(ctx, events.StreamUsage, "ledger-usage", "usage.>", func(ctx context.Context, subject string, data []byte) error {
				var ev events.UsageEvent
				if err := json.Unmarshal(data, &ev); err != nil {
					return err
				}
				id := ev.MsgID()
				inserted, err := st.RecordUsageEvent(ctx, id, ev.Kind, ev.SubjectID, ev.Window, data)
				if err != nil {
					return err
				}
				if inserted {
					return st.MarkUsageConsumed(ctx, id)
				}
				return nil
			})
		}()
	}

	if err := httpserver.ListenAndServe("ledger", httpserver.AddrFromEnv(":8085"), mux); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("ledger exited", "err", err)
		os.Exit(1)
	}
}
