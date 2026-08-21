---
tags:
  - entity
aliases:
  - node_stats
---

# Node Stats

Hourly rollups per [[Node]]: uptime ratio, latency, free bytes, CPU/mem, audits passed/failed, bytes served/ingested.

Raw heartbeats are **not** persisted — [[Health Monitor]] aggregates in memory and flushes rollups.

Feeds [[Scheduler]] scoring, [[Reputation]], and the reliability multiplier `R` in [[Provider Earnings]]. Exposed to providers as `GET /nodes/{id}/stats`.
