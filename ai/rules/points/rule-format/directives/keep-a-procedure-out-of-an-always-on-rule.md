---
kind: directive
level: MUST
stage:
rationale: plan/journal/gate-fires-outside-its-population.md
---
- **An always-on rule MUST hold prohibitions, and a PROCEDURE MUST live in its own rule under its own trigger.** `core_members` (`internal/le/rules/artifacts.go`) derives eagerness from the precedence ladder, so a procedure written inside a rung-1 rule is loaded in full by every session that will never carry it out. The ban and the how-to are separable: the ban earns its permanent seat because acting without it is unrecoverable, and the how-to is one Read away for the session that reaches the work.
