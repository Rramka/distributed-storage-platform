---
tags:
  - security
aliases:
  - Receipt
  - Node receipt
---

# Signed Receipt

[[Node Agent]] signs `(fragment_id, sha256, size, stored_at)` with its certificate key after crash-safe persist.

[[Metadata Service]] verifies receipts at [[Upload Flow]] commit — a client cannot claim placements that never happened, and a node cannot later deny having accepted a fragment.

Receipts are idempotent; re-submitting commit is safe. The client treats an upload as durable **only** after the commit response.

Also used by [[Repair Loop]] when writing reconstructed fragments.
