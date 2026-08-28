# Verification debt -- commit session c7beceff

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-28 | c7beceff | website: publish generated site | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-28 | c7beceff | website: publish generated site | discovery-index freshness | gh-pages is a generated artifact branch with no ai source tree | open |
