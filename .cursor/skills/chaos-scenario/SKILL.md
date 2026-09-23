---
name: chaos-scenario
description: >-
  Runs simulated-fleet chaos verbs (kill, drain, partition, corrupt, flap)
  against local Compose agents and reports pass/fail with repair
  latency. Use when running M4 tests, chaos, self-healing demos, or when
  the user says kill nodes, corrupt fragments, or soak.
---

# Chaos scenario

Primary test tool from `docs/10-mvp-roadmap.md`. Operate on the Compose fleet, never on a real provider machine.

## Harness verbs

```text
kill <n> nodes           # SIGKILL
drain <node>             # graceful retirement
partition [--region r]   # docker compose pause (cgroup freeze); floor ≥ 13/16
corrupt <agent> [--frac] # flip 1 byte / 4096-byte block in .frag files
flap <node> <period>     # kill then docker start
soak [--duration 2h]     # randomized kill/flap/corrupt/partition, max 6 down
```

Drive with `go run ./cmd/harness <verb>` or `make chaos-m4` / `make chaos-partition` / `make soak`. `throttle` is retired (neutral `bandwidth_factor`).

## Default M4 scenario

1. `dsp put` a known fixture file; record `content_sha256`.
2. Identify 16 agents holding the file's fragments (Postgres `fragment_placements`).
3. `kill` 6 of those 16.
4. Immediately `dsp get`; bytes must match. Download must not wait for repair.
5. Watch repair until every chunk is 16/16 healthy. Record latency.
6. `dsp get` again. Pass only if both downloads match and repair finished.

## Regional partition

`make chaos-partition` (or `harness partition --region eu-west --for 8m --heal`) freezes one region's agents. Every chunk must stay ≥ 13 healthy while they are paused. Unpause always runs, including on SIGINT. After `--heal`, flap re-validation must leave invariants clean.

## Report shape

```markdown
## Chaos report
- Scenario:
- Started:
- Kill set:
- Download during failure: pass/fail
- Repair latency:
- Final healthy placements:
- Notes:
```

Do not declare M4 done without this report. Write a copy under `business/updates/` if this run is for the demo.
