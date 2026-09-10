---
tags:
  - tech
aliases:
  - NATS
  - JetStream
  - Event bus
---

# NATS JetStream

Durable, replayable streams. Lighter than Kafka at MVP scale.

| Stream | Producers | Consumers |
|---|---|---|
| `NODE_EVENTS` (`node.online` / `suspect` / `offline`) | [[Health Monitor]] | [[Repair Service]] |
| `REPAIR_JOBS` (`repair.critical` / `high` / `normal`, work-queue) | Repair Service | Repair workers |
| Audit failures / scrub reports | Health Monitor, clients | Repair Service (not yet) |
| Usage events | [[Metadata Service]], nodes | [[Ledger Service]] (deferred) |

Idempotent consumers (deterministic event IDs) so replay after crash does not double-bill or double-repair.

Related: [[Repair Loop]], [[Map of Billing]], [[Horizontal Scalability]]
