---
tags:
  - flow
aliases:
  - Upload
  - PUT file
---

# Upload Flow

Happy path. Transform work is on [[CLI and SDK]]; [[Control Plane]] only plans and commits.

```mermaid
sequenceDiagram
    participant C as CLI and SDK
    participant GW as API Gateway
    participant MS as Metadata Service
    participant SC as Scheduler
    participant N as Node Agent

    C->>C: Encrypt, chunk, hash, erasure-code
    C->>GW: POST /upload (manifest)
    GW->>MS: Plan upload
    MS->>SC: Request placement
    SC-->>MS: 16 nodes per chunk
    MS-->>C: Placement Tickets
    par parallel PUT
        C->>N: fragment + ticket
        N-->>C: Signed Receipt
    end
    C->>GW: POST /upload/commit
    GW->>MS: Verify receipts, commit
    MS-->>C: file_id, version
```

## Thresholds

A file is reported uploaded only when every [[Chunk]] has **≥ 14 of 16** [[Fragment Placement]]s `stored`. Missing fragments (up to 2) are backfilled by [[Repair Service]] rather than blocking the client.

## Failures

Unreachable node → replacement ticket. Client crash → `pending` [[File Version]] expired by janitor. Uploads are **resumable** via re-POST of the same manifest.

Related: [[Map of Storage Pipeline]], [[Placement Ticket]], [[Erasure Coding]], [[Customer REST API]]
