package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

// Clock returns the current time. Injected so the month test needs no sleeping.
type Clock func() time.Time

// Accruer posts hourly storage and egress transactions from meters.
type Accruer struct {
	Ledger *Ledger
	Clock  Clock
	Meters Meters
}

// Meters supplies billing inputs. Tests inject fakes; production uses StoreMeters.
type Meters interface {
	CustomerStored(ctx context.Context) ([]store.CustomerStored, error)
	NodeHeld(ctx context.Context) ([]store.NodeHeld, error)
	CustomerEgress(ctx context.Context, window time.Time) ([]store.CustomerEgress, error)
	NodeServed(ctx context.Context, window time.Time) ([]store.NodeWindowServed, error)
	Reliability(ctx context.Context, nodeID uuid.UUID) (uptime, audit float32, err error)
}

// StoreMeters reads Postgres.
type StoreMeters struct {
	Store *store.Store
}

func (m StoreMeters) CustomerStored(ctx context.Context) ([]store.CustomerStored, error) {
	return m.Store.ListCustomerStoredBytes(ctx)
}
func (m StoreMeters) NodeHeld(ctx context.Context) ([]store.NodeHeld, error) {
	return m.Store.ListNodeHeldBytes(ctx)
}
func (m StoreMeters) CustomerEgress(ctx context.Context, window time.Time) ([]store.CustomerEgress, error) {
	return m.Store.ListCustomerEgressBytes(ctx, window)
}
func (m StoreMeters) NodeServed(ctx context.Context, window time.Time) ([]store.NodeWindowServed, error) {
	return m.Store.ListNodeWindowServed(ctx, window)
}
func (m StoreMeters) Reliability(ctx context.Context, nodeID uuid.UUID) (float32, float32, error) {
	return m.Store.NodeReliability(ctx, nodeID)
}

func (a *Accruer) now() time.Time {
	if a.Clock != nil {
		return a.Clock().UTC()
	}
	return time.Now().UTC()
}

func (a *Accruer) meters() Meters {
	if a.Meters != nil {
		return a.Meters
	}
	return StoreMeters{Store: a.Ledger.Store}
}

// AccrueWindow posts storage and egress for one UTC hour. Idempotent per window.
func (a *Accruer) AccrueWindow(ctx context.Context, window time.Time) error {
	window = window.UTC().Truncate(time.Hour)
	rates := a.Ledger.rates()
	m := a.meters()
	platform, err := a.Ledger.EnsureAccount(ctx, OwnerPlatform, nil, KindRevenue)
	if err != nil {
		return err
	}

	cust, err := m.CustomerStored(ctx)
	if err != nil {
		return err
	}
	for _, c := range cust {
		charge := StorageChargeµCRD(c.SizeBytes, rates)
		if charge == 0 {
			continue
		}
		acct, err := a.Ledger.EnsureAccount(ctx, OwnerCustomer, ptr(c.OwnerID), KindBalance)
		if err != nil {
			return err
		}
		txn := DeterministicTxnID("storage_charge", c.OwnerID, window.Unix())
		if err := a.Ledger.PostTxn(ctx, txn, []Entry{
			{AccountID: acct.ID, Amount: -charge, Type: TypeStorageCharge, Reference: map[string]any{"window": window, "bytes": c.SizeBytes}},
			{AccountID: platform.ID, Amount: charge, Type: TypeStorageCharge, Reference: map[string]any{"window": window, "owner_id": c.OwnerID}},
		}); err != nil {
			return fmt.Errorf("ledger.accrue storage charge: %w", err)
		}
	}

	held, err := m.NodeHeld(ctx)
	if err != nil {
		return err
	}
	for _, n := range held {
		if n.SizeBytes <= 0 {
			continue
		}
		up, audit, err := m.Reliability(ctx, n.NodeID)
		if err != nil {
			return err
		}
		bp := ReliabilityBP(up, audit, n.Reputation)
		earn := StorageEarningµCRD(n.SizeBytes, rates, bp)
		if earn == 0 {
			continue
		}
		acct, err := a.Ledger.EnsureAccount(ctx, OwnerProvider, ptr(n.OwnerID), KindEarnings)
		if err != nil {
			return err
		}
		txn := DeterministicTxnID("storage_earning", n.NodeID, window.Unix())
		if err := a.Ledger.PostTxn(ctx, txn, []Entry{
			{AccountID: platform.ID, Amount: -earn, Type: TypeStorageEarning, Reference: map[string]any{"window": window, "node_id": n.NodeID, "r_bp": bp}},
			{AccountID: acct.ID, Amount: earn, Type: TypeStorageEarning, Reference: map[string]any{"window": window, "node_id": n.NodeID}},
		}); err != nil {
			return fmt.Errorf("ledger.accrue storage earning: %w", err)
		}
	}

	eg, err := m.CustomerEgress(ctx, window)
	if err != nil {
		return err
	}
	for _, c := range eg {
		charge := EgressChargeµCRD(c.Bytes, rates)
		if charge == 0 {
			continue
		}
		acct, err := a.Ledger.EnsureAccount(ctx, OwnerCustomer, ptr(c.OwnerID), KindBalance)
		if err != nil {
			return err
		}
		txn := DeterministicTxnID("egress_charge", c.OwnerID, window.Unix())
		if err := a.Ledger.PostTxn(ctx, txn, []Entry{
			{AccountID: acct.ID, Amount: -charge, Type: TypeEgressCharge, Reference: map[string]any{"window": window, "bytes": c.Bytes}},
			{AccountID: platform.ID, Amount: charge, Type: TypeEgressCharge, Reference: map[string]any{"window": window}},
		}); err != nil {
			return err
		}
	}

	served, err := m.NodeServed(ctx, window)
	if err != nil {
		return err
	}
	for _, n := range served {
		up, audit, err := m.Reliability(ctx, n.NodeID)
		if err != nil {
			return err
		}
		var rep float32 = 1
		for _, h := range held {
			if h.NodeID == n.NodeID {
				rep = h.Reputation
				break
			}
		}
		bp := ReliabilityBP(up, audit, rep)
		earn := EgressEarningµCRD(n.Bytes, rates, bp)
		if earn == 0 {
			continue
		}
		acct, err := a.Ledger.EnsureAccount(ctx, OwnerProvider, ptr(n.OwnerID), KindEarnings)
		if err != nil {
			return err
		}
		txn := DeterministicTxnID("egress_earning", n.NodeID, window.Unix())
		if err := a.Ledger.PostTxn(ctx, txn, []Entry{
			{AccountID: platform.ID, Amount: -earn, Type: TypeEgressEarning, Reference: map[string]any{"window": window, "node_id": n.NodeID}},
			{AccountID: acct.ID, Amount: earn, Type: TypeEgressEarning, Reference: map[string]any{"window": window, "node_id": n.NodeID}},
		}); err != nil {
			return err
		}
	}
	return nil
}

// Run ticks hourly until ctx is cancelled, accruing the previous complete hour.
func (a *Accruer) Run(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	window := a.now().Truncate(time.Hour).Add(-time.Hour)
	if err := a.AccrueWindow(ctx, window); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			window := a.now().Truncate(time.Hour).Add(-time.Hour)
			_ = a.AccrueWindow(ctx, window)
		}
	}
}
