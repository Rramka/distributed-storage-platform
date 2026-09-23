---
tags:
  - map
aliases:
  - Roadmap
---

# Map of Roadmap

Build rule: **every milestone ends with something that runs end-to-end**. Healing and billing layer onto a working store-and-retrieve core.

## Solo builder track (active)

One person, ~10–15 hours/week, AI-assisted. **M0–M5 (including M5.1 honest books), pilot readiness, and M5b (quota + statements) are in the repo.**

Deferred: dashboards, JWT, installers, rebalancing, reputation beyond a scalar, S3, real money. See [[Non-goals]] and `docs/10-mvp-roadmap.md`.

## Phase 1 slice

[[MVP Scope]] is a complete vertical: register, encrypt, place, retrieve, audit, repair, meter, ledger. Admin dashboards remain deferred.

Explicitly out: compute, S3 API, real payments, sync clients, provider-set pricing. See [[Non-goals]].

## Milestones

See [[Milestones]] for M0–M6 (foundations → metadata → happy-path encrypted storage → RS 10+6 → health/repair → ledger → dashboards). **M3 (erasure coding) is shipped.** **M4 (health, NATS, repair, challenges, scored scheduler, 24-agent fleet, harness, W10 visualizer) is in the repo.**

W10: fleet visualizer at gateway `GET /demo/` (`DEMO_MODE=1`) polling `GET /v1/demo/fleet`. Scripted capture: `make demo`.

W11: soak (`make soak` / `harness soak`), hermetic invariant + red-team CI job, nightly Compose fleet job. `harness corrupt` is implemented. Solo soak is hours over tens of files, not days over thousands.

W12: `make demo` writes `business/updates/demo-metrics-YYYY-MM-DD.json`. Static waitlist at `web/landing/` (Formspree; not served by the gateway). Screen recording is a human step.

W13: `harness partition` (`docker compose pause`, not iptables) with a ≥ 13/16 floor while a region is cut. `throttle` retired (neutral `bandwidth_factor`). CI `test` job is `-short`; `invariants` job runs `go test ./...` against Postgres. Nightly runs partition + a 20-minute soak.

W14: node registration is server-authenticated TLS (`:8444`); one registration code yields one node row; DELETE tickets are single-use; `make demo-ready` + `harness m4` assert `dsp get` while six holders are down. Docs match HTTP/JSON + binary tickets.

W15–W17 (M5): honest meters, `USAGE_EVENTS` audit trail, `cmd/ledger` on `:8085`, invariant 3, accelerated-month balanced books.

W18 (M5.1): `ledger_txns` idempotency, accruer watermark/backfill, `reputation_factor` [0.8, 1.2], per-receipt egress IDs, `GET /earnings` by window/type, adjustments.

W19–W21: cert renew/revoke, agent scrub, audit_logs writes, M5b quota (`403 quota_exceeded`) and monthly statements.

## After MVP

[[Long-term Vision]]: S3 + payments → file sync / block storage → containers / serverless → GPU / VMs / K8s. Each phase reuses the storage network, [[Reputation]], and ledger rails.

## How to test it

Simulated fleet of [[Node Agent]] containers + chaos harness (kill, corrupt, partition, flap). Continuously measured invariants: ≥ 12 healthy placements per committed chunk; no plaintext in any `data_dir`; ledger txns sum to zero; [[Placement Constraints]] hold after any repair sequence.
