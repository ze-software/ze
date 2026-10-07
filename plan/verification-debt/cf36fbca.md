# Verification debt -- commit session cf36fbca

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-07 | cf36fbca | spec: re-close plugin-query-mode after its closure audit (+43 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-01T12:40:37Z) | open |
| 2026-10-07 | cf36fbca | spec-release-roadmap: one Status vocabulary, restore weakened tests (+18 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
