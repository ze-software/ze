# Verification debt -- commit session 9aafda39

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | 9aafda39 | spec(rfc): close the superseded-requirements-carry-their-successor spec | full native verification (not FRESH-green) | verify worktree not run: the shared job queue is held by a peer session's lint; ./le rfc check, the corpus marker counts and the testhealth tests are named in the body | open |
| 2026-09-05 | 9aafda39 | spec: close spec-rfc-superseded-requirements-carry-their-successor | full native verification (not FRESH-green) | verify worktree not run: the shared job queue is held by a peer session's lint | open |
