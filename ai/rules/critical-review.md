# Critical Review Is the Central Deliverable

**When:** before closing a spec, or claiming any substantive change is done
**Severity:** blocking

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
4. **Looped to zero.** Every fix is new code that needs a fresh pass. Re-review
   until a pass finds nothing. No cap on passes.
5. **Evidenced by an artifact, not narrated.** Record the pass with
   `scripts/dev/review_gate.py record` → `tmp/review/<spec-stem>-<session-id>.md`
   (session-scoped, so concurrent same-spec sessions never clobber each other). It pins the
   SHA-256 of every code/test file the reviewers examined. The spec's Review Gate
   section pastes the reviewers' actual findings and each fix.

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
