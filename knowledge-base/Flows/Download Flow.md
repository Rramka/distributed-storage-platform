---
tags:
  - flow
aliases:
  - Download
  - GET file
---

# Download Flow

```mermaid
sequenceDiagram
    participant C as CLI and SDK
    participant MS as Metadata Service
    participant N as Node Agent

    C->>MS: GET /download/{file_id}
    MS-->>C: fragment map + Retrieval Tickets + hashes + wrapped FK
    par fastest 10 of 16 fragments per chunk
        C->>N: GET fragment + ticket
        N-->>C: ciphertext
        C->>C: verify SHA-256
    end
    C->>C: RS-decode, unwrap File Key with Master Key, AES-GCM decrypt
```

Slow or dead nodes cost a redundant request, not a failure. A corrupt fragment is discarded, reported (placement `lost`, [[Reputation]] hit, [[Repair Loop]]), and replaced from the remaining 6.

If more than 6 of a chunk's nodes are unreachable, the client retries while the platform triggers urgent repair — extremely unlikely given [[Erasure Coding]] durability math plus [[Placement Constraints]].

Related: [[Retrieval Ticket]], [[Zero Knowledge]], [[Customer REST API]]
