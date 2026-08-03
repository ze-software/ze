# Rule Precedence

**When:** when two rules point in different directions, or you are deciding whether to stop, ask, delegate, or continue
**Severity:** blocking
**Related:** completion, planning, testing

## Directives

Rules that disagree almost always disagree about one thing: whether to keep going. This ladder settles that, so the question is not re-litigated under context pressure.

**The ladder. A higher rung always wins, and the rungs below it do not get a vote.**

| Rung | Governs | Rules | What it does to the decision |
|------|---------|-------|------------------------------|
| 1 | Irreversible or destructive action | `never-destroy-work`, `git-safety` bans, `CLAUDE.md` prohibitions | STOP and ask. Nothing on any lower rung licenses it, including an explicit instruction to hurry |
| 2 | Correctness owed to someone outside this repo | `rfc-compliance`, `interop-and-goal-validation` | When full compliance AND full proof of it is reachable, IMPLEMENT it. You may not pick anything narrower, and you may not ask Thomas to pick for you. Ask only when you are about to do LESS, and then ask which way to fix it |
| 3 | Scope integrity | `completion` (no partial completion, no parking, fix do not record), `testing` (no test deletion) | Never silently reduce scope, park a blocker, or weaken a test. If scope must change, the user decides |
| 4 | Phase boundaries | `planning` (model selection, spec delegation, critical review) | End the phase, report, and hand off. Do not cross onto the next phase in this context |
| 5 | Autonomy | `completion` (no asking) | Everything not caught above: finish the work, then report. Do not ask permission to do what you were already asked to do |

**Stopping at a phase boundary is NOT asking permission.** The no-asking directive in `completion` bans "would you like me to...?" before work you were already asked to do. It does not ban ending a phase, reporting the result, and letting the operator choose the next model or session. Rung 4 and rung 5 only look like a conflict if you read that directive as "never stop".

**When a higher rung forces a question, the question is HOW, never WHETHER.** "Which way do you want this fixed" is always legitimate. "May I skip it", "may I drop the test", "shall I defer this" are banned at every rung (`completion.md`).

**Deferral versus parking, settled by one question: does the goal this work exists to achieve still hold if I leave this?** If yes, it is separable future work: home it per `planning.md`. If no, it is parking with a polite name: fix it now (`completion.md`).

**Closing comes first, and the same question decides the ORDER as well as the verdict: a defect the goal does NOT depend on is fixed AFTER the work in hand is closed, never on the way to closing it.** `completion.md` makes you the owner of a defect you walked into, and it does not make you its owner this minute. Name it, home it per `planning.md`, close, then fix it. Work that was finished but never landed is the most expensive failure this repo has, and an unrelated fix folded into a closing commit is its usual cause: it costs the commit its single focus and the review its scope, and it restarts the gates that were already green.

**Recording versus fixing, settled by one question: did I try to reproduce it and fail?** Only a failure whose mechanism you actively tried and could not reproduce may be written down instead of fixed. Anything deterministic, structural, or load-explained gets fixed (`completion.md`).

**A rule's own subject matter is never overridden by this ladder.** The ladder decides stop/ask/delegate/continue. It does not license writing `fmt.Sprintf` on a hot path because you were in a hurry, and it does not exempt you from `no-fabrication` at any rung.

**If the ladder genuinely does not resolve a conflict, say so in one or two sentences, name both rules, state the reading you are taking, and proceed under it** -- unless the conflict sits on rung 1 or 2, where you stop instead. Silently picking a side and not mentioning it is the failure this clause exists to prevent.

## Rationale

Four directives give instructions about the same moment and were written independently: no-asking in `completion` ("finish the task, then report; ask only for destructive actions or genuine scope changes"), model-selection in `planning` ("announce the boundary and stop rather than crossing it on the wrong model"), spec-delegation in `planning` ("the main thread supervises only"), and `rfc-compliance` ("implement full compliance, and ask only before doing LESS"). Each is right. Read together with no ordering, they let an agent justify almost any choice at the moment it is least able to reason carefully, which is precisely when the wrong choice is expensive.

Naming the ladder costs one short rule and removes the most common runtime ambiguity in the system.

## Examples

An implementation phase finishes and the Review Gate is next. Rung 4 applies: report what was built, state that review wants the review model, stop. The no-asking directive (rung 5) does not override this, and the report is not a request for permission.

A functional test fails on a busy host and passes in isolation. Rung 3 applies via the fix-do-not-record directive in `completion`: the test waits on elapsed time instead of on state. Fix the wait. Do not write a known-failure shard, and do not report "flaky, passes in isolation" as an outcome.

An RFC MUST is implemented but has no tagged test, and the spec is otherwise finished. Rung 2 applies: do not close, do not file a deferral. Writing that test is reachable full proof, so WRITE it. No question is owed. Ask only if you are about to leave the MUST unproven, and then quote the requirement, name the producing function, say what a tagged test would cost, and ask which way he wants it fixed.
