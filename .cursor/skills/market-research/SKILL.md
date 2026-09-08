---
name: market-research
description: >-
  Competitive and pricing research (Storj, Sia, Filecoin, Backblaze B2,
  Wasabi, AWS S3) written into business/research/ with sources. Use when
  building the deck, pricing the product, comparing decentralized storage,
  or when the user says market, competitors, or TAM.
---

# Market research

Write dated notes under `business/research/`. Every claim needs a source URL and access date.

## Default comparison set

- Centralized cheap object: AWS S3 standard, Backblaze B2, Wasabi
- Decentralized / distributed: Storj, Sia, Filecoin
- Adjacent: Cloudflare R2 (egress), iDrive e2

## Output files

Create or update:

- `business/research/competitors.md` — one subsection per name: product, pricing (storage/egress), durability claim, trust model, who pays whom
- `business/research/pricing.md` — our Phase 1 platform-defined rates vs the set above (credits, not real money, but show the equivalent)
- `business/research/positioning.md` — one paragraph: why a zero-knowledge home-PC network is not "another Storj"

## Rules

- Quote list prices; do not invent. If a price is unknown, write "unknown" and the URL you tried.
- Separate **durability marketing** from **auditable repair**. Our pitch is the on-camera kill-6-of-16 test.
- Do not scrape behind logins. Public pages and docs only.
