# Status

Last updated: 2026-09-10 (M3: Reed-Solomon 10+6 + 16-way placement)

## Current milestone

**M3 — Erasure coding** (solo builder track) — **shipped**

Exit criteria: `dsp put` / `dsp get` round-trips byte-identical with any 6 of 16 agents stopped; plaintext never observable on any node's disk.

## Shipped last session

- Reed-Solomon 10+6 in `internal/pipeline` (`klauspost/reedsolomon`); property test any 10 of 16 reconstruct
- CLI emits 16 fragments per chunk; download races the first 10 clean shards and RS-decodes
- Commit threshold 14/16 stored placements per chunk (`store.CommitThreshold`)
- Scheduler hard caps: 1 fragment/node, region ≤3, ASN ≤3, owner ≤2; partial fills allowed; PlanUpload fails a chunk below 14
- Registration codes carry `country` / `region` / `asn` (provider-declared at mint, copied onto the node)
- Compose fleet `agent1..agent16` (ports 7443–7458); `fleet-seed` mints 8 providers × 2 nodes across 6 regions / 6 ASNs
- In-process e2e: encode → put 16 → stop 6 → reconstruct → decrypt; plaintext-marker scan of every `data_dir`
- `make fleet-kill6` for the Compose durability check

## Blocked

- Nothing. Next work is M4 (health state machine, NATS events, repair).

## Next three tasks

1. M4: Health Monitor `online → suspect → offline` + NATS `node.offline` events
2. M4: Repair Service ciphertext reconstruction (priority by healthy count); scheduler scoring + weighted sampling
3. M4 exit: kill 6 of 16, heal back to 16/16, download never fails; chaos harness verbs

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, challenges beyond the repair path, or S3.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, copy CA via `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Ticket seed is the compose env `TICKET_SIGNING_SEED`. Existing volumes need `make migrate` for `000004_registration_code_placement.sql`.

Invariant 1 (≥12 healthy placements) applies from M3. Invariant 2 (no plaintext in `data_dir`) remains in force.

Deferred on the agent: background scrub, GC, bandwidth limiter, self-update.

Vault: `knowledge-base/Control Plane/Scheduler.md`, `CLI and SDK.md`, `Flows/Upload Flow.md`, `Download Flow.md`, `Node Registration.md`, `Maps/Map of Roadmap.md`, `Roadmap/Milestones.md`, `Home.md`.
