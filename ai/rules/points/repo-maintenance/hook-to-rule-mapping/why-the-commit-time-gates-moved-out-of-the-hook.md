---
kind: note
level:
stage:
---
The five commit-time gates (spec-audit, deferral-in-diff, deferral-unassigned, wiring-at-commit, doc-drift) used to sit here but gated on the literal `git commit` string, which the sanctioned commit path never sends and `destructive-git` blocks when it does. They are now **creation-time gates in `scripts/dev/commit_helper.py`**. See "Commit-time gates" below.
