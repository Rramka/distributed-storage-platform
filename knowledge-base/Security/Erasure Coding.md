---
tags:
  - principle
aliases:
  - RS
  - 10+6
  - Reed Solomon coding
---

# Erasure Coding

Each [[Chunk]] → **10 data + 6 parity** [[Fragment]]s via [[Reed-Solomon]] (`klauspost/reedsolomon`). Any 10 of 16 reconstruct the chunk.

## Why not replication

To survive loss of any 6 nodes: 7× replication = 700% overhead. 10+6 = **160%**. Same tolerance at less than a quarter of the cost — decisive when [[Storage Provider]]s are paid per stored GB. [[Customer Pricing]] charges **logical** bytes; the 1.6× footprint is the platform's COGS.

## Durability (no repair)

Pessimistic 10% node unavailability, independent placements:

P(≥ 7 of 16 gone) ≈ 1.4×10⁻⁵ — *temporary* unavailability. Actual durability is "lose 7 nodes faster than [[Repair Loop]] completes," i.e. many nines, given [[Placement Constraints]].

Shared by [[CLI and SDK]] (upload/download) and [[Repair Service]] (reconstruct without decrypt).
