---
tags:
  - participant
aliases:
  - Data plane
---

# Data Plane

Provider machines and customer clients. **This is the only place file bytes exist**, and they exist only as ciphertext.

## Actors

- [[Node Agent]] — HTTPS fragment API + gRPC heartbeat stream
- [[CLI and SDK]] — encrypt / chunk / code / transfer
- [[Repair Service]] workers — reconstruct missing shards without decrypting (repair is control-plane-operated but the *bytes* still flow node ↔ repair worker, never into [[Postgres]])

## Traffic that bypasses the control plane

Dotted lines on [[Map of Architecture]]:

- Client PUT/GET of [[Fragment]]s (authorized by [[Placement Ticket]] / [[Retrieval Ticket]])
- Repair PUT of reconstructed ciphertext

[[API Gateway]] never proxies file bytes. That is why data-plane bandwidth costs the platform approximately zero on the happy path.

Related: [[Upload Flow]], [[Download Flow]], [[Node Agent]]
