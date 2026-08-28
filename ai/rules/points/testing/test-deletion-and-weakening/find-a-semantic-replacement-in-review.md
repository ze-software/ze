---
kind: directive
level: MUST
stage:
---
**Detection:** `/ze-review` step 0 runs the native structural weakening audit.
For semantic replacement, `/ze-review` step 7 (removed-behavior audit) MUST
verify that every assertion the diff replaces still has coverage elsewhere.
When reviewing a test edit that changes WHAT is asserted (not just adding new
assertions), ask: "is the old behavior still tested?"

**A row in `test/weakened.md` MUST be written for the ONE weakening in hand, and
the commit MUST carry that file.** The row is self-service: the agent that
weakened the test writes its own justification, so the only thing that makes it
safe is a human reading it. The file is replaced per commit: it holds the rows of
one change, and git history holds the rest.
