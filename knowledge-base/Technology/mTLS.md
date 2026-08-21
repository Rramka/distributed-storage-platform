---
tags:
  - tech
  - security
aliases:
  - Mutual TLS
  - Private CA
---

# mTLS

Every [[Node]] *is* its certificate. Private CA issues certs at [[Node Registration]]; fingerprint pinned in Postgres.

- Node ↔ platform: mTLS both ways; 30-day certs, auto-renew; revocation by fingerprint flag
- Client ↔ node: TLS with platform-issued node cert (client knows it hit a registered node) + ticket auth. Payload is already [[AES-256-GCM]] ciphertext — TLS protects tickets and metadata
- Repair worker ↔ node: mTLS + tickets
- Internal services: per-service certs

No plaintext listener exists anywhere. TLS 1.3 only.

Related: [[Map of Security]], [[Health Monitor]], [[Node Agent]]
