---
tags:
  - service
aliases:
  - Metadata
  - The brain
---

# Metadata Service

Owns the source of truth for everything except file bytes: [[User]], [[Bucket]], [[File]], [[File Version]], [[Chunk]], [[Fragment]], [[Fragment Placement]]. Schema: [[Map of Data Model]].

## Responsibilities

- **Upload planning:** given a manifest, asks the [[Scheduler]] for target nodes, issues signed time-limited [[Placement Ticket]]s
- **Download planning:** returns the fragment map plus [[Retrieval Ticket]]s and wrapped [[File Key]]
- **Commit:** a file becomes visible only after enough fragments are confirmed via [[Signed Receipt]]s (commit threshold: 14 of 16)
- All writes through [[Postgres]] with strict transactions; reads are cache-friendly

## Why it cannot decrypt

It stores `encryption_meta` on [[File Version]] — the *wrapped* FK. Unwrapping needs the [[Master Key]], which never leaves the [[CLI and SDK]].

Related: [[Upload Flow]], [[Download Flow]], [[File Deletion]]
