# Spec: lint-backlog-3-test-tooling

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 58 findings of the full lint in the test harness, `le` and the chaos tooling; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 5/5: tooling and fixture edits written; parent verification pending |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the test
harness and `le` (door `ai/doors/le-tooling.md`), the chaos tooling under
`internal/chaos` and `tools/freertr-entrypoint` (both covered by
`ai/doors/system.md` alone): `internal/test/fixture` 23, `internal/le` 18,
`internal/chaos` 10, `internal/test/runner` 4, `internal/test/plugins/fakefilter`
2, `tools/freertr-entrypoint` 2, `internal/test/userns` 1. By linter: goconst
21, exhaustive 14, gosec 12, errcheck 3, misspell 3, gocritic 3, unused 1,
staticcheck 1. The eight locations 752d8f19ef cleared
(`internal/le/go/staticcheck/judge.go`, `internal/le/hookruntime/timestamp*.go`,
`internal/le/test/qemu`) are not listed; this child confirms them with its
scoped run (umbrella A-2). Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/chaos/engine/action_params.go` | 241 | exhaustive |
| `internal/chaos/inprocess/chaos.go` | 151 | exhaustive |
| `internal/chaos/inprocess/runner_test.go` | 200 | exhaustive |
| `internal/chaos/inprocess/runner_test.go` | 352 | exhaustive |
| `internal/chaos/inprocess/runner_test.go` | 701 | exhaustive |
| `internal/chaos/orchestrator/types.go` | 110 | exhaustive |
| `internal/chaos/peer/receiver_test.go` | 163 | exhaustive |
| `internal/chaos/peer/simulator_test.go` | 204 | exhaustive |
| `internal/chaos/peer/simulator_test.go` | 272 | exhaustive |
| `internal/chaos/report/metrics.go` | 75 | exhaustive |
| `internal/le/go/extract/goextract.go` | 99 | gosec |
| `internal/le/go/extract/goextract.go` | 202 | gosec |
| `internal/le/go/extract/goextract.go` | 205 | gosec |
| `internal/le/go/stdpatch/gen/stdpatch.go` | 105 | gosec |
| `internal/le/go/stdpatch/gen/stdpatch.go` | 108 | gosec |
| `internal/le/go/stdpatch/gen/tree.go` | 95 | gosec |
| `internal/le/go/stdpatch/gen/tree.go` | 245 | gosec |
| `internal/le/go/stdpatch/gen/tree.go` | 271 | gosec |
| `internal/le/interoplab/bgp/prepare_inject.go` | 92 | exhaustive |
| `internal/le/interoplab/bgp/speaker_parsed_empty_mp.go` | 148 | gosec |
| `internal/le/interoplab/ipsec/ipsec.go` | 451 | unused |
| `internal/le/interoplab/ipsec/mobike_netns_linux_test.go` | 52 | staticcheck |
| `internal/le/setup/shims.go` | 409 | gosec |
| `internal/le/test/deployment/gokrazyl2tp.go` | 281 | gosec |
| `internal/le/test/functional/binaries.go` | 204 | goconst |
| `internal/le/workflowcheck/workflowcheck_test.go` | 578 | misspell |
| `internal/test/fixture/netfilter_fixture_firewall.go` | 409 | gocritic |
| `internal/test/fixture/netfilter_fixture_firewall.go` | 410 | gocritic |
| `internal/test/fixture/plugin_fixture_01_answer.go` | 811 | goconst |
| `internal/test/fixture/plugin_fixture_06_decoder.go` | 32 | goconst |
| `internal/test/fixture/plugin_fixture_08_peer_wait_test.go` | 120 | gocritic |
| `internal/test/fixture/plugin_fixture_10_process.go` | 650 | goconst |
| `internal/test/fixture/plugin_fixture_filter_thirdparty.go` | 34 | goconst |
| `internal/test/fixture/plugin_shell_extra_1.go` | 275 | goconst |
| `internal/test/fixture/plugin_shell_extra_1_auth.go` | 461 | goconst |
| `internal/test/fixture/policy_final_alternate.go` | 151 | goconst |
| `internal/test/fixture/policy_final_reload.go` | 22 | goconst |
| `internal/test/fixture/register_sflow_agent_address_linux.go` | 31 | goconst |
| `internal/test/fixture/ui_fixture_le_checks_answers.go` | 631 | goconst |
| `internal/test/fixture/ui_fixture_le_checks_answers.go` | 632 | goconst |
| `internal/test/fixture/ui_fixture_le_evidence_answers.go` | 271 | goconst |
| `internal/test/fixture/ui_fixture_le_exec_cache_env.go` | 211 | goconst |
| `internal/test/fixture/ui_fixture_le_exit_codes.go` | 63 | goconst |
| `internal/test/fixture/ui_fixture_le_inventory_answers.go` | 270 | goconst |
| `internal/test/fixture/ui_fixture_le_placement_answers.go` | 117 | goconst |
| `internal/test/fixture/ui_fixture_le_rfc_answers.go` | 252 | goconst |
| `internal/test/fixture/ui_fixture_le_rfc_answers.go` | 296 | goconst |
| `internal/test/fixture/ui_fixture_le_store_trim_answers.go` | 121 | goconst |
| `internal/test/fixture/ui_fixture_le_store_trim_answers.go` | 122 | goconst |
| `internal/test/plugins/fakefilter/fakefilter.go` | 311 | misspell |
| `internal/test/plugins/fakefilter/fakefilter.go` | 313 | misspell |
| `internal/test/runner/display.go` | 280 | exhaustive |
| `internal/test/runner/record_collection.go` | 98 | exhaustive |
| `internal/test/runner/runner_stop_test.go` | 196 | errcheck |
| `internal/test/runner/stress.go` | 21 | exhaustive |
| `internal/test/userns/userns_linux.go` | 260 | gosec |
| `tools/freertr-entrypoint/main.go` | 175 | errcheck |
| `tools/freertr-entrypoint/main.go` | 179 | errcheck |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - le, the hooks and the test trees
  → Constraint: every concurrent session runs the hooks and `./le`; a change to `internal/le/hookruntime` or the launcher can block them all
  → Constraint: a bare `go test` omits the feature tags; run each package through `./le job run label unit-pkg command go test -tags ...`
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces", the `exhaustive` C/O/P classification
  → Constraint: gosec G304/G204 on a trusted path takes `//nolint:gosec // <why the path or argv is trusted>`, as 752d8f19ef did
- [ ] `ai/META-DECISIONS.md` - "Test changes"
  → Constraint: a fixture change that could lower what a test proves goes to `ze-test-judge`; a goconst rename does not, an assertion change does

**Key insights:**
- Most fixture findings are goconst on repeated answer strings; one package-level constant per string, in the fixture package.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/go/stdpatch/gen/tree.go` - gosec sites over the patched GOROOT tree
- [ ] `internal/test/runner/display.go` - an enum switch `exhaustive` reports
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every test's assertions and every `le` command's answer.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| The precedent fix | 752d8f19ef | Cleared this session's lint in staticcheck, hookruntime and qemu with reasoned gosec nolints and goconst constants | Extends: the same treatment for the remaining `internal/le` findings |
| Weakening judge | `./le test weakened approve`, `ze-test-judge` | Approves a test change that could prove less | Applies only to an assertion change, not to a mechanical lint fix |
| Selected test comparison and approval consumption | `internal/le/test/weakened/testweakened.go`, `Check`, `comparePath`, `claimFinding`; `scope.go`, `unitsByName` | Compares selected blobs with HEAD and records live approvals only for structural findings | Extends for the blocking semantic approval omission: compare existing unit text and carry a live weakening approval for a changed selected unit even without a structural finding. Reuse rename pairing, selection, approval loading and script pruning. |
| Lint contract journal rows | `plan/journal/lint-contract-not-applied.md`, rows owned by `plan/post-release/engineering/spec-tooling-one-off-defects.md` | Record tooling lint findings | Applies: a finding here that a row names is marked `fixed:` in the same commit |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/test/... ./internal/le/... ./internal/chaos/... ./tools/freertr-entrypoint/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.
2. A real defect (an ignored error in the runner or the entrypoint) gets a red test first.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside the packages | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Source audit found all 14 listed exhaustive sites already annotated; parent schedules the matrix | broken: source list is stale; current matrix unverified |
| A-2 | 752d8f19ef cleared its eight locations | Its body: a scoped lint over the three packages reports 0 issues | They reappear in this scope | This child's scoped run covers `internal/le/...` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hook-source edit breaks the hooks for every session | `./le ai hooks unit` red; the stored digests refuse | Run `./le ai hooks unit` after any `internal/le/hookruntime` edit; re-seal the digests through their own procedure |
| R-2 | The fixture package is edited by other sessions (the peer-wait backlog) | Conflicting hunks in `internal/test/fixture` | Commit only this child's hunks |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over `internal/test`, `internal/le`, `internal/chaos`, `tools/freertr-entrypoint` | Zero findings |
| AC-2 | The package tests of each changed package, with their tags | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | each changed package | AC-2 | |

## Files to Modify
- The files in the Task table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-3-test-tooling.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. `internal/le` and `tools/`: gosec and errcheck, reasoned.
3. `internal/test`: goconst, gocritic, exhaustive, errcheck.
4. `internal/chaos`: exhaustive, classified C/O/P.
5. Scoped lint zero; package tests pass.

### Step 2 handoff: tooling-fix

The permission annotations preserve the existing source-file, GOROOT-directory,
executable-shim and credential-free image-template modes. They explain those modes,
not only why their paths are trusted. The freeRtr teardown receives deliberately
reap stopped children while `run` retains the primary result or signal.

The parent ran `TestParsedEmptyMPReadyOwnsItsFile` through the job runner against
the unchanged writer. The fresh-marker case passed; the existing-file and symlink
cases both failed: the writer returned nil and replaced the target contents with
the establishment marker. The implementation agent read that output before the fix.
The canonical test is `internal/le/interoplab/bgp/speaker_parsed_empty_mp_ready_test.go`;
the command was `CGO_ENABLED=0 ./le job run label tooling-marker-red command go test -tags "$tags" ./internal/le/interoplab/bgp -run '^TestParsedEmptyMPReadyOwnsItsFile$' -v`.

`writeParsedEmptyMPReady` now exclusively creates a 0600 file and returns both
write and close errors. It refuses existing files and symlinks rather than
overwriting them. A rerun needs a fresh marker path, as the source capture already
does. The helper fix landed in `d2c19eae5c`. The registered-speaker entry test,
owned-path routing through `--result`, and documentation landed in `fc62fd18ec`.

The marker green/red/green proof is observed. Both tests in
`internal/le/interoplab/bgp/speaker_parsed_empty_mp_ready_test.go` passed, including
all five subcases. Restoring the former `os.WriteFile` helper while retaining the
new path routing made `TestParsedEmptyMPSourcePropagatesMarkerRefusal` fail for both
existing files and symlinks. Each failure explicitly reported overwritten target
bytes, not only a lifetime expiry. Restoring the exclusive helper made both tests
and all five subcases pass again. `cmp` confirmed the restored source was identical
to the saved pre-proof source. No mutation remains.

All three runs used `CGO_ENABLED=0 ./le --name lint-backlog job run label <label> quiet command go test -tags "$tags" ./internal/le/interoplab/bgp -run '<selector>' -v`,
with `tags` containing `ze_core` and the feature-manifest tags. Initial and restored
runs selected `^TestParsedEmptyMP(SourcePropagatesMarkerRefusal|ReadyOwnsItsFile)$`.
The mutation run selected `^TestParsedEmptyMPSourcePropagatesMarkerRefusal$`.
The restored run added `-count=1` and executed rather than using a cached verdict.
The labels were `tooling-marker-entry-green`, `tooling-marker-entry-red` and
`tooling-marker-entry-restored`. The canonical tests and the two commits above
retain the proof's subject; session log locations are recorded in the state handoff.

The parent still owes tagged tests of the changed packages: `internal/le/go/extract`,
`internal/le/go/stdpatch/gen`, `internal/le/setup`,
`internal/le/test/deployment`, `internal/le/test/functional`,
`internal/le/interoplab/bgp`, `internal/le/interoplab/ipsec`,
`internal/le/workflowcheck`, and `tools/freertr-entrypoint`.
The existing exhaustive annotations need no edits. `internal/test` belongs to
the separate phase. AC-1 and AC-2 remain unverified; AC-3 is addressed by the
written annotations and still owes scoped lint and independent review.

### Step 3 handoff: fixture-fix

The sibling phase's current diff was read when incorporating its handoff. Its
23 edited files cover meaning-specific fixture constants, equivalent ASCII digit
classes, the local `signaled` spelling in fakefilter, logging the runner's expected
Wait result, and the trusted AppArmor-knob read annotation. Existing exhaustive
annotations remain unchanged. No source edit was repeated during incorporation.

AC-1 and AC-2 remain unverified. The parent owes tagged tests of
`internal/test/fixture`, `internal/test/plugins/fakefilter`, `internal/test/runner`
and `internal/test/userns`, including `TestBackgroundLifetimeStopsTheProcess`,
`TestUsernsPolicyHintNamesTheAppArmorRestriction` and
`TestPluginObservers08WaitForTheScriptedPeer`, followed by scoped lint.
Functional reach is plugin, reload, ui, runner and firewall. The handoff reports
no documentation contract change and no new daemon evidence from sFlow constants.
Concurrent `internal/test/peer` edits are not part of fixture-fix's changes.

### Mistake Log: wrong assumptions

The original CI locations are a baseline, not proof of the current tree. The
source audit found the 14 exhaustive annotations already present and fakefilter's
misspelled local identifier at a moved location. Implementation follows current
source, preserving those annotations. The full matrix remains unverified.

### Native approval blocker (2026-11-02)

The parent authorized this related blocker repair before the policy migration
commit. A judge approved a semantic test migration, but `Check` returns before
loading approvals when there are no structural findings. The native message
therefore omits the mandatory verdict. The owning layer is the selected test
comparison and its approval consumer. Manual trailers would bypass that consumer
and its single-use checks, so the repair belongs in the producer.

Constraints: carry only a live approval in the selected commit's approval
namespace for a unit whose selected text differs from HEAD. Preserve structural
refusals, RFC approval requirements, rename handling and ambiguous-unit refusals.
Do not consume unrelated, unchanged, excluded-hunk or foreign-session approvals.
A landed row cannot approve another change. Preparation preserves approval files;
only a successful generated commit script prunes used rows.

Focused evidence must exercise native `Create`, its generated message and script,
plus unchanged-unit, excluded-hunk, foreign-session, landed-row and ambiguous-unit
cases. Update `docs/contributing/committing.md` and
`docs/architecture/testing/test-health.md` with this behavior. No policy source
edit, approval edit or real checkout commit belongs to this blocker phase.

Observed proof: `TestCreateCarriesSemanticApproval` failed for both an
expected-value edit and an assertion-adding edit before the fix. Both now pass,
including native script publication and pruning. The ambiguous-unit regression
also failed before the fix and passes after it. The complete `internal/le/commit`
package passes, including unchanged-unit, excluded-hunk, selected-hunk,
foreign-session, landed-row and paired-rename cases.

The independent judge approved correcting the old malformed-file expectation
and preserving separate unchanged-unit no-read and no-consumption proof. The
exact revised proposal was applied. The existing wrapper now also proves that
a semantic verdict becomes one trailer and one pruning selection without a
structural finding. All 16 positive and 3 negative detector cases remain.
`TestSelfTestCoversEveryDetectorVerdictThroughCheck` and
`TestCheckSemanticApprovalExcludesNewUnits` passed, including all four subcases.
New functions and entirely new files spend no semantic approval. The new-function
fixture proves its existing function's text span stays unchanged.

The subsequent serial race run over the two packages hit the command harness's
240-second timeout before producing test output. It supplies no race verdict.
A fresh verification context must run the same package scope with a sufficient
execution window. No owned race process remained after the timeout. The two-page
`doc ste check file` passed. Native links, consistency and retired-command checks
offer no path scope, so those full-tree checks were not run. Independent review
of the whole tooling diff remains owed. No real checkout commit was prepared or run.

## Review Gate

### Round seven: owner authorization and scope

Owner authorization, 2026-11-02: asked "Do you authorize this correction and
independent review round seven?", the owner answered "yes". This authorizes
this correction and independent review round seven only. Round eight is not
authorized. Round six remains clean for its recorded scope.

Scope: the semantic RFC approval omission in `testweakened.Check`, its native
Create/message/publication/pruning regressions, and the shared approval consumers.
The semantic loop considers only weakening rows, while the structural loop
also considers RFC rows. A comment-only tagged edit therefore loses its approved
verdict because `ChangedTags` correctly finds no behavior change.

Extend the existing semantic loop with weakening-first, RFC-fallback matching.
Reuse exact selected existing-unit comparison, live-row checks, single-use
tracking and `ApprovalSites` collision protection. Preserve discrimination
fingerprints and the access child's source, audit and real approval files.
Manual trailers or artificial test edits would hide the producer defect.

Before independent review, observe focused red/green through native Create and
serial race results for `internal/le/commit` and `internal/le/test/weakened`.
Cover unrelated, unchanged, excluded-hunk, foreign-session and already-carried
rows, collisions, precedence and overlapping selectors. Update the committing
and test-health contracts in the same change. No commit precedes round seven.

Implementation evidence: the focused native regression first failed for the
missing comment-only trailer and for both semantic directory-collision cases.
The same tests pass after `Check` reuses `claimFinding` for RFC fallback and
records its matches in `ApprovalSites`. Publication retains the exact verdict,
prunes its row, preserves unrelated rows and cannot reuse a carried verdict.
Selection exclusions, weakening precedence and overlapping selectors pass.

Both complete packages passed serial race testing with manifest tags and
CGO enabled. Evidence logs under `tmp/session/2026-11-02-150621/scratch/`:
`job-semantic-round7-red-be5a6b56.log`,
`job-semantic-round7-green-be5a6b56.log`, and
`job-semantic-round7-race-41e8a1f4.log`.
Independent round-seven review remains pending; this is implementation evidence,
not a review verdict. The real access-child approval was not consumed.

### Round six: owner authorization and scope

Result: parent `ses_e631d0341ffeMUf9M25Y7Cdxi2` relayed independent reviewer
`ses_e627fe661fferfDfHQg4T78za8`'s CLEAN verdict within this corrective scope.
Earlier findings are closed. This supersedes the pending-review notes below.
The semantic tooling lands separately; broader child 3 lint remains owed and
this spec stays open.

Owner authorization, 2026-11-02: asked "Do you authorize fixing this defect and
running review round six?", the owner answered "yes". This authorizes this
correction and independent review round six only. It does not authorize round
seven or a standing exception.

Round five reviewer `ses_e627fe661fferfDfHQg4T78za8` found that
`scope.go`, `weakenedUnits`, computes unresolved evidence but emits details only
from the whole-file verdict. An assertion added to complete approved TestA can
offset an assertion removed from incomplete TestB. The whole-file verdict then
has no assertion-loss detail, and Proposed permits the edit under TestA's row.

Round six reviews the source fix that emits residual findings themselves, exact
evidence deduplication, its native Proposed regression, and the Check, Proposed
and audit consumers. Preserve the prior same-kind, bodyless, UTF8, incomplete
signature and inline-approval proofs. Record focused red/green and serial race
results for `internal/le/commit` and `internal/le/test/weakened` before review.

Diagnosis: the owning attribution producer discards a residual kind absent from
the global verdict. The source fix includes residual details in the file-stem
finding and deduplicates identical detail strings only. A workaround that
removes the compensating assertion from the fixture would conceal the defect.

Round-six correction evidence: native
`TestProposedIncompleteLossSurvivesCompensatingAssertion` failed before the fix
for both balanced and increased whole-file assertion counts. Both proposals
were permitted with only TestA's approval. After the fix, both are refused and
report TestB's residual assertion loss under the file stem. `weakenedUnits`
now seeds the file-stem details from the residual verdict, then adds unmatched
whole-file details unless their full text is already present.

The focused count-mode run passed. The serial race command,
`CGO_ENABLED=1 ./le --name lint-backlog job run label semantic-round6-race quiet command go test -race -p 1 -tags "$tags" ./internal/le/commit ./internal/le/test/weakened -v`,
passed both complete packages using manifest tags. No test skipped and no race
was reported. Earlier same-kind, bodyless, UTF8, incomplete-signature and
inline-approval cases have named PASS results. A count-mode attribution run
also exercised both exact-detail deduplication and distinct-detail retention.
The scoped test-health documentation check passed. The independent round-six
review has not run in this authoring context.

### Next independent re-review scope: unresolved evidence and bodyless declarations

Before the correction runs, scope is the two findings from reviewer
`ses_e627fe661fferfDfHQg4T78za8`: `weakenedUnits` must retain unresolved
whole-file evidence even when an approved complete unit has the same finding
kind; `goFunctionUnits` must continue after a valid bodyless declaration without
accepting a truncated signature. Review the fixes and their Proposed/Create
regressions, plus the shared Check, Proposed and audit consumers. Preserve the
partial-source and UTF8 contracts and the original inline-approval regression.

No numbered rounds were recorded in this spec or its per-spec handoff at the
start of this correction. The parent must reconcile the session's count before
the next independent review; this entry does not authorize a sixth round.

Diagnosis: `weakenedUnits` skips incomplete units, then suppresses whole-file
details by kinds found in complete units. That loses unresolved evidence.
The source fix retains that evidence when the inventory is incomplete. A
workaround that changes the fixture or approves the whole file hides the loss.
`goBodyEnd` treats every signature semicolon as incomplete, so the extractor
stops before later tests. The source fix distinguishes a valid bodyless
declaration from a truncated signature. Reordering the fixture only hides it.

Correction evidence: `TestProposedIncompleteUnitCannotBorrowApproval` first
permitted the truncated sibling under only TestA's approval.
`TestCreateSemanticApprovalAfterBodylessDeclaration` first refused `external`
as incomplete. Both pass after the producer fixes. The residual comparison
uses scanner offsets and retains both sides when a counterpart is incomplete.
It preserves assertion-loss evidence without duplicating the existing UTF8
finding. Bodyless signatures are parsed individually; unnamed receiver fragments
remain incomplete.

Final evidence: with manifest tags and CGO_ENABLED=1,
`./le --name lint-backlog job run label semantic-two-findings-race-complete quiet command go test -race -p 1 -tags "$tags" ./internal/le/commit ./internal/le/test/weakened -v`
passed both complete packages, with no skips or race reports. Named passes
include the new regressions and unchanged UTF8, inline-approval and incomplete
boundary tests. Count-mode evidence covers all ten statements of the residual
helper. The two-page STE check passed. Independent re-review remains owed,
with the round-count reconciliation above.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Tooling safety | Each gosec annotation traces the trusted path or argv, and real ignored-error defects have red-first proof. |
| Test and hook contracts | Fixture assertions remain intact. Hook-source changes carry hook-unit evidence and the required digest procedure (R-1). |
| Tooling evidence | Scoped lint includes the eight excluded locations (A-2), changed-package tests use their tags, and annotations state reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Tooling scopes cleared | Record scoped lint results covering the excluded locations and tagged tests of changed packages (AC-1–2, A-2). |
| Preserved harness behavior | Inspect annotation reasons and fixture assertions. Cite hook-unit results if hook source changes (AC-3, R-1). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Files and subprocesses | Each gosec annotation traces path or argv trust in extraction, stdpatch, setup and deployment tooling. |
| Test errors | Runner and entrypoint error repairs expose real failures without weakening assertions or hiding cleanup failures. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the packages>"` zero; `./le job run label unit-pkg command go test -tags <tags> <changed packages>`; `./le ai hooks unit` if a hook source changed. Full-run evidence belongs to the umbrella's CI run.
