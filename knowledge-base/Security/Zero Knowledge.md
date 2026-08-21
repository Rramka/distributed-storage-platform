---
tags:
  - security
  - principle
aliases:
  - Zero trust
  - Zero-knowledge
---

# Zero Knowledge

Structural, not a policy. [[Control Plane]] stores only wrapped [[File Key]]s. [[Node Agent]]s store opaque ciphertext. Nobody but the [[Customer]] (holding [[Master Key]]) can decrypt.

Concrete node-operator view of disk: random UUIDs, high-entropy bytes, sizes, expirations. They cannot determine filenames, owners, file types, file boundaries (fragments of one file are scattered and unlinkable without the [[Fragment Placement]] map), or whether two fragments belong to the same customer.

## Why a full DB dump is still safe

An attacker with all of [[Postgres]] gets metadata, wrapped keys, and the placement map. They do **not** get MK. Clients still verify hashes independently, so a lying platform cannot forge file contents either.

## Trade-off

Lost passphrase + recovery key = lost data. No reset path. Stated in product terms.

[[Repair Loop]] operates on ciphertext, so healing does not punch a hole in this guarantee.

Related: [[Client-side Encryption]], [[Map of Security]], [[AES-256-GCM]]
