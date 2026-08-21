# How to open this vault

This folder is an **Obsidian** vault. Do not open the git repo root — open **this directory**.

1. Install [Obsidian](https://obsidian.md)
2. **Open folder as vault** → choose `knowledge-base/`
3. Open [[Home]] (already the intended start note)
4. Open **Graph view** (`Cmd+G` on macOS, `Ctrl+G` on Windows/Linux)

## What you should see

- Notes are **atomic concepts** (one service, entity, or flow per note)
- Wiki-style links between notes are the edges — Graph view draws them automatically
- Colors are grouped by tag (red = participants, blue = services, green = data model, orange = flows, purple = security, gold = billing, gray = tech)
- [[System Canvas]] is a hand-laid spatial map of the same connections (open it from the file list)

## How this relates to `docs/`

`docs/01`–`docs/10` in the git repo are the **canonical long-form specs**. This vault is a navigable graph derived from them. When design changes, update the spec first, then the matching notes here.

## Editing conventions

- One idea per note; link instead of duplicating
- Use `#tag` in frontmatter (`service`, `entity`, `flow`, `security`, …)
- Prefer wiki links to existing note titles so the graph stays clean (unresolved links are hidden in graph settings)
