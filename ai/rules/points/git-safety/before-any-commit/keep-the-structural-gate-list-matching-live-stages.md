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
that. `ze-cli-grammar-check` was listed here until 2026-07-20 and was exactly that
dead entry: a real make target (`mk/inventory.mk`), but never a verify stage, so
`structural_gate_reds` could never match it. Its underlying gate is not lost --
`TestCLIGrammarGateStatic` (`scripts/checks/cli_grammar_test.go`) runs the real
checker under the unit stage.
