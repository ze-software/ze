# Spec: lint-backlog-5-hub-and-policy

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 18 findings of the full lint in `cmd/ze` and the policy component; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in `cmd/ze` and
`cmd/ze/hub` (door `ai/doors/hub-engine.md`), all in test files, and in
`internal/component/policy` (covered by `ai/doors/system.md` alone), all
goconst. By linter: goconst 10, gocritic 3, errcheck 2, exhaustive 1,
modernize 1, unused 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `cmd/ze/hub/build_tag_ssh_absent_test.go` | 252 | errcheck |
| `cmd/ze/hub/build_tag_ssh_absent_test.go` | 255 | errcheck |
| `cmd/ze/hub/local_bridge_match_test.go` | 184 | gocritic |
| `cmd/ze/hub/local_bridge_match_test.go` | 195 | exhaustive |
| `cmd/ze/hub/surface_parity_test.go` | 224 | modernize |
| `cmd/ze/hub/surface_parity_test.go` | 278 | gocritic |
| `cmd/ze/hub/surface_parity_test.go` | 337 | gocritic |
| `cmd/ze/schema_helpers_test.go` | 25 | unused |
| `internal/component/policy/chain.go` | 127 | goconst |
| `internal/component/policy/chain.go` | 129 | goconst |
| `internal/component/policy/export_chain.go` | 20 | goconst |
| `internal/component/policy/export_chain.go` | 41 | goconst |
| `internal/component/policy/export_chain.go` | 43 | goconst |
| `internal/component/policy/reference_source.go` | 146 | goconst |
| `internal/component/policy/reference_source.go` | 182 | goconst |
| `internal/component/policy/rib_compile.go` | 190 | goconst |
| `internal/component/policy/rib_compile.go` | 192 | goconst |
| `internal/component/policy/route_chain_path.go` | 42 | goconst |

## Required Reading

### Architecture Docs
- [x] `ai/doors/hub-engine.md` - `cmd/ze` and the hub
  → Constraint: an untagged test does not see BGP; the hub tests run with the tags their package needs
  → Constraint: `TestEveryCommandOnEverySurface`-style parity tests must keep proving every surface; a gocritic rewrite keeps every case
- [x] `ai/doors/system.md` - the policy component sits under no subsystem door
  → Constraint: a goconst constant names one concept once; a config path or chain keyword the YANG already declares is read from its declaration where one exists
- [x] `docs/contributing/ze-go-style.md` - "The rules a tool enforces"

**Key insights:**
- The policy goconst strings are config words (chain and source names); check whether a declared constant already holds each before adding one.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/policy/export_chain.go` - three repeated strings
- [x] `cmd/ze/hub/surface_parity_test.go` - the parity test's gocritic and modernize sites
- [x] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- The policy chain's config words and every hub test's assertions.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Enumeration gate | `./le arch enumeration check` | Refuses a literal that restates what a registry holds | Applies: a new constant must not restate a registry |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./cmd/ze/... ./internal/component/policy/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.

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
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Saved CI list compared with current source; parent prohibited concurrent lint | broken: the seven hub findings already have source rewrites; fresh lint remains owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deleting the `unused` helper in `cmd/ze/schema_helpers_test.go` removes something a tagged build uses | A flavor's test build fails | Check every build flavor before deleting (`./le repo compiles check`), or tag the file |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over `cmd/ze/...` and `internal/component/policy/...` | Zero findings |
| AC-2 | The package tests with their tags | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | `cmd/ze/hub`, `cmd/ze`, `internal/component/policy` | AC-2 | |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-5-hub-and-policy.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Policy goconst; hub test findings.
3. Scoped lint zero; tests pass.

### Step 2 source audit and patch (2026-11-02)

The parent authorized this child under the umbrella's one-off WIP exception;
claim used command-local `ZE_SPEC_WIP_CAP=25`. The parent holds serialized
verification and commits. No tests, lint, builds or compiles ran in this phase.

| Finding | Current source and action |
|---------|---------------------------|
| Hub errcheck, two sites | The cleanup in `build_tag_ssh_absent_test.go` already explicitly discards `Process.Kill` and log close errors. No edit. This establishes the reported lint shape is gone, not new runtime proof. |
| Hub gocritic and exhaustive in local bridge | `matchMandatoryArgs` indexes definitions; `matchValue` lists all five named argument kinds. `matchArtifact` also already uses `bytes.Equal`. No assertions changed. |
| Hub modernize and two gocritic sites in surface parity | `bashRootNames` uses `strings.Cut`; `registerStage1Commands` and `collectCommandRegistrations` index command declarations. All surface checks remain in place. No edit. |
| Policy goconst, ten sites | `chain.go` declares the shared chain container, leaf and final-value tokens. The tree, export, RIB and reference readers use those exact values. The former `ribFinalAccept` declaration is incorporated into the shared final token; match-action tokens remain separate concepts. No registry or selection behavior changes. |
| Unused schema helper | Removed only `cmd/ze/schema_helpers_test.go`'s uncalled `wireToPathsForTest`. Recursive search of every Go file under `cmd/ze`, without build-tag filtering, found only this declaration and the separate helper and caller in `cmd/ze/internal/cmdutil`. The latter are retained. No test case or executed assertion was removed. |

Diagnosis: repeated chain vocabulary, rather than a policy-selection defect,
caused the ten policy findings. The source fix shares names for the same config
concept. A lint suppression would leave the duplication and is not used. The
unused helper belongs to a package with no caller; keeping it behind a different
tag would hide the finding without restoring a use.

### Evidence still owed to step 3

- AC-1: `./le go lint run scope "./cmd/ze/... ./internal/component/policy/..."`.
  Use the runner's existing flavor matrix, including SSH-absent and SSH/web-present.
- AC-2: serialized package tests for `./cmd/ze`, `./cmd/ze/hub` and
  `./internal/component/policy`, using the declared feature tags (BGP for the
  local-bridge fixture; SSH and web for surface parity; `ze_core` for the
  removed helper's package). Include the existing SSH-absent tests in a build
  without `ze_ssh`. Derive the full shipped tag set from the existing manifest.
- Keep `TestLocalCommandDaemonMatchesLocal`, `TestEveryCommandOnEverySurface`,
  `TestExtractChain`, `TestRIBImportCompilesAndJudges` and
  `TestRIBImportRefusals` in that evidence; do not replace their assertions.
- AC-3: no suppression was added. Independent review and the spec citation-anchor
  check remain owed. No documented behavior changed, so no guide rewrite is needed.
- No product defect was identified for a red-first queue by this source audit.

### Mistake Log: Wrong Assumptions

| Assumption | Correction |
|------------|------------|
| A-1: all saved CI findings still need edits | Current source already carries the seven hub lint rewrites. Only policy literals and the unused helper were changed. This does not establish a clean lint run. |

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Tagged helper use | Establish whether each unused helper has callers in another build flavor before removal (R-1). |
| Policy and parity | Config words retain their values, reuse existing declarations, and hub parity tests retain every surface case. |
| Hub evidence | Both scopes are lint-clean, package tests use their required tags, and annotations state reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Hub and policy scopes cleared | Record zero scoped lint findings and tagged package-test results (AC-1–2). |
| Preserved tagged coverage | Trace unused-helper callers across flavors and inspect parity cases and annotation reasons (R-1, AC-3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Command exposure | Hub test rewrites retain every surface case and SSH-absent assertion. |
| Policy selection | Constants preserve config words and chain-source values rather than changing which policy applies. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "./cmd/ze/... ./internal/component/policy/..."` zero; `./le job run label hub command go test -race -tags "$tags" ./cmd/ze/ ./cmd/ze/hub/ ./internal/component/policy/` with every feature tag (`docs/contributing/running-commands.md` recipe), and `./cmd/ze/hub/` again with that set minus `ze_ssh` for the SSH-absent tests. These are the scoped subset of `./le test unit all`; `./le test weakened check` is clean for the scope. Full-run evidence belongs to the umbrella's CI run.

## Implementation Summary

### What Was Implemented
- `ea96c574f2`: `internal/component/policy/chain.go` declares `chainFilterContainer`, `chainFiltersLeaf`, `chainFinalLeaf`, `chainFinalAccept` and `chainFinalReject`. `ExtractChain` and `ParseChainFinal` (`chain.go`), `ExportChainPath` and `CompileExportChain` (`export_chain.go`), `chainLeafAt` and `ribMemberCheck` (`reference_source.go`), `readRIBChain` (`rib_compile.go`) and `MustRegisterRouteChainPath` (`route_chain_path.go`) read them. Every value is the literal it replaced; `ribFinalAccept` folds into `chainFinalAccept`.
- `e20b2ee5c8`: the uncalled `wireToPathsForTest` leaves `cmd/ze/schema_helpers_test.go` (`Weakening-approved: ze.wireToPathsForTest`). The live helper and its caller in the `cmdutil` test package stay.
- The seven hub findings were already fixed in source: `_ = cmd.Process.Kill()` and `_ = logFile.Close()` in the cleanup of `build_tag_ssh_absent_test.go`, indexed loops in `matchMandatoryArgs`, `registerStage1Commands` and `collectCommandRegistrations`, `strings.Cut` in `bashRootNames`, and `matchValue` naming all five `command.ArgKind` members.

- Closure: the three findings `7ff7614e5f` added to `cmd/ze/hub` tests after step 2 (Review Gate, finding 1), in `infra_hook_owner_test.go` and `main_system_nonappliance_test.go`.

### Bugs Found/Fixed
- None: lint findings only, no product defect.

### Documentation Updates
- None. `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-5-hub-and-policy.md` exits 0.

### Deviations from Plan
- Step 2 found the hub findings already fixed (A-1 broken), so only the policy literals and the helper changed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: every saved CI finding still needed an edit | The seven hub findings were already rewritten in source | Step 2 source audit | Recorded in Deviations; the fresh scoped lint is the evidence |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Hub errcheck x2 | Done | `cmd/ze/hub/build_tag_ssh_absent_test.go`, the daemon cleanup | Already in source |
| Hub gocritic and exhaustive (local bridge) | Done | `cmd/ze/hub/local_bridge_match_test.go` `matchMandatoryArgs`, `matchValue` | Already in source |
| Hub modernize and gocritic x2 (surface parity) | Done | `cmd/ze/hub/surface_parity_test.go` `bashRootNames`, `registerStage1Commands`, `collectCommandRegistrations` | Already in source |
| `cmd/ze` unused helper | Done | `e20b2ee5c8` | Judge-approved removal |
| Policy goconst x10 | Done | `ea96c574f2` | Shared chain tokens |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Scoped lint, Pre-Commit Verification | Main thread run |
| AC-2 | Done | Tagged package tests, Pre-Commit Verification | Main thread run |
| AC-3 | Done | `ea96c574f2` and `e20b2ee5c8` add no `//nolint` | `git show` of both |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Existing package tests | Done | `cmd/ze`, `cmd/ze/hub`, `internal/component/policy` | No test added: the fixes keep behavior |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Task-table files | Done | Policy five in `ea96c574f2`, helper in `e20b2ee5c8`, hub seven unchanged |

### Audit Summary
- **Total items:** 5 requirement groups, 3 ACs
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (hub findings already fixed; Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Clear the 18 CI findings in `cmd/ze`, `cmd/ze/hub` and `internal/component/policy` | scoped lint | `./le go lint run scope "./cmd/ze/... ./internal/component/policy/..."`: main thread 2026-11-10, 0 issues in host, Linux integration, darwin, capability, distro, appliance, personalities, tinygo, setup-standalone and compile-out (`job-lint5d-6d1ecc05.log`). One flavor's single typecheck error, `undefined: fixture10OperatorEnvironment` in `internal/test/fixture/plugin_fixture_10_process.go`, was another session's uncommitted fixture edit caught mid-write: foreign and transient, outside the scope |
| Keep policy selection and hub surface proof unchanged | tagged package tests | `TestLocalCommandDaemonMatchesLocal`, `TestEveryCommandOnEverySurface`, `TestExtractChain`, `TestRIBImportCompilesAndJudges`, `TestRIBImportRefusals`: main thread 2026-11-10, `go test -race -count=1` with every ze tag over `./cmd/ze/` and `./cmd/ze/hub/` exit 0 after the closure hub edits (`job-lint5-ze-a9dc106d.log`); `./internal/component/policy/` ok (`job-lint5-tests-501a8ea3.log`, unchanged since); SSH-absent hub (every tag but `ze_ssh`) ok (`job-lint5-nossh-20c3c357.log`); `./le repo compiles check` exit 0 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every Task row is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lint-backlog-5-hub-and-policy-49da3d3d-44f5-443f-bc7a-73b9535a17f9.md` (the five policy files of `ea96c574f2`, `cmd/ze/schema_helpers_test.go`, and the two hub test files of finding 1) |
| `./le spec review check` | clean: 8 code files, hashes match |
| Rounds | 2 |
| Reviewer lenses used | logic (each constant holds the literal it replaced, so every config read and error text is unchanged), test proof (helper removal judged; `./le test weakened check` clean), security (no new input path; the `panic` in `MustRegisterRouteChainPath` fires at init-time registration, not from a peer), style pass over the six changed Go files |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | Run 1: 0 BLOCKER, 0 ISSUE | - | - |
| 1 | ISSUE | The scoped lint (main thread 2026-11-10, log `tmp/session/2026-11-09-222501/scratch/job-lint5-6d1ecc05.log`, host, Linux integration and darwin alike) reported 3 findings that `7ff7614e5f` brought in after this spec's fixes: gocritic paramTypeCombine on `stopInfraHookOwner`, nilerr in `waitInfraHookStopSnapshot` (both `infra_hook_owner_test.go`), and unused `warnLines` (`main_system_hostname_fake_test.go`) | `cmd/ze/hub` | `start, cleanupEnded <-chan struct{}` merged. The nilerr site was correct: `Server.OnStop` refuses only once Stop has started (`stopHooks.add` answers `errServerStopped`), and that refusal is the event the test waits for, so the probe moves into `infraHookStopRefused`, which answers it as a bool with the reason in its comment; same calls, same order, same withdrawal. `warnLines` had one caller, the `ze_appliance` test, so the untagged fake file left it unused in every other flavor: `TestHostnameApplierNonApplianceNeverSets` now also asserts the distro path logs no WARN line (`reconcileKernelHostname` returns before any log when `kernelHostnameOwned` is false), which adds a check and removes none |

### Run 2
Scope: the two edited hub test files. 0 BLOCKER, 0 ISSUE. `TestInfraHookJoinsRefusedSSHOwner` still waits on the real `OnStop` refusal and fails on the context error as before; the hostname test keeps its three kernel assertions and gains the no-warning one. `./le test weakened check` lists neither file (its findings are in `internal/le/test/weakened`, another session's). Style pass: each new helper has a doc comment stating its contract; no panic, no new allocation path.

NOTE 1: `"import"` stays a literal three times in policy (`reference_source.go`, `rib_compile.go`) beside `exportDirection`; CI did not list it and the scoped lint judges it. NOTE 2: the tokens mirror YANG leaf and enum names that the package reads as plain map keys; no runtime declaration exists to read them from. NOTE 3: `./le repo check` reports only foreign issues (`internal/le/weekly/events.go`, `internal/test/dhcpprobe/step.go`).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/policy/chain.go` | yes | `grep -n chainFinalAccept internal/component/policy/*.go` hits `chain.go`, `export_chain.go`, `rib_compile.go` and `reference_source.go` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Zero scoped findings | 0 issues across the full flavor matrix after the closure hub fixes (`tmp/session/2026-11-09-222501/scratch/job-lint5d-6d1ecc05.log`); the one foreign transient typecheck error is named in Goal Validation |
| AC-2 | Tagged tests pass | Race tests with every ze tag: `./cmd/ze/` and `./cmd/ze/hub/` ok (`job-lint5-ze-a9dc106d.log`), policy ok (`job-lint5-tests-501a8ea3.log`), SSH-absent hub ok (`job-lint5-nossh-20c3c357.log`), all logs under `tmp/session/2026-11-09-222501/scratch/` |
| AC-3 | No suppression added | `git show ea96c574f2 e20b2ee5c8` adds no `nolint` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le go lint run scope` | none (lint) | yes: scoped run over `./cmd/ze/...` and `./internal/component/policy/...`, 0 issues (`job-lint5d-6d1ecc05.log`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Hub sources already carry the rewrites (Implementation Audit rows 1-3) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 16: source anchors | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-5-hub-and-policy.md` exit 0 | yes |
