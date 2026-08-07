---
kind: note
level:
stage:
---
The per-edit hook (`auto-lint` in `.claude/hooks/posttool-writeedit.py`) uses
`--new-from-rev=HEAD`, which only catches issues on lines changed since the last
commit. Cross-file effects slip through: unused functions after refactoring,
import issues from renaming, type mismatches across package boundaries.
`make ze-verify` catches these but takes minutes (see `ai/rules/testing.md`
for current timings).
