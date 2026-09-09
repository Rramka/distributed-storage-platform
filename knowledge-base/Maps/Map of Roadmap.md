---
tags:
  - map
aliases:
  - Roadmap
---

# Map of Roadmap

Build rule: **every milestone ends with something that runs end-to-end**. Healing and billing layer onto a working store-and-retrieve core.

## Solo builder track (active)

One person, ~10–15 hours/week, AI-assisted. The investable artifact is M0–M4: encrypted upload, kill 6 of 16 nodes, download still works, fleet heals, platform DB decrypts nothing.

Deferred until after that demo: [[Ledger Service]], dashboards, JWT, installers, rebalancing, reputation beyond a scalar, S3. See [[Non-goals]] and `docs/10-mvp-roadmap.md`.

## Phase 1 slice

[[MVP Scope]] is a complete vertical: register, encrypt, place, retrieve, audit, repair. Ledger (no real money) and admin view are specified but deferred on the solo track.

Explicitly out: compute, S3 API, real payments, sync clients, provider-set pricing. See [[Non-goals]].

## Milestones

See [[Milestones]] for M0–M6 (foundations → metadata → happy-path encrypted storage → RS 10+6 → health/repair → ledger → dashboards). **Active work is M3 (erasure coding) after M2.** Encryption shipped in M2 so invariant 2 holds before 16-way placement.

M4 ([[Repair Loop]] + [[Storage Challenge]] + full [[Scheduler]] scoring) is the platform's core claim and gets the most test investment.

## After MVP

[[Long-term Vision]]: S3 + payments → file sync / block storage → containers / serverless → GPU / VMs / K8s. Each phase reuses the storage network, [[Reputation]], and ledger rails.

## How to test it

Simulated fleet of [[Node Agent]] containers + chaos harness (kill, corrupt, partition, flap). Continuously measured invariants: ≥ 12 healthy placements per committed chunk; no plaintext in any `data_dir`; ledger txns sum to zero; [[Placement Constraints]] hold after any repair sequence.
