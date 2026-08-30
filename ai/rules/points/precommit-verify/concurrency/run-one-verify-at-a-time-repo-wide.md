---
kind: directive
level: MUST
stage:
---
**One `./le verify worktree` (or `./le test-chaos`) MUST run at a time
repo-wide.** Parallel runs share the build cache, the ports and the test
binaries. Every heavy native action is admitted through `./le job run`, which runs
a job now, queues it, or attaches it to an equivalent run, so a second verify
blocks automatically. The admission registry, its slot and stall settings, and
what a job entry holds are `docs/contributing/running-commands.md`.
