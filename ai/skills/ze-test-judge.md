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
on its own work. If you authored any part of the diff, stop and say so.

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

## Procedure

1. Read the old test from HEAD (`git show HEAD:<path>`), and the new test from
   the working tree or the diff. List what the old one asserts: each value,
   each error, each input it drives the producer with.
2. Read the producing code the test exercises: the function the assertion
   reaches, not its doc comment (`ai/rules/evidence.md`).
3. For a unit carrying `RFC requirement: <id> <polarity>`, read the cited
   section in `rfc/full/<stem>.txt` (fetch it if absent, per
   `ai/rules/rfc-compliance.md`). The `rfc/short/` summary is never the
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
author's reason restated are not (a) or (b).

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

One row per unit: unit, APPROVE or REFUSE, the verdict sentence, and the
command you ran (or the proof lost, for a refusal).
