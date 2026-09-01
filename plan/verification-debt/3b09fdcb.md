# Verification debt -- commit session 3b09fdcb

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-31 | 3b09fdcb | plan: keep the removed weakened.md rows where they cannot be cleaned | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(bgp): name the processes the end-of-rib waits for | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | fix(bgp): name the processes the end-of-rib waits for | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-01 | 3b09fdcb | fix(bgp): name the processes the end-of-rib waits for | discovery-index freshness | the change adds and removes no package: it changes method signatures inside packages ai/PACKAGE-MAP.md already lists | open |
| 2026-09-01 | 3b09fdcb | test: name the check file the draft README actually names | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-01 | 3b09fdcb | test: name the check file the draft README actually names | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
