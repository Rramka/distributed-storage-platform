# Pricing

Phase 1 uses **internal credits only** (no real money). The card below is the notional equivalent of the Phase 1 defaults in `docs/09-billing-ledger.md`, compared with public list prices accessed **2026-09-22**.

Peg used for display: **1 CRD ≈ $0.01** (the spec calls the peg configurable and irrelevant to ledger correctness).

## Phase 1 rate card (credits)

From `docs/09-billing-ledger.md` § Customer pricing and § Provider earnings:

| Meter | Credits | Notional USD (1 CRD ≈ $0.01) |
|---|---|---|
| Customer storage | 3.0 CRD / GB-month (logical bytes, 10+6 included) | $0.030 / GB-month ≈ $30.72 / TB-month (1024 GB) |
| Customer download egress | 1.0 CRD / GB | $0.010 / GB ≈ $10.24 / TB |
| Customer upload | free | $0 |
| Customer API requests | free (rate-limited) | $0 |
| Provider storage earning | 1.5 CRD / GB-month × R (reliability, clamped 0–1.2) | $0.015 / GB-month at R=1 |
| Provider egress earning | 0.5 CRD / GB × R | $0.005 / GB at R=1 |

Charged storage is **logical file size**, not the ~1.6× Reed–Solomon footprint. The expansion is platform COGS, covered by the gap between customer 3.0 and provider 1.5 CRD/GB-month.

These are not live SKUs. The ledger service is deferred on the solo track.

## Versus public list prices (same access date)

Per-GB-month storage and per-GB egress, hot/standard class, no committed-use discounts. Full citations live in `competitors.md`.

| Vendor | Storage / GB-month | Egress / GB | Notes |
|---|---|---|---|
| DSP Phase 1 (notional) | $0.030 | $0.010 | Credits; 10+6 included |
| AWS S3 Standard (us-east-1) | $0.023 first 50 TB | $0.09 (worked example, first 10 TB to internet) | + request charges |
| Backblaze B2 | $0.00679 ($6.95/TB-30d) | $0 up to 3× stored; then $0.01 | First 10 GB free |
| Wasabi | $0.0078 ($7.99/TB) | $0 advertised | 1 TB monthly minimum |
| Cloudflare R2 Standard | $0.015 | $0 | Class A/B request charges |
| iDrive e2 | $0.006 | $0 up to 3×; then $0.01 | $6 minimum |
| Storj Standard (from 2026-07-01) | $0.00684 ($7/TB) | $0.00684 ($7/TB) | 30-day min duration |
| Sia (network average) | ~$0.0029 ($3/TB incl. 3×) | host-set | Customer runs `renterd` |
| Filecoin Onchain Cloud | $0.00244 ($2.50/TiB per copy) | up to $0.014 ($14/TiB CDN) | + $0.12/dataset-month proofs |

TB conversions use 1024 GB where the vendor does (Wasabi FAQ, S3). Storj bills per TB as published.

## How to talk about this

- We are **not** the cheapest TB. S3 Standard storage is lower than our notional 3.0 CRD/GB-month. B2, Wasabi, e2, and Storj are lower still.
- We **are** cheaper than S3 on egress at the notional 1.0 CRD/GB vs $0.09/GB, and in the same band as B2 overage / Storj $7/TB.
- The raise claim is durability you can watch, not a price war with Wasabi. If an investor wants a price, show this card and say Phase 2 (real payments) can move the peg; the ledger math stays integer CRD.

## Sources

All competitor rows: `competitors.md` (accessed 2026-09-22). DSP rows: `docs/09-billing-ledger.md` (customer 3.0 / 1.0 CRD, provider 1.5 / 0.5 CRD, peg example 1 CRD ≈ $0.01).
