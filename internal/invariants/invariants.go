// Package invariants asserts the platform durability, placement-cap, and ledger invariants.
// docs/10-mvp-roadmap.md continuously measured invariants.
package invariants

import (
	"context"
	"fmt"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/scheduler"
	"github.com/Rramka/distributed-storage-platform/internal/store"
)

const minHealthy = 12

// CheckDurability fails if any committed chunk has fewer than 12 healthy placements.
func CheckDurability(ctx context.Context, s *store.Store) error {
	ids, err := s.UnderHealthyChunks(ctx, minHealthy)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		return fmt.Errorf("invariants.durability: %d chunks below %d healthy (first %s)", len(ids), minHealthy, ids[0])
	}
	return nil
}

// CheckCaps fails if stored placements of any committed chunk break hard caps.
func CheckCaps(ctx context.Context, s *store.Store) error {
	ids, err := s.ChunkCapViolations(ctx, scheduler.RegionCap, scheduler.ASNCap, scheduler.OwnerCap)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		return fmt.Errorf("invariants.caps: %d chunks violate placement caps (first %s)", len(ids), ids[0])
	}
	return nil
}

// CheckLedger fails if any txn_id does not sum to zero, or if customer
// charges do not equal provider earnings plus platform margin.
func CheckLedger(ctx context.Context, s *store.Store) error {
	ids, err := s.LedgerUnbalancedTxns(ctx)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		return fmt.Errorf("invariants.ledger: %d unbalanced txns (first %s)", len(ids), ids[0])
	}
	totals, err := s.LedgerTotalsByOwner(ctx)
	if err != nil {
		return err
	}
	var customer, provider, platform int64
	for k, v := range totals {
		switch k {
		case "customer":
			customer = v
		case "provider":
			provider = v
		case "platform":
			platform = v
		default:
			return fmt.Errorf("invariants.ledger: unknown owner_type %q", k)
		}
	}
	if customer+provider+platform != 0 {
		return fmt.Errorf("invariants.ledger: customer %d + provider %d + platform %d != 0", customer, provider, platform)
	}
	if customer > 0 {
		return fmt.Errorf("invariants.ledger: customer balance %d should not be positive", customer)
	}
	if -customer != provider+platform {
		return fmt.Errorf("invariants.ledger: customer charges %d != provider %d + platform %d", -customer, provider, platform)
	}
	return CheckUsageReconcile(ctx, s)
}

// CheckUsageReconcile fails if archived egress events diverge from download_receipts
// for the last complete UTC hour (and the current hour).
func CheckUsageReconcile(ctx context.Context, s *store.Store) error {
	now := time.Now().UTC().Truncate(time.Hour)
	for _, w := range []time.Time{now.Add(-time.Hour), now} {
		events, err := s.UsageEgressBytes(ctx, w)
		if err != nil {
			return err
		}
		receipts, err := s.ReceiptBytes(ctx, w)
		if err != nil {
			return err
		}
		if events == 0 {
			continue
		}
		if events != receipts {
			return fmt.Errorf("invariants.ledger: egress events %d != receipts %d for %s", events, receipts, w.Format(time.RFC3339))
		}
	}
	return nil
}

// CheckAll runs durability, cap, and ledger checks.
// Zero-knowledge is checked when DSP_ZK_NEEDLE is set (optional DSP_DATA_DIRS).
func CheckAll(ctx context.Context, s *store.Store) error {
	if err := CheckDurability(ctx, s); err != nil {
		return err
	}
	if err := CheckCaps(ctx, s); err != nil {
		return err
	}
	if err := CheckLedger(ctx, s); err != nil {
		return err
	}
	if needles := needlesFromEnv(); len(needles) > 0 {
		if err := CheckZeroKnowledge(dataDirsFromEnv(), nil, needles); err != nil {
			return err
		}
	}
	return nil
}
