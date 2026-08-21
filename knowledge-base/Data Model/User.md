---
tags:
  - entity
aliases:
  - Users
  - Account
---

# User

Postgres `users`. One identity for the whole marketplace.

| Field | Notes |
|---|---|
| email | unique |
| password_hash | argon2id |
| role | `customer` \| `provider` \| `both` \| `admin` |
| status | `active` \| `suspended` \| `deleted` |

Owns [[Bucket]]s, [[API Key]]s, and optionally [[Node]]s. Has [[Ledger Account]]s for charges and/or earnings.

Auth: login → JWT (15 min) + rotating refresh; TOTP 2FA at launch. Programmatic: [[API Key]].

Related: [[Customer]], [[Storage Provider]], [[Audit Log]]
