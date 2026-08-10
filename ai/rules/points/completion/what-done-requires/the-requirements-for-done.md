---
kind: table
level:
stage:
---
| # | Requirement |
|---|-------------|
| 1 | Every acceptance criterion in the spec has working code |
| 2 | Every acceptance criterion has a unit test (`_test.go`) that exercises its logic |
| 3 | Every user-facing behavior has a functional test (`.ci`/`.et`) per `ai/rules/testing.md` |
| 4 | Protocol features have interop tests per `ai/rules/interop-and-goal-validation.md` |
| 5 | Goal Validation table filled with concrete evidence per goal |
| 6 | The code compiles and `make ze-verify` passes |
| 7 | No TODO, FIXME, or stub remains in the new code |
| 8 | No item was silently dropped from scope |
| 9 | Every function is reachable from a user entry point (wired, not just library) |
| 10 | You READ the diff, hunk by hunk, and every hunk is one you would defend. A gate covers what somebody thought to check, so a defect on a surface no gate reads survives a fully green run |
| 11 | Every generated artifact in the diff was produced by its generator, never edited by hand. When both are in the diff, the generator's output and the artifact are compared, and the comparison is a test |
