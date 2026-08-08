# 1363 - A gate with two ends must derive both from one source, or it manufactures its own evidence

**Date:** 2026-08-08
**Scope:** agent workflow, verification, hooks

## What Changed

The spec-write gate `c_design_without_lsp` (`.claude/hooks/pretool-writeedit.py`)
became kind-aware: a spec whose `Files to Modify` or `Files to Create` names Go,
Python, shell, YANG or make must have had a file of THAT kind read this session.
The marker writer is `.claude/hooks/mark-source-read.sh`. Fixtures went from 256
to 311.

## The Failure

**The two ends of the contract disagreed, and each was defensible alone.**

`_SUBJECT_PATTERNS` demanded a kind for any `*.py`, `*.sh` or `*.mk` path a spec
named. `mark-source-read.sh` wrote a marker only for files under `scripts/`,
`.claude/hooks/` and `mk/`. So for a spec about `test/interop/interop.py` or
`packaging/deb/preinstall.sh`, **reading the very file the spec is about recorded
nothing**, and the only way past the block was to read an unrelated
`scripts/**.py`.

Measured over the open specs: 11 demanded a kind with no reachable route at all.

**The gate's sanctioned exit was reading a file with no bearing on the spec.** It
did not merely fail to demand evidence. It demanded a specific, useless act and
called it evidence. That is worse than the hole it was built to close, because
the hole was silent and this was mandatory.

## The second failure, in the same gate

Round 1 added a depth bar so a one-line read could not clear a spec. It treated
an unmeasurable payload as acceptable, reasoning that an unrecognised harness
shape must not disable the evidence path for a whole session.

**More shapes reach that default than the author expected.** Measured over 211
real transcripts: a file that is unchanged since the last read returns
`{"file":{"filePath":...}}` with no content (13 records), a failed read returns a
bare string (65 records), and an empty file returns `numLines: 1`. A zero-byte
`.py` in the tree therefore cleared every Python spec for 30 minutes.

The distinction that fixes it was available all along: ask whether the payload
carries a `file` object. A file object showing nothing is zero lines. No file
object and no error string is genuinely unrecognised, and only that keeps the
permissive default.

## What To Do Next Time

| Situation | Do |
|-----------|-----|
| A gate has a DEMANDER and a SUPPLIER | Derive both from one source, and write a fixture that walks real inputs through both ends comparing supply against demand. Two hand-maintained lists drift on the first new case, and the drift is invisible from either side |
| You are about to widen the demander to match the supplier | Check which direction leaves the gate ANSWERABLE. Narrowing the demander here would have made 13 specs subjectless and dropped them to a weaker bar: a gate that stops asking, not one that can be answered |
| You write "unmeasurable input is accepted" | Enumerate the shapes that reach it, from real transcripts rather than from the harness docs. Then split "the source told me nothing" from "I do not understand the source" and keep the permissive default only for the second |
| A refusal message states what the gate enforces | Trace the sentence to the producing function. Ours claimed a 20-line floor while a whole-file read passed at any size and one branch passed at zero |

## The thing that made both findings cheap

Both were found by **mutating the guard and watching the fixtures**, not by
reading it. Deleting a row from `_SUBJECT_PATTERNS` left 21 fixtures green for
three of five kinds, which is how the unpinned rows were found. Driving the hook
with a real transcript payload is how the depth hole was found. A guard you have
not tried to break is a guard you have not tested.

## Files

- `.claude/hooks/pretool-writeedit.py` -- `_SUBJECT_PATTERNS` keys on extension
  alone; `_spec_subject_kinds` reads both `Files to Modify` and `Files to
  Create`; `_subject_lines` scans only the first cell of a table row; the
  refusal message states what is actually enforced
- `.claude/hooks/mark-source-read.sh` -- the `case` block keys on extension and
  matches the demander exactly; `SHOWN` is derived separately from the harness
  line counts, so a read that displayed nothing writes no marker
- `scripts/dev/hook-fixture-check.py` -- per-kind pairs in both directions, plus
  `design-gate-contract-both-ends-agree-*`, which walks real spec subjects
  through writer and gate and compares supply against demand
- `ai/rules/evidence.md`, `ai/rules/repo-maintenance.md` and their point files --
  both ends now state the same extension rule
- `plan/learned/HOOK-FRICTION.md` -- the Bash bypass, the revert asymmetry, and
  the per-kind renewal cost

## Related

- `ai/rules/evidence.md` - the rule this gate backs, and the one it nearly broke
- `plan/learned/1362-partial-producer-read.md` - the same week's sibling: reading
  part of a producer and inferring the rest
