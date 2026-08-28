---
kind: note
level:
stage:
---
This list is the prose mirror of `STRUCTURAL_GATES` in `internal/le/commit`,
and every name in it must be a stage `stagesForMode` actually emits
(`internal/le/verify/run.go`) -- otherwise the entry matches nothing and gates
nothing. `test_structural_gates_are_live_stages` (`internal/le/commit/commit_test.go`)
and `TestStructuralGatesAreLiveStages` (`internal/le/verify/verify_test.go`) enforce
that. Every named gate is a live verify stage. The underlying CLI grammar gate
runs through `TestCLIGrammarGateStatic` (`internal/le/cligrammar/cligrammar_test.go`).
