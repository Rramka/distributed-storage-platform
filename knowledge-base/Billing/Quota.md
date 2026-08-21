---
tags:
  - billing
aliases:
  - Credit floor
  - quota_exceeded
---

# Quota

Customers have a credit floor (e.g. free-tier allowance in Phase 1). At the floor:

- **Uploads blocked** (`403 quota_exceeded`)
- **Downloads and [[File Deletion]] remain available** — data is never held hostage over billing

Enforced at [[API Gateway]] using balances from [[Ledger Service]]. See [[Customer Pricing]], [[Customer REST API]].
