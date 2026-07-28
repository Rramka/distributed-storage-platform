# Distributed Storage Platform

A decentralized cloud storage network that turns unused disk space on ordinary computers into a secure, globally distributed cloud storage service.

Instead of storing customer data in centralized data centers, the platform encrypts data on the client, splits it into erasure-coded fragments, and distributes those fragments across thousands of independently operated devices worldwide. The platform itself is a pure orchestration layer: it never stores customer file contents and can never decrypt them.

**Status: Phase 1 — architecture and design documentation.** No application code exists yet. These documents define the system that will be built, in Go, as described in [docs/10-mvp-roadmap.md](docs/10-mvp-roadmap.md).

## The three participants

| Participant | Role |
|---|---|
| **Storage Providers** | Run a lightweight Go **Node Agent** that contributes unused disk space, stores encrypted fragments, and earns credits based on stored capacity, uptime, and served traffic. Providers can never see what they store. |
| **Customers** | Upload, download, and manage files through a REST API and web dashboard, exactly like a traditional cloud provider. Customers never talk to storage nodes' owners and never learn where their data physically lives. |
| **Platform (control plane)** | The trusted coordinator: authentication, metadata, node selection, health monitoring, self-healing repair, integrity verification, and billing. It orchestrates everything but holds no file contents and no decryption keys. |

## Core guarantees

- **Zero trust / zero knowledge** — every file is encrypted client-side with AES-256-GCM before a single byte leaves the customer's device. Nodes store opaque encrypted fragments; neither nodes nor the platform can decrypt them or learn filenames, owners, or file types.
- **No single point of data loss** — files are chunked and Reed–Solomon erasure-coded (default 10 data + 6 parity fragments); any 10 of 16 fragments reconstruct a chunk. Fragments are placed on nodes in different regions and networks.
- **Self-healing** — missed heartbeats and failed integrity audits trigger automatic reconstruction and re-placement of lost fragments. Customers never notice node failures.
- **Horizontal scalability** — the same architecture serves 100 nodes or 1 million nodes without redesign.

## Documentation index

| Document | Contents |
|---|---|
| [01 — Overview](docs/01-overview.md) | Problem statement, vision, core principles, MVP scope and explicit non-goals |
| [02 — System Architecture](docs/02-system-architecture.md) | Control-plane services, technology choices, component and sequence diagrams, scalability story |
| [03 — Data Model](docs/03-data-model.md) | Postgres schema: users, buckets, files, chunks, fragments, nodes, ledger, audit logs |
| [04 — Storage Pipeline](docs/04-storage-pipeline.md) | Client-side encryption, chunking, hashing, erasure coding, upload/download flows |
| [05 — Node Agent](docs/05-node-agent.md) | The Go agent that storage providers run: chunk store, heartbeats, audits, GC, upgrades |
| [06 — Scheduler & Repair](docs/06-scheduler-and-repair.md) | Node selection scoring, placement constraints, rebalancing, self-healing loop |
| [07 — Security](docs/07-security.md) | Threat model, key management, mTLS and node identity, tamper detection, storage proofs |
| [08 — API](docs/08-api.md) | REST API spec for customers and the node-agent control API |
| [09 — Billing & Ledger](docs/09-billing-ledger.md) | Usage metering, double-entry internal ledger, provider earnings formula |
| [10 — MVP Roadmap](docs/10-mvp-roadmap.md) | Build milestones, testing strategy with simulated nodes, long-term evolution |

## Long-term vision

Storage is the foundation, not the destination. Once a reliable decentralized storage network exists, the same infrastructure evolves toward S3-compatible object storage, file synchronization, distributed databases, container execution, GPU marketplaces, and serverless compute — a community-powered alternative to centralized cloud providers. See [docs/10-mvp-roadmap.md](docs/10-mvp-roadmap.md) for the phased path.
