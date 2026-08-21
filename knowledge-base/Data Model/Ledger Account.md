---
tags:
  - entity
  - billing
aliases:
  - ledger_accounts
---

# Ledger Account

Double-entry account. `owner_type`: `user` | `node` | `platform`. `kind`: `customer_balance` | `provider_earnings` | `platform_revenue`. Currency: `CRD` ([[Credits]]).

Balances are derived (`SUM` of [[Ledger Entry]], plus materialized running balances for display). Entries are the truth.

[[Ledger Service]] is the only writer.
