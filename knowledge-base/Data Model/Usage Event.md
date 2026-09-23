---
tags:
  - entity
  - billing
aliases:
  - usage_events
---

# Usage Event

Deterministic metering event archived by the [[Ledger Service]] as an audit trail. Billing posts from Postgres meters, not from these events.

Primary key is `usage:{kind}:{subject}:{window}` — egress appends the receipt nonce so two reports in the same hour are not collapsed.

Kinds: `storage` (committed bytes), `egress` (one event per GET receipt), `uptime` (hourly rollup). Payload is JSON. `consumed_at` is set after archive. Egress event bytes must reconcile with `download_receipts`.

See [[Map of Billing]], [[NATS JetStream]].
