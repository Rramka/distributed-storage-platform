---
tags:
  - flow
aliases:
  - Self-healing loop
  - Reconstruction
---

# Repair Loop

Triggered by: [[Node]] offline 5+ min. Failed [[Storage Challenge]], corrupt fragment on download, and scrub self-report are specified but not yet wired (follow-up M4 slice).

```mermaid
sequenceDiagram
    participant HM as Health Monitor
    participant Q as NATS JetStream
    participant RW as Repair Service
    participant MD as Metadata Service
    participant SC as Scheduler
    participant SN as Surviving nodes
    participant NN as New node

    HM->>Q: node.offline
    Q->>RW: node.offline
    RW->>RW: mark lost, skip chunks still >= 13 healthy
    RW->>MD: plan (tickets)
    MD->>SC: Place missing fragments
    RW->>SN: GET any 10 ciphertext fragments
    RW->>RW: verify, ReconstructShards
    RW->>NN: PUT reconstructed fragments
    RW->>MD: commit receipts
```

## Properties

- **Ciphertext only** — [[Zero Knowledge]] holds; worker has no signing seed
- **Idempotent** — re-check health before acting (node may have flapped back); NATS redelivers crashed jobs
- **Flap handling** — returning node's fragments are hash-GET re-validated (interim, until [[Storage Challenge]]); only actually reconstructed copies expire the old ones as surplus

See [[Repair Service]], [[Self-Healing]], [[Erasure Coding]]
