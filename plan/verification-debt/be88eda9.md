# Verification debt -- commit session be88eda9

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-04 | be88eda9 | fix(rib): one declaration of distance, read where it decides | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | be88eda9 | fix(rib): one declaration of distance, read where it decides | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | be88eda9 | fix(rib): one declaration of distance, read where it decides | discovery-index freshness | ai/PACKAGE-MAP.md regenerates to include internal/core/rib/distance, but it ALSO encodes another session's uncommitted deletion of redistribute_ingress, already gone from the working tree. Committing the map here would describe that package as absent in a HEAD that still contains it. The session removing it regenerates the map with its own commit. | open |
| 2026-09-04 | be88eda9 | fix(rib): the declared distance applies with no rib block | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | be88eda9 | fix(rib): the declared distance applies with no rib block | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | be88eda9 | fix(rib): the declared distance applies with no rib block | discovery-index freshness | ai/PACKAGE-MAP.md is unchanged by this commit; the entry for internal/core/rib/distance is still owed from de739c8b2 and blocked on another session's uncommitted redistribute_ingress deletion, which regenerating the map would encode. | open |
| 2026-09-04 | be88eda9 | fix(rib): a rolled-back reload no longer strips every distance | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | be88eda9 | fix(rib): a rolled-back reload no longer strips every distance | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | be88eda9 | fix(rib): a rolled-back reload no longer strips every distance | discovery-index freshness | ai/PACKAGE-MAP.md is unchanged by this commit; the internal/core/rib/distance entry is still owed from de739c8b2 and blocked on another session's uncommitted redistribute_ingress deletion. | open |
| 2026-09-04 | be88eda9 | docs(isis): the distance leaf is rib.distance.isis, not admin-distance | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | be88eda9 | test(rib): assert the distance column and JSON key, not admin-distance | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | be88eda9 | test(rib): assert the distance column and JSON key, not admin-distance | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
