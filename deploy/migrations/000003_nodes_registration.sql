-- M2: node identity material and one-time registration codes.
-- docs/03-data-model.md (node registry), docs/08-api.md (POST /nodes/registration-codes).

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS public_key BYTEA NOT NULL DEFAULT '\x';
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS cert_pem TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS cert_expires_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS node_registration_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users(id),
    code_hash   TEXT NOT NULL UNIQUE,
    endpoint    TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    node_id     UUID REFERENCES nodes(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS registration_codes_owner
    ON node_registration_codes (owner_id, created_at DESC);
