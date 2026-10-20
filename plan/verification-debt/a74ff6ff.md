# Verification debt -- commit session a74ff6ff

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-20 | a74ff6ff | interop: read gobgp SR Policy from wire; bridge sr-policy scenario (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-20 | a74ff6ff | interop: read gobgp SR Policy from wire; bridge sr-policy scenario (+2 more) | native structural checks (red) | arch tier/check and arch iface-resolution charged from an old verify record; print OK on this tree | open |
| 2026-10-20 | a74ff6ff | interop: read gobgp SR Policy from wire; bridge sr-policy scenario (+1 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
