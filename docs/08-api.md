# 08 — API

Three API surfaces exist in Phase 1:

1. **Customer REST API** — public, JSON over HTTPS, served by the API Gateway. This is what the CLI, SDK, dashboard, and customers' own integrations use.
2. **Node control API** — gRPC over mTLS between Node Agents and the control plane (registration, heartbeats, challenges). Summarized here; message details in [05-node-agent.md](05-node-agent.md).
3. **Node fragment API** — HTTPS on each storage node, authorized by tickets ([07-security.md](07-security.md)). Listed here for completeness.

## Conventions

- Base URL: `https://api.<platform-domain>/v1`
- Authentication: `X-Api-Key: <key>` on the solo builder track. JWT access/refresh tokens (`Authorization: Bearer`) are specified for later and are **deferred** until after M4 (`docs/10-mvp-roadmap.md`).
- All request/response bodies are JSON except raw fragment transfer (which never touches this API).
- IDs are UUIDs. Timestamps are RFC 3339 UTC. Sizes are bytes.
- Pagination: `?limit=` (max 1000) and `?cursor=`; responses carry `next_cursor` when more data exists.

Solo-track auth bootstrap: `POST /auth/register` stores an argon2id password hash. `POST /auth/api-keys` is authenticated with HTTP Basic (`email:password`) and returns the API key secret **once**. Every other `/v1` endpoint takes `X-Api-Key` only.

### Error format

Every non-2xx response has the same shape:

```json
{
  "error": {
    "code": "not_found",
    "message": "file 7d1e… does not exist or is not accessible",
    "request_id": "req_9f3c…"
  }
}
```

| HTTP | `code` examples |
|---|---|
| 400 | `invalid_request`, `manifest_invalid` |
| 401 | `unauthenticated`, `token_expired` |
| 403 | `forbidden`, `quota_exceeded` |
| 404 | `not_found` |
| 409 | `conflict`, `already_exists` |
| 429 | `rate_limited` (with `Retry-After`) |
| 5xx | `internal`, `placement_unavailable` |

## Authentication endpoints

| Method & path | Purpose |
|---|---|
| `POST /auth/register` | Create account (email, password). Returns user + hint to generate client-side keys. |
| `POST /auth/login` | **Deferred** (JWT). Exchange credentials for access + refresh tokens. |
| `POST /auth/refresh` | **Deferred** (JWT). Rotate the refresh token. |
| `POST /auth/api-keys` | Create an API key (label, scopes, expiry). Solo track: HTTP Basic (`email:password`). Secret returned **once**. |
| `GET /auth/api-keys` | List the caller's non-revoked keys (no secrets). |
| `DELETE /auth/api-keys/{id}` | Revoke a key. |

## File operations

Because encryption, chunking, and erasure coding happen client-side ([04-storage-pipeline.md](04-storage-pipeline.md)), upload is a three-step protocol: **plan → transfer (direct to nodes) → commit**. `POST /upload` from the product brief is realized as the plan+commit pair; the SDK/CLI presents it as a single `upload` call.

### `POST /upload` — plan an upload

Request: the manifest the client computed locally.

```json
{
  "bucket_id": "b_…",
  "path": "/videos/Vacation.mp4",
  "size_bytes": 157286400,
  "content_sha256": "…",
  "encryption_meta": { "algo": "aes-256-gcm", "wrapped_fk": "…", "kdf": { … } },
  "chunk_size": 16777216,
  "ec": { "data": 10, "parity": 6 },
  "chunks": [
    { "seq": 0, "sha256": "…", "size_bytes": 16777216,
      "fragments": [ { "shard_index": 0, "sha256": "…", "size_bytes": 1677722 }, … ] },
    …
  ]
}
```

Response `201`: an `upload_id` plus one placement ticket per fragment.

```json
{
  "upload_id": "up_…",
  "expires_at": "2026-07-25T21:30:00Z",
  "placements": [
    { "fragment_ref": { "chunk_seq": 0, "shard_index": 0 },
      "node": { "id": "n_…", "endpoint": "203.0.113.7:7443" },
      "ticket": "eyJvcCI6InB1dCIsIm…" },
    …
  ]
}
```

Re-POSTing the same manifest resumes: already-confirmed fragments are omitted and fresh tickets are issued for the remainder.

### `POST /upload/{upload_id}/commit`

Request: the signed node receipts collected during transfer. Response `200`: `{ "file_id": …, "version_no": 1, "status": "committed" }`. Idempotent; fails with `409` if too few fragments are confirmed (client retries missing transfers first).

### `GET /download/{file_id}` — plan a download

Optional `?version=` (defaults to current). Response: everything the client needs to fetch and reconstruct — fragment map with retrieval tickets, all expected hashes, and the wrapped file key.

```json
{
  "file_id": "f_…",
  "version_no": 3,
  "size_bytes": 157286400,
  "content_sha256": "…",
  "encryption_meta": { … },
  "chunks": [
    { "seq": 0, "sha256": "…",
      "fragments": [
        { "shard_index": 0, "sha256": "…",
          "node": { "id": "n_…", "endpoint": "…" }, "ticket": "…" },
        …  // only currently-healthy placements are listed
      ] },
    …
  ]
}
```

### Remaining file endpoints

| Method & path | Behavior |
|---|---|
| `DELETE /file/{id}` | Soft-delete; fragments expire asynchronously ([03-data-model.md](03-data-model.md)). `204`. |
| `GET /files?bucket_id=&prefix=&limit=&cursor=` | List files/folders under a prefix (name, size, version, timestamps — metadata only). |
| `GET /files/{id}` | Single file's metadata and version history. |
| `POST /folders` | Create a folder row: `{ "bucket_id": …, "path": "/photos/2026" }`. |
| `PUT /rename` | `{ "file_id": …, "new_path": … }` — metadata-only move; folder renames update the subtree prefix. |
| `GET /storage` | Account usage summary: bytes stored, bandwidth this cycle, quota, per-bucket breakdown, current-cycle charges from the ledger ([09-billing-ledger.md](09-billing-ledger.md)). |
| `POST /buckets` / `GET /buckets` / `DELETE /buckets/{id}` | Bucket management (delete requires empty). |
| `GET /health` | Unauthenticated liveness: `{ "status": "ok", "version": … }`. |

## Provider endpoints (dashboard-facing)

| Method & path | Behavior |
|---|---|
| `POST /nodes/registration-codes` | Provider generates a one-time code to bind a new agent install to their account. Body: `{ "endpoint", "country", "region", "asn" }`. Placement attributes are platform-asserted (not agent-reported) and copied onto the node at register. |
| `GET /nodes` | Provider's nodes: status, capacity, used bytes, reputation, uptime, last seen. |
| `GET /demo/fleet` | **Demo only** (`DEMO_MODE=1`): fleet-wide nodes + per-chunk healthy counts. Unauthenticated. Not present in a normal deployment. The visualizer at `GET /demo/` polls this. |
| `GET /nodes/{id}/stats?from=&to=` | Hourly rollups: uptime, audits, bytes served/ingested. |
| `POST /nodes/{id}/drain` | Graceful retirement — migrate data off, no reputation penalty. |
| `GET /earnings` | Provider ledger view: accrued credits by node, window, and type. |

## Node control API (HTTP/JSON over mTLS)

Solo track: HTTP/JSON, not gRPC (gRPC is deferred past M4). Heartbeats are request/response; the JSON body includes a `messages` array so M4 can push expiry lists and challenges without a protocol change.

| HTTP | Direction | Purpose |
|---|---|---|
| `POST /internal/nodes/register` | agent → metadata | Identity ceremony (CSR + registration code); returns signed certificate + node ID |
| `POST /internal/heartbeat` | agent → healthmon | Agent sends heartbeats every 10 s over mTLS; platform returns `{ "messages": [] }` |
| ConfirmDeletions / ReconcileInventory / RenewCertificate | — | M4+ |

## Node fragment API (HTTPS on each node, data plane)

| Method & path | Auth | Purpose |
|---|---|---|
| `PUT /fragments/{id}` | placement ticket | Ingest fragment; returns signed receipt |
| `GET /fragments/{id}` | retrieval ticket | Serve fragment; supports `Range` |
| `DELETE /fragments/{id}` | delete ticket | Immediate deletion (also driven via heartbeat stream) |
| `GET /challenge` | challenge ticket | Storage-proof response ([07-security.md](07-security.md)). Query `fragment_id`, `offset`, `length`, `nonce`. |

## Rate limits (MVP defaults)

| Scope | Limit |
|---|---|
| Auth endpoints | 10/min per IP |
| Upload/download planning | 60/min per account |
| Metadata reads | 600/min per account |
| Ticket batch size | 10,000 fragments per plan request |

Limits are enforced at the gateway (Redis token buckets) and returned via `429` + `Retry-After`.

## Future: S3-compatible layer

A post-MVP gateway will translate the S3 API (`PUT/GET/DELETE Object`, `ListObjectsV2`, multipart upload, presigned URLs) onto this native API so existing S3 SDKs and tools work unchanged. Two notes shape the native design today: buckets/paths already map 1:1 to S3 buckets/keys, and the plan/commit upload protocol maps naturally onto S3 multipart semantics. The S3 layer will offer an optional server-side-encryption mode (platform-held keys) for compatibility, clearly distinguished from the default zero-knowledge mode.
