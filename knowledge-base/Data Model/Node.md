---
tags:
  - entity
aliases:
  - Nodes
  - Storage node
---

# Node

A provider machine in the registry. Identity **is** its [[mTLS]] client certificate (`cert_fingerprint` pinned).

| Status | Meaning |
|---|---|
| `pending` | registered, not yet reachable |
| `online` | selectable by [[Scheduler]] |
| `suspect` | 3 missed heartbeats; no new placements |
| `offline` | 5 min silent; [[Repair Loop]] evaluates |
| `draining` | capacity shrink / retirement / version floor |
| `quarantined` | operator or clone-detection |
| `retired` | done |

Also: country, region, **ASN** (ISP diversity), endpoint, capacity, used bytes, [[Reputation]] (0–1, default 0.5).

Real-time liveness lives in [[Redis]] (TTL), not Postgres. Hourly [[Node Stats]] feed scoring and [[Provider Earnings]].

Related: [[Node Agent]], [[Node Registration]], [[Heartbeat Protocol]], [[Placement Constraints]]
