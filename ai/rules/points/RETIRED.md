# Retired rule instructions

One line per point removed from `ai/rules/points/`. A point IS an instruction,
and every gate in this system reads the tree as it is: delete the file and its
manifest line together and the render check, the round trip, the gate map and
the rule lint all stay green, because the points and the rendered rule agree on
the smaller corpus. This file is what makes the removal say so.

`corpus_shrink` in `scripts/dev/rules_points.py` compares the point IDS on disk
against the ids git HEAD carried and requires every id that left to be covered
by a line added to this file since HEAD. `make ze-rules-gate-map-report` and
`make ze-doc-verify` fail otherwise.

Identity, never a count. An addition can mask a removed point, so a count cannot
prove that every instruction remains.

Scope, never an allowlist: a line stops counting the moment it is committed,
because HEAD moves with it. Nothing here pre-approves a future deletion.

A long table is the expected state, and it needs no pruning. The gate reads
only the rows added since HEAD, so growth costs it nothing. A reader arrives
with one dead point id, from a stale citation or an old binding, and reads the
one row that says where that instruction went.

A rename IS a retirement of the old id. The instruction stops being reachable
under the name a reader, a binding, or a citation carries, and the count form
could not see one at all. Repoint the binding at the new id AND write the row,
whose `Why` says where the instruction went.

A row is CHECKED, not believed. `retired_rows_since` validates every id against
the point names git HEAD actually carried, and refuses four shapes: a malformed
row, an id HEAD never held, an id whose point is still on disk, and a second row
for an id this file already declares. Without that check a fictional id cleared
the ratchet: declaring `rule/nowhere/never-existed` bought a real deletion
elsewhere in the same rule. A row naming a live point is refused for two
reasons, since it would both cover a drop the rule did not declare and excuse a
check from gating an instruction the corpus still carries.

Retiring an instruction also frees its check. `unbound_regressions` fails a
check that named a point at HEAD and declares `# ze point: none -- <why>` now,
because that is the cheapest way to launder a rename into a lost gate. A point
declared here is exempt: the instruction left the corpus on purpose, so the
check has nothing left to name. Retire only part of what a check named and the
live points are still reported.

One row per retired point, newest last. The Point cell is the id, backticked.
The Why cell says what happened to the instruction, not that it was removed.

| Point | Why |
|-------|-----|
| `planning/writing-learned-summaries/the-staleness-ceiling-is-drained-never-removed` | the ceiling counted `plan/learned/` summaries, and the corpus it drained no longer exists |
| `planning/writing-learned-summaries/keep-a-learned-summary-within-its-line-budget` | a journal row is one table line, so a 25-to-35-line budget has nothing left to bound |
| `planning/writing-learned-summaries/what-the-context-section-must-carry` | the row's `Symptom` cell replaced the `## Context` section, and "What each journal row cell holds" states it |
| `planning/writing-learned-summaries/what-the-consequences-section-must-carry` | the row's `Fix` cell replaced the `## Consequences` section, and "What each journal row cell holds" states it |
| `planning/writing-learned-summaries/name-the-rejected-alternative-in-every-decision` | the row has no `## Decisions` section; the spec's Key Design Decisions table still owes the "over" clause |
| `plugins/runtime-filter-declaration-planned-stage-1-wire-protocol/pointer-to-the-redistribution-filter-design` | the point was one sentence pointing at a deleted summary and stated nothing on its own |
| `completion/implementation-audit/run-the-audit-before-a-summary-or-a-done-claim` | the audit now precedes a journal row, and says so as `completion/implementation-audit/run-the-audit-before-a-journal-row-or-a-done-claim` |
| `git-safety/rebase-onto-diverged-main-driving-the-bookkeeping-conflicts/drive-a-diverged-rebase-with-rebase-learned-py` | `rebase_learned.py` was deleted with the corpus it drove; the bookkeeping is regenerated instead, under `.../drive-a-diverged-rebase-by-regenerating-bookkeeping` |
| `git-safety/rebase-onto-diverged-main-driving-the-bookkeeping-conflicts/finish-the-rebase-before-fixing-learned-numbers` | learned numbers no longer exist; the same instruction over derived ratchets is `.../finish-the-rebase-before-recomputing-derived-ratchets` |
| `planning/spec-closure-blocking/never-resolve-a-row-to-a-summary-that-omits-the-item` | the closure record is a journal row, so the instruction reads `.../never-resolve-a-row-to-a-record-that-omits-the-item` |
| `planning/spec-preservation/delete-the-spec-only-after-the-summary-is-written` | the artifact that must exist first is the journal row, under `.../delete-the-spec-only-after-the-journal-row-is-written` |
| `planning/spec-preservation/what-to-discard-when-writing-the-summary` | the same instruction over a row rather than a summary, under `.../what-to-discard-when-writing-the-row` |
| `planning/writing-learned-summaries/the-quality-test-every-summary-entry-must-pass` | the quality test now applies to a row, under `planning/writing-journal-rows/the-quality-test-every-journal-row-must-pass` |
| `planning/writing-learned-summaries/what-each-learned-summary-section-holds` | a row has cells, not sections; `planning/writing-journal-rows/what-each-journal-row-cell-holds` states them |
| `planning/writing-learned-summaries/write-a-summary-only-when-the-work-taught-something` | the same condition over a row, under `planning/writing-journal-rows/write-a-row-only-when-the-work-taught-something` |
| `rfc-compliance/directives/record-an-authorised-deviation-in-plan-learned` | `plan/learned/` is gone; the deviation is recorded under `rfc-compliance/directives/record-an-authorised-deviation-in-the-journal` |
| `testing/common-flaky-test-causes/where-the-flake-shape-catalogue-lives` | the catalogue lived in a deleted summary, so the point now names the shapes itself as `.../the-flake-shapes-to-check-first` |
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/every-binary-is-built-with-a-session-suffix` | a session's binaries moved from the name suffix `bin/ze-<sid>` into its own directory, so the instruction reads `commands/your-binaries-live-in-this-session-s-directory/every-binary-is-built-in-this-session-s-directory` |
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/never-hardcode-bin-ze-ask-for-the-path` | unchanged instruction, renamed with its section to `commands/your-binaries-live-in-this-session-s-directory/never-hardcode-bin-ze-ask-for-the-path` |
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/use-make-ze-path-to-get-the-binary` | unchanged instruction, renamed with its section to `commands/your-binaries-live-in-this-session-s-directory/use-make-ze-path-to-get-the-binary` |
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/why-test-binaries-live-in-a-private-bin` | test binaries now take the SAME shape as the canonical ones rather than the opposite one, under `commands/your-binaries-live-in-this-session-s-directory/why-test-binaries-live-in-a-private-bin` |
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/why-the-suffixed-binaries-stay-in-bin` | no binary stays in `bin/` under a session, so the reason it gives is void; the reason binaries live WITH the session is `commands/your-binaries-live-in-this-session-s-directory/why-the-binaries-live-with-the-session` |
| `commands/write-ad-hoc-scratch-under-your-per-session-dir/what-is-swept-at-session-end-and-what-stays` | nothing under `tmp/session/` is swept at session end or on a timer any more, so the point named a mechanism that no longer exists; what stays put, and the operator's `make ze-session-clean`, are stated by `commands/write-ad-hoc-scratch-under-your-per-session-dir/nothing-is-deleted-automatically-and-what-stays-put` |
| `writing/documentation/a-refused-example-becomes-false-evidence-in-a-review` | the parser-valid example directive remains under `writing/documentation/every-config-example-must-parse`, and the review-is-the-second-reader reason now opens `plan/journal/documentation-shows-config-the-parser-refuses.md`, above the incident rows |
| `writing/documentation/the-gate-this-owes-and-what-its-scope-costs` | the gate itself remains Goal G-3 in `plan/spec-ze-config-fmt.md`, and its two constraints, parser-recognized carriers with an opt-OUT and stating the annotation cost, now sit in that spec's Q-8 cell |
| `writing/owner-report/owner-report` | the section moved out of the writing rule, because that rule no longer triggers on a report; the directives it headed are in `ai/INSTRUCTIONS.md`, "Say it once, say it short" |
| `writing/owner-report/lead-with-what-blocks-and-what-you-need` | condensed into `ai/INSTRUCTIONS.md`, "Say it once, say it short", which every session loads: open with what is blocked, why it matters, and what you need; put the decision in the first ten lines as a table; narrative goes last or unsaid |
| `writing/owner-report/the-owner-is-not-an-agent` | condensed into `ai/INSTRUCTIONS.md`, "Say it once, say it short": rewrite a subagent report for a person, never forward it, and never pad to show effort |
