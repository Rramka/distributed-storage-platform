---
tags:
  - security
aliases:
  - Audit challenge
  - Storage proof
  - Proof of storage
---

# Storage Challenge

Proves a [[Node]] still **holds** data it isn't currently serving (hashes only prove integrity of what it *serves*).

```
challenge = { fragment_id, offset, length, nonce }
response  = SHA-256( nonce || bytes[offset : offset+length] )
```

Fresh nonce → cannot precompute. Random range → must retain the whole [[Fragment]]. Short deadline → cannot fetch just-in-time from a colluding peer.

## MVP verification

Pre-computed challenge sets: at commit, [[CLI and SDK]] (or [[Repair Service]] for reconstructed fragments) computes N future responses and registers them. Platform spends them one at a time. When a set runs low, [[Health Monitor]] fetches the fragment once (ticketed GET, verify hash) and computes a new set — audits stay cheap without storing fragment bytes on the platform.

Failed / slow challenge → [[Fragment Placement]] `lost`, [[Reputation]] damage, [[Repair Loop]]. Honest local-scrub self-report is penalized far less than being caught.

Post-MVP: Merkle proofs with unlimited verifications ([[Long-term Vision]]).
