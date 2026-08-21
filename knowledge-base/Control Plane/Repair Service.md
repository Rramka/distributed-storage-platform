---
tags:
  - service
aliases:
  - Repair
---

# Repair Service

Keeps redundancy at target levels without humans. Consumes node-failure and audit-failure events from [[NATS JetStream]].

For each affected [[Chunk]], if healthy [[Fragment Placement]] count drops to the repair threshold (**12 of 16**), it:

1. Downloads any 10 surviving fragments (**ciphertext**)
2. Verifies hashes, Reed–Solomon decodes, re-encodes missing shard indexes
3. Asks the [[Scheduler]] for fresh nodes
4. PUTs reconstructed fragments with new [[Placement Ticket]]s
5. Commits new placements; old ones stay `lost`

**Never decrypts.** Repair workers hold no keys. [[Zero Knowledge]] survives this path.

Workers are stateless NATS consumers — add workers to add throughput. Throttles: per-source-node bandwidth cap, global repair budget (mass-failure must not stampede the fleet).

Priority: ≥ 13 healthy = idle; 12 = normal; ≤ 11 escalates; 10 = urgent; < 10 = durability incident.

Full loop: [[Repair Loop]]. Related: [[Erasure Coding]], [[Self-Healing]], [[Horizontal Scalability]]
