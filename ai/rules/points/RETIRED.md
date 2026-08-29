# Retired rule instructions

One line per point removed from `ai/rules/points/`. A point IS an instruction,
and every gate in this system reads the tree as it is: delete the file and its
manifest line together and the render check, the round trip, the gate map and
the rule lint all stay green, because the points and the rendered rule agree on
the smaller corpus. This file is what makes the removal say so.

`corpus_shrink` in `internal/le/rules/points.go` compares the point IDS on disk
against the ids git HEAD carried and requires every id that left to be covered
by a line added to this file since HEAD. `./le rules gate-map-report` and
`./le doc check verify` fail otherwise.

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
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/use-make-ze-path-to-get-the-binary` | retired with Make; the native suite route is `commands/your-binaries-live-in-this-session-s-directory/use-the-owning-native-action-to-build-test-binaries` |
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/why-test-binaries-live-in-a-private-bin` | test binaries now take the SAME shape as the canonical ones rather than the opposite one, under `commands/your-binaries-live-in-this-session-s-directory/why-test-binaries-live-in-a-private-bin` |
| `commands/your-binaries-are-session-suffixed-ask-for-the-path/why-the-suffixed-binaries-stay-in-bin` | no binary stays in `bin/` under a session, so the reason it gives is void; the reason binaries live WITH the session is `commands/your-binaries-live-in-this-session-s-directory/why-the-binaries-live-with-the-session` |
| `commands/write-ad-hoc-scratch-under-your-per-session-dir/what-is-swept-at-session-end-and-what-stays` | automatic age cleanup was removed; the current proof-based cleanup is `./le session reap`, documented by `commands/write-ad-hoc-scratch-under-your-per-session-dir/nothing-is-deleted-automatically-and-what-stays-put` |
| `writing/documentation/a-refused-example-becomes-false-evidence-in-a-review` | the parser-valid example directive remains under `writing/documentation/every-config-example-must-parse`, and the review-is-the-second-reader reason now opens `plan/journal/documentation-shows-config-the-parser-refuses.md`, above the incident rows |
| `writing/documentation/the-gate-this-owes-and-what-its-scope-costs` | the gate itself remains Goal G-3 in `plan/spec-ze-config-fmt.md`, and its two constraints, parser-recognized carriers with an opt-OUT and stating the annotation cost, now sit in that spec's Q-8 cell |
| `writing/owner-report/owner-report` | the section moved out of the writing rule, because that rule no longer triggers on a report; the directives it headed are in `ai/INSTRUCTIONS.md`, "Say it once, say it short" |
| `writing/owner-report/lead-with-what-blocks-and-what-you-need` | condensed into `ai/INSTRUCTIONS.md`, "Say it once, say it short", which every session loads: open with what is blocked, why it matters, and what you need; put the decision in the first ten lines as a table; narrative goes last or unsaid |
| `writing/owner-report/the-owner-is-not-an-agent` | condensed into `ai/INSTRUCTIONS.md`, "Say it once, say it short": rewrite a subagent report for a person, never forward it, and never pad to show effort |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/the-drain-floor-is-a-schedule-not-a-ratchet` | unchanged instruction, renamed with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/the-drain-floor-is-a-schedule-not-a-ratchet` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/the-edit-time-guard-on-an-rfc-tagged-test` | unchanged instruction, renamed with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/the-edit-time-guard-on-an-rfc-tagged-test` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/the-public-ledger-s-edges-not-ratchets-hard-requirements` | unchanged instruction, renamed with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/the-public-ledger-s-edges-not-ratchets-hard-requirements` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/un-enrolment-exempts-only-the-missing-row-branch` | unchanged instruction, renamed with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/un-enrolment-exempts-only-the-missing-row-branch` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/what-each-public-ledger-guard-refuses` | unchanged instruction, renamed with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/what-each-public-ledger-guard-refuses` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/what-fires-each-ratchet` | the table gained its eighth row, `check_level_ratchet`, and moved with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/what-fires-each-ratchet` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/what-the-ratchets-miss-an-in-place-weakening` | the count of ratchets that miss an in-place weakening rose with the section, under `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/what-the-ratchets-miss-an-in-place-weakening` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/why-pre-head-summaries-are-grandfathered` | unchanged instruction, renamed with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/why-pre-head-summaries-are-grandfathered` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/why-the-public-ledger-needs-hard-requirements` | unchanged instruction, renamed with its section to `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/why-the-public-ledger-needs-hard-requirements` |
| `rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/why-the-working-tree-alone-cannot-judge-coverage` | seven comparisons against HEAD became eight, under `rfc-compliance/what-keeps-rfc-testing-valid-the-eight-ratchets/why-the-working-tree-alone-cannot-judge-coverage` |
| `git-safety/before-any-commit/a-clean-full-verify-is-unreachable-in-a-shared-tree` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/a-clean-full-verify-is-unreachable-in-a-shared-tree` |
| `git-safety/before-any-commit/a-full-gate-run-reddens-other-sessions` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/a-full-gate-run-reddens-other-sessions` |
| `git-safety/before-any-commit/a-second-verify-blocks-rather-than-overlaps` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/concurrency/a-second-verify-blocks-rather-than-overlaps` |
| `git-safety/before-any-commit/a-shared-checkout-never-gives-a-clean-ze-verify-blocking` | heading only, no instruction; replaced by the `reading-a-red` section line of `precommit-verify` |
| `git-safety/before-any-commit/a-stale-generated-file-is-structural-not-flaky` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/a-stale-generated-file-is-structural-not-flaky` |
| `git-safety/before-any-commit/a-wire-behaviour-change-owes-its-functional-suite` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/a-wire-behaviour-change-owes-its-functional-suite` |
| `git-safety/before-any-commit/attribute-every-red-before-scoping-around-it` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/attribute-every-red-before-scoping-around-it` |
| `git-safety/before-any-commit/bisect-a-broken-head-with-rev` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/bisect-a-broken-head-with-rev` |
| `git-safety/before-any-commit/clear-a-broken-head-by-committing-the-producer` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/clear-a-broken-head-by-committing-the-producer` |
| `git-safety/before-any-commit/commit-the-producer-with-its-consumer` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/commit-the-producer-with-its-consumer` |
| `git-safety/before-any-commit/concurrent-verify-runs-blocking` | heading only, no instruction; replaced by the `concurrency` section line of `precommit-verify` |
| `git-safety/before-any-commit/derive-the-owning-target-when-the-table-has-no-row` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/derive-the-owning-target-when-the-table-has-no-row` |
| `git-safety/before-any-commit/do-not-stop-to-ask-which-way` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/do-not-stop-to-ask-which-way` |
| `git-safety/before-any-commit/give-a-narrow-failure-a-narrow-re-run` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/give-a-narrow-failure-a-narrow-re-run` |
| `git-safety/before-any-commit/go-through-make-or-carry-gocache-yourself` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/go-through-make-or-carry-gocache-yourself` |
| `git-safety/before-any-commit/green-every-structural-gate-even-in-a-shared-tree` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/green-every-structural-gate-even-in-a-shared-tree` |
| `git-safety/before-any-commit/keep-the-structural-gate-list-matching-live-stages` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/keep-the-structural-gate-list-matching-live-stages` |
| `git-safety/before-any-commit/known-red-full-verify-scope-to-changed-blocking` | heading only, no instruction; replaced by the `reading-a-red` section line of `precommit-verify` |
| `git-safety/before-any-commit/list-only-your-own-files-in-the-commit-script` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/list-only-your-own-files-in-the-commit-script` |
| `git-safety/before-any-commit/never-commit-with-lint-issues-or-no-test-evidence` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/never-commit-with-lint-issues-or-no-test-evidence` |
| `git-safety/before-any-commit/never-edit-the-tree-while-a-verify-runs` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/concurrency/never-edit-the-tree-while-a-verify-runs` |
| `git-safety/before-any-commit/never-let-a-red-persist-across-sessions` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/never-let-a-red-persist-across-sessions` |
| `git-safety/before-any-commit/never-park-a-deterministic-structural-gate` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/never-park-a-deterministic-structural-gate` |
| `git-safety/before-any-commit/once-at-the-end-never-during-development-blocking` | heading only, no instruction; replaced by the `running-the-gate and reading-a-red` section line of `precommit-verify` |
| `git-safety/before-any-commit/one-run-covers-every-commit-until-the-next-go-edit` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/one-run-covers-every-commit-until-the-next-go-edit` |
| `git-safety/before-any-commit/only-one-check-compiles-what-git-holds` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/only-one-check-compiles-what-git-holds` |
| `git-safety/before-any-commit/phrases-that-activate-the-owner-override` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/phrases-that-activate-the-owner-override` |
| `git-safety/before-any-commit/plan-the-first-full-run-to-be-the-last` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/plan-the-first-full-run-to-be-the-last` |
| `git-safety/before-any-commit/read-the-whole-failure-summary-before-re-running` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/read-the-whole-failure-summary-before-re-running` |
| `git-safety/before-any-commit/run-make-ze-verify-and-check-freshness-first` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/run-native-verify-and-check-freshness-first` |
| `git-safety/before-any-commit/run-one-verify-at-a-time-repo-wide` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/concurrency/run-one-verify-at-a-time-repo-wide` |
| `git-safety/before-any-commit/run-the-full-gate-before-any-commit-carrying-go` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/run-the-full-gate-before-any-commit-carrying-go` |
| `git-safety/before-any-commit/run-the-full-gate-once-when-the-work-is-finished` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/run-the-full-gate-once-when-the-work-is-finished` |
| `git-safety/before-any-commit/run-the-gates-your-new-files-join` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/run-the-gates-your-new-files-join` |
| `git-safety/before-any-commit/run-the-target-that-owns-what-you-changed` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/run-the-target-that-owns-what-you-changed` |
| `git-safety/before-any-commit/run-verify-in-the-foreground-and-wait` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/run-verify-in-the-foreground-and-wait` |
| `git-safety/before-any-commit/run-verify-only-when-the-commit-can-affect-the-build` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/does-verify-apply/run-verify-only-when-the-commit-can-affect-the-build` |
| `git-safety/before-any-commit/run-verify-when-any-file-in-a-mixed-commit-needs-it` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/does-verify-apply/run-verify-when-any-file-in-a-mixed-commit-needs-it` |
| `git-safety/before-any-commit/running-ze-verify` | heading only, no instruction; replaced by the `running-the-gate` section line of `precommit-verify` |
| `git-safety/before-any-commit/scope-the-gate-to-changes-when-verify-is-known-red` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/scope-the-gate-to-changes-when-verify-is-known-red` |
| `git-safety/before-any-commit/step-0-does-ze-verify-apply` | heading only, no instruction; replaced by the `does-verify-apply` section line of `precommit-verify` |
| `git-safety/before-any-commit/step-1-if-ze-verify-applies-blocking` | heading only, no instruction; replaced by the `running-the-gate` section line of `precommit-verify` |
| `git-safety/before-any-commit/step-2-always` | heading only, no instruction; replaced by the `running-the-gate` section line of `precommit-verify` |
| `git-safety/before-any-commit/structural-gates-are-never-known-red-blocking` | heading only, no instruction; replaced by the `what-may-be-overridden` section line of `precommit-verify` |
| `git-safety/before-any-commit/structural-red-ok-is-an-owner-only-escape` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/structural-red-ok-is-an-owner-only-escape` |
| `git-safety/before-any-commit/take-another-sessions-red-as-working-code` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/take-another-sessions-red-as-working-code` |
| `git-safety/before-any-commit/test-one-package-with-ze-test-pkg` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/test-one-package-with-a-native-job` |
| `git-safety/before-any-commit/the-doc-test-only-checks-that-escape-the-gate` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/the-doc-test-only-checks-that-escape-the-gate` |
| `git-safety/before-any-commit/the-helper-refuses-a-commit-while-a-gate-is-red` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/the-helper-refuses-a-commit-while-a-gate-is-red` |
| `git-safety/before-any-commit/the-override-needs-both-parts-said-explicitly` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/the-override-needs-both-parts-said-explicitly` |
| `git-safety/before-any-commit/the-pre-commit-verify-checklist` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/the-pre-commit-verify-checklist` |
| `git-safety/before-any-commit/the-scoped-evidence-a-shared-checkout-owes` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/the-scoped-evidence-a-shared-checkout-owes` |
| `git-safety/before-any-commit/the-scoped-gates-to-run-instead` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/the-scoped-gates-to-run-instead` |
| `git-safety/before-any-commit/the-spec-completion-and-report-checklist` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/the-spec-completion-and-report-checklist` |
| `git-safety/before-any-commit/the-tracked-build-check-never-reads-test-files` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/the-tracked-build-check-never-reads-test-files` |
| `git-safety/before-any-commit/the-two-parts-the-override-must-name` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/the-two-parts-the-override-must-name` |
| `git-safety/before-any-commit/the-verify-lock-releases-when-the-command-exits` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/concurrency/the-verify-lock-releases-when-the-command-exits` |
| `git-safety/before-any-commit/thomas-can-override-the-verify-requirement` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/thomas-can-override-the-verify-requirement` |
| `git-safety/before-any-commit/thomas-owner-override-commit-without-verify` | heading only, no instruction; replaced by the `what-may-be-overridden` section line of `precommit-verify` |
| `git-safety/before-any-commit/traps-that-misread-a-verify-log` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/traps-that-misread-a-verify-log` |
| `git-safety/before-any-commit/unverified-is-correct-in-a-shared-checkout` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/unverified-is-correct-in-a-shared-checkout` |
| `git-safety/before-any-commit/use-evidence-scoped-to-your-own-files` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/use-evidence-scoped-to-your-own-files` |
| `git-safety/before-any-commit/what-the-override-permits-and-forbids` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/what-the-override-permits-and-forbids` |
| `git-safety/before-any-commit/what-to-do-while-another-verify-holds-the-lock` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/concurrency/what-to-do-while-another-verify-holds-the-lock` |
| `git-safety/before-any-commit/what-ze-tracked-build-check-reads` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/after-the-commit/what-native-tracked-build-check-reads` |
| `git-safety/before-any-commit/when-the-override-is-active` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/what-may-be-overridden/when-the-override-is-active` |
| `git-safety/before-any-commit/which-file-types-require-ze-verify` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/does-verify-apply/which-file-types-require-native-verify` |
| `git-safety/before-any-commit/which-target-owns-each-surface` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/which-action-owns-each-surface` |
| `git-safety/before-any-commit/who-owns-each-red-a-full-run-reports` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/reading-a-red/who-owns-each-red-a-full-run-reports` |
| `git-safety/before-any-commit/your-working-tree-is-not-what-you-committed-blocking` | heading only, no instruction; replaced by the `after-the-commit` section line of `precommit-verify` |
| `git-safety/before-any-commit/ze-test-pkg-examples` | unchanged instruction, moved with the pre-commit verification rule to `precommit-verify/running-the-gate/native-package-test-examples` |
| `cli/pipe-completeness/known-violations-to-fix` | heading only, over a table whose sole row read `_(none currently)_`; the mechanical check above it is what finds a command missing pipe support |
| `cli/pipe-completeness/the-commands-still-missing-pipe-support` | the tracker held no command, so it stated no instruction and cost every reader of the rule; a violation is found by the grep in "Mechanical Check (pipes)" |
| `plugins/5-stage-protocol/the-sdk-declares-record-answers-at-stage-3` | heading only, over the negotiation this rule now says does not exist; replaced by `plugins/5-stage-protocol/one-command-answer-encoding-blocking` |
| `plugins/5-stage-protocol/the-sdk-sets-the-protocol-name-for-every-plugin` | the SDK sets no protocol name, because Stage 3 carries no wire shape; the instruction a plugin author owes is `plugins/5-stage-protocol/a-command-answer-is-always-a-record-sequence` |
| `plugins/5-stage-protocol/the-record-answer-name-is-symmetric` | there is no name to read symmetrically; that a command answer has one encoding in both directions is stated by `plugins/5-stage-protocol/a-command-answer-is-always-a-record-sequence` |
| `plugins/5-stage-protocol/the-frame-follows-the-declaration-never-the-payload` | nothing is declared, so the same instruction over the payload alone reads `plugins/5-stage-protocol/the-frame-never-follows-the-payload` |
