# Verification debt -- commit session 6e4caf8d

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-09 | 6e4caf8d | pppoe: every PADO and PADS carries the mandatory Service-Name tag (+3 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-09-05T00:11:54Z) | open |
| 2026-09-09 | 6e4caf8d | pppoe: every PADO and PADS carries the mandatory Service-Name tag (+1 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
