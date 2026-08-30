---
kind: directive
level: MUST
stage:
---
**A new exported symbol MUST have a non-test caller, and wiring MUST be the FIRST implementation step rather than a check at the end.** Grep the symbol across `internal/` and `cmd/`: if the only hits are its definition and test files it is dead code, and dead code is a BLOCKER rather than a NOTE. A wiring test that cannot be written means the feature is BLOCKED rather than done.
**`./le doc wiring` is a STRUCTURAL stage of `./le verify worktree`, so its red says the tree is broken and MUST be fixed rather than recorded.** `docs/contributing/spec-workflow.md` says where new code has to be called from, which test each feature type owes, and where its `.ci` test lives.
