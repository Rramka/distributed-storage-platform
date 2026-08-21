---
tags:
  - tech
aliases:
  - Protobuf
  - RPC
---

# gRPC

Typed contracts + streaming for service-to-service and [[Node Agent]] ↔ [[Control Plane]].

Public [[Customer REST API]] stays REST/JSON for accessibility. Internal: gateway → metadata/ledger; agent [[Heartbeat Protocol]] is a bidirectional gRPC stream.

See [[Go]], [[mTLS]]
