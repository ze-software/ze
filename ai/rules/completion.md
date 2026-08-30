# Finishing Work

**When:** before claiming any work done, complete, or ready to commit, and whenever a defect, a red test, or a missing behavior blocks that claim
**Severity:** blocking
**Related:** planning, testing, interop-and-goal-validation, writing, evidence, rule-precedence

## Directives

**BLOCKING. This rule MUST be treated as an ABSOLUTE PROHIBITION, at the same level as git safety.**

**You MUST NOT claim work is done, complete, ready to commit, or ready for review while any in-scope acceptance criterion remains unimplemented.**

**When a defect blocks a goal the current work exists to achieve, you MUST fix the defect.** You MUST NOT park it, move it to `tmp/`, file it as a deferral, or offer to drop the deliverable.
**This point covers the BLOCKING defect, the next one covers the RELATED defect, and the one after that covers every other defect you find.** A defect that neither blocks the goal nor belongs to the problem in hand is separable: it gets one row in `plan/journal/<class>.md`, the work in hand closes, and nothing else is owed.

**The unit you fix is the PROBLEM, not the files you happened to open (owner directive, 2026-08-10).** You MUST fix the code you are editing AND the code related to the problem that you are not editing, its tests included. A related defect living in a file nobody opened is part of the work, and "I was not in that file" is not a boundary.
**Related means it shares the problem, not the diff.** The other call site of the function you corrected, the sibling path that carries the same defect, the test that asserts the behavior you just changed, the fixture that encodes the old shape: each one leaves the problem half-fixed if you leave it, so each one is in scope now.
**Everything else you notice gets ONE journal row, for later analysis.** A defect or a missing feature that belongs to a different problem is recorded in `plan/journal/<class>.md` and nothing more: no spec, no deferral row, no question, no report paragraph ("A problem you FIND", below). Rows accumulate by class, and a class that collects rows earns a deliberate pass of its own.

**A problem you FIND while working on something else gets a JOURNAL ROW, not a spec (owner directive, 2026-08-10).** You MUST append one row to `plan/journal/<class>.md`, close the work in hand, and stop. No spec, no deferral row, no question to Thomas, no report paragraph. Rows accumulate by problem class, and a class that collects rows earns a fix later, in a deliberate pass over the journal rather than by whoever tripped over it.
**Three finds are FIXED on the spot, and they are the only three.** A defect that stops a test or a gate from passing is fixed now. A test that is wrong about what it asserts is fixed now. Code related to the problem in hand is fixed now, edited or not, tests included ("The unit you fix is the PROBLEM", above). Everything else is one row.
**Fix it anyway when the fix is small, and still write the row.** A five-line correction needs no spec to license it, and `simplicity.md` governs its shape. Opening a spec to authorise a small fix is the overhead this directive removes.
**The cut is the goal, unchanged from `rule-precedence.md`: does the goal this work exists to achieve still hold if I leave this?** If it does not hold, the defect BLOCKS you and "Fix a defect that blocks your goal" (above) governs. If it holds, this point governs.
**You MUST NOT characterise the find beyond the row.** Five columns, one line each: `| Date | Spec | Surface | Symptom | Fix |`. Reproducing it, tracing its producer, sizing its blast radius and drafting its options are work nobody commissioned, and they cost the session that found it and every session that reads what it wrote.
**Before writing a row, grep `plan/journal/` for the same symptom.** Many sessions run this checkout at once and meet the same defect. A second row for a find already recorded adds nothing; a row in a class file that already holds three is the pattern that earns the fix.

**Interoperability and correctness MUST NOT be treated as "optional" and MUST NOT be a scope-reduction candidate.** A network daemon that another implementation rejects has failed at its only job.

**Recording a problem is not addressing it. You MUST fix the root cause, always.** Writing a failure down (in `plan/known-failures/`, a journal row, a deferral row, or a report to the user) changes nothing about the product. A record is a step *toward* a fix and never a substitute for one. When you find a red test, a hang, a wrong result, or a silent misbehavior, the deliverable is the FIX.
**The JOURNAL ROW is the one exception, and only on the route "A problem you FIND" (above) sets out: one row, close the work in hand, stop (owner directive, 2026-08-10).** This point governs the defect you were sent to fix, where a record instead of a fix is the failure. It does not govern the defect you merely walked into, which is not yours to fix and whose row is the whole obligation. You MUST NOT write a SPEC for a walked-into defect, and you MUST NOT ask Thomas whether to implement one.

**Before changing code to make a symptom go away (failing test, rejected input, error, red gate, broken demo), you MUST write the Diagnosis first.** Editing to silence the symptom before the root cause is named is the defect, not the fix.

**If a user could experience a problem while trying to achieve a goal, you MUST implement the missing behavior at the source.** You MUST NOT bypass, mask, special-case, weaken a check, adjust a fixture, or route around the problem just to pass a test, demo, gate, or narrow scenario.

**Every exported function, type, or constant created by a spec implementation MUST have at least one caller in the running daemon.** "Library code with tests" is not done. "Tested but not wired" is not done.

**Every new feature MUST be proven to work integrated, not just in isolation.** Every feature needs at least one end-to-end test from its intended usage point.

**Before marking any spec done, you MUST complete a line-by-line audit comparing spec to implementation.**

**You MUST NOT use phrases like `would you like me to`, `want me to`, `shall I`, or `I can` before completing work.** You MUST finish the task first, then report what was done.
**One ask is mandated rather than banned, and it comes AFTER the work is complete: the one line that names a spec you wrote for a problem you found and asks whether to implement it.** This ban is about the work you were already asked to do. It never reaches work Thomas has not commissioned yet.

**When you catch yourself explaining why a test, a gate, or a completion standard does not apply this time, you MUST answer "no."**

## The Rule

**The claim-done ban has no synonyms. "Deferred" MUST NOT be written as "done", and neither MUST "tracked in a `plan/deferrals/` shard" nor "will be handled in a follow-up".** If the spec lists it and you did not build it, the work is not done.

## What "Done" Requires

**Every requirement below MUST be true before you say "done". If ANY is false, you MUST say what remains and keep working.**
**A green gate over an unread diff is neither "done" nor "green".** Say what you have: the gates pass, and you have not read the change. A subagent's report is a claim, so a main thread that relays "green" before reading the diff has asserted something nobody checked.

**All eleven MUST hold:**

| # | Requirement |
|---|-------------|
| 1 | Every acceptance criterion in the spec has working code |
| 2 | Every acceptance criterion has a unit test (`_test.go`) that exercises its logic |
| 3 | Every user-facing behavior has a functional test (`.ci`/`.et`) per `ai/rules/testing.md` |
| 4 | Protocol features have interop tests per `ai/rules/interop-and-goal-validation.md` |
| 5 | Goal Validation table filled with concrete evidence per goal |
| 6 | The code compiles. A GREEN gate is NOT a requirement for done (`ai/rules/pre-release.md`): name each red you leave and say which side of the product-or-scaffolding line it falls on |
| 7 | No TODO, FIXME, or stub remains in the new code |
| 8 | No item was silently dropped from scope |
| 9 | Every function is reachable from a user entry point (wired, not just library) |
| 10 | You READ the diff, hunk by hunk, and every hunk is one you would defend. A gate covers what somebody thought to check, so a defect on a surface no gate reads survives a fully green run |
| 11 | Every generated artifact in the diff was produced by its generator, never edited by hand. When both are in the diff, the generator's output and the artifact are compared, and the comparison is a test |

## Banned Phrases When Work Remains

**These phrases MUST NOT be written while work remains:**

| Phrase | Why it is banned |
|--------|-----------------|
| "Ready to commit" | Implies nothing is left. If something is left, do not say this. |
| "Implementation complete" | Same. Not complete if items remain. |
| "Done, with the following deferred" | Contradicts itself. Done means nothing deferred. |
| "All core functionality implemented" | Redefining scope to exclude what you skipped. |
| "The remaining items are minor" | You do not decide what is minor. Implement them. |
| "Tests pass, ready for review" | Tests passing is step 10 of 12. Not the finish line. |
| "Should work for the common case" | Implement the uncommon cases too. |

## What To Do Instead

1. You MUST **say explicitly:** "I cannot complete X because Y. The work is NOT done."
2. You MUST **keep the spec open.** You MUST NOT close it. You MUST NOT write the closing journal row.
3. You MUST **list what works and what does not** in plain terms, no hedging.
4. You MUST **ask the user** what they want to do about the incomplete items.

**Incomplete work MUST NOT be buried in a deferral table and the task then presented as finished.** The user reads "ready to commit" as "everything works", and you MUST honor that reading.

## Scope Reduction Requires Explicit User Approval

**ABSOLUTE PROHIBITION. You MUST NOT reduce scope without explicit user authorization. No exceptions.**

1. **You MUST stop implementing.**
2. You MUST tell the user: "AC #N is harder than expected because X. Do you want me to
   continue with it, or drop it from this spec?"
3. **You MUST wait for an answer.** You MUST NOT proceed without it.
4. Only if the user explicitly says to drop it MAY you proceed without it.

**You MUST NOT unilaterally decide an acceptance criterion is "out of scope", "a follow-up", or "better handled separately". That is scope reduction dressed as planning.**

**These excuses MUST NOT be accepted as resolving a deliverable:**
- "Requires infrastructure not available" -- attempt it first; ask only if genuinely blocked
- "Noted in the spec as a deviation" -- a deviation entry is tracking, not resolution
- "Unit tests cover this; functional tests can come later" -- both are REQUIRED per the spec
- "Will be handled in QEMU/integration/follow-up" -- that is deferral, not completion

**If the deliverable is in the spec, you MUST implement it. If you cannot, you MUST stop and ask. Documentation of a gap MUST NOT be presented as closure of the gap.**
**"Documented as known limitation" and "deferred to integration tests" do NOT resolve a missing deliverable.** Each is scope reduction without permission.

## On Violation

**On violation you MUST STOP immediately, as with git safety. "The task requires it" is not valid, and nothing overrides this prohibition.**
**Catching yourself about to say "done" with items remaining is the signal to keep working, never to ship.**

## Recording is not fixing (owner directive, 2026-07-23)

**"ALWAYS" is literal.** Encountering a defect while doing something else is not a reason to catalogue it and move on. You MUST spec it, close the work in hand, and ask Thomas whether that spec runs. Only the defect that BLOCKS your goal MUST be fixed on the spot.

**Each of these MUST be fixed rather than recorded:**

| What you are about to do | Do this instead |
|---|---|
| Add a `plan/known-failures/` entry for a test that fails deterministically | Diagnose it (see "Diagnosis Before Fix" below) and fix the root cause |
| Write "pre-existing, tracked in known-failures" in a report | It is yours: "pre-existing" describes when it started, not whose it is. Blocks your goal, fix it now; does not, spec it, close, ask |
| List failures in an Executive Summary as though listing were the deliverable | Every listed failure is either fixed, or has a named reason you are blocked on it |
| Note that a tool is broken and work around it | Fix the tool. You just proved it does not work |
| Record an inert config surface, a dead registration, or an unwired symbol | Wire it, delete it, or reject the config: pick one and do it |

**The one narrow exception:** a **non-deterministic** failure whose MECHANISM you could not determine MAY get a `plan/known-failures/` shard, and only as the running record of an investigation you are still driving. It MUST carry the reproduction command, the evidence gathered, and the next step. A shard is a live investigation, never a resting place, and never a substitute for a fix on anything that reproduces.

**A structural, deterministic, or reproducible failure has no recording path at all.** You MUST fix it.

**A hypothesis in a shard is not a finding.** If you record one, the next agent will
read it as fact. Before acting on an existing shard's stated cause, you MUST verify it against
source (`ai/rules/evidence.md`), and when it turns out to be wrong, you MUST say so in
the shard.

## Verification debt is not defect debt

**VERIFICATION debt MAY be recorded. DEFECT debt MUST NOT be.** They are different
things and this rule bans only one of them. Verification debt is a gate that has not
yet run over code you believe is correct: a full `./le verify current mode full` you have not
waited for, a review not yet done, an index left stale by a concurrent session. There
is nothing broken to fix, only a check owed. Its home is
`plan/verification-debt/<session>.md`, one row per owed gate, written by
`./le commit create` when you pass an override.

**You MUST classify what you are holding, and take the action its row names:**

| What you are holding | Which debt | What to do |
|---|---|---|
| A gate you have not run yet over code you believe correct | verification | Record the row, commit, clear it later |
| A gate that ran and went red on YOUR code | defect | Fix it. The row is not available to you |
| A gate red on another session's uncommitted work | verification | Record the row naming whose work, commit |
| A test that fails deterministically anywhere | defect | Fix the root cause (see "Recording is not fixing" above) |
| A review not yet performed on a spec closure | verification | Record the row; the review is still owed before any push |
| Behavior an acceptance criterion requires and nothing implements | defect | Implement it. Nothing here makes an unfinished AC recordable |

**The override keywords on `./le commit create` are SELF-SERVICE. You MUST NOT stop
and ask Thomas before using one.** `unverified`, `structural-red-ok`,
`missing-full-verify-ok`, `stale-index-ok`, and `review-override` each take a truthful
reason, admit one unrun gate, and write its row. Several
sessions share this checkout, so the shared verify record is red for somebody else's
in-flight work nearly always, and work that was finished but never landed is the most
expensive failure this repo has (`ai/rules/rule-precedence.md`).

**Enforcement is at the PUSH, which is where code reaches users: `create push <remote>`
refuses while any row is open.** A commit that stays local costs nobody anything, so it
is not the place to hold the line. The next session reads the open rows at session
start and runs `./le commit debt-clear`, which re-runs each owed gate ONCE per pass
and writes `cleared` only where that gate exits 0. A row is never marked by hand: the
ledger records what a gate did, never what a reader believed.

**Two kinds of row the pass leaves open, and neither is a failure of it.** A row whose
gate is `independent critical review` names a human judgement with no gate to run, so
the pass reports it UNRUNNABLE and `/ze-review` answers it. So does a row whose gate
string no gate is registered for, which is what an older wording leaves behind.

## The failure this rule exists to stop

- proposed dropping the deliverable to close the spec, or
- proposed relaxing an assertion / removing coverage to reach green, or
- moved the unfinished work and the bug report into `tmp/` and called the rest done, or
- labelled the bug "pre-existing" and treated that as permission to leave it.

You MUST NOT do any of these.

**A bug being pre-existing does not make it somebody else's problem. The moment your work depends on that code path working, the bug is IN SCOPE and you MUST fix it.** You found it because you are the first person to exercise the path end to end, and that is exactly the person who fixes it.

## The distinction from legitimate deferral

**You MUST ask: "Does the goal this work exists to achieve still hold if I leave this?"**

**One question decides it, and you MUST answer it before deferring anything:**

| Situation | Verdict |
|-----------|---------|
| The goal still works; this is a distinct, larger, separable feature | Deferral is legitimate. Home it in a spec per `planning.md`, close the work in hand, then ask Thomas whether that spec runs. |
| The goal does not work / a peer rejects the output / a required test cannot pass | NOT a deferral. Fix it now. Parking it is an invisible scope reduction with a polite name. |

**When you are unsure which side you are on, you MUST take the "fix it" side.** The cost of over-fixing is some extra work; the cost of parking a real blocker is shipping something that does not do what it claims.

**A defect you own is not a defect you fix. When it does not block the goal, you MUST: spec it, close the work in hand, ask Thomas whether that spec runs, then stop.** "ALWAYS" governs WHETHER it gets fixed, never who fixes it or when. Fixing it yourself is how finished work fails to land: the closing commit loses its single focus, the review loses its scope, and the gates that were green run again. You MUST write the spec, home it per `planning.md`, close, then put the question to Thomas (`ai/rules/rule-precedence.md`).

## Banned moves

**These moves MUST NOT be made. Each reduces scope invisibly:**

| Banned | Why |
|--------|-----|
| Offering the user "drop the interop / functional test" as an option | Reducing coverage to reach green is the failure, not a choice. Do not put it on the table. |
| Weakening or deleting a test so a red goes green | `ai/rules/testing.md`, and "No Workarounds For Missing Behavior" below. The test describes the behaviour; the code is what is wrong. |
| "Pre-existing defect, out of scope for this spec" when it blocks the goal | You are the entry point that reaches it. Fix it, or say plainly that you are stuck and why. Never quietly route around it. |
| Moving unfinished work or a bug report to `tmp/` and reporting the rest as complete | `tmp/` is not a destination. Parked is not delivered. |
| Marking a goal-validation row "N/A" or "blocked" to avoid the work | An empty goal validation for a completed feature is a false completion (`ai/rules/interop-and-goal-validation.md`). |

## When you genuinely cannot finish

1. You MUST state plainly that the goal is NOT met and why, with the evidence.
2. You MUST keep the spec OPEN. You MUST NOT close it, and you MUST NOT claim the deliverable.
3. You MUST do the fix if it is at all within reach before asking. Reach for the fix
   first; ask second.
4. If you MUST ask, ask `which way do you want this fixed`, never `may I skip
   it`. Scope reduction is the user's call to volunteer, never yours to propose.

## Verification of the goal

**The goal is met only when the real, user-visible path works against the real counterpart, and you MUST NOT claim it before then:** the peer daemon accepts the routes, the functional test passes through the daemon, the interop scenario is green in the suite rather than parked.
**A passing unit test is necessary and never sufficient for a goal about interoperating with something outside this codebase.**

## Load is never an explanation. It is the bug.

**A test that passes on a quiet host and fails on a busy one is a BROKEN TEST. Load did not break it: load revealed that it asserts on elapsed time instead of on state. You MUST fix the test so load cannot reach it.**

**These MUST NOT be offered as a conclusion, in a shard, a commit body, a report, or a reply to the user. Each one already states its own diagnosis:**

| Banned | What it actually says |
|--------|-----------------------|
| "fails under load / on a loaded host" | the test waits a fixed time instead of waiting for the condition |
| "load average was ~11 vs ~2 earlier" | you measured the host instead of reading the test |
| "passes in isolation" | it depends on scheduling luck. That IS the defect, stated |
| "the failing set rotates, so it is not deterministic" | several tests share one timing assumption. Find it |
| "the contended-run detector did not trip" | that detector labels runs. It never absolves a test |
| "not reproducible, logged as non-deterministic" | you do not need a repro to fix a timing assumption. Read the test |

**There is no non-deterministic hatch for a load-sensitive test.** The exception in "Recording is not fixing" covers a failure whose MECHANISM you could not determine. It does not cover one you already explained by naming the host's load: that explanation is the diagnosis, and you MUST fix it directly rather than record it.

## Making a test load-proof

**You MUST find what the test waits ON, and make it wait for that thing rather than for a duration.**

**Each timing assumption MUST be replaced by the fix in its row:**

| Symptom | Fix |
|---------|-----|
| `time.Sleep` then assert | poll the condition with `fixture.Poll`, using `fixture.Dispatch` when the state comes from the engine |
| fixed deadline for startup, teardown or reconnect | wait on the readiness signal the daemon emits. If none exists, ADD one: a missing signal is a product gap, not a test problem |
| "at most N events in a window" | count between two state transitions, not between two clock reads |
| assert immediately after a command returns | wait for the effect to be observable, then assert |
| the test genuinely needs a kernel-global surface to itself | `option=exclusive:group=<name>` (`internal/test/runner/record.go`), not a longer timeout |
| a timeout that is "generous enough" | generous is a synonym for unknown. Bound it by a condition |

**Raising a timeout MUST NOT be offered as a fix. It only moves the load level at which the test lies.**

**Replacing a sleep with a real wait routinely exposes a genuine data race in the product. That is a feature of the technique, and it MUST NOT be treated as a reason to avoid it.**

**`./le stress-repro run` and the stress tooling exist to DIAGNOSE such a failure. They MUST NOT be used to decide whether it deserves recording.** A deliberate timer that IS the behavior under test stays, and says so in its comment (`ai/rules/testing.md`).

## Recording

**Before fixing a defect, `plan/` MUST be searched for a spec that already
describes it, and any spec found MUST be closed or corrected by the same work.**
Grep `plan/spec-fixit-*.md` first, then the rest of `plan/`, on the symbol, the
file, and the failure's own words. A fix that lands while its spec stays open
leaves the backlog counting work that is already done, and the next session
reads that spec as a task.

**A spec the fix discharges is closed by the fix, not after it.** State in the
commit which spec the work discharges, and close it on the route
`ai/rules/planning.md` gives. A spec the fix only PARTLY discharges is corrected
rather than closed: strike the acceptance criteria the fix met and say what
remains, so the reader inherits a smaller task instead of a stale one.

**A spec whose premise the fix disproves is closed too, with the disproof.** The
premise being wrong is a result, and it is worth more to the next reader than
the spec was.

The same search decides the journal row. A defect that already has a spec needs
no new row; the row belongs to a defect that has none.

**`plan/known-failures/` MUST NOT be used as a destination for a failure.** It is the running log of an investigation you are still driving, and it is empty the moment that investigation ends.

**Before writing any record of a problem you MUST answer: what did I fix?** When the answer is "nothing yet", the record is not the deliverable and does not substitute for one.

**These MUST NOT be written:**

| Do not write | Do |
|--------------|-----|
| a shard for anything that reproduces, or that load explains | fix it |
| a shard as the outcome of a session | fix it, and delete the shard if one existed |
| "pre-existing" anywhere as a reason | it is yours: the word says when it started, not whose it is. Blocks your goal, fix it now; does not, spec it, close, ask |
| the same failure in a shard, a commit body, a report and a summary | pick one place |

**A load excuse MUST NOT be written into a `plan/known-failures/` shard.** `checkLoadExcuses` (`internal/le/doc/wiring/docwiring.go`) fails a changed shard that carries one.
**The gate checks the EXCUSE, not the existence of a shard.** A red whose mechanism is genuinely unknown still belongs there.

**A journal row MUST reach git.** It is written to be read by a later session,
and an uncommitted row lives in one shared working tree and dies at the next
clean, stash or checkout. Writing it is not the obligation; landing it is.
Commit it with the work that found it, so a reader meets the row beside the diff
and needs no archaeology.

**`/ze-close` sweeps the rows when a spec closes, and most sessions do not close
a spec.** A session that ends any other way MUST commit its own rows first.

**The trap that strands them:** a row naming a spec makes `internal/le/commit`
read the commit as that spec's CLOSURE and demand the Review Gate artifact. The
obvious answer is to drop the rows "for now", and "for now" is the rest of the
session. A rows-only commit that adds no learned summary and removes no spec
closes nothing, and `--review-override` carries that reason: state in it what
the commit does NOT do, so the escape stays auditable.

## Length is not evidence

**A record earns its length from what a future reader has to DO, never from what you went through. An investigation MUST NOT be narrated:** the wrong hypotheses, the order you tried things, and how long it took are yours rather than the reader's. State the correction and move on. The budgets are in `ai/rules/writing.md`.

## Anti-Rationalization

**To every rationalization below, the answer is always "no". They MUST NOT be acted on.**

**None of these TDD excuses MUST be acted on:**

| Excuse | Answer |
|--------|--------|
| "Too simple to need a test" | Test it |
| "I'll write tests after" | Post-hoc tests validate implementation, not requirements |
| "TDD will slow me down" | Rework from bugs is slower |
| "Just a refactor" | The existing tests have to pass. None exist? Write them first |

**These test-failure excuses MUST NOT be acted on:**

| Excuse | Answer |
|--------|--------|
| "Transient" / "resource contention" | Investigate. A failure happened |
| "Only fails under load" / "passes in isolation" | That is the diagnosis, not an excuse: the test asserts on elapsed time. Make it wait on the condition (see "Load is never an explanation") |
| "Not related to our changes" | Fix it anyway. Include the fix in a separate commit script |
| "Passed on retry" | Retry is not evidence. Investigate the failure |
| "Timing-dependent" | Race condition. Fix it |
| "Pre-existing issue" | It is yours: "pre-existing" says when it started, not whose it is, and you are the entry point that reached it. Blocks your goal, fix it now; does not, spec it, close, ask |

**A test failure that says the PRODUCT is wrong MUST be fixed: by you when it blocks your goal, and by one journal row when it does not.** Logging is not an alternative outcome for a product defect (owner directive 2026-07-23; see "Recording is not fixing" above). A `plan/known-failures/` shard is the running record of an investigation you are still driving, never a place to leave a product defect.
**A test failure that says the SCAFFOLDING is wrong MUST NOT be fixed on the way past.** Name it in one line and step over it (`ai/rules/pre-release.md`). Fixture drift, a stale golden file and a broken runner path are instrument failures, and repairing them is how a session spends its budget on nothing.

1. **You MUST spec it, close the work in hand, then ask ("A problem you FIND while working on something else gets a SPEC", above). You MUST NOT block current work on a failure you did not cause, and you MUST NOT fix it in this session either: the fix runs when Thomas answers, as its own spec and its own commit, never mixed with the feature work you were closing.**
2. **A shard MAY be used for ONE case only: a failure whose MECHANISM you could not
   determine.** Deterministic reds, structural gates, anything with a reproduction
   command, and anything host load explains MUST be fixed, never sharded. When the exception
   does apply, you MUST add
   `plan/known-failures/<native-action>-<test-name>.md` with: failure output, the
   reproduction attempt and its result, evidence gathered, and the next step. You MUST label a
   root cause you have not verified against source a HYPOTHESIS, so the next agent does
   not inherit it as fact.
3. **Mechanical check before session end:** every failure your session encountered MUST be fixed, or
   MUST carry a spec that was put to Thomas, or MUST be a non-reproducible one whose shard names the next
   step. A failure that is none of the three is a violation regardless of what was written down.

**These MUST NOT be written. Each leaves a failure in place:**

| Banned | Why |
|--------|-----|
| "Pre-existing, not my changes" | Acknowledging a failure without fixing it means the next session hits the same wall |
| "Known issue with the netlink API" | Known to whom? And "known" is not "fixed" |
| Mentioning a failure only in response text | Response text is ephemeral, and describing a bug does not fix it |
| "The only failures are..." (then moving on) | Enumeration without action is rationalization |
| "Tracked in `plan/known-failures/`" offered as the outcome | Tracking is not fixing. The product is still broken. See "Recording is not fixing" |
| Adding a shard for a failure that reproduces on demand | A reproduction command IS the start of the fix, not a substitute for it |

**These completion excuses MUST NOT be acted on:**

| Excuse | Answer |
|--------|--------|
| "Should work" / "Probably fine" | Run it, paste output |
| "Tests passed earlier" | Run again now |
| "Only cosmetic differences" | Show diff, let user decide |
| "Library and interface only" | Feature is not done: library without wiring is dead code |
| "Wiring will be done in next commit" | One commit = code + tests + wiring + summary. No partial deliveries |
| "The .ci test requires infrastructure" | Then the feature is blocked, not done |
| "Unit tests prove it works" | Unit tests prove the algorithm. .ci tests prove the user can reach it |
| "SetAuthorizer is called somewhere" | Show the .ci test where a user command is denied. No test = no proof |
| "Consistent with how other plugins do it" | Other plugins missing tests is a gap, not a precedent |
| "No test infrastructure for this path" | Build the infra or flag as BLOCKER. Never downgrade to NOTE |
| "Out of scope for this review" | Missing coverage is never out of scope. Report as ISSUE |

**After three failed fixes you MUST STOP.** Report all three approaches, question the mental model, and ask the user.

**Performative agreement MUST NOT be written. Fix it, describe what changed, and move on.**
**You MUST assume your own implementation report is optimistic: re-read the spec and re-run the verification fresh.**

## Diagnosis Before Fix

**A diagnosis MUST state all five:**
1. **Symptom** -- the exact failure, verbatim (error text, rejected input, failing assertion).
2. **Root cause** -- traced to the exact function where behavior diverges from intent, named as the file plus the symbol. Read the path; you MUST NOT guess. If you cannot name it, you have not diagnosed it yet.
3. **Owning layer** -- which layer/component owns the correct fix.
4. **Two candidate fixes, labeled** -- at least one `[workaround]` and one `[source]`. Name what each changes and what each leaves broken for the next caller.
5. **Why not the workaround** -- one sentence on why the local edit is wrong.

**You MUST determine which of the three applies:**
- Is the **check** wrong? (the validation logic is incorrect)
- Is the **input** wrong? (you are doing the wrong thing)
- Is the check's **data/config** incomplete? (the check is right but its allowed-set / table / registry is missing an entry)

**You MUST always ask: am I fixing where the problem IS, or where it SHOWS UP?** A special case layered on shared infrastructure means the underlying mechanism MUST be generalized instead.

**"let me just rename", "just skip", "just special-case", "just adjust the test", "add a fallback", "quick workaround": the word "just" is the tell. You MUST stop, write the five diagnosis lines, and fix the source.**

## No Workarounds For Missing Behavior

**A workaround is evidence that the feature, the integration, the validator, or the test coverage is incomplete. The fix MUST make the user-visible goal work through the real entry point.**

**You MUST follow these steps to replace a workaround:**
1. Name the user goal the missing behavior is meant to satisfy.
2. Trace the code path meant to provide it.
3. Implement the missing behavior at the owning layer.
4. Update affected callers and tests.
5. Verify the user-visible goal directly.

**These MUST NOT be written as a fix. Each is a workaround:**

| Banned | Why |
|--------|-----|
| Weakening or simplifying a test expectation | The test describes the required behavior. The broken code is what changes. |
| Special-casing only the failing fixture | Users can hit the same class of problem outside the fixture. |
| Skipping validation, errors, or unsupported inputs | Silent acceptance hides missing behavior and ships an operator trap. |
| Adding compatibility shims, aliases, or fallbacks instead of clean cutover | Ze has no released compatibility contract. Keep one real path. |
| Bypassing the owning layer from a caller | The next caller will fail the same way. Fix the owner. |
| Hiding a failure behind retries, sleeps, or broad catches | This masks the defect instead of proving the goal works. |

**A workaround MAY be shipped only when the user explicitly asks for the workaround itself as the deliverable.** Its limitation MUST then be named in the implementation notes, and it MUST NOT be presented as the real feature.

**Verification MUST exercise the user-visible goal, not the workaround boundary.** A unit test can prove internal logic, but the behavior is not complete until a functional, integration, or command-level check proves the user reaches the feature through the intended path.

## Wiring Completeness

**Wiring MUST be the FIRST implementation step, never a verification step at the end.** Checking wiring for the first time at completion means three earlier gates already failed.

1. **Design phase:** the spec's Wiring Test table names every entry point before implementation starts.
2. **Implementation phase:** `/ze-implement` step 4 creates the entry point skeleton and a failing wiring test before any feature code is written.
3. **Review phase:** `/ze-review` step 1 checks wiring before any other analysis.
4. **Completion phase:** the mechanical check below catches anything that slipped through.

Each phase MUST perform the check it owns.

**`./le doc wiring` is a STRUCTURAL stage of `./le verify worktree`, so its red says the tree is broken and MUST be fixed rather than recorded.** It is a changed-file gate.

**The wiring gate MUST verify that:**
- new exported Go symbols under `internal/` or `cmd/` have a non-test
  production reference in `internal/` or `cmd/`;
- command declaration changes run `./le docvalid command-contract`;
- source-anchored documentation changes run doc drift and stale-anchor
  checks;
- plugin registration and generated inventory source changes run
  registry-backed inventory checks.

**A new exported symbol MUST have a non-test caller. Grep it across `internal/` and `cmd/`: if the only hits are its definition and test files, it is dead code, and dead code is a BLOCKER rather than a NOTE.**

**For multi-consumer data (route attributes, config fields, bus events) you MUST grep every consumer: UI templates, graph rendering, functional tests, CLI formatters.** Changing the producer without updating its consumers is incomplete, never done.

**These MUST NOT be offered for an unwired feature:**

| Pattern | Why it's wrong |
|---------|----------------|
| "The caller will wire it later" | Later never comes. Other sessions see it as done. |
| "It's available for callers" | Available is not wired. No caller means no effect. |
| "The review said NOTE" | A review flags unwired code as BLOCKER. |
| "The web UI doesn't need it" | If the feature produces data a UI page renders, the UI has to show it. |

**New code MUST be called from the site its row names:**

| New code in | Must be called from |
|-------------|---------------------|
| `internal/component/host/` | `cmd/ze/hub/main.go`, `loader_create.go`, `internal/component/cmd/show/system.go`, or `web/page_system.go` |
| `internal/component/config/system/` | `cmd/ze/hub/main.go` (startup + reload) |
| Any new metrics registration | `loader_create.go` telemetry block |
| Any new report bus emission | Verified via `show warnings` / `show errors` |

## Feature Integration Completeness

**Every new feature MUST be proven to work INTEGRATED, never only in isolation.**

**Each feature type MUST carry the test its row names:**

| Feature Type | Required Test |
|-------------|---------------|
| Injectable interface | Inject fake, verify component uses it |
| CLI flag | Flag changes program behavior |
| Config option | Option affects runtime behavior |
| YANG config leaf | Env var registered (`env.MustRegister`), appears in `ze env registered` |
| API/RPC | Caller reaches handler through real transport |
| Event/hook | Event fires, subscriber receives |
| Plugin capability | Engine dispatches to plugin correctly |
| Struct field | Field is read and affects a decision |

**Self-check:** "If I deleted all new code except tests, would any test fail because it tried to USE the feature through the intended path?" A "No" answer MUST be treated as isolation only, rule violated.

**Every user-facing feature MUST have a `.ci` functional test** in `test/` that exercises the feature from the user's perspective: config file, ze launch, command/event, expected output. A Go unit test proves the algorithm; a `.ci` test proves a user can reach and use the feature.

**Each feature type MUST have its `.ci` test in the directory its row names:**

| Feature Type | `.ci` Location | What the test does |
|-------------|----------------|-------------------|
| Config option | `test/parse/` | Config with option, ze parses without error |
| API/RPC command | `test/plugin/` | Config + peer, send command, verify wire/JSON output |
| Plugin behavior | `test/plugin/` | Config + plugin, trigger behavior, verify effect |
| CLI subcommand | `test/parse/` or `test/ui/` | Run subcommand, verify stdout/stderr/exit code |
| Wire encoding | `test/encode/` | Config with route, verify hex output |
| Wire decoding | `test/decode/` | Hex input, verify JSON output |

**A unit test MUST NOT substitute for a `.ci` test.** Unit tests validate logic in isolation. `.ci` tests validate the feature is wired, reachable, and usable. Both are REQUIRED.

**Deferrable (MAY be deferred):** advanced behavior (deterministic scheduler, fault injection, property testing, benchmarks).
**NOT deferrable (MUST NOT be deferred):** one `.ci` test proving the feature works from the user's entry point.

**A wiring test proves the feature is reachable from its intended entry point: config, CLI, event dispatch, or plugin launch. It is the minimum proof that a feature is integrated. For a user-facing feature it MUST be a `.ci` functional test, never a Go unit test.**

**These MUST NOT be offered for shipping an unwired feature:**

| Banned | Why |
|--------|-----|
| "Deferred to next spec" | Next spec won't pick it up. Feature ships unwired. |
| "Requires infrastructure not yet built" | Then the feature is blocked, not done. |
| "Unit tests cover the logic" | Unit tests prove the algorithm, not the wiring. |
| "./le verify worktree passes" | Passing tests that don't exercise the entry point prove nothing. |
| "Go test exercises the handler" | A Go test with mocked entry points is not a `.ci` test. |

**If the wiring test cannot be written, the feature MUST NOT be considered done: it is blocked.**

**Every spec MUST carry a `## Wiring Test` table (`plan/TEMPLATE.md`), and every row for a user-facing feature MUST name a `.ci` test file.**

**Before modifying any handler, dispatcher, or protocol step, you MUST grep for ALL implementations of it.** Ze has several code paths for one protocol step, so modifying one is not enough.

**You MUST take all four steps to find the implementation a consumer actually calls:**

| Step | Action |
|------|--------|
| 1 | Grep for the protocol method/handler name across all `.go` files |
| 2 | List every implementation found |
| 3 | For each consumer of the feature: trace which implementation it actually calls |
| 4 | Modify (and test) the implementation the consumer uses, not just any implementation |

**One implementation found MUST NOT be treated as proof there's only one.** Finding *a* handler is not the same as finding *the* handler the feature's consumer calls.

## Implementation Audit

**You MUST:**
1. Extract all requirements from spec: task items, AC-N assertions, TDD tests, files listed
2. Verify each with status: ✅ Done (file + symbol), ⚠️ Partial, ❌ Skipped, 🔄 Changed
3. Fill audit table in spec (template in `plan/TEMPLATE.md`)

- ⚠️ Partial: you MUST document what's missing, and you MUST ask the user
- ❌ Skipped: you MUST explain why, and you MUST ask the user
- 🔄 Changed: you MUST document deviation (no approval needed if improvement)

**Every item MUST be checked before the audit is complete:**
- [ ] Every Task requirement has a status
- [ ] Every AC-N has status + "Demonstrated By" evidence
- [ ] Every TDD test has a status
- [ ] Every file in plan has a status
- [ ] All Partial/Skipped have user approval
- [ ] Integration points verified (YANG, CLI, docs)
- [ ] Wiring Test table complete, every row has a test name, none deferred
- [ ] Audit Summary totals accurate

**Each claim MUST carry the evidence its row names, and MUST NOT carry what the last column lists:**

| Claim | Acceptable Evidence | NOT Acceptable |
|-------|-------------------|----------------|
| Feature works | Test name + output | "./le verify worktree passes" |
| Feature is wired in | Wiring test that exercises entry-to-feature path | Unit test with mock/fake entry point |
| AC-N done (wiring) | Functional test name exercising full path | Unit test in isolation |
| AC-N done (logic) | Unit test name + file, assertion matches AC text | "probably works" |
| AC-N done (behavior) | Test asserts the AC's expected behavior directly | Test asserts mechanism (e.g., "no error" as proxy for "rejected") |

**For each acceptance criterion you MUST quote its expected behavior from the AC table, then name the test and its assertion. The assertion MUST verify the BEHAVIOR, never only the mechanism.**  `ai/rules/testing.md` carries the behavior-versus-mechanism table.

**You MUST NOT trust the audit.** After filling the audit, you MUST independently re-verify every item.
This is a separate section in the spec (see `plan/TEMPLATE-CLOSURE.md`, appended at
`/ze-close` step 1). It requires FRESH evidence:

**Every closure table MUST be re-verified after the audit, by the method its row names:**

| Table | What to verify | How |
|-------|---------------|-----|
| Files Exist | Every file from "Files to Create" | `ls -la <path>`, paste output |
| AC Verified | Every AC-N | grep, test output, or ls, NOT a copy from audit |
| Wiring Verified | Every wiring test row | Read the .ci file, confirm it tests the claimed path |
| Assumptions Resolved | Every A-N | `confirmed` or `broken` with evidence; `unvalidated` is not a final status |
| Documentation Verified | Every Yes/No in the Documentation checklist | The edited claim checked against source, or the grep proving no update was needed |

**EVERY table MUST have at least one evidence row.** `pre_commit_verification_gaps`
(`internal/le/commit`) checks them one at a time and names the empty
ones on the closure commit. Each table is a separate obligation: a row in
`Files Exist` is not evidence for `AC Verified`.

**The following MUST NOT be used as evidence:** "Already checked in audit", `should work`, empty cells.

**Any of these MUST be treated as a sign the implementation is incomplete:**
- AC-N with no test or evidence
- Can't find where feature was implemented
- TDD test from plan doesn't exist
- File from "Files to Create" wasn't created
- New RPCs without functional tests
- New CLI commands without usage text
- Learned summary admits incompleteness ("not wired", "infrastructure only")
- Commit message says "library and interface only"

## Don't Ask, Do

**`would you like me to`, `want me to`, `shall I` and `I can` MUST NOT be written before completing work. Finish the task first, then report what was done.** The user delegated the work, so asking permission to start it wastes a round trip.
**This rule is hook-enforced: breaking it costs a blocked Stop rather than a note.**

**Two standing exceptions, where asking IS mandatory and this rule does not apply. You MUST ask on genuinely ambiguous scope, and on a destructive action the git safety rules gate behind confirmation.**

- **RFC compliance.** When full RFC compliance and full testing of that compliance is reachable, you MUST implement it and prove it: that is not a question for Thomas (`ai/rules/rfc-compliance.md`, "Implement Full Compliance. Ask Thomas Only Before Doing LESS"). You MUST ask only when you are about to choose something NARROWER, and then the question is "which way do I fix it". Doing more never needs permission.
- **Deleting or overwriting user-visible or uncommitted work** (`ai/rules/never-destroy-work.md`).
- **Reducing the scope of a spec or dropping an acceptance criterion** (see "Scope Reduction Requires Explicit User Approval" above).

- `hookStop` in `internal/le/hookruntime/lifecycle.go` scans the last assistant message and refuses a permission-seeking stop.
- The unconditional phrase list covers ownership-dodging and premature handoff. Completion questions join only while a claimed spec remains in progress.
- The harness retry flag skips the phrase scan alone. Spec-closure checks remain active, so a blocked stop is not permission to stop on the next turn.
- A banned phrase inside backticks or a closed fence is treated as QUOTED, not used, and does not block. You MAY write about the phrases freely. Four guards keep that exemption from becoming a bypass. An unclosed fence is not a code block. A fence closes only on a run at least as long as the opener. The hook scans an all-markup message raw. Inline spans are stripped only on a line whose backticks balance, so one stray backtick cannot swallow a real request.
- **A blocked Stop is not an instruction to do the work you just offered.** The block asks who wanted that work. The user wanted it: finish it, and do not ask again. You thought of it: DROP it, and MUST NOT start it, size it, or offer it a second time. The remedy read `Continue without asking permission` until 2026-08-19, which answered permission-seeking and misread an offer: a turn ending `Want me to spec the streaming writer?` was refused its end and then went and wrote that spec, so the gate against uncommissioned work was producing it.
- Neither list is exhaustive, so a green Stop is not proof you followed this rule. You MUST finish the work, then report.
- Fixtures: `./le hook-check unit`. Full hook map: `ai/rules/repo-maintenance.md`.
