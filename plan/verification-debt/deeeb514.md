# Verification debt -- commit session deeeb514

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-02 | deeeb514 | feat(cli): Tab reveals what a command is and what it does | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-02 | deeeb514 | feat(cli): Tab reveals what a command is and what it does | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
