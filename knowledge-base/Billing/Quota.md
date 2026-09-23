---
tags:
  - billing
aliases:
  - Credit floor
  - quota_exceeded
---

# Quota

Specified for Phase 1, **deferred to M5b**. Customers will have a credit floor. At the floor:

- **Uploads blocked** (`403 quota_exceeded`)
- **Downloads and [[File Deletion]] remain available** — data is never held hostage over billing

Enforced at [[API Gateway]] using balances from [[Ledger Service]]. See [[Customer Pricing]], [[Customer REST API]].
