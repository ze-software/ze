---
kind: note
level:
stage:
---
One `./le verify worktree` (or `./le test-chaos`) at a time repo-wide:
parallel runs share build cache, ports, and test binaries. Every heavy native
action is admitted by `./le job run`, which runs a job now, queues it behind the
jobs already in flight, or attaches it to an equivalent run. A second verify
therefore blocks automatically. `internal/le/lejob` owns `ZE_RUN_SLOTS`; the native
`internal/le/gotoolchain` policy derives the per-process `GOMAXPROCS` ceiling.

Admission state is one file per running job,
`tmp/.ze-jobs/<label>.<pid>.job`. `tmp/.ze-verify.lock` is dead:
nothing takes that flock any more. A job started INSIDE another job's
slot runs straight through instead of queueing behind its own parent,
which is how every stage of `./le verify current mode full` runs.
