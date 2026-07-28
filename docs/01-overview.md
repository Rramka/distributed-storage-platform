# 01 — Overview

## Problem statement

Modern cloud storage (AWS S3, Google Cloud Storage, Azure Blob Storage) is built on enormous centralized data centers. These systems are highly reliable, but the model has structural limitations:

- **Extremely expensive infrastructure** — data centers, land, power, cooling, and staff, all paid for by customers.
- **High operational costs** passed through in pricing, particularly for egress bandwidth.
- **Geographic concentration** — data lives in a handful of regions; an outage in one region affects millions of customers at once.
- **Vendor lock-in** — proprietary APIs, egress fees, and ecosystem gravity make leaving costly.
- **Single-provider dependency** — one company controls availability, pricing, and policy.

Meanwhile, an enormous storage resource sits idle. Billions of personal desktops, gaming PCs, home servers, NAS devices, office workstations, and small-business servers stay online every day with hundreds of gigabytes — often terabytes — of unused disk space. Collectively this is one of the largest storage pools on Earth, and today it does nothing.

## Vision

The platform is a **secure storage marketplace** that connects these two sides:

- Anyone becomes a cloud storage provider by installing a lightweight application (the **Node Agent**) and choosing how much disk space and bandwidth to share. They are compensated based on stored capacity, availability, uptime, and served traffic.
- Customers upload files exactly as they would to any cloud provider, through a REST API, CLI, or web dashboard. They pay only for the storage and bandwidth they consume.

Behind the scenes, the platform:

1. encrypts data on the client, before transmission,
2. divides it into chunks and erasure-codes each chunk into fragments,
3. distributes fragments across independent devices worldwide,
4. continuously monitors storage nodes via heartbeats,
5. repairs redundancy automatically when nodes fail or disappear,
6. verifies data integrity with hashes and random challenge audits,
7. meters usage and credits storage providers automatically.

The customer never knows which physical computers store their data. Storage providers never know what data they are storing. The platform coordinates everything but can decrypt nothing.

## Core principles

### Zero trust

Storage providers must never be trusted. Every stored byte is assumed to be readable, copyable, or corruptible by the machine's owner. Therefore:

- Every file is encrypted **before leaving the client** (AES-256-GCM, per-file keys).
- Every chunk is independently encrypted and hashed.
- Storage providers cannot decrypt stored data or infer filenames, owners, or file types.
- The platform itself cannot decrypt customer data — it never possesses the keys.

See [07-security.md](07-security.md) for the full threat model.

### Decentralization

No customer file exists on a single computer. Every file is split and spread across many independent devices in different regions, on different ISPs, under different owners. The failure of one — or many — devices must not affect file availability.

### Fault tolerance

Failure is the normal case, not the exception. Hardware dies, home internet drops, power goes out, and users uninstall the agent without warning. The platform detects failures within seconds via heartbeats, and the repair pipeline reconstructs and re-places lost fragments automatically. See [06-scheduler-and-repair.md](06-scheduler-and-repair.md).

### Horizontal scalability

The same architecture must support 100 nodes, 10,000 nodes, and 1 million nodes without redesign. Control-plane services are stateless and horizontally scalable; state lives in Postgres (sharded when needed) and Redis. See the scalability story in [02-system-architecture.md](02-system-architecture.md).

### Security by design

Every component assumes a hostile environment. Data remains encrypted **in transit** (TLS 1.3, mutual authentication for nodes), **at rest** (client-side AES-256-GCM), and **during replication/repair** (fragments are re-placed without ever being decrypted — repair operates on ciphertext).

## What the platform stores — and what it never stores

| Stored by the platform (metadata only) | Never stored by the platform |
|---|---|
| Users, buckets, file names and hierarchy | File contents (plaintext or ciphertext) |
| Chunk and fragment IDs, sizes, checksums | Encryption keys capable of decrypting customer data |
| Fragment-to-node placement map | — |
| Node inventory, health, and reputation | — |
| Usage meters and ledger entries | — |

Storage nodes store the inverse: opaque encrypted fragment bytes plus a fragment ID, checksum, and expiration — and nothing about filenames, owners, directory structure, or keys.

## MVP scope (Phase 1)

The first release is a narrow but **complete** vertical slice that validates the core assumptions — secure distributed storage, reliable orchestration, and automatic recovery:

1. User registration and authentication
2. Node registration and provisioning (certificates, identity)
3. Client-side encryption (in the CLI/SDK)
4. Chunking and hashing
5. Metadata service
6. Erasure coding and chunk distribution
7. Chunk retrieval and file reconstruction
8. Integrity verification (checksums + random challenge audits)
9. Automatic replication / self-healing repair
10. Health monitoring (heartbeats, reputation scoring)
11. Basic billing — internal ledger only (metering, computed charges and earnings; **no real money movement**)
12. Administrative dashboard (network health, node list, repair queue)

## Explicit non-goals for Phase 1

These are deliberately excluded to keep the MVP shippable. Each has a place in the long-term roadmap ([10-mvp-roadmap.md](10-mvp-roadmap.md)):

- **Compute of any kind** — no containers, VMs, GPU workloads, or serverless functions.
- **S3-compatible API** — the MVP exposes a simple native REST API; S3 compatibility is a later layer.
- **Real payments** — no Stripe, no crypto, no payouts. The ledger tracks balances only.
- **File sharing and collaboration** — beyond basic ownership and access control.
- **Provider-set pricing** — pricing is platform-defined in Phase 1; a pricing marketplace comes later.
- **Client-side sync clients** (Dropbox-style folder sync).
- **Public node marketplace / open enrollment at scale** — Phase 1 targets a controlled set of real and simulated nodes.

## Reading order

For a first read, follow the numbered order. If you want the fastest path to understanding the system: read [04-storage-pipeline.md](04-storage-pipeline.md) (what happens to a file), then [02-system-architecture.md](02-system-architecture.md) (who does it), then [06-scheduler-and-repair.md](06-scheduler-and-repair.md) (how it stays alive).
