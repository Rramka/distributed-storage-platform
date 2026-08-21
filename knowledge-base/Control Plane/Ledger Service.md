---
tags:
  - service
  - billing
aliases:
  - Billing service
  - Ledger
---

# Ledger Service

Meters usage and maintains the internal double-entry ledger — computed charges for [[Customer]]s and computed [[Provider Earnings]], with **no real money** in Phase 1.

- Consumes usage events from [[NATS JetStream]] (deterministic event IDs → idempotent replay)
- Hourly accrual jobs: GB-hour charges, provider earnings weighted by reliability
- Sole writer of [[Ledger Account]] / [[Ledger Entry]] in [[Postgres]]
- Enforces `SUM(amount) = 0` per `txn_id` before commit

See [[Map of Billing]], [[Credits]], [[Customer Pricing]], [[Quota]]

Exposed to users via `GET /storage` and `GET /earnings` on the [[Customer REST API]].
