---
kind: directive
level: MUST
stage:
---
**Read the working tree's SPREAD before starting new work, and land what is already finished first.** `make ze-working-tree-check` reports the changed paths grouped by area. More than one area in flight means a chunk is waiting that could already be committed, and it MUST be landed before the next piece starts. The cost of getting this wrong compounds: an unrelated fix folded into a closing commit costs that commit its single focus and its review its scope, and it restarts gates that were already green (`ai/rules/rule-precedence.md`).
