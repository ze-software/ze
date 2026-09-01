# Learned: rfc-tag-claim-discrimination

A ninth ratchet in `./le rfc check`. An RFC evidence tag carries a structured
half every gate reads and a prose half nothing could read, so a tag was able to
advertise an assertion its body never made. The gate now demands a recorded,
replayable break under which the tagged unit itself goes red.

## What the design turned on

**"Is the prose true" is undecidable; "does this break redden this test" is not.**
The proof obligation is an observation, not a judgement about a sentence. That is
what made a gate possible over 3,900 tags where an audit had reached 1 RFC in 172.

**Two proof routes, neither subordinate.** gomu runs unit tests only, so 131 of
the in-scope tags are outside mutation by construction, and a generic operator
falsifies none of "FRR installs the route via the link-local next hop". `revert`
is a first-class route with a recorded observation, not a fallback.

**The floor is change-scoped, not scheduled.** The one dated quota this repo has
shipped (`rfc/drain-budget.txt`) is still at rate 0. A monotonic obligation needs
no rate guess and cannot stall while RFC work continues.

## Assumptions that broke, and how

**A-2 was written from a struct declaration.** gomu's `Result.TestOutput`,
`TestsRun`, `TestsFailed` and `Mutant.Function` are declared in the vendored
`gomu` `mutation.Engine` source and assigned NOWHERE in the production path:
`runTestWithOverlay` sets `Status`, `Output` and `Error` only, and the sole
consumer is the HTML report template. Measured 0 of 1,042, 0 of 2,128 and 0 of
513 results carrying either. The spec's own evidence rule caught the spec: a type
is a claim by its author, never the evidence. Attribution is recovered from
`--- FAIL: <name>` in the raw output instead.

**A-9 was quoted, not measured.** Interop costs 576s cold and 227s warm on this
machine, against the spec's 353s / 21-150s. A full interop record took 722s.

**A-4's citation bound was wrong in a way a count could not see.**
`checkNoExportBoundary` numbers some assertions by expression (`fail(index+2, err)`),
so bounding a citation by the COUNT of `fail(N,` sites capped a 9-assertion checker
at 4 and refused its own assertion 7. The bound is set membership over the numbers a
checker literally writes out.

## The escape was the hard part, and it was defeated twice

A closed vocabulary with per-reason preconditions was not enough.

- Round 2 found `foreign-producer` verified on carrier kind ALONE, so any of the
  37 interop tags discharged its obligation with nothing observed; and
  `declaration-only` checked only that the file THE AUTHOR NAMED declared no
  function, so naming any `doc.go` escaped any tag on any tier.
- Round 3's target, after the claim-prose tie was added: the tie accepted any
  whole word of the claim matching a top-level declaration in ANY function-free
  file in the checkout. Measured 605 of 4,020 claims (15%) carried such a word
  (`route` 118, `path` 117, `value` 111).

The settled shape ties the escape to code the tagged unit REACHES: a unit tag
reaches its own directory plus its file's imports, `.ci` and interop reach every
compiled file, nothing under `testdata/` is reached by anything, and an
unrecognized carrier takes the strict branch.

**Lesson: a closed vocabulary constrains the WORD, not the CLAIM.** An escape is
only as strong as the tie between the fact it asserts and the thing it discharges.

## The claim text has to be hashed separately (owner decision)

`behaviorBytes` strips comments and an RFC tag's claim sentence IS a comment, so
a sealed proof survived a REWORDED claim. An author could prove a modest claim
and then widen the sentence with no code edit at all. Measured: 2,701 of 3,900
tags carry a claim running past the tag's own line, so a one-line hash would have
left two thirds of the corpus free to widen. The claim is hashed as the comment
PARAGRAPH the tag opens.

## Gate scoping: where a ratchet may bite

Two owner rulings, and they are the same ruling twice:

- Drift is judged against HEAD, not the working tree. Working-tree-only drift is
  reported and counts as proven by nothing.
- The obligation bills against `HEAD^`; the wider backlog is MEASURED against
  `origin/main` and reported, never billed.

Both came from the same measurement: the obligation was reporting 12, then 36,
then 44 violations, every one on another session's uncommitted files, while
being unable to fire in the pre-push gate at all, because `./le verify worktree`
checks the commit out detached and a new tag is therefore already at HEAD there.

**Lesson: a gate over a shared checkout has to name the commit boundary it judges
at. "The working tree" bills bystanders and misses the author.**

## Test lesson

`TestDiscriminationRealRecordsSurviveAMechanicalRename` asserted every real
record verifies after a mechanical edit. A record can be stale BEFORE the test
runs, because any commit can move a producer a record fingerprints, and that
staleness is the ratchet working. The test now replays the unheadered tree as a
baseline and requires only that the header changes no verdict, failing if every
record was already stale so the replay cannot prove nothing.

## Known limitations

- The gate judges that a break exists, lies inside the producer, and reddens the
  unit. It does not judge whether the break is a GOOD one; a reviewer reads the
  stored mutated text.
- The standing corpus is grandfathered. Enforcing the wide reading was measured
  at 0 changed units against 3,768 candidates, so it is a cheap flip when wanted.
- `.et` carries zero tags, so the editor carrier is designed for and untested
  against real data.
