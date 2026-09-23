---
tags:
  - entity
  - security
aliases:
  - audit_logs
---

# Audit Log

Insert-only table. Writers exist for API-key issue/revoke, [[Node Registration]] / renew, file deletion, ticket batches, honest scrub self-report, and ledger adjustments.

`actor_type`: user | node | service | admin. `dsp_readonly` has SELECT only; UPDATE/DELETE are revoked from PUBLIC.

Related: [[Admin Dashboard]], [[Map of Security]]
