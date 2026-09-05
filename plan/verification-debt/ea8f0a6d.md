# Verification debt -- commit session ea8f0a6d

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | ea8f0a6d | spec(le): close the site-facts-from-committed-data spec | full native verification (not FRESH-green) | verify worktree not run: the shared job queue is held by a peer session's lint; the package's own tests, ./le repository check and ./le doc check links are named in the body | open |
| 2026-09-05 | ea8f0a6d | spec(le): close the site-facts-from-committed-data spec | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-05 | ea8f0a6d | spec(le): close the site-facts-from-committed-data spec | discovery-index freshness | no package moved: ./le discovery-index check reports 756 packages and ai/PACKAGE-MAP.md up to date | open |
| 2026-09-05 | ea8f0a6d | spec: close spec-site-facts-from-committed-data | full native verification (not FRESH-green) | verify worktree not run: the shared job queue is held by a peer session's lint | open |
