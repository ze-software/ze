# Verification debt -- commit session 5fa8be0a

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-03 | 5fa8be0a | resolve: one delegation table, one parser, one reachable lookup (+8 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | resolve: one delegation table, one parser, one reachable lookup (+4 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5fa8be0a | fix(cli): read the long help from the field that declares it | discovery-index freshness | the two files rename a field they read and add no package, so ai/PACKAGE-MAP.md is unchanged by this commit and git reports it clean | open |
| 2026-09-03 | 5fa8be0a | fix(web): finish the long-help rename through the template and its tests | discovery-index freshness | the rename adds no package and touches no import, so ai/PACKAGE-MAP.md is unchanged and git reports it clean | open |
