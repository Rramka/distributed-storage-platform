---
tags:
  - entity
aliases:
  - Chunks
---

# Chunk

Encrypted slice of a [[File Version]] (default **16 MB**, last chunk may be short). Positioned by `seq`.

Has its own SHA-256 (ciphertext). Independently [[Erasure Coding|erasure-coded]] into 16 [[Fragment]]s.

Why 16 MB: uniform scheduler units, parallel upload, ~1.6 MB fragments that suit home uplinks, metadata cost vs repair cost trade-off.

A chunk is **readable** if any 10 of 16 fragments are healthy. The [[Repair Service]] acts at 12 healthy so repair is calm, not panicked.

Related: [[Map of Storage Pipeline]], [[Placement Constraints]]
