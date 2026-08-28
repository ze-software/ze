---
kind: note
level:
stage:
---
The native per-edit hook in `internal/le/hookruntime/postwrite.go` judges changed
lines only. Cross-file effects can slip through: unused functions after
refactoring, import issues after renaming, and type mismatches across packages.
`./le verify worktree` catches these but takes minutes (see `ai/rules/testing.md`
for current timings).
