---
kind: note
level:
stage:
---
> **validate-spec.sh is fixed** (2026-07-09, spec-followup-hooks) and kept
> standalone. It previously matched only the Unicode arrow `→` in the Wiring Test
> table, so an ASCII `->` spec produced an empty `grep` pipeline that exited 1 and
> `set -e` aborted the script before the output stage, swallowing every queued
> error (a silent non-blocking exit 1). Both arrow conventions are now accepted
> and the `WIRING_ROWS=` assignment is guarded with `|| true`, so the script
> always reaches its verdict: exit 2 for a structurally invalid spec, exit 0
> otherwise. A survey over all `plan/spec-*.md` (spec-followup-hooks AC-4)
> confirmed zero crashes and zero arrow false-positives. It stays out of the
> dispatcher (see spec Key Design Decisions). Covered by
> `scripts/dev/hook-fixture-check.py` (`validate-spec-*`).
