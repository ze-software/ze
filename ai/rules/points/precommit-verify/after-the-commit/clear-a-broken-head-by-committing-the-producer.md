---
kind: directive
level: MUST
stage:
---
**`./le repository tracked-build check` is the one entry whose red is cleared BY
a commit rather than before one, so a broken HEAD MUST be fixed by committing the
producer a previous commit left behind.** Every other gate on the list is fixed in
the working tree first.
**`--broken-head-fix "<reason>"` is that commit's route through, and it MUST NOT
be reached for while any other structural gate is red**: `internal/le/commit`
accepts it only when tracked-build is the ONLY structural red.
**After the script runs, `./le repository tracked-build check` MUST be run again
and confirmed green.** If it is not, HEAD is still broken for everybody who builds
it. The gate's design is `docs/architecture/testing/tracked-build-gate.md`.
