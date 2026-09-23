# Status

Last updated: 2026-09-23 (M5.1 honest books + pilot certs/scrub + M5b)

## Current milestone

**M5 exit recorded (W18)** plus **W19–W21** (pilot readiness + M5b). Dashboards, JWT, S3, installers, rebalancing, and real money remain deferred. Screen recording is still a human step.

## Shipped this session

- Spec/vault: `docs/09` meters are the billing source; `USAGE_EVENTS` is the audit trail; example rate matches 4109 µCRD/GB-h; `reputation_factor = 0.8 + 0.4 * reputation`.
- `000010_ledger_txns.sql`: `ledger_txns` PK + `ledger_watermarks`. `PostTxn` inserts atomically (`ON CONFLICT DO NOTHING`). Concurrent post test.
- Accruer watermark + bounded catch-up; first error logs and retries instead of exiting.
- Reliability premium reachable (clean node = 12000 bp).
- Egress JetStream IDs include the receipt nonce; `CheckUsageReconcile` in `CheckLedger`.
- `TypeAdjustment` / `PostAdjustment` (reason required). `GET /v1/earnings?from=&to=` by node, window, and type.
- Integration: real-meter `AccrueWindow`; gateway `/v1/storage` and `/v1/earnings` auth scope.
- `000011_pilot_m5b.sql`: `nodes.revoked_at`, `statements`, audit UPDATE/DELETE revoked from PUBLIC.
- `POST /internal/nodes/renew` (same public key); renew binds to the presented leaf fingerprint so a stale cert cannot hijack rotation; heartbeat, ticket issuance, and download placement reject revoked/quarantined/expired nodes; agent renews with < 7 days left.
- Heartbeat `messages` applied (`renew`, `scrub`). Agent scrub hashes fragments, deletes corrupt, self-reports → `lost` + repair + `SignalHonestSelfReport`.
- `audit_logs` writes: API keys, register/renew, delete, ticket batches, scrub, ledger adjustments.
- M5b: `CloseCycle` statements; `LEDGER_CREDIT_FLOOR_UCRD` (default 0) → `403 quota_exceeded` on `POST /upload`; GET/DELETE remain open.

## Chaos / soak report

Unchanged from W13. Re-run on a fresh `make fleet-down && make up` after migrations `000010` and `000011`. Existing volumes need `make migrate`.

## Blocked

- Screen recording still needs a human with `demo-script-w12.md`. Formspree `YOUR_FORM_ID` must be replaced before the landing page is public.
- Dashboards / JWT / S3 / installers / rebalancing / real money remain deferred.
- This Compose volume may still have cap-violating leftover nodes; `make fleet-down && make up` before the next recorded demo.

## Next three tasks

1. Record the W12 screen capture (visualizer + terminal) from `demo-script-w12.md` after `make fleet-down && make up && make demo-ready`
2. Replace Formspree ID and publish `web/landing/`
3. Send `business/updates/2026-09-22.md` + the clip (or `deck/outline.md` if the clip slips)

## Notes for the next session

Run the `session-brief` skill first. Do not start dashboards, JWT, S3, or installers.

Parked: `POST /nodes/{id}/drain` with migrate, version history, recovery key, passphrase re-wrap.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Gateway `DEMO_MODE=1` serves `/demo/`. Agents register on **https://metadata:8444**. Ledger is **http://127.0.0.1:8085/healthz**. Landing page is **not** on the gateway.

If `fleet-seed` fails on an old volume: `make fleet-down && make up`. 32 MiB plaintext → 3 chunks; use 31 MiB for a 16/16 start on a probation fleet. Do not `docker compose start` the whole stack to revive killed agents — that waits on `fleet-seed` forever; use `docker start dsp-agentN-1`.

Vault: `knowledge-base/Billing/`, `Control Plane/Ledger Service.md`, `Data Model/Usage Event.md`, `Data Model/Audit Log.md`, `Data Plane/Node Agent.md`, `Maps/Map of Roadmap.md`, `Home.md`.
