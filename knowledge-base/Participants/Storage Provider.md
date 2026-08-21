---
tags:
  - participant
aliases:
  - Provider
  - Storage providers
---

# Storage Provider

Runs a [[Node Agent]] on unused disk (desktop, NAS, home server). Earns [[Provider Earnings]] based on stored capacity, uptime, and served traffic.

## What they store

Opaque encrypted [[Fragment]] bytes plus fragment ID, checksum, and expiration. They **never** receive filenames, owners, directory structure, file types, or keys. See [[Zero Knowledge]].

## What they control

In `agent.yaml`: max share, min free disk, bandwidth caps, active hours. `min_free_disk_gb` always wins — the agent never competes with the owner for space. Shrinking capacity puts the [[Node]] into `draining`; the [[Scheduler]] migrates fragments away.

## Trust model

Providers are **not trusted**. Defenses: client-side ciphertext, hashes fixed before placement, [[Storage Challenge]]s, [[Reputation]], probation quota for new nodes, owner caps in [[Placement Constraints]].

Uninstall = normal node loss. Heartbeats stop, [[Repair Loop]] reconstructs. No customer data is unrecoverable because of one uninstall.

Related: [[Node Registration]], [[Heartbeat Protocol]], [[mTLS]]
