# Status

Last updated: 2026-09-23 (W14: harden the durability claim)

## Current milestone

**W14** (solo builder track) — **shipped**

Registration is server-authenticated TLS, one code yields one node, DELETE tickets are single-use, and `make demo-ready` / `harness m4` assert `dsp get` while six holders are down. M0–M4 remain the investable artifact. Screen recording is still a human step.

## Shipped this session

- Metadata node listener on `:8444` (TLS 1.3, `EnsureServiceCert("metadata")`). Agents register at `https://metadata:8444` with a CA-pinned client and a timeout. Plaintext `REGISTER_URL` is refused. Compose volume `ca_data` is writable so the service cert can be issued.
- `store.RegisterNode` claims the code, inserts the node, and binds the code in one `FOR UPDATE` transaction. Concurrent uses of one code produce one node row.
- DELETE tickets call `ConsumeNonce`. Failed API-key auth is IP-rate-limited (`AllowAuth`) without double-limiting valid keys.
- `PlanUpload` aborts a new pending version if planning fails after `BeginUpload`. Download plans prefer `online` placements and keep a floor of 10 shards per chunk. Receipt matching is one-to-one via `byFrag`.
- `dsp put` reports fragment PUT errors and uses a cancellable request context.
- `make demo-ready` waits for 24 online agents, copies `api.key`, uploads a 31 MiB fixture, writes `.local/demo.env`, and fails fast on leftover non-compose endpoints. `harness m4` runs `DSP_GET_CMD` while the kill set is down.
- Docs and vault match the HTTP/JSON + binary-ticket solo track (`docs/05`, `07`, `08`, `AGENTS.md`, `proto/README.md`). Root `harness` / `fleet-seed` binaries are untracked.

## Chaos / soak report

Unchanged from W13. Re-run on a fresh `make fleet-down && make up` after this change (registration URL and TLS listener).

## Blocked

- Screen recording still needs a human with `demo-script-w12.md`. Formspree `YOUR_FORM_ID` must be replaced before the landing page is public.
- Ledger / dashboards / JWT / S3 remain deferred.
- This Compose volume may still have cap-violating leftover nodes; `make fleet-down && make up` before the next recorded demo.

## Next three tasks

1. Record the W12 screen capture (visualizer + terminal) from `demo-script-w12.md` after `make fleet-down && make up && make demo-ready`
2. Replace Formspree ID and publish `web/landing/`
3. Send `business/updates/2026-09-22.md` + the clip (or `deck/outline.md` if the clip slips)

## Notes for the next session

Run the `session-brief` skill first. Do not start ledger, dashboards, JWT, or S3.

Parked for a later spec-completeness slice: deletion lifecycle (`expiring`/`deleted`), `audit_logs` writes, `GET /nodes/{id}/stats`, `POST /nodes/{id}/drain`, version history, 30-day `uptime_ratio`, recovery key, agent scrub.

Compose is `deploy/compose/docker-compose.yml`. After `make up`, `make export-ca` (`DSP_CA_FILE=.local/ca.crt`). Host Postgres is on **5433**. Gateway `DEMO_MODE=1` serves `/demo/`. Agents register on **https://metadata:8444**. Landing page is **not** on the gateway.

If `fleet-seed` fails on an old volume: `make fleet-down && make up`. 32 MiB plaintext → 3 chunks; use 31 MiB for a 16/16 start on a probation fleet. Do not `docker compose start` the whole stack to revive killed agents — that waits on `fleet-seed` forever; use `docker start dsp-agentN-1`.

Vault: `knowledge-base/Roadmap/Milestones.md`, `Maps/Map of Roadmap.md`, `Home.md`, `Flows/Node Registration.md`, `Data Plane/Node Agent.md`, `Technology/gRPC.md`, `Maps/Map of Security.md`.
