# Verification debt -- commit session f0e8bfff

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | f0e8bfff | spec(anomaly): close the anomaly-observe incident lifecycle store | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-05 | f0e8bfff | spec(anomaly): close the anomaly-observe incident lifecycle store | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-05 | f0e8bfff | spec: close spec-anomaly-3-observe | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
