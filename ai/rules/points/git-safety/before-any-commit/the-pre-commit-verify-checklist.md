---
kind: fence
level: MUST NOT
stage:
---
```
[ ] 0. `scripts/dev/verify-status.sh check`. FRESH -> MUST NOT run `make ze-verify` or `make ze-verify-changed` again; note timestamp. STALE -> continue only if the table above says verification applies.
[ ] 1. `make ze-verify` (240s) only when status is STALE and the table above says YES. On failure read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open that group's `tmp/verify/<nn>-<stage>.log`.
[ ] 2. Failure from current work: fix + re-run. Pre-existing: fix after primary task in separate commit; if >10 min, log to `plan/known-failures/` (one `<make-target>-<test-name>.md` shard per failure).
```
