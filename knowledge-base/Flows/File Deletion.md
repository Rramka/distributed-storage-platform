---
tags:
  - flow
aliases:
  - Delete file
  - GC
---

# File Deletion

1. `DELETE /file/{id}` sets [[File]].`deleted_at` (soft)
2. [[Fragment Placement]]s flip to `expiring`
3. [[Node Agent]]s learn expiry on the next [[Heartbeat Protocol]] sync (and via DELETE)
4. Local GC deletes bytes; `ConfirmDeletions` flips placements to `deleted`
5. Janitor hard-deletes metadata after the retention window

Customers can always delete even at [[Quota]] floor — data is never held hostage over billing.

Abandoned [[Upload Flow]]s use the same expiry path for orphaned pending fragments.

Related: [[Metadata Service]], [[Node Agent]]
