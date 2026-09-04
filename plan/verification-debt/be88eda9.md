# Verification debt -- commit session be88eda9

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-04 | be88eda9 | fix(rib): one declaration of distance, read where it decides | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | be88eda9 | fix(rib): one declaration of distance, read where it decides | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | be88eda9 | fix(rib): one declaration of distance, read where it decides | discovery-index freshness | ai/PACKAGE-MAP.md regenerates to include internal/core/rib/distance, but it ALSO encodes another session's uncommitted deletion of redistribute_ingress, already gone from the working tree. Committing the map here would describe that package as absent in a HEAD that still contains it. The session removing it regenerates the map with its own commit. | open |
