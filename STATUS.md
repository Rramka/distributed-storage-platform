# Status

Last updated: 2026-09-22 (W13: chaos completeness, honest CI, property coverage)

## Current milestone

**W13** (solo builder track) — **shipped**

`harness partition`, `throttle` retired, CI that cannot skip Postgres silently, property/fuzz tests. M0–M4 remain the investable artifact. Screen recording is still a human step.

## Shipped this session

- `harness partition <agent...> | --region <r> [--for 8m] [--heal]`: `docker compose pause`/`unpause` (cgroup freezer, not iptables). SIGINT-safe deferred unpause. Floor while partitioned: ≥ 13/16 (16 − region cap).
- `NodesByRegion` on the store; non-compose leftover endpoints (stale `19006`–`19008` rows) are skipped.
- Soak rotation includes `partition`; paused agents count against the max-6-down cap and are unpaused on soak exit. `make chaos-partition`.
- `throttle` retired: `bandwidth_factor` is a documented 1.0 constant, so a `tc` cap would not change placement. Phantom `clock-skew`/`disk-full`/`slow-disk` dropped from the verb list.
- CI `test` job is `go test ./... -short -count=1`. `invariants` job runs `go test ./...` with `DSP_REQUIRE_PG=1`. Nightly: restart killed agents via `docker start`, regional partition, 20-minute soak; timeout 120m.
- `testing/quick` properties: erasure any-10-of-16, encrypt/decrypt across the 16 MiB chunk boundary, receipts mutated-field rejection. Fuzz: `FuzzChunkDecrypt`, `FuzzReceiptParse`.
- Vault: deleted empty `Untitled.md`; heartbeat transport corrected in `Technology/gRPC.md`; JWT-deferred caveat on `Maps/Map of Security.md`; stale M4 deferral dropped from `Data Plane/Node Agent.md`.

## Chaos / soak report

W13 partition (2026-09-22): `--region eu-west --for 15s` paused agent7/8/9, min healthy stayed 16/16 (inside the 5 min offline grace), unpause left none frozen.

Partition-heal finding: lost placements do **not** silently become stored on unpause. Healthmon `offline→online` publishes `node.online`; the repair worker's flap path restores copies that were not reconstructed elsewhere and expires surplus copies (`RestorePlacements` / `ExpirePlacements`). That is (a), not a 17th stored placement. A full 8-minute cycle past `HEALTH_OFFLINE_AFTER` was not measured on this volume: `make invariants` already fails with 2 cap-violating chunks from leftover `127.0.0.1:1900x` nodes. Nightly on a fresh `make up` is the honest check.

W12 demo (2026-09-22): kill 6 every-chunk holders, `dsp get` byte-identical while down, heal to 16/16 in 296s. Visualizer live at `/demo/`.

W11 soak unchanged: 2h seed=21, min healthy 10/16.

## Blocked

- Screen recording still needs a human with `demo-script-w12.md`. Formspree `YOUR_FORM_ID` must be replaced before the landing page is public.
- Ledger / dashboards / JWT / S3 remain deferred (M4 demo recording gate still open).
- This Compose volume still has cap-violating leftover nodes; `make fleet-down && make up` before the next recorded demo.

## Next three tasks

1. Record the W12 screen capture (visualizer + terminal) from `demo-script-w12.md`
2. Replace Formspree ID and publish `web/landing/`
3. Send `business/updates/2026-09-22.md` + the clip (or `deck/outline.md` if the clip slips)

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, or S3.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Gateway `DEMO_MODE=1` serves `/demo/`. Landing page is **not** on the gateway.

If `fleet-seed` fails on an old volume: `make fleet-down && make up`. 32 MiB plaintext → 3 chunks; use 31 MiB for a 16/16 start on a probation fleet. Do not `docker compose start` the whole stack to revive killed agents — that waits on `fleet-seed` forever; use `docker start dsp-agentN-1`.

Vault: `knowledge-base/Roadmap/Milestones.md`, `Maps/Map of Roadmap.md`, `Home.md`, `Flows/Repair Loop.md`, `Data Plane/Node Agent.md`, `Technology/gRPC.md`, `Maps/Map of Security.md`.
