# Verification debt -- commit session c0beca40

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | c0beca40 | rfc: rfc-verdict-fix-bgp AC-C2 records on the units it changed | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-10-18T10:55:37Z) | open |
