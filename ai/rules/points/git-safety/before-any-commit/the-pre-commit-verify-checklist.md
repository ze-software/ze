---
kind: fence
level: MUST NOT
stage:
---
```
[ ] 0. `scripts/dev/verify-status.sh check`. FRESH -> MUST NOT run `make ze-verify` or `make ze-verify-changed` again; note timestamp. STALE -> continue only if the table above says verification applies.
[ ] 1. `make ze-verify` (foreground, largest timeout your harness allows, never killed early) only when status is STALE and the table above says YES. On failure read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open that group's `tmp/verify/<nn>-<stage>.log`.
[ ] 2. Failure from current work, or any failure that blocks this commit's goal: fix + re-run. Any other failure, and never a deterministic structural gate, which is fixed before any commit (see "Structural Gates Are Never Known-Red" below): write its spec, finish this commit, ask Thomas whether that spec runs (`ai/rules/completion.md`). A `plan/known-failures/` shard is for a failure you tried and could not reproduce, and it carries the reproduction attempt and the next step.
```
