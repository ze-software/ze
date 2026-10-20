# Verification debt -- commit session ae4ca77a

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-20 | ae4ca77a | plan(prefix-sid): phase 4 interop run on Linux, both scenarios red | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-20 | ae4ca77a | plan(prefix-sid): phase 4 interop run on Linux, both scenarios red | native structural checks (red) | plan-only commit; ./le arch tier check and ./le arch iface-resolution both exited 0 on this tree at 2026-10-20 | open |
