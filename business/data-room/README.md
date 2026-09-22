# Data room

Index of artifacts a diligence reader can open without a walkthrough. Nothing here is a secret, a dump, or customer data.

## Spec (canonical)

| Doc | What it settles |
|---|---|
| [`docs/01-overview.md`](../../docs/01-overview.md) | Product promise, zero-knowledge claim |
| [`docs/02-system-architecture.md`](../../docs/02-system-architecture.md) | Control plane vs data plane |
| [`docs/03-data-model.md`](../../docs/03-data-model.md) | Postgres entities |
| [`docs/04-storage-pipeline.md`](../../docs/04-storage-pipeline.md) | Argon2id, AES-256-GCM, 16 MB chunks, RS 10+6 |
| [`docs/05-node-agent.md`](../../docs/05-node-agent.md) | Agent, heartbeats, mTLS |
| [`docs/06-scheduler-and-repair.md`](../../docs/06-scheduler-and-repair.md) | Caps, scoring, repair loop |
| [`docs/07-security.md`](../../docs/07-security.md) | Keys, tickets, receipts |
| [`docs/08-api.md`](../../docs/08-api.md) | REST; API keys only on the solo track |
| [`docs/09-billing-ledger.md`](../../docs/09-billing-ledger.md) | Ledger design (service deferred) |
| [`docs/10-mvp-roadmap.md`](../../docs/10-mvp-roadmap.md) | Solo builder track M0–M4 / W12 |

## Measured runs

| File | What it is |
|---|---|
| [`updates/demo-metrics-2026-09-22.json`](../updates/demo-metrics-2026-09-22.json) | Kill-6-of-16 raise scene (download during failure, repair latency, dump check) |
| [`updates/demo-script-w12.md`](../updates/demo-script-w12.md) | Shot list for the screen recording |
| [`updates/soak-2026-09-21.json`](../updates/soak-2026-09-21.json) | 2h soak, seed 21; min healthy 10/16; kill withheld while already ≤12 |
| [`updates/security-review-2026-09-20.md`](../updates/security-review-2026-09-20.md) | W11 crypto-surface review; no medium+ |

## CI

| Workflow | What it asserts |
|---|---|
| [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) | `gofmt`, `go vet`, `go test`; **invariants** job with Postgres 16 + Redis 7 (store / invariants / gateway / healthmon) |
| [`.github/workflows/nightly.yml`](../../.github/workflows/nightly.yml) | Compose fleet, `dsp put`, `make chaos-m4`, `make invariants` |

## Market

| File | What it is |
|---|---|
| [`research/competitors.md`](../research/competitors.md) | Sourced comparison set (S3, B2, Wasabi, R2, e2, Storj, Sia, Filecoin) |
| [`research/pricing.md`](../research/pricing.md) | Phase 1 CRD card vs those list prices |
| [`research/positioning.md`](../research/positioning.md) | Why this is not another Storj |
| [`deck/outline.md`](../deck/outline.md) | Slide-by-slide raise arc |

## Explicitly not here

Live Postgres dumps, CA private keys, API keys, passphrases, agent `data_dir`. The red-team claim is that a dump **would** decrypt nothing; we do not check dumps into git.
