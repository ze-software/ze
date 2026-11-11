# Spec: lint-backlog-3-test-tooling

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | tooling |
| Priority | P2 |
| Release reason | 58 findings of the full lint in the test harness, `le` and the chaos tooling; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 5/5: source review complete; final validation blocked by foreign work |
| Handoff | verify after the claimed inventory build dependency is restored |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Blocker

The current checkout cannot build the native tooling while another session
owns unfinished inventory changes. The main thread observed the named-tool
rebuild fail on 2026-11-11 (exec session `39179`, exit 1), with these compiler
diagnostics in `internal/le/repo/inventory`:

| Producer location | Missing member |
|-------------------|----------------|
| `assemblyheaders.go:76:39` | `reader.snapshot.assemblyPredefines` |
| `assemblyheaders.go:120:27` | `unit.headers.snapshot.headerPresence` |
| `assemblylinkage.go:52:28` | `pkg.assembly` |
| `assemblylocalinclude.go:42:28` | `unit.headers.snapshot.localHeaders` |

`inventory/source.go` is dirty under the foreign claim for
`plan/post-release/engineering/spec-inventory-fast-registry-checks.md`, held by
session `03d43e18-c488-4511-8040-2650b2986bb3`. The claim marker is
`tmp/session/.session-03d43e18-c488-4511-8040-2650b2986bb3`. Its owner must
restore the coherent source dependency before lint3 validation resumes. The
launcher reported that the named binary was unchanged; this is an observed
build failure, not a scoped-lint verdict. No foreign source was changed here.

All lint3 repairs are committed, including the labs parity repair
`241b754837`, and the independent source review is ready. Final scoped lint,
structural/documentation prechecks and their evidence consolidation remain
owed; AC-1 and AC-2 are not certified. Resume with the current scoped evidence
listed below, rerun only the remaining gates once the dependency builds, then
record the machine Review Gate and complete closure. The owner's instruction
to move past locked or blocked work is why this spec is handed back now.

## Resolved environment blocker

The main-thread scoped lint on 2026-11-10 exited 2 before analysis. Its host
flavor `go list` could not write `/home/thomas/.cache/ze/go-cache/*`: the
sandbox reports a read-only filesystem. `Toolchain.Overrides` in
`internal/le/go/toolchain/gotoolchain.go` selects the checkout cache, whose
symlink reaches that directory; an environment-only GOCACHE override cannot
change the selected cache. No lint or package-test verdict was produced.

The owner restored write access to the configured cache and signing directories
on 2026-11-10. Closure resumed with a fresh main-thread scoped lint. The shared
cache link and tooling were not changed. AC-1 and AC-2 stay open until their
scoped evidence is available.

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
| 1-9, 11, 13-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, comparison, metadata, metrics, inventory, examples | No | Equivalent lint edits change no shipped operator or protocol contract |
| 10, 12 | Test infrastructure and tooling architecture | Yes | `docs/architecture/testing/interop.md` records exclusive marker creation; `docs/architecture/testing/test-health.md` and `docs/contributing/committing.md` record semantic approval selection and consumption, landed with `fc62fd18ec`, `726855e9be` and `9fffb6b1b3`. `docs/contributing/rfc-conformance-gates.md` records Git discovery ceilings in `b49d61572d`; `docs/contributing/gh-pages.md` describes the controlled labs parity input. |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Read current source-anchored claims for `runParsedEmptyMPSource`, `writeParsedEmptyMPReady`, `Check`, `claimFinding`, `checkSourceGates`, `insideGitCheckout` and `assertAuthoredPageContentParity`; each matches its producer. |

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

### Closure review scope, 2026-11-10

This fresh closure pass covers the two lint commits `5c403f0234` and
`bf7d87359f`, the marker fixes `d2c19eae5c` and `fc62fd18ec`, the semantic
approval fixes `726855e9be` and `9fffb6b1b3`, observer repair `39c04d6fbc`,
Git-discovery repair `b49d61572d`, DNS repair `9bca475108`, and the controlled
labs parity repair, their current producers and documentation, and the
closure record. The three lenses are behavior and
wiring, trust boundaries and cleanup, and assertion preservation and Go style.
The earlier seven semantic-approval rounds remain recorded below; this pass
does not reopen their completed correction loop. This independent source pass
found zero BLOCKER and zero ISSUE. The current shared `rfc.DeclaredUnits`
producer, rather than the retired handwritten scanner, was reviewed for the
Go unit boundary contract. Final machine artifact remains pending scoped lint
and main-thread structural prechecks; source review alone is not closure.

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
Independent round-seven review completed before commit `9fffb6b1b3`.
That commit records no code finding and a separate check of the corrected
documentation. The real access-child approval was not consumed. The pending
review statement previously recorded here was stale; this correction does not
authorize round eight.

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

## Implementation Summary

### What Was Implemented

Commits `5c403f0234` and `bf7d87359f` apply reasoned permission annotations,
equivalent loop and regular-expression forms, named fixture constants, and
explicit fixture cleanup errors. Commits `726855e9be` and `9fffb6b1b3`
preserve judged semantic approval rows through commit publication. Commits
`d2c19eae5c` and `fc62fd18ec` make the parsed-empty-MP readiness marker an
exclusively created file and prove refusal through the registered speaker.

### Bugs Found/Fixed

Scoped validation found two observer-guard defects. The family-12 census in
`plugin_fixture_12_peer_wait_test.go`, `parseFixturePackage12`, accepts exactly
two Register arguments, so adding the valid StopsDaemon property hides real
observers. The general guard in `plugin_fixture_peer_wait_test.go`,
`observerPackage.register`, treats that property as an executable driver.
Its wrapper discovery and ranged-row selection repeat the same old contract.
The owning layer is the static guard's interpretation of `fixture.Register`.
The source fix reads the second argument as the sole direct driver and admits
trailing metadata consistently. Removing the empty-census assertion or ignoring
the StopsDaemon error would hide the lost proof, so neither workaround is used.
Independent judgment approved the exact source correction. Main-thread red
`job-lint3-regressions-red-d4153b71.log` demonstrated lost direct, wrapped and
table registrations and the dropped family-12 observer. The approved repair is
applied. The main-thread focused run passed both real observer corpora, the
existing false-pass guard corpus and the new family-12 census case. Its new
property tests exposed an incomplete synthetic wait helper; independent
judgment approved supplying the recognized peer selector without changing any
expected finding. All six corrected property cases passed under the race
detector in main-thread `job-lint3-fixes-green-44b14f4e.log`. Commit
`39c04d6fbc` carries the three observer files, the native helper approvals and
the independent judgment of the receiver-ambiguous wrapper-discovery method.

The same matrix exposed bare RFC and approval fixtures inheriting empty
ancestor `.git` entries from the sandbox. `insideGitCheckout` ignored Git's
directory ceilings, so it classified the fixtures as corrupt repositories.
The repair honors the same discovery boundary as Git while preserving
refusal of a corrupt marker inside that boundary (`b49d61572d`). DNS runner failures came
from per-command exit fields on foreground commands; the repair preserves each
startup rejection in a separately asserted boot case. Main-thread validation
passed all five drafts, 80 stress invocations selecting the five cases, and all
five promoted live tests. Commit `9bca475108` carries the independently judged
split and removes only the stale accept-only exemption for `l2tp-bad-port`.
These repairs preserve the original validation scope; the remaining scoped
lint and package reruns still gate closure.

The site rerun exposed a stale editorial snapshot in
`TestTheLabsIndexPageReadsAsThePublishedPage`. The rendering contract gives
authored source authority over wording and forbids refreshing prose snapshots.
Commit `241b754837` instead feeds controlled captured content with a deliberately
stale sidebar through the original renderer. Exact visible text, ordered links
and complete Markdown assertions remain unchanged; the shared helper still
reads the live Zeledon source. Independent judgment approved both changed units.
The main-thread race run passed all four named authored-page tests in
`tmp/session/2026-11-10-234952/scratch/job-lint3-labs-parity-0b4734cb.log`
(package 3.497 seconds), including the existing sidebar and mirror controls.

- `writeParsedEmptyMPReady` previously overwrote an existing marker or symlink
  target. `TestParsedEmptyMPSourcePropagatesMarkerRefusal` proves the refusal
  reaches the registered speaker's caller and preserves the target bytes.
- `testweakened.Check` previously omitted approved semantic changes without
  structural findings. Native Create tests prove selection, publication and
  pruning, with weakening-first RFC fallback and directory-collision refusal.

### Documentation Updates

`docs/architecture/testing/interop.md` documents exclusive capture and marker
creation through `runParsedEmptyMPSource` and `writeParsedEmptyMPReady`.
`docs/contributing/committing.md` and `docs/architecture/testing/test-health.md`
document selected semantic approvals through `Check`, `claimFinding` and
`checkSourceGates`. These updates landed with their producing changes.
`docs/contributing/rfc-conformance-gates.md` documents bounded Git discovery;
`docs/contributing/gh-pages.md` documents the controlled labs parity input.
Both claims were independently checked against their named producers.

### Deviations from Plan

The CI locations had moved and the exhaustive annotations already existed.
The marker overwrite and semantic approval defects required source fixes
before the lint work could land. Current scoped lint and package evidence are
still required; the original CI list is not evidence for the current tree.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 treated the CI line list as current | Locations moved and exhaustive annotations already existed | Implementation source audit | Judge the current scoped lint |
| approach | The spec still called round seven pending | `9fffb6b1b3` records independent review and passing race tests | Closure history review | Correct the stale record without inventing another review round |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Clear the listed tooling scopes | In progress | `5c403f0234`, `bf7d87359f` | Cache access restored; fresh lint planning refused a concurrently changed source file |
| Preserve harness assertions | Reviewed | Fixture constants, equivalent regex classes and cleanup-error branches in `bf7d87359f` | No expected assertion removed |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | In progress | Scoped lint planning exited 2 | `internal/le/workflowcheck/stdpatch_cache_link_test.go` changed during planning; no lint verdict |
| AC-2 | In progress | Main-thread tagged race matrix and focused reruns | Initial observer, runner, RFC-fixture and native-tool failures resolved; named RFC real-corpus and all four named labs tests passed; final validation consolidation remains |
| AC-3 | Reviewed | `5c403f0234` and `bf7d87359f` annotations | Each names its linter and trust or result-handling reason |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Existing changed-package tests | In progress | Main-thread tagged race matrix | Partial passes recorded in the job log; focused repairs and reruns remain |
| Marker refusal red/green proof | Recorded | `fc62fd18ec`, `TestParsedEmptyMPSourcePropagatesMarkerRefusal` | The earlier proof is recorded above |
| Semantic RFC approval native proof | Recorded | `9fffb6b1b3`, `TestCreateSemanticRFCApproval` | Independent round seven completed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Task-table files | Implemented or already corrected | Current lint must verify the population, including later changes |

### Audit Summary

AC-1 and AC-2 remain open. No scope was removed.

The main-thread race matrix completed green for `internal/test/plugins/fakefilter`,
`internal/test/userns`, `internal/test/editorcase`, `internal/test/cli`,
`internal/le/cli/catalog`, `internal/le/go/extract`,
`internal/le/go/stdpatch/gen`, `internal/le/interoplab/ipsec`,
`internal/le/setup` and `internal/le/test/functional`.
`tools/freertr-entrypoint` compiled and has no tests. The RFC package exhausted
its 600-second aggregate budget during `TestRFCTreeOverRealCorpus`; its earlier
completed passes remain evidence, but the interrupted package run is not a pass.
The site package initially failed to build during concurrent inventory source
changes; workflowcheck reported missing import artifacts. The main thread retains the full log
as `job-lint3-scoped-race-9ef91d05.log`.

The fresh `job-lint3-site-workflow-18c55e1c.log` run completed the full
workflowcheck package under race. Site compiled and passed its catalog checks,
including `TestEveryPublishedCatalogFieldReachesAReader`, and
`TestThisCheckoutCanAnswerEveryPublishedFact`. Its labs page assertion exposed
a rendered HTML/Markdown wording mismatch; repair `241b754837` passed all four
named labs tests under race in `job-lint3-labs-parity-0b4734cb.log` (3.497s).
Its package-wide 600-second budget expired during
`TestTheGateSnapshotAgreesWithItself`, after that test had run for 2m53s.
That interrupted whole-tree snapshot remains unverified; it exercises a full
RFC check rather than the changed catalog comment and is a disclosed limit,
not an additional whole-tree closure requirement. The completed site passes do
not need repeating and the incomplete package run is not reported as a pass.

The two remaining native producer fixtures passed under race in
`job-lint3-native-fixtures-e269ad09.log`: `TestToolFixturesReachTheirProducer`
and `TestRulesFixtureRootAliasesReachOnlyOwnedTree`. The corrected main-thread
environment points PATH at the actual named native `le` binary rather than the
shell launcher and explicitly supplies the checkout GOCACHE. Their command
helpers use synchronous `cmd.Run`; no fixture assertion or source changed.

The seven original RFC action tests covering `disableBody` and its halt/import
handling passed under race in `job-lint3-actions-and-corpus-2c82a357.log`, as
did all three previously failed runner corpus checks: `TestCIAcceptOnlyLint`,
`TestParseCICorpusReadsUnderTheGenericParser` and
`TestPeerASDerivationReachesEveryCIFile`. The other RFC package tests do not
become owed solely because the aggregate run was interrupted; the named
real-corpus test that was running at that point was rerun separately. It passed
under race in 297.40s in `job-lint3-focused-corpus-5e2a0f64.log` (exit 0).

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Zero findings in the tooling scope | Scoped lint | Main-thread run exited 2 before analysis; no verdict |
| Preserve test and command behavior | Tagged package tests | Observer, runner, RFC fixture and native-tool repairs passed focused race runs; workflowcheck passed in full; named RFC corpus and all four labs tests passed; unrelated whole-tree site snapshot remains an explicit limitation |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Final scoped lint and changed-package validation | Current closure runs exposed defects; repairs and reruns remain in progress | `plan/release/validation/spec-lint-backlog-3-test-tooling.md` retains AC-1 and AC-2 |

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| Implementation files | Yes | Committed file lists in `5c403f0234`, `bf7d87359f`, `726855e9be`, `9fffb6b1b3`, `d2c19eae5c`, `fc62fd18ec` |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | Lint and changed-package checks pass | Not established: current lint planning refused a concurrent source edit; race matrix exposed defects under repair |
| AC-3 | Added annotations explain their permission or error handling | Read the annotation diffs in both lint commits |

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Scoped lint | Not applicable: tooling analysis | Requested over all four package roots |
| Native commit preparation | Native Create tests | `9fffb6b1b3` records selection, publication and pruning proof |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Existing annotations and moved source locations; recorded in Mistake Log |
| A-2 | Unvalidated: in progress | Scoped lint must cover the earlier eight fixes; cache access is restored |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Marker file ownership | `runParsedEmptyMPSource`, `writeParsedEmptyMPReady` | Exclusive creation and explicit result-path routing |
| Semantic approval carry | `testweakened.Check`, `claimFinding` | Weakening-first RFC fallback uses selected existing units |
| Fixture literals and loop syntax | `5c403f0234`, `bf7d87359f` | No operator contract changes |

## Closure repair: Git discovery boundaries (2026-11-10)

Implementation and documentation committed as `b49d61572d`; the lint3 claim
remains held for closure.

The scoped RFC tests exposed a tooling defect in
`internal/le/rfc/check_evidence_prior.go::insideGitCheckout`. Its ancestor
walk ignored `GIT_CEILING_DIRECTORIES`, so empty sandbox `.git` markers above
history-free fixtures made `priorRevisionState` report unreadable history.
`approvalunits.go::resolveApprovalSides` uses the same predicate and could
not take its intended plain-tree path either.

The source fix follows Git's documented discovery boundary. It preserves
markers below the ceiling and in the working directory, explicit `GIT_DIR`,
and fail-closed handling of unresolved paths and non-ENOENT inspection
errors. `gitDiscoveryCeilings` resolves absolute entries' symlinks until an
empty entry disables resolution; relative entries are ignored. Ignoring
malformed `.git` markers instead would be a workaround that also hides a
corrupt checkout, so that approach was rejected.

Files changed in this phase:

- `check_evidence_prior.go`: the two discovery producers above.
- `check_evidence_ceiling_test.go`: ceiling, symlink, current-directory,
  explicit-Git-directory and failed-inspection regressions.
- `check_test.go::checkFixtureTree` and
  `check_evidence_test.go::TestPriorRevisionState`: set the temporary root's
  parent as the discovery ceiling; all existing assertions remain intact.
- `docs/contributing/rfc-conformance-gates.md`: the exclusion-ratchet
  contract now describes discovery boundaries.

All test execution was in the main thread. The ceiling matrix failed before
the fix in `job-lint3-regressions-red-d4153b71.log`. It and the existing
`TestPriorRevisionState` corruption, shallow-clone and missing-parent
controls passed in `job-lint3-ceiling-check-7f7b8f05.log`; that invocation
remained red because the newly added non-directory regression exposed an
ignored `Lstat` error. After the error guard, that regression passed under
race in `job-lint3-fixes-green-44b14f4e.log` (exit 0). These logs live under
the 2026-11-10 sessions `231811`, `232205` and `232320`, respectively.

The independent test judge approved both fixture setup edits, recorded in
`tmp/commit-weakening-approved-a51004d0.md`, and independently re-read the
source fix with no remaining findings. The implementation agent also read
the complete phase diff against the Go style guide. No protocol assertion
or RFC proof record changed. The main thread then reran all 20 previously
failing consumers: 13 RFC tests and seven weakening-approval tests passed
under race in
`tmp/session/2026-11-10-232436/scratch/job-lint3-git-consumers-8bc99280.log`.
That process used
`GIT_CEILING_DIRECTORIES=/home/thomas/.zt:/home/thomas/.cache/tmp:/tmp` to
bound the parallel weakening tests' temporary roots without changing their
assertions or process environment mid-test. This phase does not claim the
spec's final closure Review Gate or its closing commits.
