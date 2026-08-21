---
tags:
  - security
  - principle
aliases:
  - Diversity constraints
  - Hard placement rules
---

# Placement Constraints

Hard rules the [[Scheduler]] checks **before** scoring, for the 16 [[Fragment]]s of one [[Chunk]]:

1. **One fragment per [[Node]]**
2. **Region cap:** ≤ 3 in any geographic region
3. **ASN cap:** ≤ 3 on any one ISP
4. **Owner cap:** ≤ 2 on nodes of the same [[Storage Provider]] account
5. Node must be `online`, above free-space floor, inside probation quota

A regional outage, ISP failure, or one provider rage-quit can each cost at most 3 fragments — inside the 6-fragment [[Erasure Coding]] tolerance. This keeps the independence assumption in the durability math honest.

Diversity **drift** after many repairs is corrected by weekly rebalance (low-priority [[Repair Loop]] jobs).

Continuously asserted in CI after any chaos sequence ([[Milestones]] testing).
