---
tags:
  - map
aliases:
  - System architecture
  - Architecture
---

# Map of Architecture

The platform splits into a **[[Control Plane]]** (services we run) and a **[[Data Plane]]** (machines others run). Defining rule: **file bytes flow only between [[CLI and SDK|clients]] and [[Node Agent|storage nodes]]**. The control plane handles metadata, coordination, and health — never file contents.

## Control plane services

| Service | Job | State it touches |
|---|---|---|
| [[API Gateway]] | Public HTTP entry; auth, rate limits, routing | none (stateless) |
| [[Metadata Service]] | Source of truth for files, chunks, placements | [[Postgres]] |
| [[Scheduler]] | Where every [[Fragment]] should live | [[Postgres]], [[Redis]] |
| [[Health Monitor]] | Heartbeats, liveness, [[Storage Challenge]]s | [[Redis]], [[Postgres]], [[NATS JetStream]] |
| [[Repair Service]] | Reconstruct lost fragments on ciphertext | [[NATS JetStream]], [[Metadata Service]] |
| [[Ledger Service]] | Usage meters and double-entry books | [[Postgres]], [[NATS JetStream]] |
| [[Admin Dashboard]] | Operator UI: fleet, repair queue, quarantine | read-mostly |

## Data plane

- [[Node Agent]] — Go binary on provider hardware; stores opaque fragments
- [[CLI and SDK]] — encrypts, chunks, erasure-codes, talks to nodes directly
- [[Web Dashboard]] — customer + provider UI through the gateway

## State layer

- [[Postgres]] — metadata, node registry, ledger (transactional truth)
- [[Redis]] — liveness TTLs, sessions, rate limits, scheduler cache
- [[NATS JetStream]] — node-state events, usage events, repair queue

## Sequences

- [[Upload Flow]]
- [[Download Flow]]
- [[Repair Loop]]
- [[Heartbeat Protocol]]
- [[Node Registration]]

## Technology

[[Go]] · [[gRPC]] · [[mTLS]] · [[Reed-Solomon]] · [[AES-256-GCM]]

## Scale story

See [[Horizontal Scalability]]. Two design rules make 100 → 1,000,000 nodes possible:

1. Stateless services everywhere — coordination state lives in Postgres / Redis / NATS
2. The control plane never touches file bytes — data-plane bandwidth scales with the fleet
