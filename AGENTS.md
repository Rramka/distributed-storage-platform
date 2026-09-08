# Agent operating instructions

You are working on a **decentralized cloud storage platform**. Unused disks on ordinary computers become a secure, globally distributed object store. The control plane orchestrates; it never stores file contents and can never decrypt them.

This repo is built by **one person** with AI agents. Read this file, then [`STATUS.md`](STATUS.md), then the spec section you are implementing. Do not invent product behavior.

## Canonical sources

| Source | Role |
|---|---|
| [`docs/`](docs/) | Canonical spec. Code must implement what is written there. |
| [`docs/10-mvp-roadmap.md`](docs/10-mvp-roadmap.md) § Solo builder track | Active scope: M0–M4 only. |
| [`knowledge-base/`](knowledge-base/) | Navigable index of the same design (Obsidian). Update it when behavior changes (`kb-sync` skill). |
| [`STATUS.md`](STATUS.md) | Where the last session left off. Update it before you stop. |

If implementation must diverge from a spec, **change the spec in the same change**. Cite the doc section in the commit or PR body.

## Stack

- **Language:** Go 1.26. Module path: `github.com/Rramka/distributed-storage-platform`
- **Public API:** REST/JSON (gateway)
- **Internal RPC:** gRPC + protobuf (when proto exists)
- **State:** Postgres (metadata), Redis (liveness/TTL), NATS JetStream (events, repair queue)
- **Crypto / coding:** AES-256-GCM streaming, Argon2id, Reed–Solomon 10+6 (`klauspost/reedsolomon`)
- **Node identity:** mTLS with a private CA

## Dev commands

```text
make up          # Postgres, Redis, NATS + control-plane skeletons
make down        # stop compose (keeps volumes)
make ps          # compose status
make logs        # follow compose logs
make test        # go test ./...
make test-short  # go test ./... -short
make vet         # gofmt check + go vet
make migrate     # apply SQL in deploy/migrations/ (idempotent on fresh volume)
```

Health checks after `make up`:

- gateway `http://127.0.0.1:8080/healthz`
- metadata `http://127.0.0.1:8081/healthz`
- scheduler `http://127.0.0.1:8082/healthz`
- healthmon `http://127.0.0.1:8083/healthz`
- repair `http://127.0.0.1:8084/healthz`

## Scope you must not expand

Do **not** implement until the M4 durability demo is recorded:

- ledger / billing service
- customer, provider, or admin dashboards (fleet visualizer is the only UI)
- JWT / refresh tokens (API keys only)
- agent installers / self-update
- continuous rebalancing
- reputation beyond a scalar
- S3 compatibility
- real money movement

## Definition of done

A task is done only when all of these hold:

1. `go build ./...` and `go test ./... -short` pass
2. New behavior is covered by tests (table-driven; property-based for crypto/erasure/tickets)
3. The four invariants in `docs/10-mvp-roadmap.md` are not violated (skip the ledger invariant until M5)
4. Spec and `knowledge-base/` notes match the code if behavior changed
5. `STATUS.md` lists what shipped and the next three tasks

## Cursor operating system

- **Rules:** [`.cursor/rules/`](.cursor/rules/) — spec fidelity always on; Go/crypto/test rules by glob.
- **Skills:** [`.cursor/skills/`](.cursor/skills/) — start every session with `session-brief`.
- **Hooks:** [`.cursor/hooks.json`](.cursor/hooks.json) — gofmt after edits, `go test -short` on stop, ask before destructive shell, block secrets in prompts.
- **MCP:** [`.cursor/mcp.json`](.cursor/mcp.json) — read-only Postgres (`dsp_readonly` on local Compose) and Context7. Reload MCP in Cursor Settings if the tools do not appear. Postgres MCP only works while `make up` is running.


- Start every session with the `session-brief` skill.
- Plan mode for anything that touches more than one package.
- Parallel subagents only on **disjoint** packages. Two agents in one package is merge pain, not speed.
- `bugbot` before merge. `security-review` is mandatory on keys, tickets, receipts, or anything in `internal/pipeline`, `internal/tickets`, `internal/receipts`, `internal/ca`, `cmd/dsp`.
- Strongest model for architecture, crypto, and review. Fast models for scaffolding, tests, migrations, and docs.
- You write the key hierarchy, ticket verification, and receipt validation. The human reads every line of those packages.

## Invariants (never violate)

1. Every committed chunk has ≥ 12 healthy placements (alert < 13).
2. No plaintext or unwrapped key appears in any node's `data_dir` or any control-plane store. The Master Key never crosses a network boundary.
3. Ledger transactions sum to zero (when the ledger exists).
4. Placement constraints (node/region/ASN/owner caps) hold after any repair sequence.
