---
tags:
  - flow
aliases:
  - Self-healing loop
  - Reconstruction
---

# Repair Loop

Triggered by: [[Node]] offline 5+ min, failed [[Storage Challenge]], corrupt fragment on download, agent scrub self-report, or low-priority rebalance from [[Scheduler]].

```mermaid
sequenceDiagram
    participant HM as Health Monitor
    participant Q as NATS JetStream
    participant RW as Repair Service
    participant SC as Scheduler
    participant SN as Surviving nodes
    participant NN as New node

    HM->>Q: node.offline / audit.failed
    Q->>RW: affected placements
    RW->>RW: mark lost, skip chunks still >= 13 healthy
    RW->>SN: GET any 10 ciphertext fragments
    RW->>RW: verify, RS-decode, re-encode missing shards
    RW->>SC: new placements
    RW->>NN: PUT reconstructed fragments
    RW->>RW: commit placements
```

## Properties

- **Ciphertext only** — [[Zero Knowledge]] holds
- **Idempotent** — re-check health before acting (node may have flapped back); NATS redelivers crashed jobs
- **Flap handling** — returning node's fragments are re-challenged; in-flight repair for them is cancelled; only actually reconstructed copies expire the old ones as surplus

See [[Repair Service]], [[Self-Healing]], [[Erasure Coding]]
