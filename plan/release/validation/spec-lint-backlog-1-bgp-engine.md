# Spec: lint-backlog-1-bgp-engine

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 53 findings of the full lint in the BGP engine packages; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 1/1 — implementation steps 2–4; verification owned by main |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the BGP engine
(door `ai/doors/bgp-engine.md`): reactor 39, policychain 8, config 4, cli 1,
redistribute 1. By linter: exhaustive 10, modernize 10, misspell 8, errcheck 8,
goimports 8, gocritic 5, nilerr 2, goconst 1, gosec 1. Line numbers are the CI
run's; regenerate the list with the scoped run first, since other sessions edit
the reactor.

| File | Line | Linter |
|------|------|--------|
| `internal/component/bgp/cli/register.go` | 76 | goconst |
| `internal/component/bgp/config/dynamic_group_parity_test.go` | 190 | modernize |
| `internal/component/bgp/config/dynamic_group_parity_test.go` | 191 | modernize |
| `internal/component/bgp/config/external_filters.go` | 150 | modernize |
| `internal/component/bgp/config/templated_chain.go` | 108 | modernize |
| `internal/component/bgp/policychain/chain.go` | 643 | misspell |
| `internal/component/bgp/policychain/external.go` | 111 | misspell |
| `internal/component/bgp/policychain/external_intents_test.go` | 150 | gocritic |
| `internal/component/bgp/policychain/match.go` | 139 | gocritic |
| `internal/component/bgp/policychain/prefix_overlay.go` | 139 | gocritic |
| `internal/component/bgp/policychain/sets.go` | 291 | misspell |
| `internal/component/bgp/policychain/sets.go` | 396 | modernize |
| `internal/component/bgp/policychain/sets_attributes.go` | 63 | modernize |
| `internal/component/bgp/reactor/bfd_delay_open_release_race_test.go` | 79 | errcheck |
| `internal/component/bgp/reactor/bfd_delay_open_release_race_test.go` | 105 | errcheck |
| `internal/component/bgp/reactor/capture_replay.go` | 242 | gosec |
| `internal/component/bgp/reactor/filter_chain.go` | 613 | exhaustive |
| `internal/component/bgp/reactor/filter_library.go` | 273 | modernize |
| `internal/component/bgp/reactor/filter_library_text.go` | 89 | misspell |
| `internal/component/bgp/reactor/filter_message_test.go` | 9 | goimports |
| `internal/component/bgp/reactor/filter_message_test.go` | 26 | modernize |
| `internal/component/bgp/reactor/filter_message_test.go` | 50 | misspell |
| `internal/component/bgp/reactor/filter_nlri_delta.go` | 208 | modernize |
| `internal/component/bgp/reactor/filter_nlri_delta.go` | 306 | nilerr |
| `internal/component/bgp/reactor/filter_nlri_delta.go` | 312 | nilerr |
| `internal/component/bgp/reactor/filter_nlri_delta_test.go` | 5 | goimports |
| `internal/component/bgp/reactor/filter_ordered.go` | 170 | gocritic |
| `internal/component/bgp/reactor/filter_ordered_test.go` | 4 | goimports |
| `internal/component/bgp/reactor/filter_unavailable_test.go` | 89 | modernize |
| `internal/component/bgp/reactor/forward_body.go` | 639 | exhaustive |
| `internal/component/bgp/reactor/forward_pool_barrier_test.go` | 47 | errcheck |
| `internal/component/bgp/reactor/forward_pool_barrier_test.go` | 201 | errcheck |
| `internal/component/bgp/reactor/forward_publication_ownership_test.go` | 150 | errcheck |
| `internal/component/bgp/reactor/forward_validation.go` | 633 | exhaustive |
| `internal/component/bgp/reactor/loop_policy_binding_test.go` | 9 | goimports |
| `internal/component/bgp/reactor/peer.go` | 2520 | exhaustive |
| `internal/component/bgp/reactor/peer_run_test.go` | 746 | errcheck |
| `internal/component/bgp/reactor/peer_run_test.go` | 888 | errcheck |
| `internal/component/bgp/reactor/peer_run_test.go` | 894 | errcheck |
| `internal/component/bgp/reactor/policy_final_test.go` | 6 | goimports |
| `internal/component/bgp/reactor/reactor_iface_test.go` | 230 | misspell |
| `internal/component/bgp/reactor/reactor_notify.go` | 390 | exhaustive |
| `internal/component/bgp/reactor/reactor_notify.go` | 433 | exhaustive |
| `internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go` | 8 | goimports |
| `internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go` | 38 | gocritic |
| `internal/component/bgp/reactor/rfc6996_remove_private_test.go` | 4 | goimports |
| `internal/component/bgp/reactor/rfc6996_remove_private_test.go` | 84 | misspell |
| `internal/component/bgp/reactor/rfc8212_default_reject_test.go` | 11 | goimports |
| `internal/component/bgp/reactor/rfc8277_label_rsrv.go` | 108 | exhaustive |
| `internal/component/bgp/reactor/session_accept_own.go` | 31 | exhaustive |
| `internal/component/bgp/reactor/session_bfd_strict.go` | 422 | exhaustive |
| `internal/component/bgp/reactor/session_handlers.go` | 368 | exhaustive |
| `internal/component/bgp/redistribute/consumer.go` | 120 | misspell |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/bgp-engine.md` - the engine's boundaries and traps
  → Constraint: an untagged package test does not link BGP; run the packages with `-tags ze_bgp`
  → Constraint: received bytes are borrowed and encoding is buffer-first; a modernize rewrite must not add an allocation on a hot path
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces", the `exhaustive` C/O/P classification
  → Constraint: each enum switch is classified C, O or P; a P switch takes `//exhaustive:ignore // <why>`, never a blanket nolint
- [ ] `ai/META-DECISIONS.md` - "Code and dependencies"
  → Constraint: an RFC quote in a comment stays byte-exact; a misspell inside one gets a reasoned `//nolint:misspell`, never an edit

**Key insights:**
- The two `nilerr` findings in `filter_nlri_delta.go` return nil beside a live error: read each as a possible dropped error before any annotation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/filter_nlri_delta.go` - the two `nilerr` sites
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - two enum switches `exhaustive` reports
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every wire and FSM behavior; an exhaustive fix keeps each switch's existing named outcomes and its unknown-value handling.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope over every build flavor | Applies: the evidence for this child |
| Style rule table and C/O/P | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies: every fix follows it |
| Lint contract journal rows | `plan/journal/lint-contract-not-applied.md`, the 2026-10-23 reactor row (owned by `plan/post-release/engineering/spec-gates-report-cause-and-progress.md`) | Records reactor findings found in a scoped run | Applies: a finding here that row names is marked `fixed:` in the same commit |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/bgp/reactor/... ./internal/component/bgp/policychain/... ./internal/component/bgp/config/... ./internal/component/bgp/cli/... ./internal/component/bgp/redistribute/..."`

### Transformation Path
1. Read each finding at its line; decide fix, or a reasoned annotation where the linter is wrong.
2. A finding that is a real defect gets a red test, then the fix.

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
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted; findings added or gone | Source audit found 29 recorded sites already addressed and 24 requiring treatment; current lint matrix is still unverified after interrupted baseline runs | broken |

### Source audit and execution boundary

The saved CI list guides implementation, not a current lint-clean claim. The
parent authorized steps 2–4 before a new lint run because server restarts
interrupted the baseline. It owns serialized lint and test execution after the
implementation handoff. AC-1 and AC-2 remain unverified.

The implementation patch addresses all 24 remaining recorded sites:
mechanical rewrites and spelling, plus eight test error-channel receives whose
existing completion assertions remain intact. Five intentional cleanup or
failure-session joins carry local `errcheck` reasons; three forbidden early
completions include the returned error in their unconditional failure message.
No protocol behavior, RFC-tagged test, or quoted RFC sentence changed.

The parent recovered CI run `41447216730`'s exact diagnostics:
`policychain/prefix_overlay.go:dropMPReach` reports `appendAssign`, and
`reactor/capture_replay.go:sessionCapture.openFile` reports `G302` (permissions
above 0600). The append now writes back to its local slice, still publishing
the scratch buffer only after a successful rewrite. G302 has a local reason:
`captureFilePerm` deliberately grants daemon-group read access with 0640,
paired with the existing 0750 directory contract. No permission changed.
Both source treatments still await the parent's scoped lint and tests.

### Mistake Log — Wrong Assumptions

| Assumption | Evidence | Correction |
|------------|----------|------------|
| A-1: the saved CI list still describes current source | `filter_nlri_delta.go:nlriSectionOf` already propagates MP errors; commit `5c1aea079c` also cleared reactor mechanical findings | Preserve existing repairs; patch remaining source sites and reconcile the full list against the next completed lint run |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An exhaustive fix changes an FSM or notify outcome | A reactor or functional test goes red | Keep each named outcome; add only the missing named cases with their current behavior |
| R-2 | Another session holds uncommitted reactor edits | `git status` shows reactor files dirty | Commit only this child's hunks (`./le commit create` `hunks`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the five packages | Zero findings |
| AC-2 | The package tests under `-tags ze_bgp -race` | Pass |
| AC-3 | Any `//nolint` or `//exhaustive:ignore` added | Names its linter and its reason |
| AC-4 | A `nilerr`, `errcheck` or `gosec` finding that is a real defect | Fixed with a test that was red before the fix |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | `internal/component/bgp/...` | AC-2 | |
| A red-first test per real defect | the package of the producer | AC-4 | |

## Files to Modify
- The files in the Task table, under `internal/component/bgp/reactor`, `policychain`, `config`, `cli` and `redistribute`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-1-bgp-engine.md`; a renamed symbol updates its anchor |

## Implementation Steps

1. Scoped lint; record the current list in the spec.
2. Mechanical findings (goimports, misspell, modernize, goconst, gocritic).
3. errcheck, nilerr, gosec: read each; real defects red-first.
4. exhaustive: classify C/O/P per switch, then fix.
5. Scoped lint zero; package tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| FSM outcomes | C/O/P classification preserves named, unknown and post-switch outcomes in the reactor (R-1). |
| Error paths | Each real nilerr, errcheck or gosec defect has red-first proof (AC-4). |
| Engine evidence | All five scopes are lint-clean and tagged race tests pass, with reasoned annotations and byte-exact RFC quotes (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Five engine scopes cleared | Record zero scoped lint findings and the tagged race-test results from the TDD checklist (AC-1–2). |
| Error-path repairs | Cite red-first tests for real defects and inspect reasons on each new annotation (AC-3–4). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Peer-controlled states | Exhaustive fixes preserve safe unknown handling and introduce no peer-reachable panic. |
| Dropped errors and ownership | Nilerr and errcheck repairs retain error handling, while mechanical rewrites preserve borrowed-buffer lifetimes. |
| Capture paths | Any gosec annotation establishes why the capture path is trusted. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the five packages>"` zero; `./le job run label bgp-engine command go test -race -tags ze_bgp <the five packages>`, the five-package subset of `./le test unit bgp`. Full-run evidence belongs to the umbrella's CI run.

## Implementation Summary

### What Was Implemented
- `d4b29e9b58` (this spec) cleared the 24 sites still open at the source audit. Mechanical: `cli/register.go` `bgpModeOffline` (goconst); `slices.Contains`/`ContainsFunc` in `config/external_filters.go` `checkFilterDeclarations`, `config/templated_chain.go` `checkTemplatedMember`, `policychain/sets.go` `Sets.removesCommunity` and `policychain/sets_attributes.go` `valueList.removes`; `reflect.TypeAssert` in `config/dynamic_group_parity_test.go`; `matchWalk` moved above its methods in `policychain/match.go` (typeDefFirst); one `append` in `external_intents_test.go` `labeledReach`; `policychain/prefix_overlay.go` `Chain.dropMPReach` appends onto its own local (appendAssign) and still writes `*buf = out` only after `wireu.DropCIDREntries` succeeds; US spellings in comments of `chain.go`, `external.go`, `sets.go`, `reactor_iface_test.go` and `redistribute/consumer.go`.
- errcheck (test files): the early-completion branches in `forward_pool_barrier_test.go` and `peer_run_test.go` now print the received error; five cleanup or deliberate-failure joins carry `//nolint:errcheck // <reason>`.
- gosec G302: `reactor/capture_replay.go` `sessionCapture.openFile` keeps `captureFilePerm` (0o640) under a reasoned `//nolint:gosec`; no permission changed.
- Sites cleared before this spec's commit, by other commits: `5c1aea079c` (21 reactor goimports, misspell, modernize and gocritic sites), `37a296ead9` (the two `nilerr` sites: `filter_nlri_delta.go` `nlriSectionOf` returns the MP parse error), `4abd85f6f9` (the ten `exhaustive` switches carry an empty `//exhaustive:ignore` with the reason on the line above).

### Bugs Found/Fixed
- `nlriSectionOf` read a malformed MP_REACH_NLRI or MP_UNREACH_NLRI as an absent section (the `nilerr` pair). Fixed red-first in `37a296ead9`: `TestNLRISectionOfMalformedMPIsError`, `TestNLRIDeltaRefusesMalformedMPChange` and `TestLibraryNLRIOpsRefusesStripOverMalformedMP` (`internal/component/bgp/reactor/nlri_section_test.go`) failed with the producer reverted by overlay, per that commit's body.
- No other finding was a product defect: the errcheck sites are test channel receives, and G302 is an intended group-read mode.

### Documentation Updates
- None. The diff changes no behavior a page states: comment spellings, equivalent rewrites and lint annotations. Pages anchored to the changed files (`docs/architecture/bgp/protocol-event-capture.md`, `docs/architecture/core-design.md`, `docs/guide/redistribution.md` and others) name symbols the diff did not rename, and no page states the capture file mode (grep `0640` in `protocol-event-capture.md`: no hit).

### Deviations from Plan
- Step 1 (scoped lint before the fixes) did not run: server restarts interrupted the baseline, and the parent authorized steps 2-4 from the CI list ("Source audit and execution boundary" above).
- 29 of the 53 sites were cleared by the three commits named above, not by this spec's commit.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: the saved CI list described current source | 29 of 53 sites were already addressed by other commits | Source audit before implementation | Patched only the 24 open sites |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| goconst, modernize, misspell, gocritic sites in cli, config, policychain, redistribute | Done | `d4b29e9b58` | listed in What Was Implemented |
| reactor goimports, misspell, modernize, gocritic sites | Done | `5c1aea079c`, `d4b29e9b58` | |
| errcheck (8) | Done | `d4b29e9b58` test files | 3 print the error, 5 reasoned nolint |
| nilerr (2) | Done | `37a296ead9` `nlriSectionOf` | real defect, red-first |
| gosec G302 (1) | Done | `capture_replay.go` `sessionCapture.openFile` | reasoned nolint |
| exhaustive (10) | Done | `4abd85f6f9` | reason above each directive |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | main thread 2026-11-10, `./le go lint run scope` over the five engine trees: exit 0, 0 issues in host, GOOS=linux integration and capability flavors (log `tmp/session/2026-11-09-222501/scratch/job-lint1-bb6c2f72.log`) | after the `atomicity_test.go` fix of Review Gate finding 1 |
| AC-2 | Done, one foreign known red | main thread 2026-11-09, `./le job run label lint1-race quiet command go test -race -count=1 -tags "ze_core <every ze_* tag>"` over the five package trees (log `tmp/session/2026-11-09-222501/scratch/job-lint1-race-dbedcdc1.log`): ok reactor/loopdetect, policychain, config, cli, cli/yang, redistribute; reactor FAIL on `TestOrdinaryRefreshDoesNotDoublePrepend` alone | that test is the deliberate red of `plan/release/fixes/spec-bgp-refresh-reapplies-export-policy.md` (ready), added in `1e65b6d4bb`, journal `plan/journal/non-idempotent-append-duplicates-the-record.md` row of 2026-11-06; the lint-only edits do not reach it |
| AC-3 | Done | each new `//nolint` names its linter and reason; each `//exhaustive:ignore` has its reason on the line above | read at every site |
| AC-4 | Done | `nlri_section_test.go`, three tests red-first in `37a296ead9` | the only real defect |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Existing package tests | Done | `internal/component/bgp/{reactor,policychain,config,cli,redistribute}` | AC-2: all pass except the foreign known red `TestOrdinaryRefreshDoesNotDoublePrepend` |
| Red-first test per real defect | Done | `internal/component/bgp/reactor/nlri_section_test.go` | AC-4 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Task-table files | Done | 18 in `d4b29e9b58`, the rest in the commits named above |

### Audit Summary
- **Total items:** 6 requirements, 4 ACs
- **Done:** 6 requirements, AC-1 to AC-4 (AC-2 with one foreign known red)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0 (the step-order deviation is recorded)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Clear the CI lint findings in the five BGP engine packages | scoped lint | 0 issues over the five trees in every flavor, 2026-11-10 (log `job-lint1-bb6c2f72.log`) |
| Keep wire and FSM behavior unchanged | package race tests | tagged race run 2026-11-09 green except the foreign known red `TestOrdinaryRefreshDoesNotDoublePrepend` (owned by `plan/release/fixes/spec-bgp-refresh-reapplies-export-policy.md`); red-first proof of the one behavior change in `37a296ead9` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | - | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lint-backlog-1-bgp-engine-49da3d3d-44f5-443f-bc7a-73b9535a17f9.md` (the 18 files of `d4b29e9b58` and `policychain/atomicity_test.go`) |
| `./le spec review check` | clean: 19 code files, hashes match |
| Rounds | 2 |
| Reviewer lenses used | logic and removed-behavior (each rewrite equivalent; `dropMPReach` still publishes the scratch buffer only on success), test proof (no assertion dropped; `./le test weakened check base d4b29e9b58~1` clean), security (G302 scope, no peer-reachable panic, no new hot-path allocation), style pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | Run 1: 0 BLOCKER, 0 ISSUE | - | - |
| 1 | ISSUE | The scoped lint (main thread 2026-11-09, log `tmp/session/2026-11-09-222501/scratch/job-lint1-bb6c2f72.log`) reported 4 findings in `policychain/atomicity_test.go`, which came in with `19237e47d8` after `d4b29e9b58`: gocritic dupOption on the three `atomicChain(t, prior, failed, failed, failed, gate)` calls, and exhaustive on the `attribute.AttributeCode` switch in `TestChainFailOpenMergeAtomicity` | `atomicity_test.go` | new helper `atomicFailureChain` builds the same five members in the same order (`slices.Repeat` of the one failing member, three times); the switch takes an empty `//exhaustive:ignore` with its reason on the line above. No assertion changed: `len(sets.Failures()) != 3` and the 17-operation count stay |

### Run 2
Scope: the `atomicity_test.go` edit only. 0 BLOCKER, 0 ISSUE. The chain still holds prior, three runs of the same failing member and gate, named a to e as before, so the failures still reuse the alternating overlay buffers the tests exist to exercise. `./le test weakened check` reports no finding for the file; its one finding, `internal/le/interoplab/bgp/check_local_pref_wire_test.go`, belongs to another session.

NOTE 1: golangci's errcheck does not usually report a bare channel receive; if the scoped lint reports an unused `nolint` directive at those five sites, the directive goes. NOTE 2: `./le repo check` reports one foreign issue, `internal/test/dhcpprobe/step.go` (number-parse allowlist), outside this spec.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/nlri_section_test.go` | Yes | holds the three AC-4 tests (grep) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-3 | every annotation reasoned | the ten `//exhaustive:ignore` sites each carry a reason comment on the line above (read 2026-11-09) |
| AC-4 | nilerr fixed | `nlriSectionOf` returns `source, err` when `MPReach` or `MPUnreach` fails (`filter_nlri_delta.go`, read 2026-11-09) |
| AC-1 | zero findings | scoped lint exit 0, 0 issues, 2026-11-10 (log `tmp/session/2026-11-09-222501/scratch/job-lint1-bb6c2f72.log`); policychain race rerun after the test fix ok (log `tmp/session/2026-11-09-222501/scratch/job-lint1-pc-8231c91d.log`) |
| AC-2 | race tests pass | main thread run 2026-11-09, log `tmp/session/2026-11-09-222501/scratch/job-lint1-race-dbedcdc1.log`: all pass except foreign known red `TestOrdinaryRefreshDoesNotDoublePrepend` (`refresh_prepend_double_test.go`, from `1e65b6d4bb`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le go lint run scope` over the five packages | none (lint run) | Yes: 0 issues, 2026-11-10 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | 29 sites already addressed; Mistake Log row |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No page changes | anchored pages name unchanged symbols; no page states the capture mode | Yes |
