---
tags:
  - billing
aliases:
  - Earnings
  - Node earnings
---

# Provider Earnings

Hourly, per [[Node]]:

```
storage_earning = GB_held_avg * storage_rate * R(node)
egress_earning  = GB_served   * egress_rate  * R(node)
```

Phase 1: `storage_rate` ≈ 1.5 CRD/GB-month, `egress_rate` = 0.5 CRD/GB.

```
R = uptime_30d * audit_pass_30d * reputation_factor   // clamped [0, 1.2]
```

Same family of signals as [[Scheduler]] / [[Reputation]]: flaky nodes earn less *and* get fewer placements.

Failed [[Storage Challenge]] zeroes that placement's accrual from the moment of failure (`stored` → `lost` leaves the meter). Scheduler-created placements only — a node cannot stuff itself with junk.

Anti-gaming: egress paid only for ticketed transfers reconciled against client reports; self-dealing (provider downloading own fragments via a colluding [[Customer]]) is a net loss because customer egress rates exceed provider egress earnings.

Related: [[Node Stats]], [[Ledger Entry]], [[Credits]]
