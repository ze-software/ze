# Verification debt -- commit session b8b5ca0f

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | b8b5ca0f | feat(le): verify reds answers about a run that has not finished | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-09-05T17:13:05Z) | open |
| 2026-09-05 | b8b5ca0f | feat(le): verify reds answers about a run that has not finished | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
