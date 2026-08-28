---
kind: directive
level: MUST
stage:
---
- **Before a section is split into a rule of its own, its candidate trigger MUST be scored against the task corpus.** `distinctive_terms` (`internal/le/rules/artifacts.go`) drops every trigger term that too many other triggers share, and `unreachable_blocking` names each blocking rule no past task would surface. `core_members` then makes exactly that set always-on, so a split whose trigger scores nothing returns the new rule to the core at full size and saves nothing. `./le rules router-report` prints the set and the corpus it read.
