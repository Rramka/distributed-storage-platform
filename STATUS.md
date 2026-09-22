# Status

Last updated: 2026-09-21 (W11: soak, security review, red-team CI, invariant job)

## Current milestone

**W11** (solo builder track) — **shipped**

Soak, hermetic invariant + red-team CI, nightly Compose fleet job, `harness corrupt`, and a security review of the crypto surface. M4 exit (kill 6 of 16 → 16/16) was already in tree.

## Shipped this session

- `harness corrupt <agent> [--frac]`: flips one byte per 4 KiB block in on-disk `.frag` files (bbolt hash left intact) so the next storage challenge is a hit
- Hermetic `TestCorruptFragmentDetectedAndRepaired`: challenge mismatch + RS reconstruct still byte-identical
- `harness soak --duration --seed` / `make soak`: randomized kill/flap/corrupt, max 6 down, floor 10 healthy, JSON + chaos report. Throttles to flap-only while any tracked chunk is below 13
- `scripts/apply-migrations.sh` + `make migrate-url`; `DSP_REQUIRE_PG=1` makes missing Postgres fail instead of skip
- Fixture seeder `store.SeedCommittedFleet` and table-driven durability/cap tests (11 healthy, duplicate node, owner cap)
- Red-team `pg_dump` fixture with planted-needle positive control
- CI `invariants` job (Postgres 16 + Redis 7 service containers) and `.github/workflows/nightly.yml` (Compose fleet + `chaos-m4`)
- Security review: `business/updates/security-review-2026-09-20.md` — no medium+ findings

## Chaos / soak report

2h soak (`seed=21`) on 2026-09-21: 34 rounds, 34 flap / 0 kill / 0 corrupt (kill withheld while min healthy was already ≤12). **Min healthy observed: 10/16** (floor held). Repair did not return to 16/16: three committed chunks already violate placement caps, so the scheduler cannot place replacements. Pre-existing: `invariants.caps` on chunk `0e56dba8-8334-4a20-a0d1-f1d7c5e74470`. Metrics: `business/updates/soak-2026-09-21.json`.

`partition` and `throttle` remain unimplemented.

## Blocked

- Live Compose volume has placement-cap violations from earlier repair/test fixtures. A clean `make fleet-down && make up` + `dsp put` would let soak exercise kill/corrupt again. Ledger / dashboards / JWT / S3 remain deferred.

## Next three tasks

1. W12: demo video (`make demo` after a clean fleet + `dsp put`)
2. W12: deck + data room (research stubs in `business/` still need the `market-research` skill)
3. W12: landing page with waitlist

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, or S3.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. `make soak DURATION=2h`. `make invariants`. Gateway `DEMO_MODE=1` serves `/demo/`.

If `fleet-seed` fails on an old volume: `make fleet-down && make up`.

Vault: `knowledge-base/Control Plane/Health Monitor.md`, `Flows/Repair Loop.md`, `Security/Storage Challenge.md`, `Roadmap/Milestones.md`, `Maps/Map of Roadmap.md`, `Home.md`.
