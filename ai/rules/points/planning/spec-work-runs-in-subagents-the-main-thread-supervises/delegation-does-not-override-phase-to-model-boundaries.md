---
kind: directive
level:
stage:
---
**Delegation does not override phase-to-model boundaries.** Subagents inherit the PHASE, not the task shape ("Model Selection by Work Phase", below), so the main thread still announces a boundary and stops rather than delegating an implementation phase from a review session to get around the switch.
