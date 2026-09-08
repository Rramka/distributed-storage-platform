---
name: kb-sync
description: >-
  After a feature lands, updates docs/ and the matching Obsidian notes in
  knowledge-base/ so the vault stays a live map. Use when behavior changed,
  a milestone exit was hit, a spec diverged, or the user says sync the
  knowledge base or update the vault.
---

# Knowledge-base sync

`docs/` is canonical. `knowledge-base/` is the graph of the same design. They must not drift.

## When to run

After merging a change that alters behavior, schema, API, or roadmap status.

## Steps

1. Diff the change (`git diff` / files touched).
2. Update the numbered spec in `docs/` if the implementation taught us something. Spec edits are required when code diverged.
3. Update the matching vault notes (see the table in `knowledge-base/Home.md`).
4. Keep vault notes short; link to other notes with `[[Wiki Links]]`. Do not paste whole specs into the vault.
5. If a new entity/service/flow appeared, add a note in the right folder (`Control Plane/`, `Data Model/`, `Flows/`, `Security/`) with tags matching `Home.md`.
6. Touch `knowledge-base/Maps/` hubs when a map is now wrong.
7. Mention the vault paths in `STATUS.md`.

## Do not

- Rewrite `docs/` in the vault's voice.
- Leave "no application code yet" claims in `Home.md` or `README.md` after M0.
