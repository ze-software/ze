# Verification debt -- commit session cfbf4f7e

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-19 | cfbf4f7e | interoplab: docker tests take the run's deadline, no bound of their own (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T00:22:49Z) | open |
| 2026-10-19 | cfbf4f7e | interoplab: docker tests take the run's deadline, no bound of their own | native structural checks (red) | arch tier/check and arch iface-resolution were charged for want of path attribution; both run directly exit 0 | open |
| 2026-10-19 | cfbf4f7e | interoplab: docker tests take the run's deadline, no bound of their own | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-19 | cfbf4f7e | plan: docker test bounds fixed; journal row closed, handover appended | native structural checks (red) | arch tier/check and arch iface-resolution charged for want of path attribution; both run directly exit 0 at 3681720e48's tree | open |
