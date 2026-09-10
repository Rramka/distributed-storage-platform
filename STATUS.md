# Status

Last updated: 2026-09-10 (M4 exit-critical path: health, NATS, repair, 24-agent fleet)

## Current milestone

**M4 — Health, repair, self-healing** (solo builder track) — **exit-critical path shipped**

Exit criteria: kill 6 of 16 agents holding a file; every chunk returns to 16/16 stored placements; `dsp get` succeeds throughout. Storage challenges and scheduler scoring are a follow-up slice.

## Shipped last session

- Health Monitor `online → suspect (30s) → offline (5m)` with NATS `NODE_EVENTS`
- Repair Service: mark lost, priority jobs on `REPAIR_JOBS`, ciphertext `ReconstructShards`, metadata-issued tickets only
- Flap path: hash-verified GET re-validates returning nodes (interim stand-in for challenges)
- Compose fleet `agent1..agent24` (12 owners × 2, 8 regions, 8 ASNs)
- `cmd/harness` + `make fleet-kill6` / `make chaos-m4`
- `internal/invariants` for ≥12 healthy placements and placement caps
- Compose repair dials agents via `REPAIR_ENDPOINT_HOST=host.docker.internal` (nodes advertise `127.0.0.1`)

## Chaos report

- Scenario: M4 kill-6-of-16
- Started: 2026-09-10T11:53:38Z
- Kill set: agent23, agent13, agent14, agent9, agent19, agent15
- Download during failure: pass (byte-identical)
- Repair latency: ~5 min heartbeat grace to `offline`, then reconstruction completed in seconds once the worker could reach host-published agent ports
- Final healthy placements: 16/16 stored (6 lost on killed nodes)
- Invariants: ≥12 healthy pass; caps pass on 16-way chunks; ledger skipped

## Blocked

- Nothing on the exit-critical path. Challenges and scoring not started.

## Next three tasks

1. M4 follow-up: storage challenges with pre-computed challenge sets
2. M4 follow-up: scheduler scoring + weighted sampling + reputation EWMA / probation
3. W10: fleet visualizer + scripted demo capture

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, or S3.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Repair never receives `TICKET_SIGNING_SEED`.

If `fleet-seed` fails on an old volume: `make fleet-down && make up`.

Vault: `knowledge-base/Control Plane/Health Monitor.md`, `Repair Service.md`, `Scheduler.md`, `Flows/Repair Loop.md`, `Flows/Heartbeat Protocol.md`, `Technology/NATS JetStream.md`, `Roadmap/Milestones.md`, `Maps/Map of Roadmap.md`, `Home.md`.
