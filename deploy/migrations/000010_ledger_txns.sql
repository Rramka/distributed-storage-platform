-- M5.1: idempotent PostTxn, accrual watermark.
-- docs/03-data-model.md, docs/09-billing-ledger.md

CREATE TABLE IF NOT EXISTS ledger_txns (
    txn_id          UUID PRIMARY KEY,
    kind            TEXT NOT NULL,
    window_start    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ledger_watermarks (
    name            TEXT PRIMARY KEY,
    last_window     TIMESTAMPTZ NOT NULL
);
