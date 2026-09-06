# Verification debt -- commit session de781ada

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-06 | de781ada | fix(iface): refuse a tunnel ttl the vpp backend cannot carry | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-09-05T17:25:15Z) | open |
| 2026-09-06 | de781ada | fix(iface): refuse a tunnel ttl the vpp backend cannot carry | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-06 | de781ada | docs(iface): the vpp backend refuses a tunnel ttl instead of dropping it | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-09-05T17:25:15Z) | open |
