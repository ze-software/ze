# Problem Journal

One file per problem class. Each file holds one table:

```
| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-07-15 | example-spec | reactor | FSM closed session instead of sending NOTIFICATION | fixed: the state transition table now sends it |
```

- **One file = one problem class.** The file name is the class name in kebab-case.
- **One row = one occurrence.** Recurrence is the count of rows dated after the
  class's capture, every row when it has none (below).
- **Date** is the day the occurrence was found, as `YYYY-MM-DD`. It is what the
  span is computed from, so anything else is refused.
- **Spec** names the spec that found this instance, or `-` when found outside a
  spec. The cell is an attribution key, so it holds spec stems separated by
  commas. An explanation goes in a trailing `(note)`, which the parser strips,
  or in the Symptom cell. Prose in place of a stem is refused when the commit
  is prepared. A row records an occurrence; it neither closes the named spec
  nor proves that its work is complete. The closure-candidate detector can
  use committed rows as a prompt to check an `in-progress` spec, but the commit
  review gate gets its closure stem only from a removed spec, excluding a
  relocation between release buckets (`docs/contributing/spec-workflow.md`).
- **Surface** names the subsystem where the symptom appeared.
- **Symptom** describes what went wrong.
- **Fix** starts with the marker `fixed: ` or `open: ` (below), then
  describes what was done about it.

A row holds exactly five cells and starts with `|`. `./le spec journal report`
names a row that does not and exits non-zero, so prose in one of these files
must not contain a `|`.

A row is one line of symptom and fix, at most 600 characters. The detail goes
in the fix commit, a spec, or `ai/rationale/`. `./le spec journal validate`
names a longer row that is added or rewritten against HEAD (`row-too-long`).
The post-write hook runs it on every class file written through Write or Edit
and warns, and `./le commit create` runs it on every class file it commits and
refuses, so a row appended from a shell is caught too. A row HEAD already
holds is never measured, and neither is one that only changes its spacing or
its Fix marker. The pairing strips an exact leading `fixed: ` or `open: ` from
the Fix cell on both sides, and no other spelling, so a row whose only change
is the marker is held while a row whose prose changed is measured. A row moved
unchanged into another class file, by a merge or a split, pairs the same way
with the row HEAD holds in its old class, so a move is never measured either.

## The Fix marker

The Fix cell starts with `fixed: ` or `open: `, then the prose, spelled exactly
so: `Fixed: ` and `fixed:` are not markers. `fixed:` says the occurrence was
repaired. An occurrence still broken, partly repaired, designed or owed is
`open:`; there is no third value, because a half that is still broken is an
open defect. A row without a marker is `unmarked-fix`: the report refuses its
class, and validate names the line, whether the row is new or already at HEAD.

## Owned rows

Every `open:` row is listed in the owned journal rows table of the spec that
will fix it: an existing spec whose mechanism fits the defect, or a new one. The
spec table is the only declaration of ownership; the row names no owner.

```
## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-10-21 | `Server.appendDataPlaneWriter` starts with no FIB writer |
```

The heading is exactly `## Owned journal rows`, in a `spec-*.md` file under
`plan/`, and the table runs to the next heading. The first cell names the class
file, with or without backticks and `.md`; the second is the row's Date. Further
columns are free prose for the reader and are not compared.

`./le commit create` refuses a commit that adds or changes an open row no spec
owns (`UnownedOpenRows`, `internal/le/spec/journal/owner.go`). A row whose five
cells the same class file holds at HEAD, marker included, is never judged, so a
commit carrying a legacy row is not refused for it. A new open row, an edited
one, a `fixed:` row reopened and an open row moved into another class are
judged. A row is owned when the tables list its class and date at least as often
as the class holds open rows of that date, so each open row needs its own entry.
The tables are read from the specs the commit carries, in the working tree, and
from every other spec at HEAD: an owner left uncommitted owns nothing, and a
spec the commit removes owns nothing. A `fixed:` row needs no owner. The refusal
names each row's file and line and prints the table line to add.

## The capture line

A class records what now prevents it in one line above its table:

```
Captured: 2026-10-12 check internal/le/spec/journal.ValidateFile proof internal/le/spec/journal.TestValidateFileAcceptsCanonicalJournalShard
Captured: 2026-10-12 rule completion/directives/fix-a-defect-that-blocks-your-goal
```

The date, `YYYY-MM-DD`, is the day the capture landed. A `check` capture names
the Go function that refuses the class's shape and the Go test that proves it,
each as its repo-relative directory, a dot, and the function name. A `rule`
capture names a point id under `ai/rules/points/` without `.md`, and is the
weaker kind. The line carries no pipe, no `..` and no absolute path, and a
class file holds at most one. The report resolves the references at HEAD and
validate resolves them in the working tree; either refuses:

| Problem | When |
|---------|------|
| `capture-malformed` | The line breaks the grammar, or is dated after the HEAD commit's committer date (validate: after today) |
| `capture-unresolved` | The check or proof function is declared in no `.go` file of its directory, or the point file does not exist |
| `capture-duplicated` | The class file holds two capture lines |

## The report and DUE

`./le spec journal report` prints every class with 2 or more rows: its rows,
its `open` rows, its capture, and the span between the first and last date.
When every class has 1 row it prints nothing and exits 0. It reads git HEAD,
never the working tree, so a row or a capture counts once it is committed.

The counted rows of a captured class are its rows dated strictly after the
capture date. A row dated on the capture day belongs to the pass that set it.
An uncaptured class counts every row. A class is due for a fix pass when its
counted rows reach the threshold of its capture kind, which the report prints
on its `due at:` line and `| json` carries as `thresholds` and as `due-at` on
every class. A class captured by a check is due on its first counted row,
because one repeat means the check has a gap. The report lists the due classes
first, the most counted rows first, each marked `DUE`, under one count line and
a `classes captured:` summary. Due is information: the report still exits 0.
The session-start hook prints
`journal: <N> problem classes are due for a fix pass (./le spec journal report)`
when any class is due.

## What a fix pass delivers

A fix pass on a DUE class sets a `check` capture only when it delivers all
five:

1. Every row of the class read, and the class split first when one check
   cannot cover it.
2. A check that refuses the class's shape on a path that runs before commit or
   in `./le verify`.
3. A proof test reproducing at least one historical row, cited by date, that
   goes red when the check is removed, with the red recorded in the commit body.
4. The check green over the tree, so every open occurrence it can see is fixed
   in the pass and marked `fixed:`.
5. The capture line, dated the day the check lands.

A pass that cannot build a check sets a `rule` capture naming the point it
added or tightened, and states in the commit body why no check can be built.
Per-row patches without steps 2 and 3 set no capture.

## Resolving rows

`plan/post-release/engineering/spec-journal-resolution.md` owns how rows and classes get resolved: which DUE class a pass takes next, how a session claims it, what
evidence a `fixed:` row carries, and how a row whose code is gone is closed. The
spec that owns a row is the one in whose table it is listed ("Owned rows"). Each pass appends one row to that
spec's Pass Log; the spec never closes.

## Repairing a check after a recurrence

A class DUE after a check capture has a check with a gap, and the pass repairs
the check rather than counting on:

1. Every row dated after the capture read.
2. A row whose cause the check is not meant to catch moves to its own class,
   and the class is judged again without it.
3. Otherwise the check is extended to refuse the shape it missed, and the proof
   test reproduces the recurring row, cited by date, going red without the
   extension, with the red recorded in the commit body.
4. The recurring rows marked `fixed:` once the repaired check is green over the
   tree.
5. The capture line rewritten with the repair date, naming the repaired check
   and proof.

A pass MUST NOT replace a check capture with a `rule` capture. Per-row patches
without step 3 leave the capture date unchanged, so the class stays DUE.

The seed rows come from the patterns in `plan/learned/RECURRING-PATTERNS.md`.
Each seed Date is the add-date of the learned summary that pattern cites for
the occurrence, recovered with
`git log --diff-filter=A --format=%ad --date=short -- plan/learned/<id>-*.md`.
Summaries imported in the first corpus commit all carry that commit's date, so
several seeded classes show a 0-day span.
