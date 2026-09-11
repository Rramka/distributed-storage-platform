---
tags:
  - service
aliases:
  - Gateway
---

# API Gateway

Single public HTTP entry point for [[Customer]]s and the [[Web Dashboard]].

- Terminates TLS, authenticates JWT / [[API Key]], enforces per-account rate limits ([[Redis]] token buckets). **Solo track:** API keys only; JWT is deferred.
- Routes to [[Metadata Service]] over internal HTTP/JSON in M1 (gRPC arrives in M2 with the node control API); shapes errors into the common format in [[Customer REST API]]
- Stateless — scale horizontally behind a load balancer

Does **not** sit on the fragment byte path. After [[Upload Flow]] planning, the client talks to [[Node Agent]] endpoints directly.

Solo-track demo UI: `GET /demo/` (fleet visualizer) and `GET /v1/demo/fleet` when `DEMO_MODE=1`. Not present in a normal deployment.

Operator UI traffic for [[Admin Dashboard]] also lands here (admin-role accounts, mandatory 2FA).
