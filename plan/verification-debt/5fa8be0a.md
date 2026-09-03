# Verification debt -- commit session 5fa8be0a

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-03 | 5fa8be0a | resolve: one delegation table, one parser, one reachable lookup | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | resolve: one delegation table, one parser, one reachable lookup | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5fa8be0a | spec: close spec-rir-seed-embed-and-zefs-refresh | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | journal: record the staticcheck stage ending verify with no verdict | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | fix(le): make test-unit all run the whole checkout | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | fix(le): make test-unit all run the whole checkout | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5fa8be0a | feat(config): point the RIR delegation fetch at a mirror, per registry | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | feat(config): point the RIR delegation fetch at a mirror, per registry | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5fa8be0a | chore(plan): close spec-rir-delegation-source-override | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | docs(config): give every system leaf the explanation its ? key prints | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | fix(cli): read the long help from the field that declares it | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | fix(cli): read the long help from the field that declares it | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5fa8be0a | fix(cli): read the long help from the field that declares it | discovery-index freshness | the two files rename a field they read and add no package, so ai/PACKAGE-MAP.md is unchanged by this commit and git reports it clean | open |
| 2026-09-03 | 5fa8be0a | fix(web): finish the long-help rename through the template and its tests | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | 5fa8be0a | fix(web): finish the long-help rename through the template and its tests | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | 5fa8be0a | fix(web): finish the long-help rename through the template and its tests | discovery-index freshness | the rename adds no package and touches no import, so ai/PACKAGE-MAP.md is unchanged and git reports it clean | open |
