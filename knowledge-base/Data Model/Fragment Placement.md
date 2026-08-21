---
tags:
  - entity
aliases:
  - Placement
  - Placement map
  - fragment_placements
---

# Fragment Placement

Heart of the system and its largest table: which [[Node]] holds which [[Fragment]], and in what status.

| Status | Meaning |
|---|---|
| `pending` | [[Placement Ticket]] issued |
| `stored` | [[Signed Receipt]] verified — this is what meters [[Provider Earnings]] |
| `lost` | node failed or [[Storage Challenge]] / scrub failed |
| `expiring` | [[File Deletion]] or surplus after repair |
| `deleted` | node confirmed removal |

Indexed **by fragment** (download planning, repair threshold) and **by node** (failure: "what was on N?"). At 100k+ nodes this shards by `fragment_id` plus a node-keyed inverted index ([[Horizontal Scalability]]).

A [[File]] is committed only when every [[Chunk]] has ≥ 14 `stored` placements. Repair threshold is 12. Decode needs 10.
