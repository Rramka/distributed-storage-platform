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

Auth: login → JWT is **deferred** on the solo track. Register stores an argon2id password hash; `POST /auth/api-keys` uses HTTP Basic; programmatic calls use [[API Key]] (`X-Api-Key`).

Related: [[Customer]], [[Storage Provider]], [[Audit Log]]
