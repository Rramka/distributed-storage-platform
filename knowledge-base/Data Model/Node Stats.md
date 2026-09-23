---
tags:
  - entity
aliases:
  - node_stats
---

# Node Stats

Hourly rollups per [[Node]]: uptime ratio, latency, free bytes, CPU/mem, audits passed/failed, bytes served/ingested.

`uptime_ratio` is `min(1, heartbeats_received / 360)` for that UTC hour. Offline nodes still get a row (ratio 0). `bytes_ingested` from verified PUT receipts; `bytes_served` from node-signed GET receipts the client reports.

Feeds [[Scheduler]] scoring, [[Reputation]], and the reliability multiplier `R` in [[Provider Earnings]]. Exposed as `GET /nodes/{id}/stats`.
