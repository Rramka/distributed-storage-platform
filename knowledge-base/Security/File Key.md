---
tags:
  - security
aliases:
  - FK
  - Per-file key
---

# File Key

32 random bytes per [[File Version]]. Encrypts file bytes with [[AES-256-GCM]]. Wrapped under [[Master Key]] and stored in `encryption_meta`.

Per-file keys bound blast radius: compromising one FK exposes one file, not an account.

The platform stores only the wrapped blob — see [[Zero Knowledge]] and [[Client-side Encryption]].
