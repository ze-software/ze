# Verification debt -- commit session 1beebdd6

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-22 | 1beebdd6 | spec: le shared-build launcher (M22) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-22 | 1beebdd6 | spec: le shared-build launcher (M22) | native structural checks (red) | le arch tier check and le arch iface-resolution both exit 0 in this run; the recorded red is stale and this commit carries a spec only | open |
