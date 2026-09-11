---
tags:
  - service
aliases:
  - Healthmon
---

# Health Monitor

Fan-in point for the node fleet.

## Heartbeats

Every [[Node Agent]] POSTs HTTP/JSON over [[mTLS]] (`POST /internal/heartbeat`) every **10 seconds**. Liveness is a [[Redis]] key `node:live:<id>` with 30s TTL. Durable `last_seen_at` / `used_bytes` land in [[Postgres]].

State machine (M4): `online` → 3 missed beats (30s) → `suspect` (no new placements) → 5 minutes silent → `offline`. Each transition is published on [[NATS JetStream]] `NODE_EVENTS` (`node.online` / `node.suspect` / `node.offline`). `offline` drives the [[Repair Loop]].

## Audits

Issues random [[Storage Challenge]]s. The Health Monitor mints pre-computed challenge sets via a ticketed GET (never stores fragment bytes), spends one tuple per audit, and on failure marks [[Fragment Placement]] `lost` and damages [[Reputation]]. Heartbeat CPU/mem/latency land in Redis `node:metrics:<id>`; hourly [[Node Stats]] rollups feed the [[Scheduler]].

## Control channel to the agent

The same stream pushes expiry lists, challenges, drain orders, and update notices — so the agent needs **no inbound port for control**, only the fragment API port for data.

Related: [[Repair Service]], [[Node]], [[Self-Healing]]
