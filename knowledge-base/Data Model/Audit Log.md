---
tags:
  - entity
  - security
aliases:
  - audit_logs
---

# Audit Log

Insert-only table. Every security-relevant action: logins, key issuance, [[Node Registration]], quarantine, deletions, admin operations, ticket batch issuance.

`actor_type`: user | node | service | admin. No service credential has UPDATE/DELETE grants.

Related: [[Admin Dashboard]], [[Map of Security]]
