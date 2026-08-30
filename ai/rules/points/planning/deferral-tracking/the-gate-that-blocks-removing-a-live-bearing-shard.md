---
kind: directive
level: MUST NOT
stage:
---
**A shard MUST NOT be removed while any row in it is non-terminal, and no gate refuses the removal, so this one is on you.** Every signal over these rows folds across the `plan/deferrals/` DIRECTORY, so deleting a live-bearing shard LOWERS the counts instead of raising them: the forbidden action is the one that silences every observer of the rows it destroys (`ai/rules/evidence.md`).
