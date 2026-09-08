---
tags:
  - roadmap
aliases:
  - Out of scope
  - Phase 1 non-goals
---

# Non-goals

Deliberately excluded from [[MVP Scope]] so the first release ships. Each returns in [[Long-term Vision]]:

- **Compute** — no containers, VMs, GPUs, serverless
- **S3-compatible API** — native [[Customer REST API]] only
- **Real payments** — [[Ledger Service]] tracks [[Credits]] only
- **File sharing / collaboration** — beyond basic ownership
- **Provider-set pricing** — platform-defined rates ([[Customer Pricing]])
- **Dropbox-style sync clients**
- **Public node marketplace at scale** — controlled set of real + simulated nodes

[[Node Agent]] strict-NAT relay traversal is also post-MVP.

## Also deferred on the solo builder track

The raise demo is M0–M4 (encrypted file survives killing 6 of 16 nodes). Until that demo is recorded, do **not** implement:

- [[Ledger Service]] and billing UI
- [[Web Dashboard]] and [[Admin Dashboard]] (a fleet visualizer replaces them for the demo)
- JWT / refresh tokens (API keys only)
- Agent installers and self-update
- Continuous rebalancing
- [[Reputation]] beyond a single scalar

See `docs/10-mvp-roadmap.md` § Solo builder track.
