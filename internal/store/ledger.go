package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LedgerAccount is a row in ledger_accounts.
type LedgerAccount struct {
	ID        uuid.UUID
	OwnerType string
	OwnerID   *uuid.UUID
	Kind      string
	Currency  string
}

// LedgerEntry is one double-entry leg.
type LedgerEntry struct {
	TxnID     uuid.UUID
	AccountID uuid.UUID
	Amount    int64
	EntryType string
	Reference json.RawMessage
}

// EnsureLedgerAccount creates or returns the account for (ownerType, ownerID, kind).
func (s *Store) EnsureLedgerAccount(ctx context.Context, ownerType string, ownerID *uuid.UUID, kind string) (LedgerAccount, error) {
	if ownerID == nil {
		a, err := s.LedgerAccountByKey(ctx, ownerType, nil, kind)
		if err == nil {
			return a, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return LedgerAccount{}, err
		}
	}
	var a LedgerAccount
	err := s.pool.QueryRow(ctx, `
		INSERT INTO ledger_accounts (owner_type, owner_id, kind)
		VALUES ($1, $2, $3)
		ON CONFLICT (owner_type, owner_id, kind) DO UPDATE SET currency = ledger_accounts.currency
		RETURNING id, owner_type, owner_id, kind, currency
	`, ownerType, ownerID, kind).Scan(&a.ID, &a.OwnerType, &a.OwnerID, &a.Kind, &a.Currency)
	if err != nil {
		// Concurrent insert of NULL owner_id: unique constraint does not fire; look up.
		if ownerID == nil {
			if got, e2 := s.LedgerAccountByKey(ctx, ownerType, nil, kind); e2 == nil {
				return got, nil
			}
		}
		return LedgerAccount{}, mapQueryErr("store.ensureLedgerAccount", err)
	}
	return a, nil
}

// WatermarkAccrual is the production Accruer cursor name.
const WatermarkAccrual = "accrual"

// InsertLedgerTxn writes the txn header and all legs in one transaction.
// A duplicate txn_id is a no-op. Caller must ensure the sum is zero.
func (s *Store) InsertLedgerTxn(ctx context.Context, txnID uuid.UUID, kind string, window *time.Time, entries []LedgerEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if kind == "" {
		kind = entries[0].EntryType
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapQueryErr("store.insertLedgerTxn", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		INSERT INTO ledger_txns (txn_id, kind, window_start)
		VALUES ($1, $2, $3)
		ON CONFLICT (txn_id) DO NOTHING
	`, txnID, kind, window)
	if err != nil {
		return mapQueryErr("store.insertLedgerTxn", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	for _, e := range entries {
		ref := e.Reference
		if len(ref) == 0 {
			ref = []byte("{}")
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO ledger_entries (txn_id, account_id, amount, entry_type, reference)
			VALUES ($1, $2, $3, $4, $5)
		`, e.TxnID, e.AccountID, e.Amount, e.EntryType, ref)
		if err != nil {
			return mapQueryErr("store.insertLedgerTxn", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return mapQueryErr("store.insertLedgerTxn", err)
	}
	return nil
}

// CountLedgerEntries returns how many legs exist for txnID.
func (s *Store) CountLedgerEntries(ctx context.Context, txnID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries WHERE txn_id = $1`, txnID).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.countLedgerEntries", err)
	}
	return n, nil
}

// AccrualWatermark is the last fully accrued UTC hour, or zero if none.
func (s *Store) AccrualWatermark(ctx context.Context, name string) (time.Time, error) {
	if name == "" {
		name = WatermarkAccrual
	}
	var w time.Time
	err := s.pool.QueryRow(ctx, `SELECT last_window FROM ledger_watermarks WHERE name = $1`, name).Scan(&w)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, mapQueryErr("store.accrualWatermark", err)
	}
	return w.UTC(), nil
}

// SetAccrualWatermark records the last accrued hour.
func (s *Store) SetAccrualWatermark(ctx context.Context, name string, window time.Time) error {
	if name == "" {
		name = WatermarkAccrual
	}
	window = window.UTC().Truncate(time.Hour)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ledger_watermarks (name, last_window)
		VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET last_window = EXCLUDED.last_window
	`, name, window)
	if err != nil {
		return mapQueryErr("store.setAccrualWatermark", err)
	}
	return nil
}

// LedgerEarningLine is one provider earning grouped by node, window, and type.
type LedgerEarningLine struct {
	NodeID    uuid.UUID
	Window    time.Time
	EntryType string
	Amount    int64
}

// ListProviderEarnings lists credit legs on a provider earnings account.
func (s *Store) ListProviderEarnings(ctx context.Context, accountID uuid.UUID, from, to time.Time) ([]LedgerEarningLine, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE((e.reference->>'node_id')::uuid, '00000000-0000-0000-0000-000000000000'),
			COALESCE((e.reference->>'window')::timestamptz, e.created_at),
			e.entry_type,
			e.amount
		FROM ledger_entries e
		WHERE e.account_id = $1 AND e.amount > 0
		  AND ($2::timestamptz IS NULL OR COALESCE((e.reference->>'window')::timestamptz, e.created_at) >= $2)
		  AND ($3::timestamptz IS NULL OR COALESCE((e.reference->>'window')::timestamptz, e.created_at) < $3)
		ORDER BY 2, 3
	`, accountID, nilTime(from), nilTime(to))
	if err != nil {
		return nil, mapQueryErr("store.listProviderEarnings", err)
	}
	defer rows.Close()
	var out []LedgerEarningLine
	for rows.Next() {
		var r LedgerEarningLine
		if err := rows.Scan(&r.NodeID, &r.Window, &r.EntryType, &r.Amount); err != nil {
			return nil, mapQueryErr("store.listProviderEarnings", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func nilTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

// UsageEgressBytes sums archived egress event payload bytes in [window, window+1h).
func (s *Store) UsageEgressBytes(ctx context.Context, window time.Time) (int64, error) {
	start := window.UTC().Truncate(time.Hour)
	end := start.Add(time.Hour)
	var n int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(COALESCE((payload->'payload'->>'bytes')::bigint, 0)), 0)
		FROM usage_events
		WHERE kind = 'egress' AND window_start >= $1 AND window_start < $2
	`, start, end).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.usageEgressBytes", err)
	}
	return n, nil
}

// ReceiptBytes sums download_receipts in [window, window+1h).
func (s *Store) ReceiptBytes(ctx context.Context, window time.Time) (int64, error) {
	start := window.UTC().Truncate(time.Hour)
	end := start.Add(time.Hour)
	var n int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(bytes), 0) FROM download_receipts
		WHERE created_at >= $1 AND created_at < $2
	`, start, end).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.receiptBytes", err)
	}
	return n, nil
}

// AccountBalance is SUM(amount) for one account.
func (s *Store) AccountBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE account_id = $1`, accountID).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.accountBalance", err)
	}
	return n, nil
}

// LedgerAccountByKey loads an account if it exists.
func (s *Store) LedgerAccountByKey(ctx context.Context, ownerType string, ownerID *uuid.UUID, kind string) (LedgerAccount, error) {
	var a LedgerAccount
	var err error
	if ownerID == nil {
		err = s.pool.QueryRow(ctx, `
			SELECT id, owner_type, owner_id, kind, currency
			FROM ledger_accounts
			WHERE owner_type = $1 AND owner_id IS NULL AND kind = $2
		`, ownerType, kind).Scan(&a.ID, &a.OwnerType, &a.OwnerID, &a.Kind, &a.Currency)
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT id, owner_type, owner_id, kind, currency
			FROM ledger_accounts
			WHERE owner_type = $1 AND owner_id = $2 AND kind = $3
		`, ownerType, *ownerID, kind).Scan(&a.ID, &a.OwnerType, &a.OwnerID, &a.Kind, &a.Currency)
	}
	if err != nil {
		return LedgerAccount{}, mapQueryErr("store.ledgerAccountByKey", err)
	}
	return a, nil
}

// LedgerUnbalancedTxns returns txn_ids whose amounts do not sum to zero.
func (s *Store) LedgerUnbalancedTxns(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT txn_id FROM ledger_entries
		GROUP BY txn_id
		HAVING SUM(amount) <> 0
	`)
	if err != nil {
		return nil, mapQueryErr("store.ledgerUnbalancedTxns", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapQueryErr("store.ledgerUnbalancedTxns", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// LedgerTotalsByOwner sums amounts grouped by account owner_type.
func (s *Store) LedgerTotalsByOwner(ctx context.Context) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.owner_type, COALESCE(SUM(e.amount), 0)
		FROM ledger_entries e
		JOIN ledger_accounts a ON a.id = e.account_id
		GROUP BY a.owner_type
	`)
	if err != nil {
		return nil, mapQueryErr("store.ledgerTotalsByOwner", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var n int64
		if err := rows.Scan(&k, &n); err != nil {
			return nil, mapQueryErr("store.ledgerTotalsByOwner", err)
		}
		out[k] = n
	}
	return out, rows.Err()
}

// LedgerTotalsByType sums amounts grouped by entry_type.
func (s *Store) LedgerTotalsByType(ctx context.Context) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT entry_type, COALESCE(SUM(amount), 0) FROM ledger_entries GROUP BY entry_type
	`)
	if err != nil {
		return nil, mapQueryErr("store.ledgerTotalsByType", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var n int64
		if err := rows.Scan(&k, &n); err != nil {
			return nil, mapQueryErr("store.ledgerTotalsByType", err)
		}
		out[k] = n
	}
	return out, rows.Err()
}

// RecordUsageEvent inserts a usage event; duplicate id is ignored.
func (s *Store) RecordUsageEvent(ctx context.Context, id, kind string, subjectID uuid.UUID, windowStart interface{}, payload []byte) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO usage_events (id, kind, subject_id, window_start, payload)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, id, kind, subjectID, windowStart, payload)
	if err != nil {
		return false, mapQueryErr("store.recordUsageEvent", err)
	}
	return tag.RowsAffected() > 0, nil
}

// MarkUsageConsumed stamps consumed_at.
func (s *Store) MarkUsageConsumed(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE usage_events SET consumed_at = now() WHERE id = $1`, id)
	if err != nil {
		return mapQueryErr("store.markUsageConsumed", err)
	}
	return nil
}

// OwnerNodes returns nodes owned by userID.
func (s *Store) OwnerNodes(ctx context.Context, ownerID uuid.UUID) ([]Node, error) {
	return s.ListNodesByOwner(ctx, ownerID)
}

// ErrNoLedger is reserved for callers that treat a missing account as empty.
var ErrNoLedger = errors.New("store: no ledger account")

// MustAccount is like LedgerAccountByKey but maps missing to ErrNoLedger.
func (s *Store) MustAccount(ctx context.Context, ownerType string, ownerID *uuid.UUID, kind string) (LedgerAccount, error) {
	a, err := s.LedgerAccountByKey(ctx, ownerType, ownerID, kind)
	if errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
		return LedgerAccount{}, ErrNoLedger
	}
	return a, err
}

// NodeStatsWindowBytes returns bytes_served for one node in one hour window.
func (s *Store) NodeStatsWindowBytes(ctx context.Context, nodeID uuid.UUID, windowStart interface{}) (served, ingested int64, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(bytes_served, 0), COALESCE(bytes_ingested, 0)
		FROM node_stats
		WHERE node_id = $1 AND window_start = $2
	`, nodeID, windowStart).Scan(&served, &ingested)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, mapQueryErr("store.nodeStatsWindowBytes", err)
	}
	return served, ingested, nil
}

// Statement is an immutable cycle snapshot.
type Statement struct {
	ID           uuid.UUID
	AccountID    uuid.UUID
	CycleStart   time.Time
	CycleEnd     time.Time
	OpeningUCRD  int64
	ClosingUCRD  int64
	ChargesUCRD  int64
	EarningsUCRD int64
}

// InsertStatement writes one cycle snapshot. Duplicate cycle is a no-op.
func (s *Store) InsertStatement(ctx context.Context, st Statement) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO statements (account_id, cycle_start, cycle_end, opening_ucrd, closing_ucrd, charges_ucrd, earnings_ucrd)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (account_id, cycle_start) DO NOTHING
	`, st.AccountID, st.CycleStart, st.CycleEnd, st.OpeningUCRD, st.ClosingUCRD, st.ChargesUCRD, st.EarningsUCRD)
	if err != nil {
		return mapQueryErr("store.insertStatement", err)
	}
	return nil
}

// ListLedgerAccounts returns every account.
func (s *Store) ListLedgerAccounts(ctx context.Context) ([]LedgerAccount, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, owner_type, owner_id, kind, currency FROM ledger_accounts`)
	if err != nil {
		return nil, mapQueryErr("store.listLedgerAccounts", err)
	}
	defer rows.Close()
	var out []LedgerAccount
	for rows.Next() {
		var a LedgerAccount
		if err := rows.Scan(&a.ID, &a.OwnerType, &a.OwnerID, &a.Kind, &a.Currency); err != nil {
			return nil, mapQueryErr("store.listLedgerAccounts", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AccountBalanceAt is SUM(amount) for entries created before at.
func (s *Store) AccountBalanceAt(ctx context.Context, accountID uuid.UUID, at time.Time) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM ledger_entries
		WHERE account_id = $1 AND created_at < $2
	`, accountID, at).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.accountBalanceAt", err)
	}
	return n, nil
}

// StatementByAccount loads the snapshot for one cycle.
func (s *Store) StatementByAccount(ctx context.Context, accountID uuid.UUID, start time.Time) (Statement, error) {
	var st Statement
	err := s.pool.QueryRow(ctx, `
		SELECT id, account_id, cycle_start, cycle_end, opening_ucrd, closing_ucrd, charges_ucrd, earnings_ucrd
		FROM statements WHERE account_id = $1 AND cycle_start = $2
	`, accountID, start).Scan(&st.ID, &st.AccountID, &st.CycleStart, &st.CycleEnd, &st.OpeningUCRD, &st.ClosingUCRD, &st.ChargesUCRD, &st.EarningsUCRD)
	if err != nil {
		return Statement{}, mapQueryErr("store.statementByAccount", err)
	}
	return st, nil
}
