# Problem Journal

One file per problem class. Each file holds one table:

```
| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-07-15 | example-spec | reactor | FSM closed session instead of sending NOTIFICATION | fixed the state transition table |
```

- **One file = one problem class.** The file name is the class name in kebab-case.
- **One row = one occurrence.** Recurrence is the row count.
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
- **Fix** describes what was done about it.

A row holds exactly five cells and starts with `|`. `./le journal report` names a
row that does not and exits non-zero, so prose in one of these files must not
contain a `|`.

`./le journal report` prints every class with 2 or more rows, its count, and
the span between the first and last date. When every class has 1 row it
prints nothing and exits 0. It reads git HEAD, never the working tree, so a
row counts once it is committed.

The seed rows come from the patterns in `plan/learned/RECURRING-PATTERNS.md`.
Each seed Date is the add-date of the learned summary that pattern cites for
the occurrence, recovered with
`git log --diff-filter=A --format=%ad --date=short -- plan/learned/<id>-*.md`.
Summaries imported in the first corpus commit all carry that commit's date, so
several seeded classes show a 0-day span.
