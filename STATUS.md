# Status

Last updated: 2026-09-11 (M4 follow-up + W10: challenges, scoring, visualizer, demo capture)

## Current milestone

**M4 follow-up + W10** (solo builder track) — **shipped**

Exit-critical M4 (kill 6 of 16 → 16/16) was already in tree. This session added storage challenges, scheduler scoring, the fleet visualizer, and `make demo`.

## Shipped last session

- Storage challenges: `fragment_challenges` table; Health Monitor mints sets via ticketed GET (never stores bytes); `GET /challenge` on the agent with metadata-issued `challenge` tickets; failed audit marks one placement lost and enqueues repair
- Flap re-validation uses a challenge (GET only to seed a new set)
- Scheduler scoring + weighted sampling (spec weights); reputation EWMA; 14-day probation quota; heartbeat metrics in Redis `node:metrics:{id}` + hourly `node_stats`
- Fleet visualizer at gateway `GET /demo/` when `DEMO_MODE=1`, polling `GET /v1/demo/fleet`
- `make demo` / `harness demo` writes `business/updates/demo-metrics-YYYY-MM-DD.json`; `make invariants`; zero-knowledge scan in `internal/invariants`

## Chaos report

Unchanged from 2026-09-10 kill-6-of-16 (see previous STATUS). Re-record with `make demo` after `make up` + `dsp put`.

## Blocked

- Nothing on the solo track through W10. Ledger / dashboards / JWT / S3 remain deferred.

## Next three tasks

1. W11: soak run, security review, red-team DB-dump fixture in CI, invariant job
2. W12: demo video, deck, data room, landing page with waitlist
3. After the raise demo: M5 ledger (only then)

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, or S3.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Repair and Health Monitor never receive `TICKET_SIGNING_SEED`. Gateway `DEMO_MODE=1` serves `/demo/`. Health Monitor uses `HEALTHMON_ENDPOINT_HOST=host.docker.internal` and `CHALLENGE_INTERVAL=30s` in Compose.

Visualizer: http://127.0.0.1:8080/demo/

`make demo` records honest `download_ok_during_failure` only if `DSP_GET_CMD` is set.

If `fleet-seed` fails on an old volume: `make fleet-down && make up` (needed once for migrations `000007` / `000008`).

Vault: `knowledge-base/Control Plane/Health Monitor.md`, `Repair Service.md`, `Scheduler.md`, `API Gateway.md`, `Flows/Repair Loop.md`, `Security/Storage Challenge.md`, `Roadmap/Milestones.md`, `Maps/Map of Roadmap.md`, `Maps/Map of Data Model.md`, `Home.md`.
