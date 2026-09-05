# Verification debt -- commit session bcd10c0f

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | bcd10c0f | fix(bgp): the announce rail asks before a Prefix-SID leaves the AS | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-09-05T15:41:36Z) | open |
| 2026-09-05 | bcd10c0f | fix(bgp): the announce rail asks before a Prefix-SID leaves the AS | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
