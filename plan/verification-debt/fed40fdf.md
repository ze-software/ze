# Verification debt -- commit session fed40fdf

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-02 | fed40fdf | fix(deps): take grpc 1.83.1 for the HTTP/2 DATA frame OOM | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-02 | fed40fdf | fix(deps): take grpc 1.83.1 for the HTTP/2 DATA frame OOM | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-02 | fed40fdf | fix(deps): take grpc 1.83.1 for the HTTP/2 DATA frame OOM | discovery-index freshness | ai/PACKAGE-MAP.md regenerated and byte-identical: a vendored dependency bump adds no Ze package | open |
| 2026-09-02 | fed40fdf | docs(journal): record the vulnerability gate that hid its own cause | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
