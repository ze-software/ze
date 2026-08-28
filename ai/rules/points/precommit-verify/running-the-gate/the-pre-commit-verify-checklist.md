---
kind: fence
level: MUST NOT
stage:
---
```
[ ] 0. `./le verify-status check`. FRESH -> MUST NOT run `./le verify worktree` or `./le verify worktree` again; note timestamp. STALE -> continue only if the table above says verification applies.
[ ] 1. `./le verify worktree` (foreground, largest timeout your harness allows, never killed early) only when status is STALE and the table above says YES. On failure read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open the stage log its `detail-log` field names in `tmp/ze-verify-failures.json`. Each run keeps its own directory, so a path from an earlier run is a different run's evidence.
[ ] 2. Failure from current work, or any failure that blocks this commit's goal: fix + re-run. Any other failure, and never a deterministic structural gate, which is fixed before any commit (see "Structural Gates Are Never Known-Red" below): write its spec, finish this commit, ask Thomas whether that spec runs (`ai/rules/completion.md`). A `plan/known-failures/` shard is for a failure you tried and could not reproduce, and it carries the reproduction attempt and the next step.
```
