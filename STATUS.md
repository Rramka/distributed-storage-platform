# Status

Last updated: 2026-09-08 (M1 metadata CRUD + API-key auth)

## Current milestone

**M1 — Metadata service + auth** (solo builder track, weeks 2–3 of 12)

Exit criteria: full metadata CRUD via the CLI against a running gateway — no bytes stored yet.

## Shipped last session

- `internal/apierr` public error envelope (`code`, `message`, `request_id`)
- `internal/auth` argon2id passwords + `dsp_` API keys (SHA-256 at rest, secret shown once)
- `internal/store` Postgres access for users, API keys, buckets, files/folders
- `internal/metadata` internal HTTP/JSON API; `internal/gateway` public `/v1` with API-key auth, Basic minting, Redis token buckets
- `cmd/dsp` metadata commands: register, api-keys, buckets, folders, ls, mv, rm
- Migration `000002_api_key_lookup.sql`; Compose wires `POSTGRES_URL` / `REDIS_URL` / `METADATA_URL`
- Solo-track notes in `docs/02-system-architecture.md` (HTTP/JSON not gRPC) and `docs/08-api.md` (JWT deferred)
- Vault: `knowledge-base/Control Plane/API Gateway.md`, `Metadata Service.md`, `Flows/Customer REST API.md`, `Data Model/User.md`, `API Key.md`, `Home.md`

## Blocked

- Nothing. Next work is M2 (node agent + single-copy round trip).

## Next three tasks

1. M2: node agent chunk store, registration ceremony (CA, mTLS), Redis heartbeats
2. Fragment PUT/GET with tickets and receipts; naive scheduler
3. CLI `dsp put` / `dsp get` through 5 local agent containers (no erasure coding yet)

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, or JWT. Compose is `deploy/compose/docker-compose.yml` (`make up` / `make down`). After `make up`, `dsp register` / `dsp api-keys create` / bucket+folder CRUD should work against `http://127.0.0.1:8080`. Host Postgres is published on **5433** (Mac already uses 5432): `postgres://dsp:dsp@127.0.0.1:5433/dsp?sslmode=disable`.
