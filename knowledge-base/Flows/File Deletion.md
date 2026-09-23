---
tags:
  - flow
aliases:
  - Delete file
  - GC
---

# File Deletion

1. `DELETE /file/{id}` sets [[File]].`deleted_at` (soft) and [[File Version]]s to `expired`
2. [[Fragment Placement]]s flip to `expiring` in the same transaction — billing GB-hours stop here
3. The lifecycle reaper (inside [[Repair Service]]) issues DELETE tickets and calls the [[Node Agent]] `DELETE /fragments/{id}`
4. Successful deletion flips the placement to `deleted`
5. `expiring`/`deleted` placements are excluded from the ≥12 healthy invariant

Customers can always delete even at [[Quota]] floor — data is never held hostage over billing.

Abandoned [[Upload Flow]]s use the same expiry path for orphaned pending fragments.

Related: [[Metadata Service]], [[Node Agent]], [[Usage Event]]
