# W12 demo shot list

Screen recording is a human step. The agent run on 2026-09-22 produced `demo-metrics-2026-09-22.json` and the terminal log. Use that JSON; do not type numbers from memory.

## Fixture

| Field | Value |
|---|---|
| Path | `demo:/Vacation.mp4` |
| Logical bytes | 32 505 856 (31 MiB) |
| Plaintext SHA-256 | `469938efa3b6bb7da96061088cf41a0443542a47102a2029f84d78fd4b6ff709` |
| Ciphertext `content_sha256` | `4931b9ae49ebb7e0395ce2b2a87be73ce960695cac943f1b6d4b9fd34d6325a3` |
| Chunks | 2 × 16/16 before kill |
| Passphrase | not for the recording (env only) |
| CA | `DSP_CA_FILE=.local/ca.crt` |

31 MiB, not 32: 32 MiB of plaintext encrypts past two 16 MiB ciphertext chunks (GCM segment tags). Three chunks × 16 fragments = 48 placements, which is exactly the 24-node × 2 probation quota, so the third chunk stuck at 15/16. Two chunks is the clean 16/16 start.

## Beats (narrate, then cut)

1. **Fleet up.** Visualizer at `http://127.0.0.1:8080/demo/` — 24 green nodes, two chunk pills 16/16. One line: “idle disks, 24 agents, eight regions.”
2. **Put.** Terminal: `dsp put Vacation.mp4`. Show the committed JSON. “Encrypted on the laptop. Control plane got hashes and wrapped keys, not the movie.”
3. **Kill 6 of 16.** `make demo` (or `harness kill -n 6`) with the visualizer on the other half of the screen. Six cards go red. Narrate the M4 claim.
4. **Get during failure.** `dsp get` + `cmp` against the original. Byte-identical. This is the shot that matters.
5. **Wait for heal.** Health monitor: 30s suspect, 5 min silent → offline → repair. Chunk pills go amber/red, then back to 16/16. This run: **296 seconds** from kill to 16/16 (`demo-metrics-2026-09-22.json`).
6. **Dump.** `pg_dump` on screen, grep for the passphrase / a plaintext marker, show no hits. “The platform’s own database decrypts nothing.”
7. **Metrics file.** Open `business/updates/demo-metrics-2026-09-22.json`. Leave it on screen.

## What the harness already did

`make demo` with `DSP_GET_CMD` set: killed six holders, ran get+cmp during failure, waited until health dropped below 16/16 and returned, dumped Postgres, ran invariants, wrote the JSON.

## Do not

- Fake a repair latency. If the JSON says the run failed, the recording stops.
- Show API keys, the passphrase, or `.local/api.key`.
- Claim ledger, S3, or dashboards.
