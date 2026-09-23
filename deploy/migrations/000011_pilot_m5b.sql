-- Pilot readiness + M5b: cert revocation, statements, audit grants.
-- docs/07-security.md, docs/09-billing-ledger.md

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS statements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES ledger_accounts(id),
    cycle_start     TIMESTAMPTZ NOT NULL,
    cycle_end       TIMESTAMPTZ NOT NULL,
    opening_ucrd    BIGINT NOT NULL,
    closing_ucrd    BIGINT NOT NULL,
    charges_ucrd    BIGINT NOT NULL,
    earnings_ucrd   BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, cycle_start)
);

REVOKE UPDATE, DELETE ON audit_logs FROM PUBLIC;
REVOKE UPDATE, DELETE ON audit_logs FROM dsp_readonly;
GRANT SELECT, INSERT ON audit_logs TO dsp;
GRANT USAGE, SELECT ON SEQUENCE audit_logs_id_seq TO dsp;
