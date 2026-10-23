# Rule Precedence

**When:** when two rules point in different directions, or you are deciding whether to stop, ask, delegate, or continue
**Severity:** blocking
**Related:** completion, planning, testing

## Directives

**The ladder. A higher rung MUST win, and the rungs below it MUST NOT get a vote.**

**The ladder. Each rung MUST decide what its row names:**

| Rung | Governs | Rules | What it does to the decision |
|------|---------|-------|------------------------------|
| 1 | Irreversible or destructive action | `never-destroy-work`, `git-safety` bans, `AGENTS.md` hard bans | Stop and ask. Nothing on any lower rung licenses it, including an explicit instruction to hurry |
| 1 | What every other rule is an instance of | `principles` | Ten statements the rest of the corpus derives from. A rule that merely restates one of them carries nothing the reader did not already have |
| 2 | Correctness owed to someone outside this repo | `rfc-compliance`, `interop-and-goal-validation`, `documentation` | Verify implemented capabilities through their real entry points and fix their defects. An absent requirement of an implemented RFC is owed work, never an owner question. Correct unsupported public claims. Read the relevant page before investigation and repair documentation made wrong by the change |
| 3 | Scope integrity | `completion`, `testing` | Complete the agreed acceptance criteria without weakening proof. The owner decides scope changes, and what `ai/META-DECISIONS.md` already decides is not a scope change |
| 4 | Phase boundaries | `planning` (spec delegation, independent review) | End the phase, report, and hand off. Do not cross onto the next phase in this context |
| 5 | Autonomy | `completion` (no asking) | Everything not caught above: finish the work, then report. Do not ask permission to do what you were already asked to do |

**Stopping at a phase boundary is not asking permission.** The no-asking directive in `completion` bans "would you like me to...?" before work you were already asked to do. You MAY end a phase, report the result, and let the operator choose the next model or session. Rung 4 and rung 5 only look like a conflict if you read that directive as "never stop".

**When a higher rung forces a question, the question MUST be how, and it MUST NOT be whether.** "Which way do you want this fixed" is always legitimate. `May I skip it`, `may I drop the test`, `shall I defer this` are banned at every rung (`completion.md`).

**A question that `ai/META-DECISIONS.md`, the text of an RFC Ze implements, or an agreed design answers MUST NOT be put to the owner.** Act on the answer and name its source in the report. Three shapes recur and none of them is a question: whether to fix a logic bug or a misbehavior (fix it), whether to implement an absent requirement of an implemented RFC or follow its text (implement it), and whether to do what an agreed design needs, such as ExaBGP 6.0, 5.0 and 4.2 compatibility in the bridge and migration (do it). A problem that exists only because a test guest runs Alpine is out of scope, and its fix is moving the guest to Ze's own system.

**When the owner states a rule about how agents work or what is in scope, the main session MUST add it to `ai/META-DECISIONS.md` as a plain rule in the right topic section, in the same turn, and commit it then. The entry MUST NOT carry an id, a date or a quote.** A memory entry, a handover, a spec, a session state file and a Claude-only `.claude/` file MUST NOT be its only home: none of them loads into every agent's main session, so the rule is lost to the next one. A one-off permission for a single piece of work, such as a cap exception or approval to change named tests, is not such a rule: it goes in the spec or commit it applies to, never in that file. A rule elsewhere that depends on it MUST state the rule rather than cite it, and a rule that contradicts it is rewritten in the same commit.

**Deferral versus parking, settled by one question: does the goal this work exists to achieve still hold if I leave this?** If no, it is parking with a polite name: MUST fix it now (`completion.md`). If yes, it is separable future work, and its home depends on what it is.
**A defect you walked into MUST get one row in `plan/journal/<class>.md`, listed in the owned journal rows table of the spec that will fix it, then the work in hand closes and you stop: no row anywhere else, no question to Thomas. A distinct, larger, separable feature MUST be homed in a spec per `planning.md`, and Thomas MUST be asked whether that spec runs, unless an RFC Ze implements, an agreed design or a correctness defect already decides it: then the spec is written and runs without asking.**
**Automatic admission of RFC work MUST respect the owner's release timing (`ai/META-DECISIONS.md`): a missing RFC feature MAY wait until after the first release.** Owed post-release coverage MUST NOT become immediate implementation solely because its RFC is partially implemented.

**Closing comes first, and the same question decides the order as well as the verdict: a defect the goal does not depend on MUST NOT be fixed on the way to closing the work in hand.** `completion.md` makes you the owner of a defect you walked into, and it does not make you its owner this minute. Work that was finished but never landed is the most expensive failure this repo has, and an unrelated fix folded into a closing commit is its usual cause: it costs the commit its single focus and the review its scope, and it restarts the gates that were already green.
**The route is fixed and it has three steps: MUST write one row in `plan/journal/<class>.md` and list it in the owned journal rows table of the spec that will fix it, close the work in hand, and stop.** MUST NOT record it anywhere else, and MUST NOT ask Thomas whether to implement it (`completion.md`). MUST NOT fix it after closing either, unless its investigation is complete and the next point's conditions hold. Rows accumulate by problem class, and a class that collects rows earns its fix in a deliberate pass over the journal.
**Every open row MUST be owned: the owning spec is an existing one whose mechanism fits the defect, or a new one homed per `planning.md`.** `./le commit create` refuses a commit that adds or changes an open row no spec's table lists (`plan/journal/README.md`, "Owned rows"). Resolving rows follows `plan/post-release/engineering/spec-journal-resolution.md`.
**Two finds are fixed on the spot, and only these two: a test that is wrong about what it asserts, and code related to the problem in hand.** A red test or a red gate is not a third: `pre-release.md` routes it. A defect that blocks the goal of the work in hand is not a find at all. It MUST be fixed, because there is no closing the work in hand around it (`completion.md`).

**A defect whose investigation is complete, naming its cause at the producer and a concrete fix plan, MAY be handed to an implementation agent by the main thread without asking Thomas first.** The permission holds only while the fix touches no file another in-flight agent is changing and runs no whole-tree gate that would disturb test work in progress. The main thread MUST still ask before a fix that needs a product choice no RFC, agreed design or owner meta decision answers, or that changes an RFC-tagged test. Restoring correct behavior is never that choice, and the fix MUST still pass independent review before it lands.

**An exposed defect in an implemented capability MUST be handled under `completion.md`, according to the agreed task scope.** A failure that cannot be reproduced MUST carry the attempted reproduction and next step. An absent requirement of an implemented RFC MUST follow `rfc-compliance.md`: a gap record and owed implementation, never a question to the owner.

**Simplest-correct-solution MUST sit under rungs 2 and 3; it MUST NOT sit beside them.** `ai/rules/simplicity.md` requires the simplest fully correct answer, and "fully correct" is what rungs 2 and 3 already own. It cuts machinery: an abstraction with one user, an option nobody asked for, a layer that transforms nothing. It MUST NOT cut correctness, conformance, a test, a guard, or an error path, and quality is 0% compromise.
**The simplest design is usually the hardest to find. "This was the pragmatic option under time pressure" is the tell that a lower rung is being read as a license.** Not seeing the simple design is a reason to think longer, or to ask which way. It MUST NOT be treated as a reason to ship the complicated answer or the incomplete one.

**A rule's own subject matter MUST NOT be overridden by this ladder.** The ladder decides stop/ask/delegate/continue. It does not license writing `fmt.Sprintf` on a hot path because you were in a hurry, and it does not exempt you from `no-fabrication` at any rung.

**If the ladder does not resolve a conflict, MUST say so in one or two sentences, name both rules, state the reading you are taking, and proceed under it** -- unless the conflict sits on rung 1 or 2, where you MUST stop instead. Silently picking a side and not mentioning it is the failure this clause exists to prevent.
