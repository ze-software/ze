---
kind: note
level:
stage:
---
There is no list to keep matching. A stage declares whether its red means the
tree is BROKEN, on the stage itself: `structural(...)` rather than `stage(...)`
in `fullStages` (`internal/le/verify/engine/stages.go`), read back through
`verifyengine.Structural`. A rename moves the name and its membership together,
so the prose above is a description rather than a second source.

This replaced a hand-written map of the same eight names in
`internal/le/commit`. Two lists of one population drift, and this pair drifted
in the dangerous direction: a stage renamed in the population silently left the
commit gate's set, its red filed as unattributable, and the commit proceeded
over a tree the gate had called broken.

This note previously claimed `test_structural_gates_are_live_stages` and
`TestStructuralGatesAreLiveStages` enforced the agreement. Neither has ever
existed, and it named `STRUCTURAL_GATES`, a symbol from the retired Python
helper. A rule naming an enforcement nobody wrote is worse than one naming
none: a reader who checks the claim stops looking.

What is tested now is what survives derivation:
`TestStructuralStagesAreMembersOfThePopulation` and
`TestStructuralIsASubsetOfFull`
(`internal/le/verify/engine/stages_structural_test.go`) hold that the set is
non-empty in both modes, names only stages that run, and never marks a stage
structural in the cheaper mode alone. The CLI grammar gate runs through
`TestCLIGrammarGateStatic` (`internal/le/cligrammar/cligrammar_test.go`).
