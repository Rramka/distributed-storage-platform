---
tags:
  - billing
aliases:
  - Pricing
  - Storage price
---

# Customer Pricing

Phase 1 defaults, config-driven:

| Item | Rate |
|---|---|
| Storage | 3.0 CRD / GB-month, accrued hourly (~4.1 µCRD / GB-hour) |
| Download egress | 1.0 CRD / GB |
| Upload | free |
| API requests | free (rate-limited) |

Charged storage is **logical** [[File Version]] sizes, not the ~1.6× [[Erasure Coding]] footprint. Redundancy overhead is platform COGS, covered by the margin vs [[Provider Earnings]] (1.5 CRD/GB-month equivalent).

Premium redundancy (e.g. 10+10) is post-MVP. See [[Quota]], [[Credits]], [[Ledger Service]].
