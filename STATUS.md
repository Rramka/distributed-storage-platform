# Status

Last updated: 2026-09-09 (compose: CA key perms + node register FK)

## Current milestone

**M2 — Node agent + happy-path storage** (solo builder track)

Exit criteria: `dsp put` / `dsp get` round-trips byte-identical through 5 local agent containers. Encryption is live (invariant 2). Reed-Solomon and ≥12 placements are M3.

## Shipped last session

- Private CA (`internal/ca`), Ed25519 tickets (`internal/tickets`), node receipts (`internal/receipts`)
- Client pipeline: Argon2id, AES-256-GCM wrap of the file key, streaming 64 KiB GCM segments, 16 MB chunking
- Node agent: crash-safe chunk store (bbolt), TLS fragment API, registration CSR, 10s mTLS heartbeats
- Node registry + registration codes; healthmon Redis `node:live:<id>` TTL 30s; naive scheduler
- Gateway `POST /v1/upload`, `POST /v1/upload/{id}/commit`, `GET /v1/download/{id}`, `POST /v1/nodes/registration-codes`
- CLI `dsp put` / `dsp get` / `dsp nodes` / `dsp provider codes create`
- Compose 5-agent fleet (`ca-init`, `fleet-seed`, `agent1..5`); `DSP_CA_FILE=.local/ca.crt` after `make up`
- In-process e2e: encrypted put/get plus plaintext-marker scan of `data_dir`
- Spec: HTTP/JSON node control (gRPC post-M4); `/healthz` plaintext exception; GCM wrap; encryption in M2

## Blocked

- Nothing. Next work is M3 (Reed-Solomon 10+6, 16-way placement).

## Notes

- `ca-init` chowns `/ca` to uid 10001 (`CA_OWNER_UID`) so metadata/healthmon can read `ca.key` (0600). Existing Postgres volumes need `make migrate` for `000003_nodes_registration.sql`. Node register now inserts the `nodes` row before binding `node_registration_codes.node_id` (FK).

## Next three tasks

1. M3: Reed-Solomon 10+6 in `internal/pipeline`; 16 fragments per chunk; commit threshold 14/16
2. Placement caps (region/ASN/owner) with a 16-node Compose fleet
3. Round-trip with any 6 of 16 agents stopped; plaintext-on-disk assertion on every node

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, repair, or challenges.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, copy CA via `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Ticket seed is the compose env `TICKET_SIGNING_SEED`.

Invariant 1 (≥12 healthy placements) does **not** apply until M3. Invariant 2 (no plaintext in `data_dir`) is enforced from M2.

Deferred on the agent: background scrub, GC, bandwidth limiter, self-update.

Vault: `knowledge-base/Data Plane/Node Agent.md`, `CLI and SDK.md`, `Control Plane/Scheduler.md`, `Health Monitor.md`, `Flows/Node Registration.md`, `Heartbeat Protocol.md`, `Upload Flow.md`, `Download Flow.md`.
