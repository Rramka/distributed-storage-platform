package ledger

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/invariants"
	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func TestPostTxnConcurrentIdempotent(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	led := &Ledger{Store: s, Rates: DefaultRates()}
	a, err := led.EnsureAccount(ctx, OwnerCustomer, ptr(uuid.New()), KindBalance)
	if err != nil {
		t.Fatal(err)
	}
	b, err := led.EnsureAccount(ctx, OwnerPlatform, nil, KindRevenue)
	if err != nil {
		t.Fatal(err)
	}
	txn := uuid.New()
	entries := []Entry{
		{AccountID: a.ID, Amount: -100, Type: TypeStorageCharge, Reference: map[string]any{"window": time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}},
		{AccountID: b.ID, Amount: 100, Type: TypeStorageCharge},
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- led.PostTxn(ctx, txn, entries)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.CountLedgerEntries(ctx, txn)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("entries %d want 2", n)
	}
}

func TestPostAdjustmentRequiresReason(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	led := &Ledger{Store: s, Rates: DefaultRates()}
	debit, err := led.EnsureAccount(ctx, OwnerCustomer, ptr(uuid.New()), KindBalance)
	if err != nil {
		t.Fatal(err)
	}
	credit, err := led.EnsureAccount(ctx, OwnerPlatform, nil, KindRevenue)
	if err != nil {
		t.Fatal(err)
	}
	if err := led.PostAdjustment(ctx, debit.ID, credit.ID, 50, ""); err == nil {
		t.Fatal("empty reason")
	}
	if err := led.PostAdjustment(ctx, debit.ID, credit.ID, 50, "correct overcharge"); err != nil {
		t.Fatal(err)
	}
	bal, err := s.AccountBalance(ctx, debit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bal != -50 {
		t.Fatalf("balance %d", bal)
	}
}

func TestCatchUpBackfillsMissedHours(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	cust, err := s.CreateUser(ctx, "catchup-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 2, 1, 5, 0, 0, 0, time.UTC)
	meters := fakeMeters{
		customers: []store.CustomerStored{{OwnerID: cust.ID, SizeBytes: BytesPerGB}},
	}
	led := &Ledger{Store: s, Rates: DefaultRates()}
	accrue := &Accruer{Ledger: led, Meters: meters, Clock: func() time.Time { return now }, Name: "test-" + uuid.NewString()}
	if err := accrue.CatchUp(ctx); err != nil {
		t.Fatal(err)
	}
	mark, err := s.AccrualWatermark(ctx, accrue.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !mark.Equal(time.Date(2026, 2, 1, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("watermark %s", mark)
	}
	now = time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC)
	if err := accrue.CatchUp(ctx); err != nil {
		t.Fatal(err)
	}
	acct, err := s.MustAccount(ctx, OwnerCustomer, ptr(cust.ID), KindBalance)
	if err != nil {
		t.Fatal(err)
	}
	bal, err := s.AccountBalance(ctx, acct.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := -StorageChargeµCRD(BytesPerGB, DefaultRates()) * 4 // 04,05,06,07
	if bal != want {
		t.Fatalf("balance %d want %d", bal, want)
	}
}

func TestRunRetriesAfterError(t *testing.T) {
	s := store.OpenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cust, err := s.CreateUser(ctx, "retry-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 1, 2, 0, 0, 0, time.UTC)
	m := &failOnceMeters{
		fakeMeters: fakeMeters{customers: []store.CustomerStored{{OwnerID: cust.ID, SizeBytes: BytesPerGB}}},
		fails:      1,
	}
	led := &Ledger{Store: s, Rates: DefaultRates()}
	accrue := &Accruer{Ledger: led, Meters: m, Clock: func() time.Time { return now }, Tick: 20 * time.Millisecond, Name: "test-" + uuid.NewString()}
	done := make(chan struct{})
	go func() {
		accrue.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		acct, err := s.MustAccount(ctx, OwnerCustomer, ptr(cust.ID), KindBalance)
		if err == nil {
			bal, err := s.AccountBalance(ctx, acct.ID)
			if err == nil && bal < 0 {
				cancel()
				<-done
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	t.Fatal("accruer did not recover")
}

type failOnceMeters struct {
	fakeMeters
	fails int32
}

func (f *failOnceMeters) CustomerStored(ctx context.Context) ([]store.CustomerStored, error) {
	if atomic.AddInt32(&f.fails, -1) >= 0 {
		return nil, errors.New("transient")
	}
	return f.fakeMeters.CustomerStored(ctx)
}

func TestAccrueWindowRealMeters(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	fleet, err := s.SeedCommittedFleet(ctx, nil, nil, BytesPerGB)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Exec(ctx, `UPDATE nodes SET reputation = 1`); err != nil {
		t.Fatal(err)
	}
	for _, id := range fleet.PlaceIDs {
		if err := s.Exec(ctx, `UPDATE fragments SET size_bytes = $1 WHERE id = $2`, int64(BytesPerGB), id); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range fleet.Nodes {
		if err := s.Exec(ctx, `
			INSERT INTO node_stats (node_id, window_start, uptime_ratio, audits_passed, audits_failed)
			VALUES ($1, date_trunc('hour', now()), 1, 1, 0)
			ON CONFLICT (node_id, window_start) DO UPDATE SET uptime_ratio = 1, audits_passed = 1
		`, n.ID); err != nil {
			t.Fatal(err)
		}
	}
	window := time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC)
	led := &Ledger{Store: s, Rates: DefaultRates()}
	accrue := &Accruer{Ledger: led, Clock: func() time.Time { return window.Add(time.Hour) }}
	held, err := s.ListNodeHeldBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var held0 int64
	for _, h := range held {
		if h.NodeID == fleet.Nodes[0].ID {
			held0 = h.SizeBytes
		}
	}
	up, audit, err := s.NodeReliability(ctx, fleet.Nodes[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if held0 < BytesPerGB || up < 1 || audit < 1 {
		t.Fatalf("meter held=%d up=%v audit=%v", held0, up, audit)
	}
	if err := accrue.AccrueWindow(ctx, window); err != nil {
		t.Fatal(err)
	}
	if err := invariants.CheckLedger(ctx, s); err != nil {
		t.Fatal(err)
	}
	view, err := led.Storage(ctx, fleet.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.BytesStored != BytesPerGB {
		t.Fatalf("stored %d", view.BytesStored)
	}
	if view.ChargesUCRD <= 0 {
		t.Fatalf("charges %d", view.ChargesUCRD)
	}
	prov := fleet.Nodes[0].OwnerID
	earn, err := led.Earnings(ctx, prov, window, window.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if earn.EarningsUCRD <= 0 {
		t.Fatalf("earnings %d", earn.EarningsUCRD)
	}
	found := false
	for _, n := range earn.Nodes {
		if n.NodeID == fleet.Nodes[0].ID && n.StorageUCRD > 0 && len(n.Windows) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("missing per-node storage window")
	}
}
