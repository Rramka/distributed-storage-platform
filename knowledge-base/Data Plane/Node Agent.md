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

- Fragment API (HTTPS) — PUT/GET/DELETE + `GET /challenge`, authorized by tickets
- Heartbeat loop — [[gRPC]] stream to [[Health Monitor]]
- Chunk store — fragment files + bbolt `meta.db` + WAL journal
- Integrity auditor — local SHA-256 scrub (~every two weeks, throttled)
- GC worker — expired fragments
- Bandwidth / resource limiter — token bucket, low I/O priority, target RSS < 150 MB
- Self-updater — signed releases, staged rollout

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
