# Verification debt -- commit session ac6cc6ef

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-06 | ac6cc6ef | feat(firewall): close domain-group, and say why a cache entry is missing | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-09-05T17:25:15Z) | open |
| 2026-09-06 | ac6cc6ef | feat(firewall): close domain-group, and say why a cache entry is missing | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-06 | ac6cc6ef | feat(firewall): close domain-group, and say why a cache entry is missing | discovery-index freshness | this commit adds and removes no package: the three changed Go files join packages already in the map, and ./le discovery-index check answers up to date over 763 packages | open |
