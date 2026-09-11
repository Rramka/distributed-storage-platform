-- M4 follow-up: pre-computed storage-challenge sets.
-- docs/07-security.md § Storage proofs. Health Monitor mints sets via a
-- ticketed GET (the spec refresh path); the client does not register them.

CREATE TABLE IF NOT EXISTS fragment_challenges (
    fragment_id     UUID NOT NULL REFERENCES fragments(id),
    node_id         UUID NOT NULL REFERENCES nodes(id),
    seq             INTEGER NOT NULL,
    offset_bytes    INTEGER NOT NULL,
    length_bytes    INTEGER NOT NULL,
    nonce           BYTEA NOT NULL,
    expected        BYTEA NOT NULL,
    spent_at        TIMESTAMPTZ,
    PRIMARY KEY (fragment_id, node_id, seq)
);

CREATE INDEX IF NOT EXISTS fragment_challenges_unspent
    ON fragment_challenges (fragment_id, node_id)
    WHERE spent_at IS NULL;

-- Sets running low: count unspent per placement for the challenger refill path.
CREATE INDEX IF NOT EXISTS fragment_challenges_low_set
    ON fragment_challenges (node_id, fragment_id, spent_at);
