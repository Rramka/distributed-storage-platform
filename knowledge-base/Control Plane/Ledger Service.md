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

- Consumes usage events from [[NATS JetStream]] (`USAGE_EVENTS`, deterministic event IDs → idempotent replay)
- Hourly accrual jobs: GB-hour charges, provider earnings weighted by reliability `R` (integer basis points)
- Sole writer of [[Ledger Account]] / [[Ledger Entry]] in [[Postgres]]
- Enforces `SUM(amount) = 0` per `txn_id` in Go and via a deferred Postgres trigger
- Injectable clock so the M5 exit test can drive 720 hourly windows without sleeping

See [[Map of Billing]], [[Credits]], [[Customer Pricing]], [[Quota]] (quota is M5b)

Exposed to users via `GET /storage` and `GET /earnings` on the [[Customer REST API]].
