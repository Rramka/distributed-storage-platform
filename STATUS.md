# Status

Last updated: 2026-09-22 (W12: demo capture, deck + data room, landing page)

## Current milestone

**W12** (solo builder track) — **shipped** (harness + artifacts; screen recording is a human step)

Demo metrics, sourced market notes, deck outline, data-room index, and a static waitlist page. M0–M4 remain the investable artifact.

## Shipped this session

- Clean Compose fleet (`make fleet-down && make up`); 31 MiB `Vacation.mp4` (two ciphertext chunks; 32 MiB plaintext overflows to a third chunk and exhausts the 24×2 probation quota)
- `make invariants` clean on that file before the take
- `harness demo`: clock starts after kill; waits for health to dip then return to 16/16; `pg_dump` no longer fails because stdout was already attached
- Kill set is nodes that hold **every** chunk, so each chunk drops below the repair threshold
- Repair backfill to 16/16: skip failed PUTs, mark those pendings lost, queue 13–15 at normal priority (docs/06 updated)
- `LatestNodeStats` coalesces NULL audit-only rollup columns (Place was 500 after the first challenge bump)
- `business/updates/demo-metrics-2026-09-22.json`: download during failure **true**, **16/16**, dump **true**, **296s** from kill (includes 5 min offline grace)
- Market research, deck outline, data room, investor note, `web/landing/` Formspree waitlist (no gateway route, no table)

## Chaos / soak report

W12 demo (2026-09-22): kill 6 every-chunk holders, `dsp get` byte-identical while down, heal to 16/16 in 296s. Visualizer live at `/demo/`.

W11 soak unchanged: 2h seed=21, min healthy 10/16. `partition` and `throttle` remain unimplemented.

## Blocked

- Screen recording still needs a human with `demo-script-w12.md`. Formspree `YOUR_FORM_ID` must be replaced before the landing page is public.
- Ledger / dashboards / JWT / S3 remain deferred.

## Next three tasks

1. Record the W12 screen capture (visualizer + terminal) from `demo-script-w12.md`
2. Replace Formspree ID and publish `web/landing/`
3. Send `business/updates/2026-09-22.md` + the clip (or `deck/outline.md` if the clip slips)

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, or S3.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Gateway `DEMO_MODE=1` serves `/demo/`. Landing page is **not** on the gateway.

If `fleet-seed` fails on an old volume: `make fleet-down && make up`. 32 MiB plaintext → 3 chunks; use 31 MiB for a 16/16 start on a probation fleet.

Vault: `knowledge-base/Roadmap/Milestones.md`, `Maps/Map of Roadmap.md`, `Home.md`, `Flows/Repair Loop.md`, `Control Plane/Repair Service.md`, `Security/Self-Healing.md`.
