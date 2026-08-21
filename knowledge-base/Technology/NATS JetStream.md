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
| Node state (`online → suspect → offline`) | [[Health Monitor]] | [[Repair Service]] |
| Audit failures / scrub reports | Health Monitor, clients | Repair Service |
| Usage events | [[Metadata Service]], nodes | [[Ledger Service]] |
| Repair jobs (priority) | Repair Service | Repair workers |

Idempotent consumers (deterministic event IDs) so replay after crash does not double-bill or double-repair.

Related: [[Repair Loop]], [[Map of Billing]], [[Horizontal Scalability]]
