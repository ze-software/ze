---
kind: table
level:
stage:
---
| Rung | Governs | Rules | What it does to the decision |
|------|---------|-------|------------------------------|
| 1 | Irreversible or destructive action | `never-destroy-work`, `git-safety` bans, `CLAUDE.md` prohibitions | STOP and ask. Nothing on any lower rung licenses it, including an explicit instruction to hurry |
| 2 | Correctness owed to someone outside this repo | `rfc-compliance`, `interop-and-goal-validation` | When full compliance AND full proof of it is reachable, IMPLEMENT it. You may not pick anything narrower, and you may not ask Thomas to pick for you. Ask only when you are about to do LESS, and then ask which way to fix it |
| 3 | Scope integrity | `completion` (no partial completion, no parking, fix do not record), `testing` (no test deletion) | Never silently reduce scope, park a blocker, or weaken a test. If scope must change, the user decides |
| 4 | Phase boundaries | `planning` (model selection, spec delegation, critical review) | End the phase, report, and hand off. Do not cross onto the next phase in this context |
| 5 | Autonomy | `completion` (no asking) | Everything not caught above: finish the work, then report. Do not ask permission to do what you were already asked to do |
