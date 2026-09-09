-- Lookup index for API-key authentication (docs/03-data-model.md, docs/07-security.md).
CREATE UNIQUE INDEX IF NOT EXISTS api_keys_key_hash_uidx ON api_keys (key_hash);
