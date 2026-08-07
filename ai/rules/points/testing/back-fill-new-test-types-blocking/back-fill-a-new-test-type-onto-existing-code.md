---
kind: note
level:
stage:
---
When you introduce a new test type, technique, or infrastructure (fuzz target, property test, mutation gate, `-race` sweep, clock-injection audit, new `.ci`/`.et` category, QEMU harness), apply it to the existing code it covers, not only to the code added alongside it. Coverage that grows only forward from the introduction date is the trap (`plan/learned/RECURRING-PATTERNS.md`, "New test type added but not back-filled to existing code").
