---
tags:
  - map
aliases:
  - Roadmap
---

# Map of Roadmap

Build rule: **every milestone ends with something that runs end-to-end**. Healing and billing layer onto a working store-and-retrieve core.

## Phase 1 slice

[[MVP Scope]] is a complete vertical: register, encrypt, place, retrieve, audit, repair, ledger (no real money), admin view.

Explicitly out: compute, S3 API, real payments, sync clients, provider-set pricing. See [[Non-goals]].

## Milestones

See [[Milestones]] for M0–M6 (foundations → metadata → happy-path storage → encryption + EC → health/repair → ledger → dashboards).

M4 ([[Repair Loop]] + [[Storage Challenge]] + full [[Scheduler]] scoring) is the platform's core claim and gets the most test investment.

## After MVP

[[Long-term Vision]]: S3 + payments → file sync / block storage → containers / serverless → GPU / VMs / K8s. Each phase reuses the storage network, [[Reputation]], and ledger rails.

## How to test it

Simulated fleet of [[Node Agent]] containers + chaos harness (kill, corrupt, partition, flap). Continuously measured invariants: ≥ 12 healthy placements per committed chunk; no plaintext in any `data_dir`; ledger txns sum to zero; [[Placement Constraints]] hold after any repair sequence.
