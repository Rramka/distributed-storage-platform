---
tags:
  - security
  - flow
aliases:
  - Download ticket
---

# Retrieval Ticket

Same capability model as [[Placement Ticket]], `op: get`. Issued during [[Download Flow]] planning. Only currently-healthy [[Fragment Placement]]s are listed.

[[Node Agent]] verifies signature against pinned platform public key, node ID, expiry, nonce — and needs nothing else. No authorization callback to the [[Control Plane]], so the [[Data Plane]] stays fast and bytes stay off the control path.
