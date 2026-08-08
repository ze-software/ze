---
kind: table
level:
stage:
---
| What you are about to do | Do this instead |
|---|---|
| Add a `plan/known-failures/` entry for a test that fails deterministically | Diagnose it (see "Diagnosis Before Fix" below) and fix the root cause |
| Write "pre-existing, tracked in known-failures" in a report | It is yours: "pre-existing" describes when it started, not whose it is. Blocks your goal, fix it now; does not, spec it, close, ask |
| List failures in an Executive Summary as though listing were the deliverable | Every listed failure is either fixed, or has a named reason you are blocked on it |
| Note that a tool is broken and work around it | Fix the tool. You just proved it does not work |
| Record an inert config surface, a dead registration, or an unwired symbol | Wire it, delete it, or reject the config: pick one and do it |
