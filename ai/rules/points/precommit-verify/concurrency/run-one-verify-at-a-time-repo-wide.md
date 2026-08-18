---
kind: note
level:
stage:
---
One `make ze-precommit-verify*` (or `ze-chaos-verify`) at a time repo-wide --
parallel runs share build cache + ports + `bin/ze` processes and
trash each other. Every heavy target is wrapped by
`scripts/dev/ze-run.sh`, which runs a job now, queues it behind the
jobs already in flight, or attaches it to an equivalent run;
`scripts/dev/verify-lock.sh` is an alias for it, so a second verify
blocks automatically. The slot count is `ZE_RUN_SLOTS` in the
`Makefile`, beside the `GO_TEST_PROCS` ceiling that sizes each job.

Admission state is one file per running job,
`tmp/.ze-jobs/<label>.<pid>.job`. `tmp/.ze-verify.lock` is dead:
nothing takes that flock any more. A job started INSIDE another job's
slot runs straight through instead of queueing behind its own parent,
which is how every stage of `ze-precommit-verify` runs.
