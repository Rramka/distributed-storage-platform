---
tags:
  - service
aliases:
  - Dashboard
  - Customer dashboard
  - Provider dashboard
---

# Web Dashboard

SPA for humans. Talks only to [[API Gateway]] ([[Customer REST API]]). Never talks to [[Node Agent]]s — the [[CLI and SDK]] (or a browser-side WASM pipeline later) handles bytes.

## Customer view

Files, folders, usage, current-cycle charges from [[Ledger Service]] (`GET /storage`).

## Provider view

[[Node]] list, [[Node Stats]], registration codes for [[Node Registration]], [[Provider Earnings]] (`GET /earnings`), drain action.

Distinct from [[Admin Dashboard]] (operators). Milestone M6 in [[Milestones]].
