---
name: investor-update
description: >-
  Turns the week's commits, demo metrics, and chaos results into a short
  investor update in business/updates/. Use on the Sunday cadence, when
  the user says investor update, weekly update, or fundraising email.
---

# Investor update

Write `business/updates/YYYY-MM-DD.md`. Keep it under one page. Send from week 2 onward even if the demo is not ready — cadence is the asset.

## Inputs

- `git log --since='7 days ago' --oneline`
- `STATUS.md`
- Latest `business/updates/demo-metrics-*.json` if any
- Latest chaos report if any

## Template

```markdown
# DSP weekly — YYYY-MM-DD

## This week
- One sentence on the milestone (M0–M4).
- 2–4 bullets of shipped work. Outcome, not activity.

## Demo metric
- Repair latency / download-during-failure / or "not yet — target week 9".

## Next week
- The single most important risk and what we will do about it.

## Ask
- One concrete ask (intro, design partner, or none).
```

## Rules

- No vapor. If M4 is not green, do not claim self-healing works.
- Do not attach secrets, dumps, or customer data.
- Voice: founder writing to a busy angel. Short sentences.
