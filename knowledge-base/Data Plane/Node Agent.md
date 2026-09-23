---
tags:
  - service
aliases:
  - Agent
  - Storage agent
---

# Node Agent

Single static [[Go]] binary that a [[Storage Provider]] installs. Turns a configured disk slice into rentable capacity. The platform's only presence on provider hardware.

Design goals, in order: **do no harm to the host**, **be verifiably honest**, **be operationally invisible**.

## Internals

- Fragment API (HTTPS) — PUT/GET/DELETE, authorized by [[Placement Ticket]] / [[Retrieval Ticket]]. `GET /challenge` is M4.
- Registration — HTTP/JSON POST over server-authenticated TLS to metadata `:8444` (one-time code)
- Heartbeat loop — HTTP/JSON POST every 10s over [[mTLS]] to [[Health Monitor]] (gRPC deferred past M4)
- Chunk store — fragment files fanned by ID prefix + bbolt `meta.db`
- Integrity auditor is the M4 [[Storage Challenge]] path. GC worker, bandwidth limiter, and self-updater remain deferred past M4.

## On-disk

```
<data_dir>/identity/     # node.key (0600), node.crt
<data_dir>/fragments/    # fanned out by ID prefix
<meta.db>                # local index: authoritative for *what this node holds*
<journal/>               # crash-safe writes
```

The [[Control Plane]] [[Fragment Placement]] map is authoritative for *what this node should hold*. Periodic reconcile deletes unknown/expired fragments and marks missing ones `lost`.

A fragment is acknowledged ([[Signed Receipt]]) only after temp write → fsync → hash verify → rename → index.

## Reachability

MVP requires inbound connections for the fragment API. Strict NAT is out of scope; registration tests reachability. Relay traversal is post-MVP ([[Long-term Vision]]).

See [[Heartbeat Protocol]], [[Node Registration]], [[Storage Challenge]], [[mTLS]]
