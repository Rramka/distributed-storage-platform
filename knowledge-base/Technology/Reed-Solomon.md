---
tags:
  - tech
aliases:
  - klauspost/reedsolomon
  - RS codec
---

# Reed-Solomon

Library: `klauspost/reedsolomon` (SIMD-accelerated [[Go]]). Used by [[CLI and SDK]] and [[Repair Service]] for [[Erasure Coding]] (default 10+6).

Repair decodes/re-encodes **ciphertext shards** — the codec never sees plaintext.
