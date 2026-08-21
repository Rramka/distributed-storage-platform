---
tags:
  - security
  - principle
aliases:
  - Node reputation
---

# Reputation

Score in [0, 1] on each [[Node]]. EWMA so recent behavior dominates. New nodes start at **0.5** with a 14-day probation quota — attackers cannot spin up a thousand fresh nodes and immediately attract data.

| Event | Effect |
|---|---|
| [[Storage Challenge]] passed | small increase |
| Fragment served OK | small increase |
| Challenge fail / corrupt serve | large decrease |
| Unplanned `offline` | medium decrease |
| Honest scrub self-report | tiny decrease |
| Graceful drain | no penalty |

Used twice, on purpose: [[Scheduler]] placement weight (`w_rep` 0.30) and reliability multiplier `R` in [[Provider Earnings]]. Incentives point the same way.

Related: [[Node Stats]], [[Placement Constraints]]
