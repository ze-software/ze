---
kind: directive
level: MUST
stage:
rationale: plan/journal/gate-excludes-part-of-its-population.md
---
**Guard with early returns, state the invariant POSITIVELY, and test ONE fact per guard: a compound `if a || b` or `if a && b && !c` MUST be split.** A happy path MUST NOT be wrapped in an `else`, `if index < length` MUST be preferred to its negation, and a compound test that earns a name MUST get one rather than sit inline.
