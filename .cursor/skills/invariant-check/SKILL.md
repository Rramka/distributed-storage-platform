---
name: invariant-check
description: >-
  Asserts the platform invariants: committed chunks have at least 12 healthy
  placements, no plaintext or unwrapped keys on disk, ledger sums to zero
  when it exists, and placement constraints hold after repair. Use before
  merge, during soak, after chaos, or when the user says invariants or
  red-team DB dump.
---

# Invariant check

From `docs/10-mvp-roadmap.md` continuously measured invariants.

## Checks

1. **Durability.** Every `file_versions.status = committed` chunk has ≥ 12 `fragment_placements` with `status = stored` on nodes that are not `offline`/`quarantined`. Alert if any chunk has < 13.
2. **Zero knowledge.** Grep agent `data_dir` and control-plane stores for plaintext fixtures and unwrapped keys. Platform DB dump must not decrypt the fixture file (red-team). Master Key never appears in logs or Postgres.
3. **Ledger.** `SUM(amount) GROUP BY txn_id` is 0 for every transaction. **Skip on the solo track until M5.**
4. **Placement constraints.** No two fragments of the same chunk share a node; region/ASN/owner caps from `docs/06-scheduler-and-repair.md` hold after repair.

## How to run

- Prefer SQL against the Compose Postgres (read-only role `dsp_readonly`) plus filesystem greps.
- Put durable assertions in Go tests under `internal/invariants/` once that package exists.
- `-short` may skip fleet checks; full check is required after chaos and before a demo recording.

## Report

```markdown
## Invariants
- [ ] ≥ 12 healthy placements (min observed: _)
- [ ] no plaintext / unwrapped key
- [ ] ledger balanced (or skipped — M5 deferred)
- [ ] placement constraints
```

Fail the session if any required box is unchecked.
