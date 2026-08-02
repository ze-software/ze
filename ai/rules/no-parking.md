# No Parking: Fix Blockers, Never Reduce Coverage To Reach Green

**When:** when a defect blocks a goal the current work exists to achieve
**Severity:** blocking

## Directives

When a defect blocks a goal the current work exists to achieve, you FIX the defect. You do not park it, move it to `tmp/`, file it as a deferral, or offer to drop the deliverable. Interoperability and correctness are never "optional" and never a scope-reduction candidate. A network daemon that another implementation rejects has failed at its only job.

**Recording a problem is not addressing it. Fix the root cause, always.** Writing a
failure down -- in `plan/known-failures/`, a spec, a learned summary, a deferral row,
or a report to the user -- changes nothing about the product. A record is a step
*toward* a fix and never a substitute for one. When you find a red test, a hang, a
wrong result, or a silent misbehavior, the deliverable is the FIX.

## Recording is not fixing (owner directive, 2026-07-23)

**"ALWAYS" is literal.** Encountering a defect while doing something else is not a
reason to catalogue it and move on. It is the reason you are now the one who fixes it.

| What you are about to do | Do this instead |
|---|---|
| Add a `plan/known-failures/` entry for a test that fails deterministically | Diagnose it (`ai/rules/diagnosis-before-fix.md`) and fix the root cause |
| Write "pre-existing, tracked in known-failures" in a report | Fix it. "Pre-existing" describes when it started, not whose it is |
| List failures in an Executive Summary as though listing were the deliverable | Every listed failure is either fixed, or has a named reason you are blocked on it |
| Note that a tool is broken and work around it | Fix the tool. You just proved it does not work |
| Record an inert config surface, a dead registration, or an unwired symbol | Wire it, delete it, or reject the config -- pick one and do it |

**The one narrow exception**, unchanged from `ai/rules/anti-rationalization.md`: a
**non-deterministic** failure whose MECHANISM you could not determine may get a
`plan/known-failures/` shard, and only as the running record of an investigation you
are still driving. It must carry the reproduction command, the evidence gathered, and
the next step. A shard is a live investigation, never a resting place, and never a
substitute for a fix on anything that reproduces.

**Host load does not qualify** (`ai/rules/fix-dont-record.md`, owner directive
2026-07-26). Once you can say "it fails when the machine is busy" you have the
mechanism: the test asserts on elapsed time instead of on state. Fix the test to wait
on the condition. "Passes in isolation", "the failing set rotates", and "could not
reproduce on a quiet host" are restatements of that diagnosis, not grounds for a shard.

**A structural, deterministic, or reproducible failure has no recording path at all.**
Fix it.

**A hypothesis in a shard is not a finding.** If you record one, the next agent will
read it as fact. Before acting on an existing shard's stated cause, verify it against
source (`ai/rules/no-fabrication.md`) -- and when it turns out to be wrong, say so in
the shard. On 2026-07-23 a shard's "the plugin connection closes before verify is
dispatched" hypothesis was disproved by the first real stress run: the signature
appeared nowhere in the capture, and the true cause was a test-harness race
(archived in `plan/known-failures/RESOLVED.md`, "fixed startup deadlines fail
under CPU oversubscription").

## The failure this rule exists to stop

A required deliverable (an interop test, a functional test, a goal-validation
row) was blocked by a bug. Instead of fixing the bug, the agent:

- proposed dropping the deliverable to close the spec, or
- proposed relaxing an assertion / removing coverage to reach green, or
- moved the unfinished work and the bug report into `tmp/` and called the rest done, or
- labelled the bug "pre-existing" and treated that as permission to leave it.

Every one of these is banned. The bug being pre-existing does not make it
someone else's problem: **the moment your work depends on that code path
working, the bug is in scope.** You found it because you are the first person
to exercise the path end to end. That is exactly the person who fixes it.

## The distinction from legitimate deferral

`ai/rules/deferral-tracking.md` exists for genuinely separable, out-of-scope
future work. It is NOT a hatch for a blocker. Decide with one question:

**"Does the goal this work exists to achieve still hold if I leave this?"**

| Situation | Verdict |
|-----------|---------|
| The goal still works; this is a distinct, larger, separable feature | Deferral is legitimate. Home it per `deferral-tracking.md`. |
| The goal does not work / a peer rejects the output / a required test cannot pass | NOT a deferral. Fix it now. Parking it is an invisible scope reduction with a polite name. |

If you are unsure which side you are on, you are on the "fix it" side. The cost
of over-fixing is some extra work; the cost of parking a real blocker is
shipping something that does not do what it claims.

**A defect you own is not a defect you fix this minute. When it does not block the goal, the order is: close the work in hand, then fix it.** "ALWAYS" governs WHETHER you fix it, never WHEN. Fixing it first is how finished work fails to land: the closing commit loses its single focus, the review loses its scope, and the gates that were green run again. Name it, home it per `deferral-tracking.md`, close, then come back for it (`ai/rules/rule-precedence.md`).

## Banned moves

| Banned | Why |
|--------|-----|
| Offering the user "drop the interop / functional test" as an option | Reducing coverage to reach green is the failure, not a choice. Do not put it on the table. |
| Weakening or deleting a test so a red goes green | `ai/rules/no-test-deletion.md`, `ai/rules/no-workarounds-for-missing-behavior.md`. The test describes the behaviour; the code is what is wrong. |
| "Pre-existing defect, out of scope for this spec" when it blocks the goal | You are the entry point that reaches it. Fix it, or say plainly that you are stuck and why — never quietly route around it. |
| Moving unfinished work or a bug report to `tmp/` and reporting the rest as complete | `tmp/` is not a destination. Parked is not delivered. |
| Marking a goal-validation row "N/A" or "blocked" to avoid the work | An empty goal validation for a completed feature is a false completion (`ai/rules/interop-and-goal-validation.md`). |

## When you genuinely cannot finish

Being blocked is allowed. Hiding it is not. If a fix is beyond the session
(needs hardware you lack, a decision only the owner can make, or is a deep
redesign), then:

1. State plainly that the goal is NOT met and why, with the evidence.
2. Keep the spec OPEN. Do not close it, do not claim the deliverable.
3. Do the fix if it is at all within reach before asking. Reach for the fix
   first; ask second.
4. If you must ask, ask "which way do you want this fixed", never "may I skip
   it". Scope reduction is the user's call to volunteer, never yours to propose.

## Verification

The goal is met when the real, user-visible path works against the real
counterpart — the peer daemon accepts the routes, the functional test passes
through the daemon, the interop scenario is green in the suite (not parked).
A passing unit test is necessary, never sufficient, for a goal that is about
interoperating with something outside this codebase.
