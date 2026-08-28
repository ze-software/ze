# Spec: reject-literal-has-a-producer

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Report a `.ci` negative assertion whose literal no production code can emit.

A `reject=stdout` or `reject=stderr` names a string the test forbids. When no
producer in the tree can write that string, the line cannot fire. It reads as
coverage on every audit and it constrains nothing. Deleting the mechanism it
guards leaves the same green, which is the property that makes it worthless
rather than merely redundant.

The failure is quiet in both directions. A line that never matched is
indistinguishable from a line that guards something rare, and the reader who
adds a sibling assertion beside it inherits the assumption that the first one
works.

**Measured 2026-08-22.** A mechanical sweep read all 616 `reject=stdout` and
`reject=stderr` sites in the `.ci` corpus and reduced them to 107 distinct
non-boilerplate literals. For each, the longest literal run of the pattern was
searched for a producer outside an assertion line. One survivor, and it was
real: the phrase lived in a single doc comment and in no format string, so the
branch it described did not exist. Nine other flags were composition false
positives, each confirmed by reading its producer.

So the yield today is one. The value is not the backlog, which is already
drained. It is that nothing stops the next one, and the sweep that found this
one was a scratch script that no longer exists.

## What the sweep did NOT cover

**Roughly 384 observer negatives test a VARIABLE rather than a literal.** A
`require('X' not in out, ...)` whose `X` is composed at runtime cannot be
searched for a producer without per-site analysis, and that is roughly one file
read each. This is the larger half of the corpus and the harder half. A first
implementation that covers only literals must SAY so where a reader will meet
it, or it recreates the defect it exists to catch: a check whose green is read
as a property of the whole corpus when it holds for part of it.

## Sketch (not a design)

| Element | Note |
|---------|------|
| Input | every `reject=stdout` and `reject=stderr` line in `test/**/*.ci` |
| Literal extraction | the longest literal run of the pattern, with the regex metacharacters removed |
| Boilerplate exclusion | one reviewable list, held in one place, for `panic`, `fatal error` and the rest |
| Producer search | the literal against non-test Go, with `%s` and `%q` composition tolerated |
| Output | one row per site: the file, the line, the literal, and that no producer was found |
| Severity | report only, exit 0, until the corpus has run clean for long enough to ratchet |

## Open questions

- Is a producer search over composed format strings cheap enough to be a gate, or does tolerating `%s` make the false-positive rate carry the check into the noise the day it lands?
- Should the same check read the POSITIVE assertions? An `expect=` whose literal no producer emits fails loudly, so it needs no gate. But an `expect=` that a producer emits unconditionally is the same defect wearing the other sign, and nothing finds that either.
- Does this belong beside `internal/le/weakened/audit.go`, which already reasons about assertion counts per file, or as its own stage?

## Why this does not block the first release

No shipped behaviour depends on it. No defect reaches an operator through it.
The one instance it would have caught is fixed, so it starts with an empty
backlog, and a check with nothing to report is the cheapest possible moment to
introduce one. It belongs after the first release, per `plan/future/README.md`.
