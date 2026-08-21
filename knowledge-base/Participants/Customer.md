---
tags:
  - participant
aliases:
  - Customers
---

# Customer

Pays for storage and bandwidth. Uploads, downloads, and manages files through the [[Customer REST API]], [[CLI and SDK]], or [[Web Dashboard]] — exactly like a traditional cloud provider.

## What they see

Buckets, paths, versions, usage, charges. They never talk to storage-node owners and never learn which [[Node]] holds their data.

## What they hold that the platform does not

The [[Master Key]] (from passphrase / recovery key). Without it, wrapped [[File Key]] blobs in [[File Version]] metadata are useless. Lose both passphrase and recovery key → data is unrecoverable. There is no platform reset.

## How they interact

1. Register / login ([[User]], JWT or [[API Key]])
2. [[Upload Flow]] — client encrypts locally, then plan → transfer to nodes → commit
3. [[Download Flow]] — plan → fetch any 10 of 16 fragments → decode → decrypt
4. Pay [[Customer Pricing]] in [[Credits]] (Phase 1: ledger only, no real money)

A single [[User]] can also be a [[Storage Provider]] (`role = both`).

Related: [[Zero Knowledge]], [[CLI and SDK]], [[Bucket]], [[Quota]]
