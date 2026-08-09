---
kind: note
level:
stage:
---
A Bash `git commit` is blocked outright by destructive-git. Commit via `scripts/dev/commit_helper.py create`, then `bash` on the path its `script=` line prints; the creation-time gates (verify-status, discovery-index, deferral-unassigned, deferral-in-diff, journal-row, spec-audit block; wiring-at-commit, doc-drift warn) run then. See "Commit-time gates" above.
