# No Parking: Fix Blockers, Never Reduce Coverage To Reach Green

**When:** when a defect blocks a goal the current work exists to achieve
**Severity:** blocking

## Directives

When a defect blocks a goal the current work exists to achieve, you FIX the defect. You do not park it, move it to `tmp/`, file it as a deferral, or offer to drop the deliverable. Interoperability and correctness are never "optional" and never a scope-reduction candidate. A network daemon that another implementation rejects has failed at its only job.

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
