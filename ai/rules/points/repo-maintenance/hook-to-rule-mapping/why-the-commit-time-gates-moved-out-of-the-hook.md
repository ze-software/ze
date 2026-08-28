---
kind: note
level:
stage:
---
The five commit-time gates (spec-audit, deferral-in-diff, deferral-unassigned, wiring-at-commit, doc-drift) belong in **creation-time gates in `internal/le/commit`** because the sanctioned commit path does not send the literal `git commit` string to this hook. See "Commit-time gates" below.
