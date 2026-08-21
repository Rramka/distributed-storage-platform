---
tags:
  - tech
aliases:
  - Cache
  - Liveness store
---

# Redis

Hot, ephemeral state:

- [[Heartbeat Protocol]] liveness keys with TTL (failure detection without touching [[Postgres]])
- Rate limits at [[API Gateway]]
- Sessions
- [[Scheduler]] candidate cache and capacity reservations (expire with [[Placement Ticket]]s)

100k nodes × 10s interval ≈ 10k writes/s — the reason heartbeats do not go to Postgres. Clustered Redis at that scale; Health Monitor replicas shard by node ID.
