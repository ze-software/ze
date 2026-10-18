# Verification debt -- commit session fa5bfe15

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-18 | fa5bfe15 | verify: current runs full in a worktree, fast runs in place | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-18T17:16:37Z) | open |
| 2026-10-18 | fa5bfe15 | verify: current runs full in a worktree, fast runs in place | native structural checks (red) | doc wiring is red on other sessions' doc-drift (runYANGConfig, bgp reactor and fsm, ike, config loader, plugin boundary) and doc check verify on the gh-pages command surfaces; the one finding on this commit, rfc carriersFor under rfc-implementation-guide.md, changes only pipeline label strings the page does not spell and is answered by a doc-reviewed row in the next commit | open |
| 2026-10-18 | fa5bfe15 | verify: current runs full in a worktree, fast runs in place | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
