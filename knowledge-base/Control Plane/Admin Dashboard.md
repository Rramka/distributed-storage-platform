---
tags:
  - service
aliases:
  - Operator dashboard
  - Admin
---

# Admin Dashboard

Read-mostly backend + operator UI: network map, [[Node]] list with health and [[Reputation]], [[Repair Service]] queue depth, durability alerts, ledger summaries.

Operator actions (all attributed in [[Audit Log]], admin role + mandatory 2FA):

- Quarantine a node (stolen cert, concurrent heartbeat clone, persistent audit failure)
- Force repair
- Adjust rates ([[Customer Pricing]] / [[Provider Earnings]] are config-driven)
- Halt an agent rollout by version

Related: [[Web Dashboard]] (customer/provider, not operator), [[Map of Architecture]]
