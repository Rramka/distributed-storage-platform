---
tags:
  - service
aliases:
  - Gateway
---

# API Gateway

Single public HTTP entry point for [[Customer]]s and the [[Web Dashboard]].

- Terminates TLS, authenticates JWT / [[API Key]], enforces per-account rate limits ([[Redis]] token buckets)
- Routes to internal services over [[gRPC]]; shapes errors into the common format in [[Customer REST API]]
- Stateless — scale horizontally behind a load balancer

Does **not** sit on the fragment byte path. After [[Upload Flow]] planning, the client talks to [[Node Agent]] endpoints directly.

Operator UI traffic for [[Admin Dashboard]] also lands here (admin-role accounts, mandatory 2FA).
