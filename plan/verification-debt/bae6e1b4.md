# Verification debt -- commit session bae6e1b4

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-30 | bae6e1b4 | fix(ike): compare algorithm identifiers with bytes.Equal | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T14:01:28Z) | open |
| 2026-08-30 | bae6e1b4 | fix(ike): compare algorithm identifiers with bytes.Equal | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
