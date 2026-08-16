---
kind: note
level:
stage:
---
> **validate-spec.sh is standalone.** It accepts both `→` and `->` in the
> Wiring Test table. The `WIRING_ROWS=` assignment is guarded with `|| true`, so
> the script always reaches its verdict: exit 2 for a structurally invalid spec,
> exit 0 otherwise.
> It stays out of the dispatcher (see spec Key Design Decisions). Covered by
> `scripts/dev/hook-fixture-check.py` (`validate-spec-*`).
