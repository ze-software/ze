# Verification debt -- commit session f8a8ccf7

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-02 | f8a8ccf7 | fix(web): vendor htmx 4.0.0 and follow its renamed SSE events | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-02 | f8a8ccf7 | fix(web): vendor htmx 4.0.0 and follow its renamed SSE events | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-02 | f8a8ccf7 | fix(web): vendor htmx 4.0.0 and follow its renamed SSE events | discovery-index freshness | le discovery-index update regenerated ai/PACKAGE-MAP.md and produced no diff: no package was added, removed or renamed | open |
