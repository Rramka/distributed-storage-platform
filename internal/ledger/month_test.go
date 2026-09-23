package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

type fakeMeters struct {
	customers []store.CustomerStored
	held      []store.NodeHeld
	uptime    map[uuid.UUID]float32
	audit     map[uuid.UUID]float32
}

func (f fakeMeters) CustomerStored(context.Context) ([]store.CustomerStored, error) {
	return f.customers, nil
}
func (f fakeMeters) NodeHeld(context.Context) ([]store.NodeHeld, error) { return f.held, nil }
func (f fakeMeters) CustomerEgress(context.Context, time.Time) ([]store.CustomerEgress, error) {
	return nil, nil
}
func (f fakeMeters) NodeServed(context.Context, time.Time) ([]store.NodeWindowServed, error) {
	return nil, nil
}
func (f fakeMeters) Reliability(_ context.Context, id uuid.UUID) (float32, float32, error) {
	u, a := f.uptime[id], f.audit[id]
	if a == 0 {
		a = 1
	}
	return u, a, nil
}

func TestAcceleratedMonthBalancedBooks(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	cust, err := s.CreateUser(ctx, "m5-cust-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	prov, err := s.CreateUser(ctx, "m5-prov-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	cleanNode := uuid.New()
	flakyNode := uuid.New()
	held := int64(10 * BytesPerGB)
	meters := fakeMeters{
		customers: []store.CustomerStored{{OwnerID: cust.ID, SizeBytes: 10 * BytesPerGB}},
		held: []store.NodeHeld{
			{NodeID: cleanNode, OwnerID: prov.ID, SizeBytes: held, Reputation: 1},
			{NodeID: flakyNode, OwnerID: prov.ID, SizeBytes: held, Reputation: 1},
		},
		uptime: map[uuid.UUID]float32{cleanNode: 1, flakyNode: 0.5},
		audit:  map[uuid.UUID]float32{cleanNode: 1, flakyNode: 1},
	}
	led := &Ledger{Store: s, Rates: DefaultRates()}
	accrue := &Accruer{Ledger: led, Meters: meters, Clock: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 720; i++ {
		if err := accrue.AccrueWindow(ctx, start.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("window %d: %v", i, err)
		}
	}
	ids, err := s.LedgerUnbalancedTxns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("unbalanced txns %v", ids)
	}
	cAcct, err := s.MustAccount(ctx, OwnerCustomer, ptr(cust.ID), KindBalance)
	if err != nil {
		t.Fatal(err)
	}
	pAcct, err := s.MustAccount(ctx, OwnerProvider, ptr(prov.ID), KindEarnings)
	if err != nil {
		t.Fatal(err)
	}
	plat, err := s.EnsureLedgerAccount(ctx, OwnerPlatform, nil, KindRevenue)
	if err != nil {
		t.Fatal(err)
	}
	cBal, err := s.AccountBalance(ctx, cAcct.ID)
	if err != nil {
		t.Fatal(err)
	}
	pBal, err := s.AccountBalance(ctx, pAcct.ID)
	if err != nil {
		t.Fatal(err)
	}
	platBal, err := s.AccountBalance(ctx, plat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cBal >= 0 {
		t.Fatalf("customer should be charged, balance %d", cBal)
	}
	_ = platBal
	cleanEarn := StorageEarningµCRD(held, DefaultRates(), ReliabilityBP(1, 1, 1)) * 720
	flakyEarn := StorageEarningµCRD(held, DefaultRates(), ReliabilityBP(0.5, 1, 1)) * 720
	if flakyEarn >= cleanEarn {
		t.Fatalf("flaky %d clean %d", flakyEarn, cleanEarn)
	}
	if pBal != cleanEarn+flakyEarn {
		t.Fatalf("provider %d want %d", pBal, cleanEarn+flakyEarn)
	}
}
