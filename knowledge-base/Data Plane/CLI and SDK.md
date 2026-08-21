---
tags:
  - service
aliases:
  - Client
  - SDK
  - dsp CLI
---

# CLI and SDK

Customer client. **All transform work lives here**: [[Client-side Encryption]], chunking, hashing, [[Erasure Coding]]. The [[Control Plane]] only coordinates.

## Upload (product `dsp put`)

1. Derive [[Master Key]], generate [[File Key]], encrypt with [[AES-256-GCM]] (64 KiB streaming segments)
2. Split into 16 MB [[Chunk]]s, SHA-256 each
3. Reed–Solomon 10+6 → [[Fragment]]s
4. `POST /upload` with manifest → [[Placement Ticket]]s
5. Parallel PUT to [[Node Agent]]s
6. `POST /upload/{id}/commit` with [[Signed Receipt]]s

Resumable: re-POST the same manifest; already-confirmed fragments are omitted.

## Download (`dsp get`)

Plan → race the fastest 10 of 16 fragments per chunk → verify → decode → unwrap FK → decrypt. See [[Download Flow]].

Shared library `internal/pipeline` is also used by [[Repair Service]] for encode/decode (repair skips encrypt/decrypt).

Related: [[Customer REST API]], [[Map of Storage Pipeline]], [[Zero Knowledge]]
