---
tags:
  - participant
aliases:
  - Platform
---

# Control Plane

The trusted coordinator we operate: authentication, metadata, node selection, health, self-healing, integrity verification, billing. It orchestrates everything but **holds no file contents and no decryption keys**.

## Services

[[API Gateway]] · [[Metadata Service]] · [[Scheduler]] · [[Health Monitor]] · [[Repair Service]] · [[Ledger Service]] · [[Admin Dashboard]]

## State

[[Postgres]] (truth) · [[Redis]] (liveness, cache) · [[NATS JetStream]] (events, queues)

## What it stores vs never stores

| Stores | Never stores |
|---|---|
| [[User]], [[Bucket]], file names, hierarchy | File contents (plaintext or ciphertext) |
| Chunk/fragment IDs, sizes, checksums | Keys that can decrypt customer data |
| [[Fragment Placement]] map | [[Master Key]] |
| [[Node]] inventory, health, [[Reputation]] | — |
| Usage meters, [[Ledger Entry]] | — |

The [[Data Plane]] stores the inverse: opaque fragment bytes.

See [[Map of Architecture]], [[Horizontal Scalability]], [[Zero Knowledge]]
