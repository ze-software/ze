# Verification debt -- commit session baa5cad6

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | baa5cad6 | spec: verify-gate-warm-timed-first-red closure review round 2 (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=3, at 2026-10-21T17:22:47Z) | open |
| 2026-10-21 | baa5cad6 | spec: verify-gate-warm-timed-first-red closure review round 2 | native structural checks (red) | arch tier/check is foreign (internal/le/setup/dockerhost/static.go runs docker outside interoplab); staticcheck parts 2 and 3 of 6 were red at ee108f4731 from build-cache entries evicted mid-run, not from this change; this commit carries a spec and a doc paragraph only | open |
| 2026-10-21 | baa5cad6 | spec: close spec-verify-gate-warm-timed-first-red | native structural checks (red) | arch tier/check is foreign (internal/le/setup/dockerhost/static.go runs docker outside interoplab); staticcheck parts 2 and 3 of 6 were red at ee108f4731 from build-cache entries evicted mid-run; this commit only removes a spec and its handover | open |
