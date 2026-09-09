---
tags:
  - service
aliases:
  - Healthmon
---

# Health Monitor

Fan-in point for the node fleet.

## Heartbeats

Every [[Node Agent]] POSTs HTTP/JSON over [[mTLS]] (`POST /internal/heartbeat`) every **10 seconds**. Liveness is a [[Redis]] key `node:live:<id>` with 30s TTL. Durable `last_seen_at` / `used_bytes` land in [[Postgres]]. `suspect`/`offline` transitions and [[NATS JetStream]] events are M4.

State machine: `online` → 3 missed beats → `suspect` (no new placements) → 5 minutes silent → `offline` (repair evaluation). Events go to [[NATS JetStream]] (`node.offline`, etc.).

## Audits

Issues random [[Storage Challenge]]s. Failures mark [[Fragment Placement]] `lost` and damage [[Reputation]].

## Control channel to the agent

The same stream pushes expiry lists, challenges, drain orders, and update notices — so the agent needs **no inbound port for control**, only the fragment API port for data.

Related: [[Repair Service]], [[Node]], [[Self-Healing]]
