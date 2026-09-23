// Package ledger is the internal double-entry billing ledger (docs/09-billing-ledger.md).
// All amounts are integer micro-credits. The ledger is the sole writer of ledger tables.
package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Rramka/distributed-storage-platform/internal/store"
	"github.com/google/uuid"
)

const (
	MicroPerCRD   int64 = 1_000_000
	BytesPerGB    int64 = 1_000_000_000
	HoursPerMonth int64 = 730

	OwnerCustomer = "customer"
	OwnerProvider = "provider"
	OwnerPlatform = "platform"

	KindBalance  = "customer_balance"
	KindEarnings = "provider_earnings"
	KindRevenue  = "platform_revenue"

	TypeStorageCharge  = "storage_charge"
	TypeEgressCharge   = "egress_charge"
	TypeStorageEarning = "storage_earning"
	TypeEgressEarning  = "egress_earning"
	TypeAdjustment     = "adjustment"
)

// Rates are Phase 1 defaults from docs/09, overridable via env.
type Rates struct {
	CustomerStoragePerGBMonth int64 // µCRD
	ProviderStoragePerGBMonth int64
	CustomerEgressPerGB       int64
	ProviderEgressPerGB       int64
}

// DefaultRates is 3.0 / 1.5 CRD per GB-month and 1.0 / 0.5 CRD per GB egress.
func DefaultRates() Rates {
	r := Rates{
		CustomerStoragePerGBMonth: 3 * MicroPerCRD,
		ProviderStoragePerGBMonth: 3 * MicroPerCRD / 2,
		CustomerEgressPerGB:       1 * MicroPerCRD,
		ProviderEgressPerGB:       MicroPerCRD / 2,
	}
	if v := os.Getenv("LEDGER_CUSTOMER_STORAGE_UCRD_GB_MONTH"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			r.CustomerStoragePerGBMonth = n
		}
	}
	if v := os.Getenv("LEDGER_PROVIDER_STORAGE_UCRD_GB_MONTH"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			r.ProviderStoragePerGBMonth = n
		}
	}
	if v := os.Getenv("LEDGER_CUSTOMER_EGRESS_UCRD_GB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			r.CustomerEgressPerGB = n
		}
	}
	if v := os.Getenv("LEDGER_PROVIDER_EGRESS_UCRD_GB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			r.ProviderEgressPerGB = n
		}
	}
	return r
}

// Ledger posts balanced transactions.
type Ledger struct {
	Store *store.Store
	Rates Rates
}

func (l *Ledger) rates() Rates {
	if l.Rates.CustomerStoragePerGBMonth == 0 {
		return DefaultRates()
	}
	return l.Rates
}

// EnsureAccount creates the named account.
func (l *Ledger) EnsureAccount(ctx context.Context, ownerType string, ownerID *uuid.UUID, kind string) (store.LedgerAccount, error) {
	return l.Store.EnsureLedgerAccount(ctx, ownerType, ownerID, kind)
}

// Entry is one posting leg.
type Entry struct {
	AccountID uuid.UUID
	Amount    int64
	Type      string
	Reference map[string]any
}

var (
	errUnbalanced       = fmt.Errorf("ledger: transaction does not sum to zero")
	errAdjustmentReason = fmt.Errorf("ledger.adjustment: reason required")
	ErrQuota            = fmt.Errorf("ledger: quota exceeded")
)

// CreditFloor is LEDGER_CREDIT_FLOOR_UCRD (default 0).
func CreditFloor() int64 {
	if v := os.Getenv("LEDGER_CREDIT_FLOOR_UCRD"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// CheckQuota blocks new uploads when the customer balance is at or below the floor.
func (l *Ledger) CheckQuota(ctx context.Context, userID uuid.UUID) error {
	acct, err := l.Store.MustAccount(ctx, OwnerCustomer, ptr(userID), KindBalance)
	if err != nil {
		if err == store.ErrNoLedger {
			return nil
		}
		return err
	}
	bal, err := l.Store.AccountBalance(ctx, acct.ID)
	if err != nil {
		return err
	}
	if bal <= CreditFloor() {
		return ErrQuota
	}
	return nil
}

// CloseCycle snapshots every account for [start, end).
func (l *Ledger) CloseCycle(ctx context.Context, start, end time.Time) error {
	start = start.UTC()
	end = end.UTC()
	accts, err := l.Store.ListLedgerAccounts(ctx)
	if err != nil {
		return err
	}
	for _, a := range accts {
		open, err := l.Store.AccountBalanceAt(ctx, a.ID, start)
		if err != nil {
			return err
		}
		closeBal, err := l.Store.AccountBalanceAt(ctx, a.ID, end)
		if err != nil {
			return err
		}
		var charges, earnings int64
		switch a.Kind {
		case KindBalance:
			if closeBal < open {
				charges = open - closeBal
			}
		case KindEarnings:
			if closeBal > open {
				earnings = closeBal - open
			}
		}
		if err := l.Store.InsertStatement(ctx, store.Statement{
			AccountID:    a.ID,
			CycleStart:   start,
			CycleEnd:     end,
			OpeningUCRD:  open,
			ClosingUCRD:  closeBal,
			ChargesUCRD:  charges,
			EarningsUCRD: earnings,
		}); err != nil {
			return err
		}
	}
	return nil
}

// PostTxn writes a balanced transaction. Duplicate txnID is a no-op.
func (l *Ledger) PostTxn(ctx context.Context, txnID uuid.UUID, entries []Entry) error {
	if err := SumZero(entries); err != nil {
		return err
	}
	kind := ""
	var window *time.Time
	if len(entries) > 0 {
		kind = entries[0].Type
		if w, ok := entries[0].Reference["window"]; ok {
			switch t := w.(type) {
			case time.Time:
				u := t.UTC()
				window = &u
			}
		}
	}
	rows := make([]store.LedgerEntry, 0, len(entries))
	for _, e := range entries {
		ref, _ := json.Marshal(e.Reference)
		rows = append(rows, store.LedgerEntry{
			TxnID:     txnID,
			AccountID: e.AccountID,
			Amount:    e.Amount,
			EntryType: e.Type,
			Reference: ref,
		})
	}
	return l.Store.InsertLedgerTxn(ctx, txnID, kind, window, rows)
}

// PostAdjustment writes a balanced correction. Reason is required.
func (l *Ledger) PostAdjustment(ctx context.Context, debit, credit uuid.UUID, amount int64, reason string) error {
	if reason == "" {
		return errAdjustmentReason
	}
	if amount <= 0 {
		return fmt.Errorf("ledger.adjustment: amount must be positive")
	}
	ref := map[string]any{"reason": reason}
	err := l.PostTxn(ctx, uuid.New(), []Entry{
		{AccountID: debit, Amount: -amount, Type: TypeAdjustment, Reference: ref},
		{AccountID: credit, Amount: amount, Type: TypeAdjustment, Reference: ref},
	})
	if err == nil {
		_ = l.Store.InsertAudit(ctx, "service", nil, "ledger.adjustment", "ledger_account", &debit, map[string]any{"reason": reason, "amount": amount})
	}
	return err
}

// SumZero rejects a non-zero entry set.
func SumZero(entries []Entry) error {
	var sum int64
	if len(entries) == 0 {
		return errUnbalanced
	}
	for _, e := range entries {
		sum += e.Amount
	}
	if sum != 0 {
		return errUnbalanced
	}
	return nil
}

// StorageChargeµCRD is the hourly customer charge for logical bytes.
func StorageChargeµCRD(sizeBytes int64, rates Rates) int64 {
	if sizeBytes <= 0 {
		return 0
	}
	return sizeBytes * rates.CustomerStoragePerGBMonth / BytesPerGB / HoursPerMonth
}

// StorageEarningµCRD is the hourly provider earning after reliability basis points.
func StorageEarningµCRD(sizeBytes int64, rates Rates, reliabilityBP int64) int64 {
	if sizeBytes <= 0 {
		return 0
	}
	base := sizeBytes * rates.ProviderStoragePerGBMonth / BytesPerGB / HoursPerMonth
	return base * reliabilityBP / 10000
}

// EgressChargeµCRD is the customer download charge.
func EgressChargeµCRD(bytes int64, rates Rates) int64 {
	if bytes <= 0 {
		return 0
	}
	return bytes * rates.CustomerEgressPerGB / BytesPerGB
}

// EgressEarningµCRD is the provider download earning after reliability.
func EgressEarningµCRD(bytes int64, rates Rates, reliabilityBP int64) int64 {
	if bytes <= 0 {
		return 0
	}
	base := bytes * rates.ProviderEgressPerGB / BytesPerGB
	return base * reliabilityBP / 10000
}

// ReputationFactor maps the [0, 1] EWMA onto [0.8, 1.2].
func ReputationFactor(reputation float32) float64 {
	return float64(reputationFactorBP(reputation)) / 10000
}

func reputationFactorBP(reputation float32) int64 {
	return 8000 + int64(float64(clamp01(reputation))*4000)
}

// ReliabilityBP is R = uptime * audit * reputation_factor, clamped to [0, 1.2], as 1/10000 units.
func ReliabilityBP(uptime, audit, reputation float32) int64 {
	up := int64(float64(clamp01(uptime)) * 10000)
	aud := int64(float64(clamp01(audit)) * 10000)
	fac := reputationFactorBP(reputation)
	r := up * aud / 10000 * fac / 10000
	if r > 12000 {
		r = 12000
	}
	if r < 0 {
		r = 0
	}
	return r
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func ptr(id uuid.UUID) *uuid.UUID { return &id }

// DeterministicTxnID hashes kind+subject+window into a UUID.
func DeterministicTxnID(kind string, subject uuid.UUID, windowUnix int64) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s:%s:%d", kind, subject, windowUnix)))
}
