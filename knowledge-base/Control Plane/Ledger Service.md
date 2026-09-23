---
tags:
  - service
  - billing
aliases:
  - Billing service
  - Ledger
---

# Ledger Service

Meters usage and maintains the internal double-entry ledger — computed charges for [[Customer]]s and computed [[Provider Earnings]], with **no real money** in Phase 1. Binary `cmd/ledger`, healthz on `:8085`.

- Hourly Accruer reads [[Postgres]] meters (committed sizes, `stored` placements, download receipts, [[Node Stats]]) and posts balanced `ledger_txns`
- Archives [[Usage Event]]s from [[NATS JetStream]] (`USAGE_EVENTS`) as a replayable audit trail; reconciles egress bytes against receipts
- `ledger_txns.txn_id` is the idempotency key (`ON CONFLICT DO NOTHING`); a watermark backfills missed hours after a crash
- Sole writer of [[Ledger Account]] / [[Ledger Entry]]
- Enforces `SUM(amount) = 0` per `txn_id` in Go and via a deferred Postgres trigger
- Reliability `R` uses `reputation_factor = 0.8 + 0.4 * reputation` (clamped [0, 1.2]), applied as integer basis points
- Injectable clock so the M5 exit test can drive 720 hourly windows without sleeping
- Internal `PostAdjustment` (reason required) for corrections; no public adjustment API

See [[Map of Billing]], [[Credits]], [[Customer Pricing]], [[Quota]] (credit floor + monthly `statements`).

Exposed to users via `GET /storage` and `GET /earnings` on the [[Customer REST API]].
