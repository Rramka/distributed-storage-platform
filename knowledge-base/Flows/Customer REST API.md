---
tags:
  - service
aliases:
  - REST API
  - Public API
---

# Customer REST API

Public JSON over HTTPS at `https://api.<platform-domain>/v1`, served by [[API Gateway]]. Used by [[CLI and SDK]], [[Web Dashboard]], and customer integrations.

Auth: `Authorization: Bearer <jwt>` or `X-Api-Key`. IDs are UUIDs. Errors share one `{ error: { code, message, request_id } }` shape.

## Surfaces that matter to the graph

| Protocol | Who | Bytes? |
|---|---|---|
| This REST API | customers | **no** — plan/commit/metadata only |
| Node control gRPC | [[Node Agent]] ↔ [[Health Monitor]] | no |
| Node fragment HTTPS | client/repair ↔ agent | **yes** — ciphertext |

## Upload is three steps

Because encryption lives in [[CLI and SDK]], product `upload` is **plan** (`POST /upload`) → direct transfer → **commit** (`POST /upload/{id}/commit`). Same idea for [[Download Flow]].

Provider routes: registration codes, node list/stats, drain, earnings. See also [[Quota]] (`403 quota_exceeded` on upload).

Post-MVP: S3-compatible gateway on top of this native API ([[Long-term Vision]]). Buckets/paths already map 1:1 to S3; plan/commit maps onto multipart.
