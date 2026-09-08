---
name: spec-to-service
description: >-
  Scaffolds a control-plane or data-plane service from a docs/ section:
  proto, cmd binary, internal package, Compose entry, health endpoint, and
  tests. Use when adding a new service, when the user names gateway,
  metadata, scheduler, healthmon, repair, agent, or dsp, or says
  "scaffold", "new service", or "implement this spec".
---

# Spec to service

Turn one spec section into a thin but runnable vertical. Do not implement deferred solo-track services (`ledger`, `admind`).

## Inputs

The user names a service and a spec file (default mapping):

| Service | Spec |
|---|---|
| gateway | `docs/08-api.md`, `docs/02-system-architecture.md` |
| metadata | `docs/03-data-model.md`, `docs/08-api.md` |
| scheduler | `docs/06-scheduler-and-repair.md` |
| healthmon | `docs/05-node-agent.md`, `docs/06-scheduler-and-repair.md` |
| repair | `docs/06-scheduler-and-repair.md` |
| agent | `docs/05-node-agent.md` |
| dsp | `docs/04-storage-pipeline.md`, `docs/08-api.md` |

## Checklist

Copy and tick:

```
- [ ] Read the spec section; quote the behavior you will implement
- [ ] cmd/<svc>/main.go with GET /healthz (CLI: --help instead)
- [ ] internal/<pkg> for the domain logic
- [ ] proto/ stubs only if a new RPC contract is required
- [ ] deploy/compose entry + Makefile target if it is long-running
- [ ] migration in deploy/migrations/ if new tables are required (from docs/03)
- [ ] table-driven tests; -short skips Compose
- [ ] STATUS.md updated
```

## Rules

- Implement the **thinnest** slice that meets the milestone exit, not the whole spec.
- Shared crypto lives in `internal/pipeline`, `internal/tickets`, `internal/receipts`, `internal/ca` — do not duplicate it inside a service.
- Health endpoint JSON: `{"status":"ok","service":"<name>"}`.
- Compose service names match binary names. HTTP ports: gateway 8080, metadata 8081, scheduler 8082, healthmon 8083, repair 8084, agent 9000+.
- After scaffolding, run `go build ./...` and `go test ./... -short`.
