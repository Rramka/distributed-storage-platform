---
tags:
  - map
aliases:
  - Billing
  - Ledger
---

# Map of Billing

Phase 1 is an **internal ledger**: real meters, real charges and earnings, **no actual money**. Settlement (Stripe, payouts) later plugs in without restructuring.

## Unit

[[Credits]] (`CRD`), stored as integer micro-credits. No floating-point money.

## What gets metered

| Meter | Source | Used for |
|---|---|---|
| GB-hours stored (customer) | [[Metadata Service]] committed sizes | [[Customer Pricing]] |
| Egress bytes (customer) | Download plans vs node transfer logs | [[Customer Pricing]] |
| GB-hours held (node) | `stored` [[Fragment Placement]]s | [[Provider Earnings]] |
| Egress bytes served (node) | Node-signed transfer logs | [[Provider Earnings]] |
| Uptime & audits (node) | [[Node Stats]] | reliability multiplier `R` |

Events flow: sources → [[NATS JetStream]] usage stream → [[Ledger Service]] → [[Ledger Account]] / [[Ledger Entry]] in [[Postgres]].

Upload (ingest) bandwidth is **free** in Phase 1 — it flows client → node and costs the platform nothing.

## Double-entry rule

Every economic fact is one `txn_id` whose [[Ledger Entry]] amounts **sum to zero**. Corrections are new `adjustment` txns, never edits. [[Ledger Service]] is the sole writer.

## Incentive alignment

[[Reputation]] used by the [[Scheduler]] is the same family of metrics as `R` in [[Provider Earnings]]. Flaky nodes earn less *and* receive fewer placements.

Anti-gaming: a provider downloading their own fragments through a colluding account pays customer egress that exceeds provider egress earnings — self-dealing is a net loss.

## Later (out of scope now)

`deposit` / `payout` transaction types on the same ledger. See [[Long-term Vision]] Phase 2.
