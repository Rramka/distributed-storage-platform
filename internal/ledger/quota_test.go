package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

func TestCheckQuotaAndCloseCycle(t *testing.T) {
	s := store.OpenForTest(t)
	ctx := context.Background()
	led := &Ledger{Store: s, Rates: DefaultRates()}
	u, err := s.CreateUser(ctx, "quota-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := led.CheckQuota(ctx, u.ID); err != nil {
		t.Fatalf("new customer should upload: %v", err)
	}
	acct, err := led.EnsureAccount(ctx, OwnerCustomer, ptr(u.ID), KindBalance)
	if err != nil {
		t.Fatal(err)
	}
	plat, err := led.EnsureAccount(ctx, OwnerPlatform, nil, KindRevenue)
	if err != nil {
		t.Fatal(err)
	}
	window := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if err := led.PostTxn(ctx, uuid.New(), []Entry{
		{AccountID: acct.ID, Amount: -10, Type: TypeStorageCharge, Reference: map[string]any{"window": window}},
		{AccountID: plat.ID, Amount: 10, Type: TypeStorageCharge},
	}); err != nil {
		t.Fatal(err)
	}
	if err := led.CheckQuota(ctx, u.ID); err == nil {
		t.Fatal("floor 0 should block negative balance")
	}
	start := time.Now().UTC().Add(-time.Minute)
	end := time.Now().UTC().Add(time.Minute)
	if err := led.CloseCycle(ctx, start, end); err != nil {
		t.Fatal(err)
	}
	st, err := s.StatementByAccount(ctx, acct.ID, start)
	if err != nil {
		t.Fatal(err)
	}
	if st.ClosingUCRD != -10 || st.ChargesUCRD != 10 {
		t.Fatalf("statement %+v", st)
	}
}
