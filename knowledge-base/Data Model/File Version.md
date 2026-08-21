---
tags:
  - entity
aliases:
  - File versions
  - Version
---

# File Version

One immutable snapshot of a [[File]]. Holds:

- `size_bytes` — original **plaintext** size (what [[Customer Pricing]] charges)
- `content_sha256` — hash of the **encrypted** file (end-to-end verify)
- `encryption_meta` — algo, **wrapped** [[File Key]], KDF params (see [[Client-side Encryption]])
- `chunk_size`, `ec_data_shards` (10), `ec_parity_shards` (6)
- status: `pending` → `committed` → `superseded` / `expired`

Key rotation of the passphrase re-wraps FKs here (metadata-sized). Rotating an FK requires re-encrypting the file.

Crash during [[Upload Flow]] leaves a `pending` version; a janitor expires it and GC runs on nodes.
