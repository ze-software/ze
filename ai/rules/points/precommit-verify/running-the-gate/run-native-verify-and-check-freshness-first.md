---
kind: directive
level: MUST NOT
stage:
---
**Freshness MUST be checked before any verify target, and a FRESH answer MUST NOT
be answered with another run.** `./le verify status check` with no arguments asks
about the whole tree; `check <PATH>...` asks about the named paths alone. Treat
`FRESH(full)` and `FRESH(changed)` alike for commit preparation. A full run on a
`FRESH(changed)` tree is permitted when the work explicitly needs the full gate.
**A per-stage log MUST be reached through the `detail-log` field of the failure
index, never by constructing a path**: each run gets its own directory under
`tmp/verify/`. What each mode covers and what each run writes is
`docs/architecture/testing/verify-freshness-scope.md` and
`docs/contributing/running-commands.md`.
