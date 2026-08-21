---
tags:
  - principle
  - tech
aliases:
  - Scale
  - 1 million nodes
---

# Horizontal Scalability

Same architecture at 100 and at 1,000,000 [[Node]]s. What changes is replication and sharding of state — not the service design.

## MVP (100–10k)

Single [[Postgres]] primary + replicas, single [[Redis]], single [[NATS JetStream]] cluster, 2+ replicas of each stateless [[Go]] service. 10k nodes at 10s heartbeat ≈ 1k writes/s — one [[Health Monitor]] replica is enough.

## 100k+ pressure points

1. **Heartbeat fan-in** — Health Monitor replicas by consistent hash on node ID; liveness stays in Redis; only rollups/transitions touch Postgres
2. **Metadata volume** — [[Fragment Placement]] shards by file/fragment ID; node-centric repair queries use an inverted index sharded by `node_id`
3. **Repair throughput** — stateless NATS consumers; per-source and global bandwidth caps so mass failure degrades gracefully

Two rules: **stateless services everywhere**, and **[[Control Plane]] never touches file bytes** (happy-path transfer costs the platform ~zero bandwidth; only [[Repair Loop]] traffic transits platform infrastructure, and that can later move to edge repair nodes).
