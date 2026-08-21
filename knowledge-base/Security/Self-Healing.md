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

Thresholds on healthy [[Fragment Placement]]s per [[Chunk]]: idle at ≥ 13, repair at 12, urgent at ≤ 11, incident below 10.

Related: [[Repair Service]], [[Health Monitor]], [[Erasure Coding]]
