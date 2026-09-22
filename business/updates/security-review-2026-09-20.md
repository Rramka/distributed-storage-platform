# Security review — 2026-09-21 (W11)

Scope: keys, tickets, receipts, crypto path (`internal/pipeline`, `internal/tickets`, `internal/receipts`, `internal/ca`, `internal/agent`, `cmd/dsp`) plus this session's `harness corrupt` and Health Monitor challenge-fail path.

## Result

No medium, high, or critical findings with realistic production exploitability. The W11 diff is security-neutral to positive: red-team `pg_dump` zero-knowledge coverage, corrupt-fragment detection, and a test-only `OnRepair` hook. Production still publishes repair via NATS (`Bus`). Ticket verification, receipt validation, and key wrapping were not changed.

## Checks

| Area | Outcome |
|---|---|
| Challenge fail → `MarkPlacementLost` + reputation + repair | Production `Bus` path unchanged; `OnRepair` is tests only |
| `harness corrupt` | Compose-local operator; argv-separated exec; XOR on ciphertext `.frag` files only |
| Red-team dump | Marker, passphrase, file key, and master key absent from `pg_dump`; planted needle is detected |
| Tickets / receipts / pipeline / CA / agent / `dsp` | Unchanged in this diff; e2e corrupt test still signs and verifies `challenge` tickets |

## Below threshold (not fixed)

- `OnRepair` plus a nil `Bus` would skip repair — production always sets `Bus`.
- `store.Exec` is a test helper, not reachable from HTTP.
- `POST /internal/challenges/tickets` mint auth is out of this diff; worth a later pass.

## Follow-up

None required before the W12 raise demo. Pre-existing live-fleet placement-cap violations (3 chunks) are a durability-constraint issue, not a crypto leak.
