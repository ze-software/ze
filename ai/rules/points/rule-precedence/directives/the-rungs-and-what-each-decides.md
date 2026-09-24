---
kind: directive
level: MUST
stage:
rationale: ai/rationale/rule-precedence.md
---
**The ladder. Each rung MUST decide what its row names:**

| Rung | Governs | Rules | What it does to the decision |
|------|---------|-------|------------------------------|
| 1 | Irreversible or destructive action | `never-destroy-work`, `git-safety` bans, `AGENTS.md` prohibitions | STOP and ask. Nothing on any lower rung licenses it, including an explicit instruction to hurry |
| 1 | What every other rule is an instance of | `principles` | Ten statements the rest of the corpus derives from. A rule that merely restates one of them carries nothing the reader did not already have |
| 2 | Correctness owed to someone outside this repo | `rfc-compliance`, `interop-and-goal-validation`, `documentation` | Verify implemented capabilities through their real entry points and fix defects in the agreed scope. Record absent RFC requirements as gaps without authorizing implementation. Correct unsupported public claims. Read the relevant page before investigation and repair documentation made wrong by the change |
| 3 | Scope integrity | `completion`, `testing` | Complete the agreed acceptance criteria without weakening proof. The owner decides scope changes. An absent RFC feature requires separate implementation scope under `rfc-compliance` |
| 4 | Phase boundaries | `planning` (spec delegation, independent review) | End the phase, report, and hand off. Do not cross onto the next phase in this context |
| 5 | Autonomy | `completion` (no asking) | Everything not caught above: finish the work, then report. Do not ask permission to do what you were already asked to do |
