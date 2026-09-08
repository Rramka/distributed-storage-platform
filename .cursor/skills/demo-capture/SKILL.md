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

`dsp put Vacation.mp4` → kill 6 of 16 nodes → download is byte-identical → fleet heals to 16/16 → platform DB dump decrypts nothing.

## Script

1. Start Compose fleet + visualizer (`make up`). Confirm 16 agents healthy.
2. `dsp put` a fixture (use a small video or a generated 32 MB file named `Vacation.mp4`).
3. Record `content_sha256` and start a timer.
4. Kill 6 agents that hold fragments of that file.
5. `dsp get` immediately. Compare hash.
6. Wait until placements are 16/16. Record repair latency.
7. Dump Postgres (`pg_dump`) and assert the dump cannot produce plaintext (red-team fixture).
8. Write `business/updates/demo-metrics-YYYY-MM-DD.json`:

```json
{
  "date": "",
  "file_bytes": 0,
  "killed_nodes": 6,
  "download_ok_during_failure": true,
  "repair_latency_seconds": 0,
  "final_healthy_per_chunk": 16,
  "db_dump_decrypts_nothing": true
}
```

Until a visualizer exists, capture the terminal only and note that in the JSON as `"visualizer": "terminal-only"`.

Do not fake metrics. If the run failed, write `download_ok_during_failure: false` and stop.
