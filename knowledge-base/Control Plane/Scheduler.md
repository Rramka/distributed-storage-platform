---
tags:
  - service
aliases:
  - Node scheduler
---

# Scheduler

Decides where every [[Fragment]] should live. Pure decision-making — it moves no bytes.

## Inputs

- [[Redis]] — real-time liveness and last-heartbeat metrics
- [[Postgres]] `nodes` / [[Node Stats]] — country, region, ASN, capacity, uptime, audit pass rate

## Scoring (MVP weights)

```
score = 0.20·capacity_headroom + 0.30·uptime_30d + 0.30·reputation
      + 0.10·latency + 0.10·bandwidth − pressure_penalty
```

[[Reputation]] is an EWMA over challenge pass/fail, served fragments, unplanned offline, honest self-reports. New nodes start at 0.5 with a **probation quota**.

## Hard rules before scoring

[[Placement Constraints]]: one fragment per node; region / ASN caps of 3; owner cap of 2; must be `online`.

**M2 (solo track):** naive placement — filter `online` + Redis live key + free space, **one fragment per node per chunk**, uniform random sample. No scoring, no region/ASN/owner caps yet (those land with 16-way placement in M3 / scoring in M4). Capacity reserved in Redis until the [[Placement Ticket]] expires.

Capacity is reserved in Redis until the [[Placement Ticket]] expires, so abandoned uploads free space automatically.

## Rebalancing

Continuous trickle: draining, pressure relief (~85% full), diversity healing, spreading onto new capacity. Rebalance jobs are low-priority [[Repair Loop]] work.

Related: [[Upload Flow]], [[Provider Earnings]] (same reliability signals)
