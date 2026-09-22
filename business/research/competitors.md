# Competitive research

Access date for every URL below: **2026-09-22**. Quote list prices; do not treat durability marketing as an audited repair demo. Our pitch is the on-camera kill-6-of-16 test.

## Snapshot

| Name | Product | Storage (list) | Egress (list) | Durability claim | Trust model | Who pays whom |
|---|---|---|---|---|---|---|
| AWS S3 Standard | Centralized object | $0.023/GB-month first 50 TB, us-east-1 | $0.09/GB first 10 TB to internet (worked example on the pricing page) | 11 nines (marketing; AWS-operated AZs) | AWS holds keys unless the customer brings KMS/SSE | Customer → AWS |
| Backblaze B2 | Centralized object | $6.95/TB per 30 days; first 10 GB free | Free to 3× stored bytes; then $0.01/GB. Unlimited free via listed CDN/compute partners | Backblaze-operated; no public 11-nines math | Provider-side encryption optional; B2 is not zero-knowledge | Customer → Backblaze |
| Wasabi | Centralized hot object | $7.99/TB-month all regions (from 2026-07-01); 1 TB monthly minimum | Free (fair-use in ToS) | Centralized; no independent repair demo | Provider holds the store | Customer → Wasabi |
| Cloudflare R2 | Centralized object, zero egress | $0.015/GB-month Standard; $0.01/GB-month Infrequent Access | Free to the internet | Cloudflare-operated | Provider-side; R2 is not zero-knowledge | Customer → Cloudflare |
| iDrive e2 | Centralized S3-compatible | $0.006/GB-month; $6/TB minimum on pay-as-you-go | Free to 3× storage; then $0.01/GB | Centralized | Provider holds the store | Customer → iDrive |
| Storj DCS | Distributed node network + Satellite | Standard $7/TB-month; Advanced $10/TB-month (from 2026-07-01) | $7/TB both tiers | 11 nines + 99.95% availability (Storj marketing). RS across ~80 pieces | Client-side encryption by default; **Satellite is trusted for metadata, audit, repair, billing** | Customer → Storj Labs; Storj pays storage node operators |
| Sia | Open host marketplace (`renterd`) | Network average **~$3/TB-month including 3× redundancy** (host-set, in SC). Docs also cite older ~$2/TB-month | Host-set bandwidth; recommended host egress >$5/TB | Host contracts + renter-side repair. No single operator SLA | Client encryption; renter must keep `renterd` online to maintain contracts | Renter → hosts in Siacoin |
| Filecoin | Deal marketplace + proofs | No single list price. Filecoin Onchain Cloud docs: **$2.50/TiB-month per copy** + $0.12/dataset-month proving. Paid-deal dashboard samples (Jul 2026): $5–$41.58/TiB-month | Optional CDN (Filecoin Beam) up to $14/TiB | Proof of spacetime / PDP; retrieval is a separate market | On-chain deals; Fil+ DataCap is a social-trust layer (10× QAP) | Client → storage provider (FIL / USDFC); SPs also earn block rewards |

## AWS S3 Standard

- **Product:** Hyperscaler object store. Default hot class.
- **Pricing:** Storage $0.023 / $0.022 / $0.021 per GB-month for first 50 TB / next 450 TB / over 500 TB in US East (N. Virginia). PUT $0.005 per 1,000 and GET $0.0004 per 1,000 appear in the official worked examples. Data transfer OUT to internet is $0.09/GB in the Europe (Ireland) example on the same page.
- **Durability claim:** Eleven 9s is AWS marketing for multi-AZ classes; not an on-camera independent-node kill test.
- **Trust model:** AWS can read objects unless the customer manages keys. Control plane and bytes live in one operator.
- **Sources:** [aws.amazon.com/s3/pricing](https://aws.amazon.com/s3/pricing/) (accessed 2026-09-22).

## Backblaze B2

- **Product:** Always-hot S3-compatible object storage.
- **Pricing:** $6.95/TB per 30 days from 2026-05-01 (was $6/TB). API classes A/B/C free. Egress free up to 3× average monthly storage; $0.01/GB after. Partner CDN/compute egress listed as unlimited free.
- **Durability claim:** Operator durability; no public independent-node repair demo.
- **Trust model:** Centralized. Optional SSE; the platform can be given keys.
- **Sources:** [backblaze.com/cloud-storage/pricing](https://www.backblaze.com/cloud-storage/pricing), [transaction-pricing](https://www.backblaze.com/cloud-storage/transaction-pricing), [Backblaze 2026-03-17 pricing update](https://noise.getoto.net/2026/03/17/backblaze-pricing-and-product-updates/) (accessed 2026-09-22).

## Wasabi

- **Product:** Hot cloud object storage, capacity-only bill.
- **Pricing:** $7.99/TB-month all regions from 2026-07-01. 1 TB monthly minimum ($7.99 even if you store less). Ingress, egress, and API requests advertised free.
- **Durability claim:** Centralized; no independent repair demo.
- **Sources:** [wasabi.com/pricing](https://wasabi.com/pricing), [docs.wasabi.com May 2026 pricing](https://docs.wasabi.com/docs/may-2026-wasabi-pricing), [minimum charge FAQ](https://docs.wasabi.com/docs/how-does-wasabis-monthly-minimum-storage-charge-work) (accessed 2026-09-22).

## Cloudflare R2

- **Product:** S3-compatible object storage whose pitch is **zero egress**.
- **Pricing:** Standard $0.015/GB-month; Infrequent Access $0.01/GB-month. Class A $4.50/million (Standard), Class B $0.36/million. IA retrieval $0.01/GB and 30-day minimum. Free tier: 10 GB-month + 1M Class A + 10M Class B (Standard only).
- **Durability claim:** Cloudflare-operated.
- **Trust model:** Not zero-knowledge. Complementary to us on price; opposite on “dump the operator DB.”
- **Sources:** [developers.cloudflare.com/r2/pricing](https://developers.cloudflare.com/r2/pricing/) (updated 2026-08-07; accessed 2026-09-22).

## iDrive e2

- **Product:** Budget S3-compatible object storage (adjacent to B2/Wasabi).
- **Pricing:** $0.006/GB-month. Pay-as-you-go minimum $6 for 1 TB. Free egress up to 3× active storage; $0.01/GB beyond.
- **Sources:** [idrive.com/s3-storage-e2/pricing](https://www.idrive.com/s3-storage-e2/pricing), [faq-pricing](https://www.idrive.com/s3-storage-e2/faq-pricing) (accessed 2026-09-22).

## Storj DCS

- **Product:** Encrypted, erasure-coded object storage on independently operated nodes, with Storj Labs **Satellites** for metadata, audit, repair, and billing. Closest architectural cousin.
- **Pricing (simplified, 2026-07-01):** Standard $7/TB-month storage + $7/TB egress. Advanced (region-specific / SOC 2) $10/TB-month + $7/TB egress. 30-day minimum duration, 50 kB minimum object, usage rounded up to whole GB. Public pricing page lists a $5 monthly minimum; an official forum post dated for the same change says the paid-account minimum becomes $50 (STORJ-token payers exempt). Treat the minimum as **disputed across official pages** until Storj reconciles it.
- **Durability claim:** Eleven 9s, 99.95% availability, ~80 pieces, automated audit/repair ([storj.io availability](https://www.storj.io/object-storage/cloud-storage-availability/), [storj.dev intro](https://storj.dev/learn)).
- **Trust model:** Uplink encrypts by default (AES-GCM or Secretbox). Storage nodes cannot decrypt. The Satellite **does** see encrypted metadata, picks nodes, audits, repairs, and bills. Users *may* run their own Satellite. This is “zero-knowledge nodes,” not “zero-knowledge operator.”
- **Who pays whom:** Customer pays Storj Labs; Labs pays node operators.
- **Why we are not another Storj:** We demo kill-6-of-16 on camera and dump the control-plane DB. Storj’s durability number is a model; the Satellite remains a trusted control plane. See `positioning.md`.
- **Sources:** [storj.dev/dcs/pricing/simplified](https://storj.dev/dcs/pricing/simplified), [storj.io/pricing](https://www.storj.io/pricing), [pricing change FAQs](https://www.storj.io/pricing/change-FAQs/), [forum SNO impact thread](https://forum.storj.io/t/important-update-upcoming-storj-customer-pricing-adjustments-and-sno-impact/31827), [encryption](https://storj.dev/learn/concepts/encryption-key/how-encryption-is-implemented) (accessed 2026-09-22).

## Sia

- **Product:** Host marketplace. The customer runs `renterd`, forms blockchain contracts, and must keep the renter online to renew and repair.
- **Pricing:** Current docs: network average **~$3 per TB per month, including 3× redundancy**, paid in Siacoin. Legacy renter docs still say ~$2/TB-month. Host configuration docs recommend ~$1/TB-month storage and >$5/TB egress for hosts. Prices move with the host market and SC.
- **Durability claim:** Contract-level redundancy; the renter is the repair loop. Not a hosted SLA.
- **Trust model:** Client-side encryption. No central operator to dump — and no single party that will run repair if you turn `renterd` off.
- **Sources:** [docs.sia.tech store-your-data/about-renting](https://docs.sia.tech/store-your-data/about-renting.md), [legacy renterd about-renting](https://docs.sia.tech/legacy/renting/renterd/about-renting), [host configuration](https://docs.sia.tech/provide-storage/configuring-your-host) (accessed 2026-09-22).

## Filecoin

- **Product:** Storage deals with cryptographic proofs. Retrieval and “hot” serving are extra (markets, aggregators, Onchain Cloud).
- **Pricing:** Unknown as a single SKU. [Filecoin Onchain Cloud storage costs](https://docs.filecoin.cloud/developer-guides/storage/storage-costs/): $2.50/TiB-month per copy + $0.12/data set/month proving; CDN egress up to $14/TiB. [Paid deals dashboard](https://filecointldr.io/paid-deals-dashboard) (as of Jul 2026): CIDgravity $5/TiB-month, Akave $14.99, Lighthouse $41.58 (includes gateway/CDN). Fil+ verified deals are often discounted because SPs get a 10× quality-adjusted-power multiplier.
- **Durability claim:** Proofs that a provider stored bytes at a time; not a live 16-way self-heal demo.
- **Trust model:** On-chain deals plus Fil+ social trust (allocators → DataCap).
- **Sources:** URLs above plus [docs.filecoin.io Filecoin Plus](https://docs.filecoin.io/getting-started/how-storage-works/filecoin-plus) (accessed 2026-09-22).

## How to read this against DSP

DSP Phase 1 has **no real money** (internal CRD only). The comparison is for the raise: centralized cheap object (S3, B2, Wasabi, R2, e2) wins on ops simplicity; Storj is the honest distributed peer; Sia/Filecoin are crypto-native markets. DSP’s differentiator is not a cheaper TB. It is **auditable repair** plus a control plane that decrypts nothing. See `pricing.md` and `positioning.md`.
