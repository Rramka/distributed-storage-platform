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

Pre-computed challenge sets minted by [[Health Monitor]] (refresh path): ticketed GET, verify hash, store N `(offset, length, nonce, expected)` tuples — never the bytes. Spend one per audit; refill when the set is low. The client does **not** register responses at commit. `GET /challenge` uses a metadata-minted `challenge` ticket ([[Placement Ticket]] family); Metadata is the sole signer.

Failed / slow challenge → [[Fragment Placement]] `lost`, [[Reputation]] damage, [[Repair Loop]]. Honest local-scrub self-report is penalized far less than being caught.

Post-MVP: Merkle proofs with unlimited verifications ([[Long-term Vision]]).
