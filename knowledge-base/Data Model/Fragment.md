---
tags:
  - entity
aliases:
  - Fragments
  - Shard
---

# Fragment

One Reed–Solomon shard of a [[Chunk]]. `shard_index` 0–9 = data, 10–15 = parity (default 10+6). ~1.6 MB for a full 16 MB chunk.

What a [[Node Agent]] receives: `fragment_id`, encrypted bytes, expected checksum, expiration. **Never:** filename, owner, directory, file type, keys, or sibling fragments' locations.

Hash is fixed in the upload manifest **before** any node sees bytes — substitution is detectable. Used in [[Storage Challenge]]s.

Lives on a node via [[Fragment Placement]].
