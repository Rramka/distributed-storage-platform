-- M5 meters + ledger: download receipts, usage events, balanced-txn trigger.
-- docs/03-data-model.md, docs/09-billing-ledger.md

CREATE TABLE IF NOT EXISTS download_tickets (
    nonce           BYTEA PRIMARY KEY,
    file_id         UUID NOT NULL REFERENCES files(id),
    fragment_id     UUID NOT NULL REFERENCES fragments(id),
    node_id         UUID NOT NULL REFERENCES nodes(id),
    size_bytes      BIGINT NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS download_tickets_by_file ON download_tickets (file_id);

CREATE TABLE IF NOT EXISTS download_receipts (
    nonce           BYTEA PRIMARY KEY,
    file_id         UUID NOT NULL REFERENCES files(id),
    fragment_id     UUID NOT NULL REFERENCES fragments(id),
    node_id         UUID NOT NULL REFERENCES nodes(id),
    bytes           BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usage_events (
    id              TEXT PRIMARY KEY,
    kind            TEXT NOT NULL,
    subject_id      UUID,
    window_start    TIMESTAMPTZ,
    payload         JSONB,
    consumed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS usage_events_by_kind ON usage_events (kind, window_start);

CREATE INDEX IF NOT EXISTS ledger_by_txn ON ledger_entries (txn_id);

CREATE UNIQUE INDEX IF NOT EXISTS ledger_accounts_platform_kind
ON ledger_accounts (kind) WHERE owner_type = 'platform' AND owner_id IS NULL;

CREATE OR REPLACE FUNCTION assert_txn_balanced() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF (SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE txn_id = NEW.txn_id) <> 0 THEN
        RAISE EXCEPTION 'ledger txn % not balanced', NEW.txn_id;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS ledger_txn_balanced ON ledger_entries;
CREATE CONSTRAINT TRIGGER ledger_txn_balanced
AFTER INSERT ON ledger_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_txn_balanced();
