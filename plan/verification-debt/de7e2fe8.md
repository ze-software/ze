# Verification debt -- commit session de7e2fe8

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | de7e2fe8 | le: one test environment for two verify runs of one commit (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=3, at 2026-10-21T17:22:47Z) | open |
| 2026-10-21 | de7e2fe8 | le: one test environment for two verify runs of one commit (+1 more) | native structural checks (red) | arch tier/check red is a foreign docker exec in internal/le/setup/dockerhost/static.go:211; staticcheck parts 2 and 3 were red at ee108f4731 from mid-run build-cache eviction, not from this change | open |
| 2026-10-21 | de7e2fe8 | le: one test environment for two verify runs of one commit (+1 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
