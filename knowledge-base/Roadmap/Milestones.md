---
tags:
  - roadmap
aliases:
  - Build plan
  - M0 M1 M2
---

# Milestones

Every milestone ends with something that **runs end-to-end**.

**Active plan:** [[Map of Roadmap]] solo builder track — ship M0–M4 as the raise demo. M5–M6 are specified but deferred.

| ID | Slice | Exit | Solo track |
|---|---|---|---|
| **M0** Foundations | Monorepo, CI, proto, migrations, Compose ([[Postgres]], [[Redis]], [[NATS JetStream]]), CA | Empty skeletons pass health checks | yes |
| **M1** Metadata + auth | [[User]], [[Bucket]], [[File]], [[API Key]], [[API Gateway]] (JWT deferred) | Metadata CRUD via CLI — no bytes yet | yes |
| **M2** Agent + happy path | [[Node Agent]], [[mTLS]], tickets, naive [[Scheduler]], CLI **without** EC | `dsp put && dsp get` through 5 local agents, byte-identical | yes |
| **M3** Encryption + EC | Full [[Map of Storage Pipeline]] | Round-trip with any 6 of 16 agents down; no plaintext on disks | yes |
| **M4** Health + repair | [[Health Monitor]] state machine, [[Repair Service]], [[Storage Challenge]], full scoring | Kill 6 of 16; minutes later 16/16 healthy; download works throughout | yes — the demo |
| **M5** Ledger | Usage stream, double-entry, `R`, [[Quota]] | Accelerated "month" produces balanced books | deferred |
| **M6** Dashboards + harden | [[Web Dashboard]], [[Admin Dashboard]], installers, chaos pass | Full [[MVP Scope]], demoable by a non-developer | deferred |

M4 is the core claim — heaviest test investment. Chaos harness: kill, drain, partition, corrupt, throttle, flap.

Related: [[Map of Roadmap]], [[Go]]
