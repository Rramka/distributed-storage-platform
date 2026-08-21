---
tags:
  - tech
  - security
aliases:
  - AES-GCM
  - Authenticated encryption
---

# AES-256-GCM

File encryption algorithm in [[Client-side Encryption]]. Streaming construction: 64 KiB segments, derived nonce (segment counter), authentication tag per segment.

Last line of integrity ([[Map of Storage Pipeline]]): even if every SHA-256 check were bypassed, tampered ciphertext fails authenticated decryption. Corrupted data can never silently become wrong plaintext.

Fragment bytes on nodes are this ciphertext (plus [[Erasure Coding]] expansion). TLS on the [[Data Plane]] does not replace this.
