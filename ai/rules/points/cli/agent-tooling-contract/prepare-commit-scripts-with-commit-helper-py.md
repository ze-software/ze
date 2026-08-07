---
kind: note
level:
stage:
---
Use `scripts/dev/commit_helper.py` for commit script preparation. It owns
session ID reuse, message file creation, executable per-commit script
generation (the path comes from its `script=` line), ignored-path rejection, `git commit -F`, and the learned-summary
gate for workflow/tooling/rule changes. Hand-write a commit script only when the
helper cannot express the commit shape, and keep the same generated-script
contract from `ai/rules/git-safety.md`.
