---
tags:
  - security
  - flow
aliases:
  - Tickets
  - Upload ticket
---

# Placement Ticket

Ed25519, platform-signed, issued by [[Metadata Service]] during [[Upload Flow]] planning. The [[Node Agent]] never sees customer identity; the ticket **is** authorization.

Scoped: one `put`, one `fragment_id`, one `node_id`, expected SHA-256, `max_bytes`, minutes-scale `expires_at`, single-use `nonce`. A leak is nearly worthless.

Expected hash rides in the ticket so a node cannot be tricked into storing bytes that don't match what the customer registered.

Capacity reservation in [[Redis]] expires with the ticket.

Mirror image on success: [[Signed Receipt]]. Download uses [[Retrieval Ticket]].
