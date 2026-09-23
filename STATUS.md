# Status

Last updated: 2026-09-23 (M5: meters + internal ledger)

## Current milestone

**M5 / W15–W17** (solo builder track) — **shipped in this session**

Honest meters, `USAGE_EVENTS`, `cmd/ledger` on `:8085`, `GET /storage` / `GET /earnings` / `GET /nodes/{id}/stats`, invariant 3 un-skipped, accelerated-month balanced books. Dashboards, JWT, S3, installers, rebalancing, real money, and M5b statements/quotas remain deferred. Screen recording is still a human step.

## Shipped this session

- Un-gated M5 in `AGENTS.md`, spec-fidelity, and `docs/10-mvp-roadmap.md` (W15–W17). Demo recording and landing publish stay open human tasks.
- Real `uptime_ratio`: per-node hourly Redis heartbeat counters; rollup is `min(1, received/expected)` over **all** nodes (`ExpectedHeartbeats` = 360 at the 10s interval). `store.NodeReliability` (30d uptime + audit pass rate) and `ListNodeStats`. Scheduler scoring uses non-constant uptime.
- `bytes_ingested` from verified PUT receipts inside commit (`BumpNodeBytes`).
- Node-signed GET receipts (`X-DSP-Receipt` bound to ticket nonce). `dsp get` POSTs `/v1/download/{id}/report`. Metadata verifies signatures, matches issued GET tickets, rejects duplicates, increments `bytes_served`.
- Soft delete marks versions `expired` and placements `expiring` in one transaction (billing stops). `internal/lifecycle` reaper in `cmd/repair` drives agent DELETE → `deleted`. Under-healthy / cap checks ignore expiring/deleted.
- Migration `000009_ledger_m5.sql`: `usage_events`, `download_tickets`/`download_receipts`, deferred `SUM(amount)=0` trigger, `ledger_entries(txn_id)` index.
- `USAGE_EVENTS` JetStream (`usage.storage` / `usage.egress` / `usage.uptime`) with deterministic `WithMsgID`.
- `internal/ledger`: integer µCRD, `EnsureAccount`, idempotent `PostTxn`, Accruer with injectable clock + explicit window, env rates with docs/09 defaults.
- `cmd/ledger` healthz on `:8085`, Compose service, nightly health curl.
- Gateway `GET /v1/storage`, `GET /v1/earnings`, `GET /v1/nodes/{id}/stats`.
- Invariant 3: `CheckLedger` (per-txn sum-zero and customer charges = provider earnings + platform margin) in `CheckAll`, harness, CI, invariant-check skill.
- M5 exit: `internal/ledger/month_test.go` (720 windows); `testing/quick` property that `PostTxn` rejects non-zero sums.

## Chaos / soak report

Unchanged from W13. Re-run on a fresh `make fleet-down && make up` after this change (new ledger service + migration 000009). Existing Compose volumes need `make migrate` (or a fresh volume) so `000009` applies.

## Blocked

- Screen recording still needs a human with `demo-script-w12.md`. Formspree `YOUR_FORM_ID` must be replaced before the landing page is public.
- Dashboards / JWT / S3 / installers / rebalancing / real money remain deferred. Statements and quota are M5b.
- This Compose volume may still have cap-violating leftover nodes; `make fleet-down && make up` before the next recorded demo.

## Next three tasks

1. Record the W12 screen capture (visualizer + terminal) from `demo-script-w12.md` after `make fleet-down && make up && make demo-ready`
2. Replace Formspree ID and publish `web/landing/`
3. Send `business/updates/2026-09-22.md` + the clip (or `deck/outline.md` if the clip slips)

## Notes for the next session

Run the `session-brief` skill first. Do not start dashboards, JWT, S3, installers, or M5b quota/statements.

Parked for a later spec-completeness slice: `audit_logs` writes, `POST /nodes/{id}/drain`, version history, recovery key, agent scrub, cert renewal/revocation.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Gateway `DEMO_MODE=1` serves `/demo/`. Agents register on **https://metadata:8444**. Ledger is **http://127.0.0.1:8085/healthz**. Landing page is **not** on the gateway.

If `fleet-seed` fails on an old volume: `make fleet-down && make up`. 32 MiB plaintext → 3 chunks; use 31 MiB for a 16/16 start on a probation fleet. Do not `docker compose start` the whole stack to revive killed agents — that waits on `fleet-seed` forever; use `docker start dsp-agentN-1`.

Vault: `knowledge-base/Billing/`, `Control Plane/Ledger Service.md`, `Data Model/Usage Event.md`, `Maps/Map of Roadmap.md`, `Home.md`, `Flows/File Deletion.md`.
