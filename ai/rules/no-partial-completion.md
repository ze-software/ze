# No Partial Completion

**When:** Read before claiming any work "done"; every acceptance criterion needs working code plus a test, "deferred" is not "done," and scope cuts require explicit user approval.
**Severity:** advisory

## Directives

**BLOCKING. ABSOLUTE PROHIBITION. Same level as git safety.**

## The Problem

Claude claims "done" or "ready to commit" while in-scope work remains unimplemented,
buried in a deferral list or silently dropped. This is the single most damaging
pattern in this project: the user trusts the completion claim, only to discover
later that 1/3 or 2/3 of the feature was never built.

## The Rule

**You may not claim work is done, complete, ready to commit, or ready for review
while any in-scope acceptance criterion remains unimplemented.**

"Deferred" does not mean "done." "Tracked in a plan/deferrals/ shard" does not mean "done."
"Will be handled in a follow-up" does not mean "done." If the spec lists it and
you did not build it, the work is not done.

## What "Done" Requires

Every single one of these must be true before you say "done":

| # | Requirement |
|---|-------------|
| 1 | Every acceptance criterion in the spec has working code |
| 2 | Every acceptance criterion has a unit test (`_test.go`) that exercises its logic |
| 3 | Every user-facing behavior has a functional test (`.ci`/`.et`) per `ai/rules/functional-test-gate.md` |
| 4 | Protocol features have interop tests per `ai/rules/interop-and-goal-validation.md` |
| 5 | Goal Validation table filled with concrete evidence per goal |
| 6 | The code compiles and `make ze-verify` passes |
| 7 | No TODO, FIXME, or stub remains in the new code |
| 8 | No item was silently dropped from scope |
| 9 | Every function is reachable from a user entry point (wired, not just library) |

If ANY of these is false, you are not done. Say what remains and keep working.

## Banned Phrases When Work Remains

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

If you genuinely cannot complete an item (missing infrastructure, blocked by
another component, would require user decision):

1. **Say explicitly:** "I cannot complete X because Y. The work is NOT done."
2. **Keep the spec open.** Do not close it. Do not write a learned summary.
3. **List what works and what does not** in plain terms, no hedging.
4. **Ask the user** what they want to do about the incomplete items.

Never bury incomplete work in a deferral table and then present the task as finished.
The user reads "ready to commit" as "everything works." Honor that reading.

## Scope Reduction Requires Explicit User Approval

**ABSOLUTE PROHIBITION. No self-authorized scope reduction. No exceptions.**

If during implementation you discover that an acceptance criterion or deliverable
is harder than expected:

1. **Stop implementing.**
2. Tell the user: "AC #N is harder than expected because X. Do you want me to
   continue with it, or drop it from this spec?"
3. **Wait for an answer.** Do not proceed without it.
4. Only if the user explicitly says to drop it may you proceed without it.

You may NOT unilaterally decide an AC is "out of scope," "a follow-up," or
"better handled separately." That is scope reduction dressed as planning.

### Documentation Is Not a Substitute for Implementation

Writing "documented as known limitation" or "deferred to integration tests"
does NOT resolve a missing deliverable. It is scope reduction without permission.
The same applies to:

- "Requires infrastructure not available" -- attempt it first; ask only if genuinely blocked
- "Noted in the spec as a deviation" -- a deviation entry is tracking, not resolution
- "Unit tests cover this; functional tests can come later" -- both are required per the spec
- "Will be handled in QEMU/integration/follow-up" -- that is deferral, not completion

If the deliverable is in the spec, implement it. If you cannot, stop and ask.
Never present documentation of a gap as closure of the gap.

## On Violation

Same as git safety: STOP immediately. "The task requires it" is not valid.
Nothing overrides this prohibition. If you catch yourself about to say "done"
with items remaining, that is the signal to keep working, not to ship.
