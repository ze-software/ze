---
kind: table
level:
stage:
---
| Rule | Detail |
|------|--------|
| Always a destination spec | Every deferral names a `plan/spec-*.md` that exists on disk, whatever its Status. Only `cancelled` / `user-approved-drop` may name no spec |
| No prose destinations | "later", "future work", "a follow-up", "TBD" are not destinations. A destination is a filename |
| No vague What | "Edge cases" is not acceptable. Name the specific case |
| Record immediately | Do not batch. Record when the decision is made, not at commit time |
| Review at session end | Live rows are expected and fine. Check that each still names a real home, and close only the ones whose work actually landed |
