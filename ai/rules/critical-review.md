# Critical Review Is the Central Deliverable

**When:** before closing a spec, or claiming any substantive change is done
**Severity:** blocking
**Related:** no-parking, deferral-tracking, model-selection, quality

## Directives

Before closing a spec or claiming a substantive change is done -- review is INDEPENDENT (subagents / fresh session), never the author's own inline reasoning, and is enforced by `commit_helper.py`.

Review is not the last box before commit. It is the highest-leverage step in
development, and it is the one most easily faked. This rule makes it independent,
evidenced, and structurally unskippable. Rationale and the failure that motivated
it: `ai/rationale/critical-review.md`.

## The one load-bearing rule

**A review is performed by a DIFFERENT context than the author.** Independent
review subagents (`Agent` / `fork`) over the actual diff, or a fresh session.

**Your own inline reasoning about code you just wrote is NOT a review.** The
author is the one party guaranteed to share the blind spot that produced the bug.
Writing "I checked it, 0 issues" into a Review Gate from your own analysis is the
exact failure this rule exists to stop. It has shipped real bugs that independent
reviewers caught on the same diff minutes later.

## What a real review pass is

1. **Independent.** Spawn ≥2 reviewer subagents over the diff, each a distinct
   lens (logic/wiring/removed-behavior; security/edge-cases/test-quality; the
   feature's own risk area). They read the PRODUCER, not the caller,
   and verify claims against source (`ai/rules/no-fabrication.md`).
2. **Adversarial.** The question is "what can go wrong that nobody planned for?"
   Default findings PLAUSIBLE, not dismissed. Never discard wiring, removed-guard,
   logic, or vacuous-test findings.
3. **Verify the reviewers too.** A reviewer can be wrong. Before acting on a
   finding, reproduce it (an empirical check beats an argument — a `.ci` exit
   assertion that "should fire" either fires or does not; run it).
4. **Looped to zero over a SHRINKING scope.** Every fix is new code and earns a fresh pass. Each pass reviews less than the one before it. There is no cap on the NUMBER of passes, and a hard bound on what each one covers. See "Bounding the loop" below.
5. **Evidenced by an artifact, not narrated.** Record the pass with
   `scripts/dev/review_gate.py record` → `tmp/review/<spec-stem>-<session-id>.md`
   (session-scoped, so concurrent same-spec sessions never clobber each other). It pins the
   SHA-256 of every code/test file the reviewers examined. The spec's Review Gate
   section pastes the reviewers' actual findings and each fix.

## Bounding the loop

- **Round 1 reviews the whole diff. Round N+1 reviews ONLY the fixes round N made, plus what those fixes touched.** By default, a finding outside that scope does not re-open the round. Three bullets below override that default. Each override costs another pass (step 4). The overrides are: the goal depends on it, you are unsure whether it does, or it belongs to the always-in-scope list.
- **The loop ends when a round finds no BLOCKER and no ISSUE inside its OWN scope, AND no always-in-scope finding anywhere.** Both halves are required. The scope half alone lets an unconditional class satisfy the end condition by surfacing outside the round. A NOTE never re-opens a round, wherever it was found (`ai/rules/planning.md`, Review Gate). An always-in-scope finding is NEVER a NOTE, and its severity floor is ISSUE. Severity is the reviewer's own call. Without that floor, tagging one down is the cheapest exit from a list whose purpose is to have no exits.
- **The loop never required a round that finds nothing anywhere.** On a diff of any size, a full-diff pass always finds something. That reading has no state in which it stops, so finished work cannot close.
- **A finding outside the round's scope is fixed in this round when the goal this work exists to achieve depends on it. It is homed otherwise.** That is `ai/rules/no-parking.md`'s question unchanged. The test is DEPENDENCY, never causation. A defect this change did not introduce is in scope the moment the work depends on that path, which is what "pre-existing" never excuses.
- **If you are unsure whether the goal depends on it, you are on the fix-it side.** `ai/rules/no-parking.md` sets that tie-break and this bound does not soften it. Over-fixing costs some work. Homing a real blocker ships something that does not do what it claims. A rule that licenses closure is where an unsure call must fall towards fixing.
- **Eight classes are ALWAYS in scope, whatever round surfaces them and whoever caused them: an unwired symbol, a vacuous test, an acceptance criterion with no test, a user-facing behavior with no functional test, Linux-only code with no QEMU test, a removed guard, a newly added guard that fails open, and any RFC or interop non-conformance.** Each one passes a "no wrong result, no red gate" screen because its failure mode is silence. Nothing is wrong on the surface. The path is never exercised.
- **Where the round's scope and that list disagree, the list wins and the loop takes another pass.** The scope bound is a rung-3 instrument (`ai/rules/rule-precedence.md`). Conformance owed outside this repo sits on rung 2. Nothing about bounding a review loop CAN retire an RFC or interop obligation (`ai/rules/rfc-compliance.md`, `ai/rules/interop-and-goal-validation.md`).
- **Each class has its own authority.** Step 2 above covers wiring, removed-guard, logic and vacuous-test findings. `ai/rules/no-partial-completion.md` covers an untested acceptance criterion, `ai/rules/functional-test-gate.md` user-facing behavior, `ai/rules/qemu-testing.md` Linux-only code, and `ai/rules/fail-closed-guards.md` a guard that fails open.
- **The home is a destination spec that OUTLIVES this closure, never this spec's own deferral shard.** A shard whose rows are all terminal is `git rm`d at closure (`ai/rules/deferral-tracking.md`), and a row written into it minutes before closing is either resolved by that closure or is the thing keeping the shard alive. Neither outcome is a home: the shard records where a row came FROM. Name a `plan/spec-*.md` that exists on disk.
- **Two readings, and the one that governs.** "Fresh eyes on every pass, the full diff each time" asks a pass to see the whole change. "Loop until a pass finds nothing" asks the loop to converge. Applied to every round at once they contradict, and the agent that tries to satisfy both cannot close its work. Round 1 owns the whole-diff reading. Rounds 2 and later own convergence.
- **Write the round's scope down BEFORE the round runs, in the spec's Review Gate section.** Unwritten, "what those fixes touched" is chosen after the findings are known, and shrinks to whatever produces a clean round. Written first, it holds when the reviewer is tired, invested, or wrong about severity. It includes the sibling call sites of every changed function (`ai/rules/quality.md`, question 8), not only the edited hunks.

## State the review effort before you spend it

- **Name the pass count and the lenses BEFORE the first agent is spawned, so the operator can stop you.** An unannounced fan-out is a decision taken on the operator's behalf.
- **Match the effort to the ask ABOVE the floor step 1 sets, never below it: two lenses on round 1 always, three or more when the ask is "audit this" or "be thorough".** Round 1 is the only pass that ever sees the whole diff. Its lens count IS the whole change's coverage, and is never cut to one. Effort is chosen from the request, never from how interesting the code turned out to be.

## The review model

Review runs on Opus 5 (`ai/rules/model-selection.md`). Two gates enforce it: the spawn of a review agent, and `review_gate.py record`. A review performed on the implementation model is usually the session that wrote the code, which is the independence this rule exists to protect.

## Enforcement (structural — a hook, not discipline)

`scripts/dev/commit_helper.py` refuses a spec-closure commit (one that adds a
`plan/learned/NNN-*.md` or removes a `plan/spec-*.md`) unless `review_gate.py
check` passes: a CLEAN artifact exists, covers every reviewable file in the commit
(the ze-close closure commits all of a spec's code in commit A, so that is
full coverage), and its hashes still match (any edit after the review invalidates
it, forcing a fresh pass). A code-free closure still requires a clean artifact to
exist. Override with `--review-override <reason>` only as an explicit owner
decision (printed in the helper output alongside `--unverified`).

**What the hook can and cannot prove.** It proves a *fresh, hash-pinned, clean
artifact covering this commit's code exists* — so you cannot close by narrating
"0 issues" into the spec, and you cannot review then quietly edit. It does NOT
prove a genuinely independent context did the reviewing: the artifact is recorded
by convention (record the reviewer subagents' ids in `--reviewers`). It raises the
floor from "type clean into the doc" to "record a covering, still-matching review";
real independence rests on the skill mandate above, not on the gate. Known
residuals to not lean on: the coverage check only sees THIS commit (code committed
in earlier feature commits then closed code-free is under-covered — commit all of
a spec's code at closure), and the check runs when the commit script is generated,
so do not edit code after generating the script.

## Banned rationalizations

Each is a signal you are about to skip the independent pass. If you think one,
stop and spawn the reviewers.

| Banned | Reality |
|--------|---------|
| "I reviewed it as I wrote it." | That is authoring, not reviewing. Same blind spot. |
| "The tests pass, so it's correct." | Tests can be vacuous (dead exit codes, cumulative-match needles). A reviewer finds the vacuous test; the green bar does not. |
| "It's a small/mechanical change." | Renames collide roots; one-line guards fail open. Size is judged after review. |
| "I already know this code is correct." | The bug is precisely what you're sure isn't there. |
| "ze-validate / lint passed." | Those are mechanical gates. They are not a critical review. |
| "Re-running review is wasteful." | The fix is new code. Unreviewed new code is the next bug. |

## Scope

Every spec closure and every substantive code change runs this. Trivial doc-only
or generated-file-only changes are exempt (nothing to review). When unsure,
review.
