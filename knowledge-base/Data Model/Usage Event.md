---
tags:
  - entity
  - billing
aliases:
  - usage_events
---

# Usage Event

Deterministic metering event consumed by the [[Ledger Service]]. Primary key is `usage:{kind}:{subject}:{window}` so NATS replay cannot double-bill.

Kinds: `storage` (committed bytes), `egress` (download report), `uptime` (hourly rollup). Payload is JSON. `consumed_at` is set after the ledger records it.

See [[Map of Billing]], [[NATS JetStream]].
