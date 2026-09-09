---
tags:
  - flow
  - security
aliases:
  - Registration
  - Identity ceremony
---

# Node Registration

1. [[Storage Provider]] generates a one-time code in [[Web Dashboard]] (`POST /nodes/registration-codes`)
2. [[Node Agent]] generates a keypair **locally**, submits CSR + code (`POST /internal/nodes/register`)
3. Platform CA issues a cert with node UUID as subject; fingerprint pinned on [[Node]]
4. Reachability + disk benchmark; unreachable endpoints are rejected (MVP: no NAT relay)
5. Node goes `online` with [[Reputation]] 0.5 and a **probation quota** (limited placements for 14 days)

Private key never leaves the machine. Certs are 30-day, renewed over the existing [[mTLS]] channel. Revocation = fingerprint flag, not CRL wait.

Cloning identity onto a second machine (concurrent heartbeat streams from different addresses) → quarantine via [[Admin Dashboard]].

Related: [[mTLS]], [[Map of Security]]
