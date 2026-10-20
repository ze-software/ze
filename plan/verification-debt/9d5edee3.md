# Verification debt -- commit session 9d5edee3

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-20 | 9d5edee3 | deployment: a canceled gokrazy build kills the compilers gok started | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-20 | 9d5edee3 | deployment: a canceled gokrazy build kills the compilers gok started | native structural checks (red) | ./le arch tier check and ./le arch iface-resolution both exit 0 over this tree; the red is the recorded stage, charged for want of path attribution | open |
| 2026-10-20 | 9d5edee3 | deployment: a canceled gokrazy build kills the compilers gok started | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
