# Deck outline (W12)

Slides later. Numbers come from `business/updates/demo-metrics-2026-09-22.json`. If that file and this outline disagree, the JSON wins.

## 1. Title

Idle disks are the unused data center.

One line under: encrypted object storage on ordinary computers. Solo-built, AI-assisted, demoable this week.

## 2. The waste

Most of the world’s NVMe sits in homes and offices at ~0% utilization. Cloud object storage is priced for centralized capex and egress (S3 Standard $0.023/GB-month + ~$0.09/GB out, us-east-1 / pricing-page example, accessed 2026-09-22 — `research/competitors.md`).

## 3. The product

`dsp put` / `dsp get`. Client: Argon2id → AES-256-GCM → 16 MB chunks → Reed–Solomon 10+6. Control plane: metadata, scheduler, health, repair. It never stores file contents and cannot decrypt them.

## 4. Why this is not Storj

Storj encrypts at the Uplink; the Satellite is still trusted for metadata, audit, repair, and billing. Token networks (Sia, Filecoin) make the customer run a renter or a deal flow. We sell a hosted control plane whose **own database dump decrypts nothing**, and we show repair instead of quoting eleven 9s. Full paragraph: `research/positioning.md`.

## 5. Live demo (the raise scene)

On camera:

1. `dsp put Vacation.mp4` (31 MiB fixture; two chunks at 16/16).
2. Kill 6 of 16 holders.
3. `dsp get` is byte-identical while those nodes are down.
4. Fleet returns to 16/16.
5. `pg_dump` has no passphrase and no plaintext.

**Measured (2026-09-22, `demo-metrics-2026-09-22.json`):**

- `file_bytes`: 32 505 856
- `killed_nodes`: 6
- `download_ok_during_failure`: true
- `repair_latency_seconds`: 296 (clock starts at kill; includes the 5 min `offline` grace)
- `final_healthy_per_chunk`: 16
- `db_dump_decrypts_nothing`: true

Do not put a number on this slide that is not in the JSON.

## 6. How repair actually works

Heartbeat 10s → suspect 30s → offline 5 min → NATS `node.offline` → reconstruct ciphertext (any 10 of 16) → place under region/ASN/owner caps (3/3/2) onto spare nodes in a 24-agent fleet. Invariant: every committed chunk stays ≥ 12 healthy (alert < 13).

## 7. What we measured besides the take

- W11 2h soak (`updates/soak-2026-09-21.json`): 34 rounds, floor 10/16 held. Kill withheld while already ≤12. Pre-existing cap violations on a dirty volume; wiped before this demo.
- W11 security review: no medium+ on keys, tickets, receipts, pipeline.
- CI: hermetic invariants + red-team job every PR; nightly Compose `chaos-m4`.

## 8. Why now

Home NVMe is cheap. NATS / Postgres / Go are boring on purpose. One person plus AI agents shipped M0–M4 on the twelve-week solo calendar in `docs/10-mvp-roadmap.md`.

## 9. Scope honesty

Shipped: metadata, gateway (API keys), agent, tickets/receipts, RS 10+6, scored scheduler, healthmon, repair, chaos harness, fleet visualizer, this demo.

Not shipped, not promised this raise: ledger/billing, customer dashboards, JWT, installers, rebalancing, S3, real money.

## 10. The plan and the ask

Solo-builder M0–M4 is the investable artifact. Next: record the video, send this outline, land a waitlist. Ask: one intro to an angel who has written a storage or infra check and will watch a 90-second terminal + visualizer recording.
