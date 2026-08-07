---
kind: directive
level:
stage:
---
**`deferral_shard_removal_problems` (`scripts/dev/commit_helper.py`) refuses the removal, so this is not honor-system.** It reads the shard at HEAD and BLOCKS when any row is non-terminal. It has to block rather than warn: every other signal over these rows folds across the `plan/deferrals/` DIRECTORY, so deleting a live-bearing shard LOWERS their counts instead of raising them, and the forbidden action is the one that silences every observer of the rows it destroys (`ai/rules/evidence.md`).
