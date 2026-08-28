---
kind: directive
level: MUST NOT
stage:
---
**`deferral_shard_removal_problems` (`internal/le/commit`) refuses the removal, so this is not honor-system: a shard MUST NOT be removed while any row is non-terminal.** It reads the shard at HEAD and BLOCKS when any row is non-terminal. It has to block rather than warn: every other signal over these rows folds across the `plan/deferrals/` DIRECTORY, so deleting a live-bearing shard LOWERS their counts instead of raising them, and the forbidden action is the one that silences every observer of the rows it destroys (`ai/rules/evidence.md`).
