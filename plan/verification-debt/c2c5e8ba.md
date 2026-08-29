# Verification debt -- commit session c2c5e8ba

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-29 | c2c5e8ba | fix(site): stamp the publication time the native build stopped writing | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T11:55:15Z) | open |
| 2026-08-29 | c2c5e8ba | fix(site): stamp the publication time the native build stopped writing | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
