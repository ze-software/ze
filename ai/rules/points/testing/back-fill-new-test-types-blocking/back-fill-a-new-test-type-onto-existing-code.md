---
kind: directive
level: MUST
stage:
---
**A new test type, technique, or infrastructure (a fuzz target, a property test,
a mutation gate, a `-race` sweep, a clock-injection audit, a new `.ci` or `.et`
category, a QEMU harness) MUST be applied to the existing code it covers, in the
same work, not only to the code added alongside it.** Coverage that grows only
forward from the introduction date is the trap
(`plan/learned/RECURRING-PATTERNS.md`).
