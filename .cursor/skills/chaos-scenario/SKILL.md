---
name: chaos-scenario
description: >-
  Runs simulated-fleet chaos verbs (kill, drain, partition, corrupt, throttle,
  flap) against local Compose agents and reports pass/fail with repair
  latency. Use when running M4 tests, chaos, self-healing demos, or when
  the user says kill nodes, corrupt fragments, or soak.
---

# Chaos scenario

Primary test tool from `docs/10-mvp-roadmap.md`. Operate on the Compose fleet, never on a real provider machine.

## Harness verbs

```text
kill <n> nodes           # SIGKILL
drain <node>             # graceful retirement
partition <nodes>        # network cut
corrupt <node> <frac>    # flip bytes in stored fragments
throttle <node> <mbps>   # degrade bandwidth
flap <node> <period>     # reboot loops
```

Until `cmd/harness` exists, drive verbs with `docker compose` in `deploy/compose/`:

- kill: `docker compose kill <agent>`
- drain: send SIGTERM, wait, then `stop`
- flap: kill + start in a loop

## Default M4 scenario

1. `dsp put` a known fixture file; record `content_sha256`.
2. Identify 16 agents holding the file's fragments (Postgres `fragment_placements`).
3. `kill` 6 of those 16.
4. Immediately `dsp get`; bytes must match. Download must not wait for repair.
5. Watch repair until every chunk is 16/16 healthy. Record latency.
6. `dsp get` again. Pass only if both downloads match and repair finished.

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
