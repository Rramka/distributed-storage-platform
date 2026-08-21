---
tags:
  - entity
aliases:
  - Files
---

# File

Logical object in a [[Bucket]], identified by path (`/photos/2026/img.jpg`). Folders are rows with `is_folder = true`; hierarchy is path-based (folder rename = prefix update).

| Status | Meaning |
|---|---|
| `uploading` | version pending |
| `active` | `current_version` points at a committed [[File Version]] |
| `deleted` | soft delete; see [[File Deletion]] |

A file **exists** (for customers) only when metadata says so **and** enough [[Fragment Placement]]s are confirmed. That invariant is enforced at [[Upload Flow]] commit by the [[Metadata Service]].
