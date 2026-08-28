---
kind: note
level:
stage:
---
Losing a failure line to `| head` means re-running the whole thing.
`./le verify worktree*` writes to `tmp/ze-verify.log` (+ `-failures.log`
summary) by default. Override with `ZE_VERIFY_LOG=tmp/ze-verify-$$.log`
to avoid collisions between concurrent sessions. Read logs with the
Read tool, with `offset`/`limit` for paging.
