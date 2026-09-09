---
tags:
  - flow
aliases:
  - Heartbeats
  - Liveness
---

# Heartbeat Protocol

[[Node Agent]] → [[Health Monitor]], every **10 seconds**, HTTP/JSON over [[mTLS]] (`POST /internal/heartbeat`). Response `{ "messages": [] }` is a slot for M4 control messages.

Payload: free/used bytes, CPU, memory, fragment count, agent version, disk read p95, optional SMART temp.

## Server-side liveness

Written to [[Redis]] with TTL. 3 misses → [[Node]] `suspect`. 5 minutes silent → `offline` → [[NATS JetStream]] `node.offline` → [[Repair Loop]].

At 10k nodes this is ~1k heartbeats/s — one Health Monitor replica is enough. At 100k+, replicas shard by consistent hash on node ID ([[Horizontal Scalability]]).

The stream is bidirectional: platform pushes expiry lists, [[Storage Challenge]]s, drain orders, update notices. Agent needs no inbound control port.
