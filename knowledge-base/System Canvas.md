---
tags:
  - map
aliases:
  - Canvas
  - Spatial map
---

# System Canvas

Hand-laid spatial view of the same connections Graph view infers from wiki links.

Open the sibling file **`System Canvas.canvas`** in Obsidian (it will not render as markdown). Groups:

- **Customers** — [[Customer]], [[CLI and SDK]], [[Web Dashboard]]
- **Control Plane** — [[API Gateway]] through [[Admin Dashboard]]
- **State** — [[Postgres]], [[Redis]], [[NATS JetStream]]
- **Data Plane** — [[Storage Provider]], [[Node Agent]]
- **Data model** — [[User]] → [[Bucket]] → [[File]] → [[File Version]] → [[Chunk]] → [[Fragment]] → [[Fragment Placement]] → [[Node]]

Dotted-style edges on the canvas (orange) are **ciphertext**. They skip the control plane.

Then open **Graph view** for the full web (security, billing, flows, roadmap). Start from [[Home]].
