# 1273 -- plan template lifecycle split

## Context

`plan/TEMPLATE.md` had grown to 543 lines and was doing two jobs at once: it
described what a spec must contain BEFORE code, and what it must contain at
CLOSURE. Measured across all 161 specs on 2026-07-25, 123 of them were shorter
than the template itself (median spec 343 lines), so the typical spec was a
subset of a superset catalogue rather than a filled-in form.

The two halves behaved completely differently. The design-time sections (Task,
Required Reading, Current Behavior, Data Flow, Wiring Test, Acceptance Criteria,
TDD Plan, Files to Modify) were present in 98-100% of specs and byte-identical to
the template in ~2%, including in the 60 `skeleton` specs: `validate-spec.sh`
enforces them at Write time and it works. The closure sections rotted. Among the
26 `in-progress` specs, the tables were still byte-identical to the template in
`Wiring Verified` 9/12, `Files Exist` 11/15, `AC Verified` 11/15,
`Assumptions Resolved` 8/12, `Final status` 11/17, `Failed Approaches` 12/17,
`Escalation Candidates` 10/15.

The discriminator was NOT difficulty or lateness. `Goal Validation`,
`Implementation Summary` and `Key Design Decisions` were untouched in 0% of the
same specs. What those have in common is that authors write them when they reach
them. The rotted ones were copied 300 lines ahead of first use, and an empty
header-plus-separator table asks nothing of the reader.

## Decisions

- **Split by lifecycle, not by topic.** `plan/TEMPLATE.md` (328 lines) is
  design-time only; `plan/TEMPLATE-CLOSURE.md` (161 lines) is appended by
  `/ze-implement` at stage 11. Chose append-when-needed over one big template
  with "fill later" markers, because the measurement says distance from use, not
  instruction wording, is what empties a section.
- **Made the Pre-Commit Verification gate check per sub-table.**
  `_pre_commit_verification_filled` counted `pipe_rows - 2*sep_rows > 0` across
  the WHOLE section, so one row in `Files Exist` satisfied it while four other
  tables stayed empty. That is precisely the measured 73-75%. Replaced with
  `pre_commit_verification_gaps`, which returns the names of the empty tables.
  A section with no `###` sub-headings keeps the old floor, so widening the gate
  cannot retroactively block a spec whose section never had sub-tables.
- **One verification command.** The template shipped three spellings of the same
  gate (`make ze-lint && ...`, `make ze-verify`, `make ze-test`) and the hook
  required the literal `make ze-test`, which is the fuzz-inclusive target
  (`Makefile`), not the pre-commit gate that `ai/rules/git-safety.md` names.
  The Goal Gates now name `make ze-verify` only. The hook accepts the legacy
  string with a warning rather than an error, because 50 existing specs carry it
  and none carry the new one: a hard cutover would have made them un-editable.
- **Placeholder guards became status-aware.** A `skeleton` spec is the documented
  shape of a deferral holder (fill `## Task`, leave the rest,
  `ai/rules/planning.md`), yet the hook rejected placeholders on every
  edit. That contradiction made one correctly-authored skeleton un-editable.
  Placeholders now warn at `skeleton` and block from `design` onward, where the
  status is itself a claim that the section is written.
- **Deleted what duplicated a file that already existed.** The 24-row BGP Family
  Checklist (present in 8/161 specs, half of those untouched) became one row
  pointing at `ai/patterns/bgp-family.md`, which the template already cited.
  `## Post-Compaction Recovery` (byte-identical across 150 specs) became one line
  pointing at `.claude/rules/post-compaction.md`. The `/implement Stage Mapping`
  table moved into the skill it describes.
- **Collapsed the Mistake Log** from three empty tables into one with a `Kind`
  column and a shipped `none` row. Three separate empty tables produced three
  separate 67-82% untouched rates.
- **Answer columns take Yes/No/N-A, never a checkbox.** The template used `[ ]`
  with three incompatible meanings (never-tick marker, answer cell, real
  checkbox). 286 Documentation rows and 138 Integration rows were still
  unanswered, with 25 and 24 specs respectively where every row was blank, and
  only 6/161 specs ever ticked anything.

## Consequences

- A new spec carries 328 lines instead of 543, and none of it is closure
  bookkeeping the author cannot yet fill.
- `/ze-implement` stage 11 now has a BLOCKING first action: append
  `plan/TEMPLATE-CLOSURE.md`. If that step is skipped the later stages have no
  tables to fill, which is a visible failure rather than a silent one.
- Closing a spec now requires evidence in EVERY Pre-Commit sub-table. Specs
  currently in flight that filled only one table will be asked for the rest at
  closure. That is the intended cost.
- The Review Gate section stops asking authors to hand-transcribe what
  `scripts/dev/review_gate.py` already records; it now carries the artifact path
  plus only the findings worth forwarding to the learned summary.
- Existing specs are untouched. All 161 still validate (verified by driving
  `validate-spec.sh` over every one of them).

## Gotchas

- **The hook validates existing specs on every edit, so any tightening is
  retroactive.** Changing the required checklist string would have broken 50
  specs. Measure the corpus before changing a required-string gate.
- **`validate-spec.sh`'s Entry Point guard was failing open.** It matched
  `\[Where data enters\]`, but the template's actual placeholder is
  `[Where data enters: wire bytes, ...]`, which that pattern never matched. The
  guard only ever fired through the second alternative, `[Format at entry]`, so
  editing that one line let a placeholder through. Fixed by dropping the closing
  bracket from the pattern.
- **The fixture spec carries unrelated warnings, and the hook prints a warning
  COUNT rather than a list.** A fixture asserting "no warnings" for the new
  verification command failed for reasons unconnected to it; the two spellings
  have to be told apart by the count delta.
- **A concurrent session added `cmd/ze/hub/statestore.go` mid-run**, which made
  `ai/DOCS-TO-CODE.md` stale and failed `make ze-doc-test` with nothing to do
  with this change. Regenerating a shared index picks up other sessions' work:
  expect foreign entries in the diff and do not "clean" them out
  (`ai/rules/git-safety.md`).

## Files

- `plan/TEMPLATE.md` -- rewritten as the design-time half (543 -> 328 lines)
- `plan/TEMPLATE-CLOSURE.md` -- new, appended at `/ze-implement` stage 11
- `scripts/dev/commit_helper.py` -- `pre_commit_verification_gaps` replaces
  `_pre_commit_verification_filled`; `spec_audit_problems` names empty tables
- `scripts/dev/commit_helper_test.py` -- `TestPreCommitVerificationFilled`
- `.claude/hooks/validate-spec.sh` -- one verification command, status-aware
  `placeholder_problem`, Entry Point guard no longer fails open
- `scripts/dev/hook-fixture-check.py` -- 6 new validate-spec fixtures (14 -> 20)
- `scripts/dev/check_doc_links.py` -- link-check the closure template
- `ai/skills/ze-implement.md`, `ai/skills/ze-spec.md`, `ai/rules/planning.md`,
  `ai/rules/completion.md`, `ai/INDEX.md`, `plan/README.md`
