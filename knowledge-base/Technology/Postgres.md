---
tags:
  - tech
aliases:
  - PostgreSQL
  - Database
---

# Postgres

Source of truth for metadata, [[Node]] registry, and the ledger. Transactional integrity is non-negotiable for [[Fragment Placement]] and double-entry [[Ledger Entry]].

MVP: single primary + replicas. At 100k+ nodes the placement map shards by `file_id` / `fragment_id`, with a node-keyed inverted index for repair queries ([[Horizontal Scalability]]).

Least-privilege roles: only [[Ledger Service]] writes ledger tables; nothing can UPDATE [[Audit Log]].

Related: [[Map of Data Model]], [[Metadata Service]]
