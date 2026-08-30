---
kind: directive
level: MUST NOT
stage:
---
- **Step 0. Run `./le verify status check`. A FRESH answer MUST NOT be answered with another run; note its timestamp. A STALE answer continues only when the table above says verification applies.**
- **Step 1. Run `./le verify worktree` in the foreground, with the largest timeout your harness allows, and never kill it early. On failure you MUST read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open the stage log its `detail-log` field names in `tmp/ze-verify-failures.json`.**
- **Step 2. A failure from the current work, or any failure blocking this commit's goal, MUST be fixed and re-run. Any other failure gets its spec, this commit finished, and the question to Thomas whether that spec runs (`ai/rules/completion.md`). A deterministic structural gate is NEVER on this branch and MUST be fixed before any commit. A `plan/known-failures/` shard is only for a failure you tried and could not reproduce, and it carries the reproduction attempt and the next step.**
