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

**M2 (solo track, superseded by M3):** naive placement — filter `online` + Redis live key + free space, **one fragment per node per chunk**, uniform random sample.

**M3:** hard caps from [[Placement Constraints]] — one fragment per node; region / ASN caps of 3; owner cap of 2; `online` + free-space floor. Repair placement seeds existing pending/stored occupants into the cap counters. `country` / `region` / `asn` are declared on the registration code, not by the agent.

**M4 follow-up:** score-proportional weighted sampling with the weights above. `bandwidth_factor` is a neutral constant (1.0) until throughput is measured. New nodes have `probation_until` = register + 14 days and a placement quota.

Capacity is reserved in Redis until the [[Placement Ticket]] expires, so abandoned uploads free space automatically.

## Rebalancing

Continuous trickle: draining, pressure relief (~85% full), diversity healing, spreading onto new capacity. Rebalance jobs are low-priority [[Repair Loop]] work.

Related: [[Upload Flow]], [[Provider Earnings]] (same reliability signals)
