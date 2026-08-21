---
tags:
  - map
aliases:
  - Security
  - Threat model
---

# Map of Security

Starting assumption: **every storage node is hostile**. Owners can read disks, sniff the network, modify the agent, lie about what they store, and collude. Security must hold *by construction*.

## Threats and defenses

| Adversary | Primary defenses |
|---|---|
| Malicious [[Storage Provider]] | [[Zero Knowledge]], hashes fixed before placement, [[Storage Challenge]], [[Reputation]], owner caps in [[Placement Constraints]] |
| Network attacker | TLS 1.3, [[mTLS]], single-use tickets, nonces |
| Malicious [[Customer]] | Authz on every metadata object, scoped tickets, rate limits |
| Compromised platform | No unwrapped keys server-side; client verifies hashes independently |
| Insider / operator | Same structural limits + [[Audit Log]] + least-privilege DB roles |

What is **not** defended: a compromised customer device (stolen [[Master Key]]), global traffic-analysis inference, platform-level DoS (availability requires the platform to be online).

## Key hierarchy

[[Master Key]] (client-only, from passphrase via Argon2id) wraps [[File Key]] (random per file). File Key encrypts bytes with [[AES-256-GCM]]. Platform stores only the wrapped blob in [[File Version]].`encryption_meta`.

Lose passphrase **and** recovery key → data is gone. The platform cannot reset it.

## Identity

| Principal | Mechanism |
|---|---|
| [[Customer]] | Password → JWT; [[API Key]] for machines; TOTP 2FA |
| [[Node]] | Local keypair + CSR → platform CA cert; traffic is [[mTLS]] |
| Services | Per-service mTLS; Ledger is sole writer of [[Ledger Entry]]; nothing can UPDATE [[Audit Log]] |

## Data-plane authorization

[[Node Agent]] never sees customer identity. [[Placement Ticket]] / [[Retrieval Ticket]] (Ed25519, minutes-lived, single fragment, single node) plus [[Signed Receipt]] on ingest.

## Proofs of storage

[[Storage Challenge]]: `SHA-256(nonce ‖ random byte range)`. MVP uses pre-computed challenge sets registered at commit. Failed challenge → placement `lost` → [[Repair Loop]] + reputation damage.

See also: [[Client-side Encryption]], [[Node Registration]], [[Customer REST API]]
