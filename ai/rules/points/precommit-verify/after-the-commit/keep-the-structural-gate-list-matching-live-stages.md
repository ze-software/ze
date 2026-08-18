---
kind: note
level:
stage:
---
This list is the prose mirror of `STRUCTURAL_GATES` in `scripts/dev/commit_helper.py`,
and every name in it must be a stage `stagesForMode` actually emits
(`scripts/status/verify_run.go`) -- otherwise the entry matches nothing and gates
nothing. `test_structural_gates_are_live_stages` (`scripts/dev/commit_helper_test.py`)
and `TestStructuralGatesAreLiveStages` (`scripts/status/verify_run_test.go`) enforce
that. Every named gate is a live verify stage. The underlying CLI grammar gate
runs through `TestCLIGrammarGateStatic` (`scripts/checks/cli_grammar_test.go`).
