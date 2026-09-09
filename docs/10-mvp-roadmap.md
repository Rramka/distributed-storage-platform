# 10 — MVP Roadmap

The build order follows one rule: **every milestone ends with something that runs end-to-end**, even if the slice is thin. Redundancy, healing, and billing layer onto a working store-and-retrieve core rather than being built in parallel and integrated late.

## Solo builder track (the current plan)

The original M0–M6 timeline below assumed a small team (2–4 engineers) over ~17 weeks. This repo is built by **one person at ~10–15 hours/week**, with AI agents doing the mechanical work. That is ~150 hours, not ~2,000. Scope is therefore cut to a single investable artifact:

> `dsp put Vacation.mp4` → kill 6 of 16 storage nodes on camera → file still downloads byte-identical → fleet self-heals back to 16/16 in minutes → and the platform's own database, dumped on screen, decrypts nothing.

That scene is **M0–M4**, minus anything that does not appear in it. Weeks in this section are calendar weeks for one person, not the original team sizing.

### What we build (M0–M4)

Metadata service, minimal API gateway (**API-key auth only**), node agent, tickets/receipts, client pipeline (Argon2id → AES-256-GCM → 16 MB chunks → Reed–Solomon 10+6), naive-then-scored scheduler, health monitor, repair service, chaos harness, one small fleet-visualizer web page.

### Explicitly deferred (do not start)

These remain specified in the docs for later phases. They are **out of the solo track**. Do not implement them until the durability demo is recorded.

| Deferred | Why | Spec |
|---|---|---|
| Ledger / billing service | Investors fund the durability claim, not the invoice screen | [09-billing-ledger.md](09-billing-ledger.md) |
| Customer, provider, and admin dashboards | Replaced by the fleet visualizer + CLI for the demo | M6 below |
| JWT / refresh tokens | API keys are enough for CLI and agents | [08-api.md](08-api.md) |
| Agent installers and self-update | Compose fleet is the demo; real installers come after | M6 below |
| Continuous rebalancing | Repair is the claim; rebalancing is an optimization | [06-scheduler-and-repair.md](06-scheduler-and-repair.md) |
| Reputation beyond a scalar | A single `reputation REAL` is enough for scoring | [06-scheduler-and-repair.md](06-scheduler-and-repair.md) |
| S3-compatible API | Native REST only | [08-api.md](08-api.md) |
| Real payments (Stripe / crypto / payouts) | Phase 2 | this document, long-term evolution |

### Twelve-week calendar

- **W1** — Repo skeleton, AI operating system (rules, skills, hooks, MCP), Compose (Postgres/Redis/NATS), CI. Exit: `docker compose up` gives green health checks.
- **W2–3** — Metadata service and gateway with API-key auth only. Exit: full metadata CRUD from the CLI.
- **W4–5** — Node agent, fragment PUT/GET with tickets and receipts, naive scheduler, 5-agent Compose fleet, Argon2id + AES-256-GCM (no Reed-Solomon). Exit: single-copy encrypted round trip.
- **W6–7** — Reed-Solomon 10+6, 16-way placement, plan/transfer/commit with resume. Exit: `dsp put` / `dsp get` byte-identical with 6 agents stopped.
- **W8–9** — Health monitor state machine, NATS events, repair service, chaos harness. Exit: the M4 test — kill 6 of 16, back to 16/16, downloads never fail.
- **W10** — Fleet visualizer web page and the scripted demo.
- **W11** — Soak run, security review, red-team DB-dump fixture, invariant CI.
- **W12** — Demo video, deck, data room, landing page with waitlist.

A fundraise track runs in parallel (~2 hours/week): market research, competitive comparison, deck, weekly investor update. See `business/` and the Cursor skills `market-research` / `investor-update` / `demo-capture`.

## Proposed repository layout (monorepo)

```text
distributed-storage-platform/
  docs/                    # this documentation set (canonical spec)
  AGENTS.md                # how AI agents work in this repo
  STATUS.md                # where the last session left off
  proto/                   # protobuf definitions (control API, tickets, receipts)
  cmd/
    gateway/  metadata/  scheduler/  healthmon/  repair/
    agent/                 # node agent binary
    dsp/                   # customer CLI
    # deferred: ledger/  admind/
  internal/
    pipeline/              # encryption, chunking, erasure coding (shared by CLI and repair)
    tickets/  receipts/  ca/   # crypto primitives
    store/                 # postgres + redis access layers
  web/
    visualizer/            # solo-track fleet visualizer (the demo UI)
    # deferred: dashboard/  admin/
  deploy/
    compose/               # local dev + simulated fleet
    migrations/
  business/                # research, investor updates, deck (not product code)
  .cursor/                 # rules, skills, hooks, project MCP
```

## Milestones

### M0 — Foundations (week 1–2)
Monorepo scaffolding, CI, protobuf toolchain, Postgres migrations from [03-data-model.md](03-data-model.md), Docker Compose dev environment (Postgres, Redis, NATS), private CA utility.
**Exit:** `docker compose up` brings up empty control-plane skeletons that pass health checks.

### M1 — Metadata service + auth (week 3–4)
Users, buckets, files, folders, rename, soft delete; **API keys only** on the solo track (JWT deferred); API Gateway with error format and rate limits from [08-api.md](08-api.md).
**Exit:** full metadata CRUD via the CLI against a running gateway — no bytes stored yet.

### M2 — Node agent + happy-path storage (week 5–7)
Agent with chunk store, registration ceremony (CA, mTLS), heartbeats to Redis liveness; fragment PUT/GET with tickets and receipts; naive scheduler (random online nodes, hard constraints only); CLI pipeline **with** Argon2id + streaming AES-256-GCM + 16 MB chunking, **without** erasure coding (single-copy fragments) so invariant 2 holds from the first stored byte.
**Exit:** `dsp put Vacation.mp4 && dsp get Vacation.mp4` round-trips through 5 local agent containers, byte-identical. Invariant 1 (≥12 healthy placements) does not apply until M3's 10+6.

### M3 — Erasure coding (week 8–9)
Reed–Solomon 10+6 on the existing encrypted chunks, plan/transfer/commit with 16-way placement, hash verification at every hop.
**Exit:** round-trip succeeds with any 6 of 16 agents stopped; plaintext never observable on any node's disk (asserted by test).

### M4 — Health, repair, self-healing (week 10–12)
Health Monitor state machine (`online → suspect → offline`), NATS events, Repair Service with priority queue, ciphertext reconstruction, flap handling; storage challenges with pre-computed challenge sets; scheduler upgraded to full scoring + weighted sampling ([06-scheduler-and-repair.md](06-scheduler-and-repair.md)).
**Exit:** kill 6 of 16 agents holding a file; within minutes all chunks are back to 16/16 healthy on survivors + fresh nodes, download works throughout. This milestone is the platform's core claim — it gets the most test investment.

### M5 — Ledger + billing (week 13–14) — deferred on the solo track
Usage event stream, double-entry ledger, hourly accruals, reliability multiplier, `GET /storage` and `GET /earnings`, quota enforcement ([09-billing-ledger.md](09-billing-ledger.md)).
**Exit:** a simulated month (accelerated clock) produces balanced books — every txn sums to zero, customer charges reconcile with provider earnings + platform margin.

### M6 — Dashboards + MVP hardening (week 15–17) — deferred on the solo track
Customer dashboard (files, usage), provider dashboard (nodes, earnings, registration codes), admin dashboard (network map, repair queue, quarantine actions); audit logging wired through; agent installers + self-update for the three OSes; load and chaos test pass (below).
**Exit:** the full MVP scope of [01-overview.md](01-overview.md), demoable end-to-end by a non-developer through the dashboards.

The original timeline assumed a small focused team (2–4 engineers). **The active plan is the Solo builder track above.** Treat M5–M6 weeks as relative sizing for after the raise demo.

## Testing strategy

### Simulated fleet (the primary tool)

Docker Compose (and later a k8s profile) runs the control plane plus **N agent containers** (50–200 on one dev machine — the agent is a small static binary). A fleet-controller test harness manipulates the fleet programmatically:

```text
harness verbs:
  kill <n> nodes           # abrupt loss (SIGKILL)
  drain <node>             # graceful retirement
  partition <nodes>        # iptables-based network cuts
  corrupt <node> <frac>    # flip bytes in stored fragments
  throttle <node> <mbps>   # degrade bandwidth
  flap <node> <period>     # reboot loops
  clock-skew, disk-full, slow-disk (via tc/cgroups)
```

### Test layers

| Layer | What | Examples |
|---|---|---|
| Unit | Pure logic | erasure code round-trips, ticket signing/expiry, scoring function, ledger invariants (property-based: entries always sum to zero) |
| Integration | Service pairs against real Postgres/Redis/NATS | upload commit transactionality, heartbeat expiry → state transition, challenge verify |
| End-to-end | CLI against full compose stack | put/get/delete/rename/list; resume after interrupt; wrong-passphrase fails cleanly |
| **Chaos** | Harness scenarios against the fleet | the M4 exit test; corrupt-fragment detection → repair; regional partition (all "EU" nodes cut) → availability maintained; mass failure repair within budget |
| Durability soak | Long-running (days) randomized kill/flap/corrupt schedule against thousands of files | zero chunks ever below 10 healthy; measured repair latency distribution |
| Security | Adversarial | node serving tampered bytes is caught and penalized; replayed/expired/cross-node tickets rejected; platform DB dump decrypts nothing (red-team fixture) |

### Continuously measured invariants (CI + soak)

1. Every committed chunk has ≥ 12 healthy placements (alert < 13).
2. No plaintext or unwrapped key ever appears in any node's `data_dir` or any control-plane store.
3. Ledger: every transaction sums to zero; meters reconcile with the placement map. **(Invariant exists; the ledger service is deferred on the solo track — skip until M5 starts.)**
4. Placement constraints (node/region/ASN/owner caps) hold for every chunk after any sequence of repairs.

## Long-term evolution

Each phase reuses the previous one's infrastructure — the storage network is the foundation, not a detour:

```mermaid
flowchart LR
    p1[Phase 1 MVP - native object storage] --> p2[Phase 2 - S3-compatible API + real payments]
    p2 --> p3[Phase 3 - file sync + sharing, block storage]
    p3 --> p4[Phase 4 - compute: containers, serverless]
    p4 --> p5[Phase 5 - GPU marketplace, VMs, K8s, AI inference]
```

- **Phase 2:** S3 gateway ([08-api.md](08-api.md)), Stripe deposits and provider payouts on the existing ledger ([09-billing-ledger.md](09-billing-ledger.md)), open provider enrollment, NAT relay traversal for agents, Merkle-based storage proofs.
- **Phase 3:** folder-sync client on the same pipeline; sharing via re-wrapped file keys; block-storage volumes (fixed-size chunk maps over the same fragment substrate); provider-set pricing marketplace.
- **Phase 4:** the node agent grows a sandboxed execution runtime (containers first); the scheduler generalizes from placing fragments to placing workloads — same scoring, reputation, and challenge machinery.
- **Phase 5:** GPU capacity as a first-class resource class, VM hosting on high-tier nodes, distributed K8s node pools, AI inference serving — community-powered cloud infrastructure with enterprise-grade reliability, built on the trust, reputation, and payment rails proven by storage.
