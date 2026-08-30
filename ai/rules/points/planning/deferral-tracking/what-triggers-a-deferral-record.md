---
kind: directive
level: MUST
stage:
---
**Each trigger below MUST produce a row, recorded when the decision is made rather than at commit time.**

| Trigger | Action |
|---------|--------|
| Deciding work is "out of scope" | Record with reason |
| Moving work to another spec | Record with destination spec |
| Skipping a task item from a spec | Record with reason |
| Postponing for any reason | Record with reason |
| User asks to skip something | Record (user-requested, still tracked) |
| Finding a problem the work in hand does not depend on | Write its spec now, record the row against it, close the work in hand, then ask Thomas whether the spec runs (`completion.md`) |
