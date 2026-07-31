# Rule Precedence

**When:** when two rules point in different directions, or you are deciding whether to stop, ask, delegate, or continue
**Severity:** blocking
**Related:** no-asking, model-selection, spec-delegation, no-parking, no-partial-completion

## Directives

Rules that disagree almost always disagree about one thing: whether to keep going. This ladder settles that, so the question is not re-litigated under context pressure.

**The ladder. A higher rung always wins, and the rungs below it do not get a vote.**

| Rung | Governs | Rules | What it does to the decision |
|------|---------|-------|------------------------------|
| 1 | Irreversible or destructive action | `never-destroy-work`, `git-safety` bans, `CLAUDE.md` prohibitions | STOP and ask. Nothing on any lower rung licenses it, including an explicit instruction to hurry |
| 2 | Correctness owed to someone outside this repo | `rfc-compliance`, `interop-and-goal-validation` | When full compliance AND full proof of it is on the table, you may not pick anything narrower. Ask Thomas which way to fix it |
| 3 | Scope integrity | `no-partial-completion`, `no-parking`, `fix-dont-record`, `no-test-deletion` | Never silently reduce scope, park a blocker, or weaken a test. If scope must change, the user decides |
| 4 | Phase boundaries | `model-selection`, `spec-delegation`, `critical-review` | End the phase, report, and hand off. Do not cross onto the next phase in this context |
| 5 | Autonomy | `no-asking` | Everything not caught above: finish the work, then report. Do not ask permission to do what you were already asked to do |

**Stopping at a phase boundary is NOT asking permission.** `no-asking` bans "would you like me to...?" before work you were already asked to do. It does not ban ending a phase, reporting the result, and letting the operator choose the next model or session. Rung 4 and rung 5 only look like a conflict if you read `no-asking` as "never stop".

**When a higher rung forces a question, the question is HOW, never WHETHER.** "Which way do you want this fixed" is always legitimate. "May I skip it", "may I drop the test", "shall I defer this" are banned at every rung (`no-parking.md`).

**Deferral versus parking, settled by one question: does the goal this work exists to achieve still hold if I leave this?** If yes, it is separable future work: home it per `deferral-tracking.md`. If no, it is parking with a polite name: fix it now (`no-parking.md`).

**Recording versus fixing, settled by one question: did I try to reproduce it and fail?** Only a failure whose mechanism you actively tried and could not reproduce may be written down instead of fixed. Anything deterministic, structural, or load-explained gets fixed (`fix-dont-record.md`).

**A rule's own subject matter is never overridden by this ladder.** The ladder decides stop/ask/delegate/continue. It does not license writing `fmt.Sprintf` on a hot path because you were in a hurry, and it does not exempt you from `no-fabrication` at any rung.

**If the ladder genuinely does not resolve a conflict, say so in one or two sentences, name both rules, state the reading you are taking, and proceed under it** -- unless the conflict sits on rung 1 or 2, where you stop instead. Silently picking a side and not mentioning it is the failure this clause exists to prevent.

## Rationale

Four rules give instructions about the same moment and were written independently: `no-asking` ("finish the task, then report; ask only for destructive actions or genuine scope changes"), `model-selection` ("announce the boundary and stop rather than crossing it on the wrong model"), `spec-delegation` ("the main thread supervises only"), and `rfc-compliance` ("STOP and ask Thomas rather than choosing anything narrower"). Each is right. Read together with no ordering, they let an agent justify almost any choice at the moment it is least able to reason carefully, which is precisely when the wrong choice is expensive.

Naming the ladder costs one short rule and removes the most common runtime ambiguity in the system.

## Examples

An implementation phase finishes and the Review Gate is next. Rung 4 applies: report what was built, state that review wants the review model, stop. `no-asking` (rung 5) does not override this, and the report is not a request for permission.

A functional test fails on a busy host and passes in isolation. Rung 3 applies via `fix-dont-record`: the test waits on elapsed time instead of on state. Fix the wait. Do not write a known-failure shard, and do not report "flaky, passes in isolation" as an outcome.

An RFC MUST is implemented but has no tagged test, and the spec is otherwise finished. Rung 2 applies: do not close, do not file a deferral. Quote the requirement and name the producing function, say what a tagged test would cost, ask Thomas which way to proceed.
