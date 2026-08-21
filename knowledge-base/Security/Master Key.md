---
tags:
  - security
aliases:
  - MK
---

# Master Key

Derived from the [[Customer]] passphrase via **Argon2id** (memory-hard), or generated and stored in the OS keychain with a printable recovery key. **Never leaves the client.**

Wraps each [[File Key]] with AES-KW. KDF parameters (memory, iterations, salt) live in [[File Version]].`encryption_meta` so they can be strengthened over time.

A compromised customer device that steals MK is out of the threat model ([[Map of Security]]). The platform cannot reset a lost MK because it has nothing to reset.
