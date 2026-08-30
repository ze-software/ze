---
kind: directive
level: MUST
stage:
---
**Round 1 reviews the WHOLE diff with at least two lenses; round N+1 reviews ONLY the fixes round N made plus the sibling call sites they touched, and each round's scope MUST be written into the spec's Review Gate section BEFORE it runs**, or it shrinks to whatever produces a clean round. The loop ends when a round finds no BLOCKER and no ISSUE inside its OWN scope AND no always-in-scope finding anywhere. A NOTE MUST NOT re-open a round, wherever it was found.
**Eight classes are ALWAYS in scope, whatever round surfaces them and whoever caused them: an unwired symbol, a vacuous test, an acceptance criterion with no test, a user-facing behavior with no functional test, Linux-only code with no QEMU test, a removed guard, a newly added guard that fails open, and any RFC or interop non-conformance.** Each passes a "no wrong result, no red gate" screen because its failure mode is silence, so none is ever a NOTE and the severity floor is ISSUE. Where the round's scope and this list disagree, the LIST wins.
**A finding outside the round's scope is fixed in this round when the goal this work exists to achieve depends on it, and otherwise gets ONE row in `plan/journal/<class>.md`.** The test is DEPENDENCY, never causation, and an unsure call falls on the fix-it side.
