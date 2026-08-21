---
tags:
  - tech
aliases:
  - Golang
---

# Go

Language for every [[Control Plane]] service and the [[Node Agent]].

Why: one language; concurrency for heartbeat fan-in and parallel fragment transfer; static cross-compiled binaries (Windows / macOS / Linux) for providers.

Proposed layout: `cmd/gateway`, `metadata`, `scheduler`, `healthmon`, `repair`, `ledger`, `admind`, `agent`, `dsp` plus `internal/pipeline`, `tickets`, `store`. See [[Milestones]] M0.
