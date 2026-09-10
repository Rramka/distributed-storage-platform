---
tags:
  - service
aliases:
  - Repair
---

# Repair Service

Keeps redundancy at target levels without humans. Consumes `node.offline` from [[NATS JetStream]] `NODE_EVENTS` and jobs from `REPAIR_JOBS`.

For each affected [[Chunk]], if healthy [[Fragment Placement]] count drops below 13 of 16, it:

1. Asks [[Metadata Service]] for a repair plan (GET tickets on survivors, PUT tickets on fresh nodes)
2. Downloads any 10 surviving fragments (**ciphertext**)
3. Verifies hashes, `ReconstructShards` fills missing indexes, verifies rebuilt hashes against the manifest
4. PUTs reconstructed fragments with those tickets
5. Commits receipts to metadata; old placements stay `lost`

The worker **never holds `TICKET_SIGNING_SEED`** and **never decrypts**. [[Zero Knowledge]] survives this path.

Workers are stateless NATS consumers. Priority is three subjects: `repair.critical` (≤10 healthy), `repair.high` (11), `repair.normal` (12). Throttles: `REPAIR_MAX_CONCURRENT`, `REPAIR_BUDGET_MBPS`.

**Flap (interim):** on `node.online`, hash-verified GET of that node's `lost` placements restores them or marks them `expiring` if reconstructed elsewhere. Storage challenges are a follow-up slice.

Full loop: [[Repair Loop]]. Related: [[Erasure Coding]], [[Self-Healing]], [[Horizontal Scalability]]
