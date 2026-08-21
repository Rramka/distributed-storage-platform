---
tags:
  - entity
  - security
aliases:
  - API keys
---

# API Key

Programmatic credential for the [[Customer REST API]].

- 256-bit secret, shown **once**, stored as SHA-256 hash
- Scoped (`read` / `write`), expirable, revocable
- Header: `X-Api-Key`

Object IDs are UUIDs but are **not** relied on as secrets — every metadata object is owner-checked.

Related: [[User]], [[API Gateway]]
