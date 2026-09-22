---
tags:
  - principle
aliases:
  - Self healing
  - Automatic repair
---

# Self-Healing

Failure is the normal case: hardware dies, home internet drops, users uninstall without warning.

Detect within seconds via [[Heartbeat Protocol]]. Reconstruct via [[Repair Loop]] on ciphertext. [[Customer]]s never notice node failures.

Thresholds on healthy [[Fragment Placement]]s per [[Chunk]]: idle at 16, backfill 13–15 at normal priority, repair at 12, urgent at ≤ 11, incident below 10. Target after any repair is 16/16 (M4 exit).

Related: [[Repair Service]], [[Health Monitor]], [[Erasure Coding]]
