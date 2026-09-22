---
kind: directive
level: MUST
stage:
---
**A new exported symbol introduced by an agreed implementation MUST have a non-test caller, and wiring MUST be the first implementation step rather than a check at the end.** Search the symbol across `internal/` and `cmd/`: if the only hits are its definition and test files, the feature remains unfinished. A wiring test that cannot be written leaves the feature blocked, and a baseline report MUST preserve that status without inferring permission to finish it.
**`./le doc wiring` is a STRUCTURAL stage of `./le verify worktree`, and its failure MUST be diagnosed and reported.** Fix a failure that blocks the agreed implementation or scoped proof; preserving unrelated unfinished changes does not authorize completing them. `docs/contributing/spec-workflow.md` says where new code has to be called from, which test each feature type owes, and where its `.ci` test lives.
