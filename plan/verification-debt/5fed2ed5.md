# Verification debt -- commit session 5fed2ed5

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-31 | 5fed2ed5 | rfc: sign off fourteen Supported RFCs against their own text | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | rfc: sign off fourteen Supported RFCs against their own text | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | fix(rfc): nine RFC 8092 citations naming the wrong section | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | fix(rfc): nine RFC 8092 citations naming the wrong section | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
