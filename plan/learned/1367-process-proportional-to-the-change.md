# A review round that audits an over-engineered change ratifies it

**Date:** 2026-08-08

## What happened

A two-line regex change in a hook carve-out cost three review rounds and two
implementation agents. The first implementation was over-engineered. The rounds
that followed audited its details instead of asking whether the change should
have been two lines. The owner stopped the session and named the cost.

## Why the loop did not catch it

`ai/rules/planning.md`, "Bounding the loop", bounds the SCOPE of each round and
states plainly that there is no cap on the number of passes. Every term in it is
about what a round may look at. No term is about how big the change should have
been, so the loop applies identically to a two-line edit and a new subsystem.

The failure is self-feeding. Machinery that should not exist produces findings,
each fix is new code, and each fix earns a fresh pass over more machinery. The
loop converges on a well-audited version of the wrong shape.

## The rule

`ai/rules/context-economy.md` now carries "Process Is Proportional to the
Change": the directive that the first review question on any diff is whether the
change is bigger than its problem, a statement of what size does and does not
decide, and a table of the process each change earns. The size finding
is a BLOCKER naming the smaller change, and the review stops rather than
auditing on. `/ze-review` gained step 1 for the same check.

The saving is in making the CHANGE smaller. It is never in reviewing a given
change less, which `never-cut-review-gates-or-rule-reading-to-save-tokens` still
forbids.

## What two review rounds corrected

The value of this rule is almost entirely in where its boundary sits, and the
first two drafts put it in the wrong place twice. Both errors were the same
error: letting a small diff buy less process.

Round 1 found the table capping review passes, at "One review pass" for its two
smallest rows. `ai/rules/planning.md` "Bounding the loop" caps that number
nowhere, and only its RFC-and-interop class routes to rung 2, so a two-line
change that removed a guard would have earned a second round under planning.md
and none under the table.

Round 2 found the repair had moved the same error onto the agent axis: the table
then read "no implementation agent" for a few-line change. That is the excuse
`ai/rules/planning.md` "Banned Reasoning (delegation)" names first, "this edit is
small, I will just do it inline", answered by "size is judged after review". It
also contradicts this rule's own directive, "lower cost by SIZING agents, never
by spawning fewer of them".

So line count now decides the spec and the phase sequence, and nothing else. The
pass count and the agent count were never the load-bearing part. What stops the
runaway is the size question ending a review at one finding, so the
over-engineered implementation is rejected before its details generate the
findings that drive the next round.

## Files

- `ai/rules/context-economy.md`
- `ai/rules/points/context-economy/process-proportional-to-the-change/what-each-size-of-change-earns.md`
- `ai/rules/points/context-economy/what-this-rule-never-targets/never-cut-review-gates-or-rule-reading-to-save-tokens.md`
- `ai/rules/planning.md`
- `ai/rules/simplicity.md`
- `ai/skills/ze-review.md`
