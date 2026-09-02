# Verification debt -- commit session deeeb514

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-02 | deeeb514 | feat(cli): Tab reveals what a command is and what it does | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-02 | deeeb514 | feat(cli): Tab reveals what a command is and what it does | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-02 | deeeb514 | feat(cli): the attached console reaches configuration mode | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-02 | deeeb514 | feat(cli): the attached console reaches configuration mode | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-02 | deeeb514 | feat(cli): the attached console reaches configuration mode | discovery-index freshness | session_editor.go joins cmd/ze/hub, a package the map already names, so the derived index does not move. le discovery-index update was run and rewrote ai/PACKAGE-MAP.md byte-identical to HEAD | open |
| 2026-09-02 | deeeb514 | fix(cli): hold the message row to one line | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-02 | deeeb514 | fix(cli): hold the message row to one line | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-02 | deeeb514 | docs(plan): close spec-cli-tab-reveals-command-help | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
