---
name: session-brief
description: >-
  Reads STATUS.md, recent git history, and open branches, then reports where
  the solo builder left off and proposes this session's tasks. Use at the
  start of every coding session, when the user says "where were we",
  "session brief", or "what should I work on".
---

# Session brief

Run this at the start of every session. Do not write code until the brief is delivered.

## Steps

1. Read [`STATUS.md`](STATUS.md) and [`AGENTS.md`](AGENTS.md).
2. Run:
   - `git status -sb`
   - `git log -8 --oneline`
   - `git branch -vv`
3. Skim the current milestone section in [`docs/10-mvp-roadmap.md`](docs/10-mvp-roadmap.md) (solo builder track).
4. Reply with this exact shape:

```markdown
## Where we are
- Milestone:
- Last shipped:
- Dirty tree:

## Proposed this session (pick 1–3)
1. ...
2. ...
3. ...

## Do not touch
Deferred solo-track items (ledger, dashboards, JWT, installers, rebalancing, S3).
```

5. Prefer finishing an in-progress slice over starting a new service.
6. After the user confirms, switch to Plan mode if the work spans more than one package.
