# Verification debt -- commit session 5fa8be0a

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-03 | 5fa8be0a | resolve: one delegation table, one parser, one reachable lookup | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | resolve: one delegation table, one parser, one reachable lookup | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5fa8be0a | spec: close spec-rir-seed-embed-and-zefs-refresh | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
