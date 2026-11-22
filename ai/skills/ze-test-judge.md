---
name: ze-test-judge
description: Judge a test change that could lower what a test proves, as an independent agent
---

# Judge a Test Change

A test change that could lower what a test proves is judged here, never put to
Thomas (`ai/META-DECISIONS.md`, "Test changes"). That covers a weakening, any
change to an RFC-tagged test, and a golden file, fixture, `.ci` expectation or
expected output refreshed to turn a red run green.

**You did not make the change.** The agent that wrote it never runs this skill
on its own work. If you authored any part of a unit's diff, do not judge that
unit: say so and hand it back ("Several units in one run").

See also: `/ze-test` (writing a `.ci`), `/ze-rfc-audit` (whether RFC tests still
enforce their requirement), `docs/contributing/testing.md` ("When a test must
be weakened").

## Input

| Item | Form |
|------|------|
| The unit | `<package>.<TestName>`, or the `.ci`, golden or fixture path |
| The diff | the author's change to the test, and to the code if any |
| The reason | the author's one-paragraph claim of why the change is right |

A missing item is a REFUSE: you cannot judge what you were not shown.

## Several units in one run

A phase that owes several judgments sends them to ONE judge agent in ONE brief,
with the three Input items for each unit. One agent judges them all and gives
one verdict per unit. Batching shares the agent's start and changes nothing
else:

- **Each unit is judged on its own.** Run the Procedure for each unit. Its
  verdict rests only on its own old test, new test, producer and source.
  Another unit's verdict, diff or reason is never evidence for this one.
- **Each unit has its own verdict.** One verdict sentence per unit, one
  approval command per unit, so the commit carries one `Weakening-approved:` or
  `RFC-approved:` line per test. No sentence or row covers two units.
- **A refusal stands, unit by unit.** Refusing one unit does not reopen, soften
  or decide another. A unit missing an Input item is REFUSED and the others are
  still judged.
- **Independence holds for every unit.** If you authored any part of one unit's
  diff, say so, judge the others, and hand that unit back for another judge.

A judgment that must not see another's context stays alone, in a judge agent
of its own: a unit whose change the briefed agent authored, and a unit whose
author's reason rests on another unit's change, so that judging them together
would let one claim carry the other. The main thread decides this when it
writes the brief. A unit you find mid-batch to rest on another unit's change is
handed back for a judge of its own, and is not refused.

A handed-back unit gets the verdict HANDED BACK with its reason (you authored
part of its diff, or it rests on another unit's change). HANDED BACK is not a
refusal: it records no approval and decides nothing, and the main thread sends
the unit to another judge.

## Procedure

1. Read the old test from HEAD (`git show HEAD:<path>`), and the new test from
   the working tree or the diff. List what the old one asserts: each value,
   each error, each input it drives the producer with.
2. Read the producing code the test exercises: the function the assertion
   reaches, not its doc comment (`ai/rules/evidence.md`).
3. For a unit carrying `RFC requirement: <id> <polarity>`, read the cited
   section in `rfc/full/<stem>.txt` (fetch it if absent, per
   `ai/rules/rfc-conformance.md`). The `rfc/short/` summary is never the
   authority.
4. Decide.

## Verdict

**APPROVE** only when one of these holds, stated in one sentence:

- (a) The new test proves at least what the old one did: the same or stronger
  assertions, over the same inputs reached. A moved or renamed check counts
  when you found its new home.
- (b) The old expectation is wrong against its source: an RFC sentence quoted
  with its section, a spec acceptance criterion named by id, or the producing
  code's documented contract quoted. The new expectation matches that source.

**REFUSE** otherwise, naming the proof that is lost: the assertion that leaves,
the input no longer reached, the outcome now accepted that was refused. "The
new output looks right", "the code changed", "the test was flaky" and the
author's reason restated are not (a) or (b). HANDED BACK is given only for the
two reasons in "Several units in one run"; any other unit you cannot approve
is REFUSED.

A refusal stands. The author fixes the code; the test stays. Never ask Thomas,
and never soften a refusal into a question.

## Recording an APPROVE

You record it yourself, one call per unit, with your verdict sentence as the
reason. The row lands in the main thread's namespace, so the author's edit hook
and `./le commit create` see it.

| Change | Command |
|--------|---------|
| Weakening of a Go or `.ci` unit | `./le test weakened approve unit <package>.<TestName> reason "<verdict sentence>"` |
| RFC-tagged unit (also covers its weakening) | `./le rfc approve unit <package>.<TestName> reason "<verdict sentence>"` |
| Golden, fixture or expected-output refresh no command covers | no command: hand the verdict sentence back, and the commit body carries it as `Test-judge: <path>: <verdict sentence>` |

A `.ci` unit is named `<dir>.<stem>` (`docs/functional-tests.md`).

## Report

One row per unit: unit, APPROVE, REFUSE or HANDED BACK, the verdict sentence,
and the command you ran (or the proof lost, for a refusal, or the reason, for a
unit handed back). A batched run has one row for every unit in the brief, a
unit handed back included.

## Done when

Done when the report has one row per unit with APPROVE, REFUSE or HANDED BACK, the verdict sentence, and the approval command that ran, the proof lost, or the reason the unit was handed back.
