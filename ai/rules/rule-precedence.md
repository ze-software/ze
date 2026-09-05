# Rule Precedence

**When:** when two rules point in different directions, or you are deciding whether to stop, ask, delegate, or continue
**Severity:** blocking
**Related:** completion, planning, testing

## Directives

**The ladder. A higher rung MUST win, and the rungs below it MUST NOT get a vote.**

**The ladder. Each rung MUST decide what its row names:**

| Rung | Governs | Rules | What it does to the decision |
|------|---------|-------|------------------------------|
| 1 | Irreversible or destructive action | `never-destroy-work`, `git-safety` bans, `CLAUDE.md` prohibitions | STOP and ask. Nothing on any lower rung licenses it, including an explicit instruction to hurry |
| 1 | What every other rule is an instance of | `principles` | Ten statements the rest of the corpus derives from. A rule that merely restates one of them carries nothing the reader did not already have |
| 2 | Correctness owed to someone outside this repo | `rfc-compliance`, `interop-and-goal-validation`, `documentation` | When full compliance AND full proof of it is reachable, IMPLEMENT it. Picking anything narrower is not permitted, and neither is asking Thomas to pick for you. Ask only when you are about to do LESS, and then ask which way to fix it. A page your change made wrong is a debt to a reader outside this repo. Read the page before you investigate. Repair it in the work that broke it |
| 3 | Scope integrity | `completion` (no partial completion, no parking, fix do not record), `testing` (no test deletion) | Never silently reduce scope, park a blocker, or weaken a test. If scope has to change, the user decides |
| 4 | Phase boundaries | `planning` (model selection, spec delegation, critical review) | End the phase, report, and hand off. Do not cross onto the next phase in this context |
| 5 | Autonomy | `completion` (no asking) | Everything not caught above: finish the work, then report. Do not ask permission to do what you were already asked to do |

**Stopping at a phase boundary is NOT asking permission.** The no-asking directive in `completion` bans "would you like me to...?" before work you were already asked to do. You MAY end a phase, report the result, and let the operator choose the next model or session. Rung 4 and rung 5 only look like a conflict if you read that directive as "never stop".

**When a higher rung forces a question, the question MUST be HOW; it MUST NOT be WHETHER.** "Which way do you want this fixed" is always legitimate. `May I skip it`, `may I drop the test`, `shall I defer this` are banned at every rung (`completion.md`).

**Deferral versus parking, settled by one question: does the goal this work exists to achieve still hold if I leave this?** If no, it is parking with a polite name: MUST fix it now (`completion.md`). If yes, it is separable future work, and its home depends on what it is.
**A DEFECT you walked into MUST get ONE row in `plan/journal/<class>.md`, then the work in hand closes and you stop: no spec, no row anywhere else, no question to Thomas (owner directive, 2026-08-10). A distinct, larger, separable FEATURE MUST be homed in a spec per `planning.md`, and Thomas MUST be asked whether that spec runs.**

**Closing comes first, and the same question decides the ORDER as well as the verdict: a defect the goal does NOT depend on MUST NOT be fixed on the way to closing the work in hand.** `completion.md` makes you the owner of a defect you walked into, and it does not make you its owner this minute. Work that was finished but never landed is the most expensive failure this repo has, and an unrelated fix folded into a closing commit is its usual cause: it costs the commit its single focus and the review its scope, and it restarts the gates that were already green.
**The route is fixed and it has three steps: MUST write ONE row in `plan/journal/<class>.md`, close the work in hand, and stop (owner directive, 2026-08-10).** MUST NOT write a spec for it, MUST NOT record it anywhere else, and MUST NOT ask Thomas whether to implement one (`completion.md`). MUST NOT fix it after closing either. Rows accumulate by problem class, and a class that collects rows earns its fix in a deliberate pass over the journal.
**Three finds are fixed on the spot, and `completion.md` names all three: a defect that stops a test or a gate from passing, a test that is wrong about what it asserts, and code related to the problem in hand.** The blocking defect is the first of them, because there is no closing the work in hand around it.

**Recording versus fixing, settled by one question: did I try to reproduce it and fail?** Only a failure whose mechanism you actively tried and could not reproduce MAY be written down instead of fixed. Anything deterministic, structural, or load-explained MUST be fixed (`completion.md`).
**A spec is not a record, so this question MUST decide WHAT you write; it MUST NOT decide WHETHER the fix happens.** A reproducible defect that does not block the work in hand MUST still get a spec and an ask rather than a same-session fix (the point above); the shard route stays reserved for the failure you could not reproduce.

**Simplest-correct-solution MUST sit UNDER rungs 2 and 3; it MUST NOT sit beside them.** `ai/rules/simplicity.md` requires the simplest fully correct answer, and "fully correct" is what rungs 2 and 3 already own. It cuts machinery: an abstraction with one user, an option nobody asked for, a layer that transforms nothing. It MUST NOT cut correctness, conformance, a test, a guard, or an error path, and quality is 0% compromise.
**The simplest design is usually the HARDEST to find. "This was the pragmatic option under time pressure" is the tell that a lower rung is being read as a license.** Not seeing the simple design is a reason to think longer, or to ask which way. It MUST NOT be treated as a reason to ship the complicated answer or the incomplete one.

**A rule's own subject matter MUST NOT be overridden by this ladder.** The ladder decides stop/ask/delegate/continue. It does not license writing `fmt.Sprintf` on a hot path because you were in a hurry, and it does not exempt you from `no-fabrication` at any rung.

**If the ladder genuinely does not resolve a conflict, MUST say so in one or two sentences, name both rules, state the reading you are taking, and proceed under it** -- unless the conflict sits on rung 1 or 2, where you MUST stop instead. Silently picking a side and not mentioning it is the failure this clause exists to prevent.
