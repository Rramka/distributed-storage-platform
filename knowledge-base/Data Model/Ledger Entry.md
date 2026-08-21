---
tags:
  - entity
  - billing
aliases:
  - ledger_entries
---

# Ledger Entry

One leg of a double-entry transaction. `amount` in micro-credits (positive = credit, negative = debit). Grouped by `txn_id` — **must sum to zero**.

Types: `storage_charge` | `egress_charge` | `storage_earning` | `egress_earning` | `adjustment` (later: `deposit`, `payout`).

Immutable. Corrections are new adjustment txns. `reference` JSON links to window / node / file / event ID.

Related: [[Map of Billing]], [[Ledger Service]], [[Audit Log]]
