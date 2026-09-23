---
tags:
  - tech
aliases:
  - Protobuf
  - RPC
---

# gRPC

Typed contracts + streaming for service-to-service and [[Node Agent]] ↔ [[Control Plane]].

Typed contracts + streaming for service-to-service and [[Node Agent]] ↔ [[Control Plane]] **after** the solo track. There are no `.proto` files yet.

Public [[Customer REST API]] stays REST/JSON. Internal: gateway → metadata is HTTP/JSON; registration is HTTP/JSON over server-authenticated TLS; agent [[Heartbeat Protocol]] is HTTP/JSON over [[mTLS]] (gRPC deferred past M4).

See [[Go]], [[mTLS]]

See [[Go]], [[mTLS]]
