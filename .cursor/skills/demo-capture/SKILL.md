---
name: demo-capture
description: >-
  Runs the scripted kill-6-of-16 durability demo, captures terminal and
  fleet-visualizer output, and writes metrics JSON. Use when recording the
  investor demo, after M4, on Sunday cadence, or when the user says demo,
  capture, or screen recording.
---

# Demo capture

The raise scene from `docs/10-mvp-roadmap.md`:

`dsp put Vacation.mp4` → kill 6 of 16 **holders** (Reed–Solomon shards; the Compose fleet is **24** agents) → download is byte-identical → fleet heals to 16/16 → platform DB dump decrypts nothing.

## Script

1. Fresh fleet: `make fleet-down && make up`. Wait until 24 agents are online, then `make demo-ready` (copies `api.key`, uploads a **31 MiB** fixture — 32 MiB becomes 3 chunks and can start at 15/16 on a probation fleet).
2. Open the fleet visualizer at `http://127.0.0.1:8080/demo/` (`DEMO_MODE=1`).
3. Record `content_sha256` and start a timer.
4. `source .local/demo.env` then `make chaos-m4` (or `make demo`). That kills 6 every-chunk holders and **runs `dsp get` while they are down**.
5. Confirm the visualizer shows the dip and the heal.
6. Wait until placements are 16/16. Record repair latency.
7. Dump Postgres (`pg_dump`) and assert the dump cannot produce plaintext (red-team fixture). `make demo` does this automatically.
8. Write `business/updates/demo-metrics-YYYY-MM-DD.json`:

```json
{
  "date": "",
  "file_bytes": 0,
  "killed_nodes": 6,
  "download_ok_during_failure": true,
  "repair_latency_seconds": 0,
  "final_healthy_per_chunk": 16,
  "db_dump_decrypts_nothing": true,
  "visualizer": "live"
}
```

The visualizer shipped in W10. Capture **both** the terminal and `/demo/`. Shot list: `business/updates/demo-script-w12.md`.

Do not fake metrics. If the run failed, write `download_ok_during_failure: false` and stop.

Dirty-volume footgun: leftover `127.0.0.1:1900x` node rows fail `make invariants`. Wipe with `make fleet-down && make up`. After kills, revive agents with `docker start dsp-agentN-1`, not `docker compose start`.
