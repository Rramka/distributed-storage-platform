-- Initial schema from docs/03-data-model.md.
-- Ledger tables exist so the schema is complete; the ledger *service* is deferred on the solo track.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'customer',
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    key_hash        TEXT NOT NULL,
    label           TEXT NOT NULL,
    scopes          TEXT[] NOT NULL DEFAULT '{read,write}',
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS buckets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID NOT NULL REFERENCES users(id),
    name            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, name)
);

CREATE TABLE IF NOT EXISTS files (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id       UUID NOT NULL REFERENCES buckets(id),
    path            TEXT NOT NULL,
    is_folder       BOOLEAN NOT NULL DEFAULT false,
    current_version UUID,
    status          TEXT NOT NULL DEFAULT 'uploading',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (bucket_id, path)
);

CREATE INDEX IF NOT EXISTS files_bucket_prefix ON files (bucket_id, path text_pattern_ops);

CREATE TABLE IF NOT EXISTS file_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id         UUID NOT NULL REFERENCES files(id),
    version_no      INTEGER NOT NULL,
    size_bytes      BIGINT NOT NULL,
    content_sha256  BYTEA NOT NULL,
    encryption_meta JSONB NOT NULL,
    chunk_size      INTEGER NOT NULL,
    ec_data_shards  SMALLINT NOT NULL DEFAULT 10,
    ec_parity_shards SMALLINT NOT NULL DEFAULT 6,
    status          TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    committed_at    TIMESTAMPTZ,
    UNIQUE (file_id, version_no)
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'files_current_version_fk'
    ) THEN
        ALTER TABLE files
            ADD CONSTRAINT files_current_version_fk
            FOREIGN KEY (current_version) REFERENCES file_versions(id);
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS chunks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id      UUID NOT NULL REFERENCES file_versions(id),
    seq             INTEGER NOT NULL,
    size_bytes      INTEGER NOT NULL,
    sha256          BYTEA NOT NULL,
    UNIQUE (version_id, seq)
);

CREATE TABLE IF NOT EXISTS fragments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id        UUID NOT NULL REFERENCES chunks(id),
    shard_index     SMALLINT NOT NULL,
    size_bytes      INTEGER NOT NULL,
    sha256          BYTEA NOT NULL,
    UNIQUE (chunk_id, shard_index)
);

CREATE TABLE IF NOT EXISTS nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID NOT NULL REFERENCES users(id),
    cert_fingerprint BYTEA NOT NULL UNIQUE,
    hostname_label  TEXT,
    os              TEXT NOT NULL,
    agent_version   TEXT NOT NULL,
    country         TEXT,
    region          TEXT,
    asn             INTEGER,
    endpoint        TEXT NOT NULL,
    capacity_bytes  BIGINT NOT NULL,
    used_bytes      BIGINT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'pending',
    reputation      REAL NOT NULL DEFAULT 0.5,
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS nodes_selectable ON nodes (status, reputation DESC) WHERE status = 'online';

CREATE TABLE IF NOT EXISTS fragment_placements (
    id              BIGSERIAL PRIMARY KEY,
    fragment_id     UUID NOT NULL REFERENCES fragments(id),
    node_id         UUID NOT NULL REFERENCES nodes(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    stored_at       TIMESTAMPTZ,
    last_audit_at   TIMESTAMPTZ,
    lost_at         TIMESTAMPTZ,
    UNIQUE (fragment_id, node_id)
);

CREATE INDEX IF NOT EXISTS placements_by_node ON fragment_placements (node_id, status);
CREATE INDEX IF NOT EXISTS placements_by_fragment ON fragment_placements (fragment_id, status);

CREATE TABLE IF NOT EXISTS node_stats (
    node_id         UUID NOT NULL REFERENCES nodes(id),
    window_start    TIMESTAMPTZ NOT NULL,
    uptime_ratio    REAL NOT NULL,
    avg_latency_ms  REAL,
    free_bytes      BIGINT,
    cpu_load        REAL,
    mem_used_ratio  REAL,
    audits_passed   INTEGER NOT NULL DEFAULT 0,
    audits_failed   INTEGER NOT NULL DEFAULT 0,
    bytes_served    BIGINT NOT NULL DEFAULT 0,
    bytes_ingested  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (node_id, window_start)
);

CREATE TABLE IF NOT EXISTS ledger_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type      TEXT NOT NULL,
    owner_id        UUID,
    kind            TEXT NOT NULL,
    currency        TEXT NOT NULL DEFAULT 'CRD',
    UNIQUE (owner_type, owner_id, kind)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id              BIGSERIAL PRIMARY KEY,
    txn_id          UUID NOT NULL,
    account_id      UUID NOT NULL REFERENCES ledger_accounts(id),
    amount          BIGINT NOT NULL,
    entry_type      TEXT NOT NULL,
    reference       JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ledger_by_account ON ledger_entries (account_id, created_at);

CREATE TABLE IF NOT EXISTS audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    actor_type      TEXT NOT NULL,
    actor_id        UUID,
    action          TEXT NOT NULL,
    subject_type    TEXT,
    subject_id      UUID,
    detail          JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_by_actor ON audit_logs (actor_type, actor_id, created_at);

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'dsp_readonly') THEN
        CREATE ROLE dsp_readonly LOGIN PASSWORD 'dsp_readonly';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE dsp TO dsp_readonly;
GRANT USAGE ON SCHEMA public TO dsp_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO dsp_readonly;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO dsp_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO dsp_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO dsp_readonly;
