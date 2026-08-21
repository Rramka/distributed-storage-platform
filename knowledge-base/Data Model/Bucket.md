---
tags:
  - entity
aliases:
  - Buckets
---

# Bucket

Per-[[User]] named container. Unique `(owner_id, name)`. Maps 1:1 to future S3 buckets ([[Long-term Vision]]).

Contains [[File]] rows (files and folders). Delete requires empty.

Created via `POST /buckets` on the [[Customer REST API]].
