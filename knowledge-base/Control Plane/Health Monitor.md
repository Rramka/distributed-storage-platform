---
tags:
  - service
aliases:
  - Healthmon
---

# Health Monitor

Fan-in point for the node fleet.

## Heartbeats

Every [[Node Agent]] holds a long-lived [[gRPC]] stream over [[mTLS]] and sends a [[Heartbeat Protocol]] message every **10 seconds**. Liveness is a [[Redis]] key with TTL — expiry means presumed offline. Durable attributes and hourly [[Node Stats]] land in [[Postgres]].

State machine: `online` → 3 missed beats → `suspect` (no new placements) → 5 minutes silent → `offline` (repair evaluation). Events go to [[NATS JetStream]] (`node.offline`, etc.).

## Audits

Issues random [[Storage Challenge]]s. Failures mark [[Fragment Placement]] `lost` and damage [[Reputation]].

## Control channel to the agent

The same stream pushes expiry lists, challenges, drain orders, and update notices — so the agent needs **no inbound port for control**, only the fragment API port for data.

Related: [[Repair Service]], [[Node]], [[Self-Healing]]
