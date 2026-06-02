# Spec: Doctor Check Registry

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-06-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `ai/rules/planning.md` - spec lifecycle and completion rules.
3. `ai/rules/doctor-checks.md` - current doctor-check rule that this spec updates.
4. `ai/patterns/registration.md` - registry pattern and startup invariants.
5. `cmd/ze/doctor/doctor.go` - current centralized runner and first migrated check candidates.
6. `internal/core/diagnostic/codes.go` - existing diagnostic code metadata registry.

## Task

Move `ze doctor` check execution from a manually appended central list to an explicit doctor check registry. The first implementation slice must prove the pattern by migrating a small, non-risky check group while preserving the user-visible `ze doctor --json` contract.

The target outcome is that adding a new runtime dependency check does not require editing the central `runChecks` append list. New checks register metadata and a check function, the doctor runner executes registered checks in deterministic order, and tests mechanically tie emitted doctor diagnostic codes back to registered diagnostic metadata.

## Required Reading

### Architecture Docs and Rules

- [ ] `ai/rules/doctor-checks.md` - current rule for runtime dependency readiness checks.
  -> Decision: update this rule so future checks register with the doctor check registry instead of editing the central run list.
  -> Constraint: every new runtime dependency still needs unit and functional coverage through `ze doctor --json`.
- [ ] `ai/patterns/registration.md` - registration model used across Ze.
  -> Decision: use explicit registration metadata and runtime query of sorted registered checks.
  -> Constraint: registrations complete before concurrent access and registries are read-only during normal execution.
- [ ] `ai/rules/design-principles.md` - single responsibility, explicit behavior, minimal coupling.
  -> Decision: keep the registry boring and explicit. Do not build dynamic discovery, reflection, or convention-only metadata.
  -> Constraint: every check declares its phase, order, component, dependencies, platform constraints, and diagnostic codes.
- [ ] `ai/rules/wiring-completeness.md` - exported symbols and features must have real callers.
  -> Decision: the doctor runner must call the registry in production, not only in tests.
  -> Constraint: any exported registry API must have a non-test production caller.
- [ ] `docs/architecture/core-design.md` - core registration and component principles.
  -> Decision: preserve Ze's registration-first architecture and avoid hidden coupling through a central file.
  -> Constraint: the core remains ignorant of individual doctor checks; the doctor command owns readiness execution.
- [ ] `plan/learned/755-ze-doctor.md` - original doctor design history.
  -> Decision: preserve offline command behavior, severity model, platform split, and stable diagnostic code taxonomy.
  -> Constraint: JSON `ready` remains false when any error-severity diagnostic is emitted.
- [ ] `plan/learned/727-diag-core.md` - diagnostic command and platform split lessons.
  -> Decision: keep Linux-only code behind build tags with `_other.go` stubs where needed.
  -> Constraint: build-split files need explicit build tags.

### RFC Summaries

- [ ] Not applicable - this spec does not add or change wire protocol behavior.
  -> Constraint: no RFC summaries or protocol interop tests are required.

**Key insights:**
- `cmd/ze/doctor/doctor.go` currently mixes command orchestration, readiness probes, metadata-adjacent behavior, and direct sequencing.
- The registry should model doctor execution phases explicitly because the current runner has pre-config checks, config-missing behavior, parse-failure early return, and post-config checks.
- The safest first migrated group is plugin binary checks. They are post-config only, already have unit and functional coverage, and do not depend on platform-specific build tags.
- Listener checks are a good later migration but are broader because they aggregate schema-derived listeners, BGP, BFD, IPsec, TFTP, image-server, and NTP listeners.
- Diagnostic code metadata can remain in `internal/core/diagnostic/codes.go` for the first slice if a registry consistency test proves every registered doctor check code has metadata.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `gpt/05-doctor-check-registry.md` - handover goal, work list, and acceptance criteria.
- [ ] `cmd/ze/doctor/doctor.go` - `runChecks` loads storage and config, then appends every doctor check directly in a fixed sequence.
- [ ] `cmd/ze/doctor/register.go` - `init` registers the doctor command and `runChecks` as the diagnostic provider.
- [ ] `cmd/ze/doctor/checks_linux.go` - Linux-only readiness checks and test-only env overrides live behind `//go:build linux`.
- [ ] `cmd/ze/doctor/checks_other.go` - non-Linux stubs keep the doctor package buildable outside Linux.
- [ ] `cmd/ze/doctor/doctor_test.go` - current unit coverage includes plugin checks, listener checks, diagnostic metadata checks, and dependency inventory checks.
- [ ] `test/ui/doctor-plugin-external-builtin.ci` - functional coverage for an external plugin command resolving to a built-in plugin.
- [ ] `test/ui/doctor-plugin-internal-clean.ci` - functional coverage proving internal plugin declarations do not emit the advisory.
- [ ] `test/ui/doctor-listeners.ci` - functional coverage for service listener diagnostics, useful for later listener migration.
- [ ] `internal/core/diagnostic/codes.go` - diagnostic metadata is registered centrally through `RegisterBuiltinCodes`.

**Current execution behavior:**

| Area | Current behavior |
|------|------------------|
| Command entry | `ze doctor` is an offline local command registered by `cmd/ze/doctor/register.go`. |
| JSON contract | `Run` returns a `DoctorResult` with `schema-version`, `ready`, and `diagnostics`. |
| Ready calculation | `ready` is false if any diagnostic has error severity. |
| Pre-config checks | Storage, platform, store integrity, service install, machine ID, and random seed checks run before config load. |
| Missing config | `doctor-config-missing` is emitted, `checkKernelModules(nil)` runs, and the runner returns. |
| Parse failure | `doctor-config-parse` is emitted and the runner returns without post-config checks. |
| Post-config checks | `runChecks` directly appends semantic validation, interface, kernel, TLS, plugin, SSH, listener, network, procfs, netlink, update, archive, writable, and SMART checks. |
| Plugin checks | `checkPlugins` emits `doctor-plugin-missing` for missing external binaries and `doctor-plugin-external-builtin` for external declarations that resolve to built-ins. |
| Platform split | Linux-specific probes live in `checks_linux.go`; non-Linux stubs live in `checks_other.go`. |
| Code metadata | Doctor codes are defined in `internal/core/diagnostic/codes.go`; tests sample known codes but do not prove all doctor emitted codes are tied to check metadata. |

**Behavior to preserve:**

- Existing `ze doctor --json` shape and field names.
- Existing diagnostic code strings, severity choices, messages, paths, expected values, actual values, and related records for migrated checks unless a test explicitly documents a change.
- Existing exit-code behavior: zero when no error-severity diagnostic is emitted, non-zero when any error-severity diagnostic is emitted.
- Existing pre-config, config-missing, parse-failure, and post-config boundaries.
- Existing Linux vs non-Linux build behavior.
- Existing command registration and diagnostic provider wiring.
- Existing plugin check behavior covered by `TestCheckPlugins_*` and `test/ui/doctor-plugin-*.ci`.

**Behavior to change:**

- Doctor check execution for migrated checks becomes registry-driven instead of direct append-list-driven.
- Future checks declare metadata at registration time instead of being silently inserted into a central run list.
- The doctor rule and registration pattern docs explain the new registration path.
- Tests mechanically prove registry order and diagnostic-code metadata consistency.

## Data Flow (MANDATORY)

### Entry Point

- Entry point: `ze doctor [--json] [config-file]`.
- Input format: CLI arguments and optional Ze config file.
- Output format: existing text or `DoctorResult` JSON.

### Transformation Path

1. CLI command dispatch calls `doctor.Run`.
2. `Run` parses `--json` and optional config path.
3. `runChecks` resolves storage and platform data.
4. `runChecks` executes pre-config registered checks and any remaining legacy pre-config checks.
5. `runChecks` loads config or emits `doctor-config-missing`.
6. On config-missing, the runner executes only checks registered for the missing-config phase plus any remaining legacy missing-config check calls, then returns.
7. On parse failure, the runner emits `doctor-config-parse` and returns.
8. On parse success, the runner builds a doctor check context with the config tree, config directory, plugin list, storage handle, and platform info.
9. `runChecks` executes post-config registered checks in deterministic order and any remaining legacy post-config checks.
10. `Run` computes readiness from diagnostic severities and emits existing text or JSON output.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI to doctor package | Local command registry calls `Run` | [ ] Functional `ze doctor --json` test |
| Doctor runner to check registry | Runner queries registered checks by phase | [ ] Registry wiring unit test |
| Check registry to diagnostic metadata | Registered check codes are looked up with `diagnostic.Lookup` | [ ] Metadata consistency unit test |
| Config parser to doctor checks | `config.LoadConfig` result populates check context | [ ] Migrated plugin functional test |
| Platform-specific files to registry | Build-tagged files register only checks valid for that platform | [ ] Linux and non-Linux package tests or build coverage |

### Integration Points

- `cmd/ze/doctor/runChecks` - production runner must call the registry.
- `cmd/ze/doctor/register.go` - command registration remains unchanged.
- `cmd/ze/doctor/checkPlugins` - first migrated check group.
- `internal/core/diagnostic.Lookup` - consistency test for registered doctor check codes.
- `ai/rules/doctor-checks.md` - contributor rule must point to registration, not central appends.
- `ai/patterns/registration.md` - add doctor check registry to known registration mechanisms.

### Architectural Verification

- [ ] No bypassed layers: doctor checks are still reached only through `ze doctor`, support bundles, or diagnostic provider paths that call `runChecks`.
- [ ] No unintended coupling: internal components do not import `cmd/ze/doctor`; the first slice keeps check registration in the doctor package.
- [ ] No duplicated functionality: existing check functions move behind registration instead of being reimplemented.
- [ ] Output allocation behavior unchanged for migrated checks: no new per-diagnostic string formatting or reflection.

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ze doctor --json external-builtin.conf` | -> | registered post-config plugin check | `test/ui/doctor-plugin-external-builtin.ci` |
| `runChecks` post-config phase | -> | registered plugin check wrapper | `TestRunChecksExecutesRegisteredPluginCheck` |
| doctor registry metadata validation | -> | diagnostic metadata lookup for registered codes | `TestDoctorRegisteredCheckCodesHaveMetadata` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A doctor check is registered with a phase and order | The production runner executes it without adding a direct append call to `runChecks`. |
| AC-2 | Two checks are registered in unsorted order | The registry returns checks sorted by phase, order, then name. |
| AC-3 | A check registration reuses an existing check name | Registration fails in tests or panics through a `MustRegister` helper at init time. |
| AC-4 | A check registration declares the same diagnostic code twice in its metadata | Registration validation rejects the check. Shared codes across different checks remain allowed when intentional. |
| AC-5 | A registered check declares a `doctor-*` code not known to `diagnostic.Lookup` | A unit test fails and identifies the missing metadata. |
| AC-6 | `ze doctor --json` runs against an external plugin declaration whose command resolves to a built-in plugin | JSON output still contains `doctor-plugin-external-builtin` with warning severity. |
| AC-7 | `ze doctor --json` runs against an internal plugin declaration using a built-in plugin | JSON output still does not contain `doctor-plugin-external-builtin`. |
| AC-8 | A migrated check emits diagnostics | Existing JSON field names and result shape are unchanged. |
| AC-9 | Config is missing | Existing missing-config flow is preserved, including the special kernel-module check behavior. |
| AC-10 | Config parse fails | Existing parse-failure early return is preserved. |
| AC-11 | A future runtime dependency needs a doctor check | The documented path says to add a registered check and tests, not to edit the central `runChecks` list. |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorCheckRegistryOrdersByPhaseOrderName` | `cmd/ze/doctor/registry_test.go` | AC-2 deterministic ordering | planned |
| `TestDoctorCheckRegistryRejectsDuplicateName` | `cmd/ze/doctor/registry_test.go` | AC-3 duplicate check names | planned |
| `TestDoctorCheckRegistryRejectsDuplicateCodeWithinCheck` | `cmd/ze/doctor/registry_test.go` | AC-4 duplicate diagnostic code declaration in one check | planned |
| `TestDoctorRegisteredCheckCodesHaveMetadata` | `cmd/ze/doctor/registry_test.go` | AC-5 every registered `doctor-*` code has diagnostic metadata | planned |
| `TestRunChecksExecutesRegisteredPluginCheck` | `cmd/ze/doctor/doctor_test.go` | AC-1 runner reaches registered plugin check through production path | planned |
| Existing `TestCheckPlugins_*` tests | `cmd/ze/doctor/doctor_test.go` | AC-6 and AC-7 plugin behavior remains unchanged | existing, must still pass |
| Existing missing-config and parse tests | `cmd/ze/doctor/doctor_test.go` | AC-9 and AC-10 early returns remain unchanged | existing, must still pass |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Registry order | signed integer order values | highest registered order in test fixture | negative order accepted only if deliberately used in fixture | very large order accepted and sorted |
| Phase enum | closed set of doctor phases | all defined phases | unknown phase rejected by registration validation | unknown phase rejected by registration validation |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-plugin-external-builtin` | `test/ui/doctor-plugin-external-builtin.ci` | `ze doctor --json` reports migrated plugin advisory through user entry point | existing, must still pass |
| `doctor-plugin-internal-clean` | `test/ui/doctor-plugin-internal-clean.ci` | `ze doctor --json` does not warn for proper internal plugin declaration | existing, must still pass |
| Optional JSON shape guard | `test/ui/doctor-plugin-external-builtin.ci` or new UI test | Migrated check appears in existing JSON envelope | add only if existing expectations do not prove AC-8 |

### Interop Tests (MANDATORY for protocol features)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not applicable | - | - | No protocol or wire behavior changes | N/A |

### Future (if deferring any tests)

- No tests are deferred by this spec. Listener migration is intentionally outside the first implementation slice and will need its own tests when selected.

## Files to Modify

- `cmd/ze/doctor/doctor.go` - replace the migrated direct append call with registry execution while preserving all non-migrated calls.
- `cmd/ze/doctor/doctor_test.go` - add or adapt runner tests and preserve existing plugin tests.
- `cmd/ze/doctor/register.go` - no behavior change expected; verify provider still points at `runChecks`.
- `ai/rules/doctor-checks.md` - change contributor guidance from central append list to registry registration.
- `ai/patterns/registration.md` - document doctor check registry as a Ze registration mechanism.
- `ai/CODE-TO-DOCS.md` or docs anchored to `cmd/ze/doctor/` - update if source mapping changes after splitting files.
- `docs/features.md` and `docs/guide/health-checks.md` - update only if the implementation changes documented behavior or source anchors become stale.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | `ze doctor` command syntax is unchanged |
| CLI grammar (action before identifier) | No | No command grammar change |
| Editor autocomplete | No | No YANG or CLI completion change |
| Functional test for new RPC/API | Yes | Existing `test/ui/doctor-plugin-external-builtin.ci` covers migrated user entry point |
| Pipe completeness | No | `ze doctor` output behavior is unchanged and is not a pipe-producing CLI command change |
| Env var registration | No | No new environment config leaf |
| Doctor check for runtime dependencies | No | This spec changes doctor infrastructure and introduces no new runtime dependency |
| Prometheus counters/metrics | No | No observable runtime state added |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | No | Behavior is internal infrastructure for existing `ze doctor` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Possibly | `docs/guide/health-checks.md` only if source anchors or behavior claims change |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | Functional test format unchanged |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `ai/patterns/registration.md`, `ai/rules/doctor-checks.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | Doctor check registry must be documented in `ai/patterns/registration.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | Check `docs/`, `ai/CODE-TO-DOCS.md`, and update stale `cmd/ze/doctor/` mappings |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Confirm `docs/guide/health-checks.md` examples remain valid |

## Files to Create

- `cmd/ze/doctor/registry.go` - doctor check registry types, registration validation, deterministic listing, and phase filtering.
- `cmd/ze/doctor/registry_test.go` - registry ordering, duplicate-name, duplicate-code, and metadata consistency tests.
- `cmd/ze/doctor/check_plugins.go` - plugin check registration wrapper if splitting the check out of `doctor.go` keeps file responsibility clearer.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior, Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Review gate | Review Gate section |
| 6. Full verification | Targeted unit and functional tests, then `make ze-lint-changed` per Go edit rules |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | Fix every issue from review |
| 9. Re-verify | Re-run targeted tests and required lint gate |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Present summary | Executive summary per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring first**
   - Tests: `TestRunChecksExecutesRegisteredPluginCheck`, `test/ui/doctor-plugin-external-builtin.ci`.
   - Files: `cmd/ze/doctor/registry.go`, `cmd/ze/doctor/doctor.go`, `cmd/ze/doctor/registry_test.go`.
   - Verify: production `runChecks` calls the registry for at least one phase and the migrated plugin functional path remains reachable.

2. **Phase: Registry validation**
   - Tests: `TestDoctorCheckRegistryOrdersByPhaseOrderName`, `TestDoctorCheckRegistryRejectsDuplicateName`, `TestDoctorCheckRegistryRejectsDuplicateCodeWithinCheck`.
   - Files: `cmd/ze/doctor/registry.go`, `cmd/ze/doctor/registry_test.go`.
   - Verify: tests fail before validation exists, then pass after registry validation is implemented.

3. **Phase: Plugin check migration**
   - Tests: existing `TestCheckPlugins_*`, `TestRunChecksExecutesRegisteredPluginCheck`, `doctor-plugin-external-builtin`, `doctor-plugin-internal-clean`.
   - Files: `cmd/ze/doctor/doctor.go`, optional `cmd/ze/doctor/check_plugins.go`.
   - Verify: `runChecks` no longer directly appends `checkPlugins(result.Plugins)`, but the same diagnostics appear through the registered check.

4. **Phase: Diagnostic metadata consistency**
   - Tests: `TestDoctorRegisteredCheckCodesHaveMetadata`.
   - Files: `cmd/ze/doctor/registry_test.go`, possibly `internal/core/diagnostic/codes.go` if a missing code is found.
   - Verify: every registered `doctor-*` check code resolves through `diagnostic.Lookup` after `RegisterBuiltinCodes`.

5. **Phase: Documentation and rules**
   - Tests: doc validation target required by the changed docs.
   - Files: `ai/rules/doctor-checks.md`, `ai/patterns/registration.md`, any stale source-anchor docs.
   - Verify: future agents can discover the doctor registry path through the rule and registration pattern.

6. **Full verification**
   - Run the targeted doctor package tests and the changed UI functional tests.
   - Run `make ze-lint-changed` before claiming done because Go files change.
   - Run broader verification required by the implementation context if touched files trigger changed-file gates.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a test or documented source evidence. |
| Correctness | Registry phases preserve pre-config, missing-config, parse-failure, and post-config behavior. |
| Ordering | Registry order is deterministic and matches the pre-migration sequence for migrated checks. |
| Naming | Check names, components, and diagnostic codes use lower-kebab-case. |
| Data flow | `runChecks` remains the only doctor execution owner; registry does not bypass config load or severity handling. |
| Platform split | Linux-only checks stay behind build tags or explicit platform predicates. |
| Diagnostic metadata | Registered check codes are explainable through `diagnostic.Lookup`. |
| Rule: no-layering | Do not make internal components import `cmd/ze/doctor`; if a reusable registry package is created, justify the boundary. |
| Rule: no-sprintf-alloc | Do not add `fmt.Sprintf` or reflection for metadata or diagnostics. |
| Rule: no partial completion | Do not claim the registry conversion complete if only a private test fixture uses it. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Doctor check registry exists | Read `cmd/ze/doctor/registry.go` and confirm it is called from production `runChecks`. |
| Plugin check migrated | Read `cmd/ze/doctor/doctor.go` and confirm the direct plugin append was removed while registered execution remains. |
| Deterministic ordering covered | Run or inspect `TestDoctorCheckRegistryOrdersByPhaseOrderName`. |
| Duplicate validation covered | Run or inspect duplicate-name and duplicate-code tests. |
| Diagnostic metadata consistency covered | Run or inspect `TestDoctorRegisteredCheckCodesHaveMetadata`. |
| Functional plugin coverage preserved | Run `test/ui/doctor-plugin-external-builtin.ci` and `test/ui/doctor-plugin-internal-clean.ci` through the project functional test runner. |
| Rule docs updated | Read `ai/rules/doctor-checks.md` and confirm it references registration, not central append editing. |
| Registration docs updated | Read `ai/patterns/registration.md` and confirm doctor check registry is listed. |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Untrusted config data | Registry must not parse or execute extra user input; checks receive the same parsed config data as before. |
| External binary checks | Migrated plugin check must preserve existing `os.Stat` and `exec.LookPath` behavior without executing plugin commands. |
| Error leakage | Diagnostic messages must not add secrets or raw config values beyond existing behavior. |
| Resource exhaustion | Registry execution must not spawn goroutines or run checks multiple times per phase. |
| Platform probes | Platform-specific probes stay behind existing build tags and test env overrides. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Registry test fails due ordering mismatch | Re-read current `runChecks` ordering and fix registration order metadata. |
| Functional doctor plugin test loses diagnostic | Check whether plugin check context carries `result.Plugins`. |
| Missing metadata consistency failure | Add or fix `internal/core/diagnostic/codes.go` metadata unless the registered code is wrong. |
| Parse or missing-config tests change | Back out phase wiring and preserve existing early return behavior first. |
| Lint failure from new exported symbols | Either make symbols unexported or prove non-test production callers. |
| Documentation gate failure | Update source anchors and registration rule docs before claiming completion. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- A single generic check function can receive a doctor execution context, avoiding many one-off adapter signatures while preserving explicit metadata.
- Registry phases are mandatory. A single flat list would risk running post-config checks after config load failures or changing parse-failure behavior.
- Duplicate diagnostic codes cannot be globally forbidden across checks because shared codes such as generic listener failures are legitimate. Duplicate detection should reject duplicates inside one check declaration and rely on diagnostic metadata registration for global code metadata uniqueness.
- The first implementation slice should keep diagnostic code metadata in `internal/core/diagnostic/codes.go` and add a consistency test. Moving metadata into doctor check registration can be a later spec if this first registry proves stable.

## Core Insight

The useful cutover is not moving all checks at once. The useful cutover is making the production runner execute at least one real check through a registry with enough metadata and validation that every subsequent runtime dependency can follow the same path without editing the central append list.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| Use explicit phases | One flat ordered registry | Phases preserve missing-config and parse-failure early returns. |
| Migrate plugin checks first | Listener checks first, Linux checks first | Plugin checks are post-config only, platform-neutral, and already have functional tests. |
| Keep diagnostic metadata central for first slice | Move doctor code metadata into the check registry immediately | Central metadata is already wired to `ze explain`; consistency tests reduce risk without a larger diagnostic architecture change. |
| Allow shared codes across checks | Globally reject duplicate declared diagnostic codes | Some code families are intentionally shared by multiple checks, especially listener checks. |
| Keep first registry in doctor package | Put registry in a new internal package immediately | The doctor command owns readiness execution; avoiding cross-package imports reduces initial coupling. Revisit only if future specs need component-owned registration. |

## Known Limitations

- This spec migrates only one real check group first. The registry is not proven complete for every platform-specific check until later checks are migrated.
- This spec does not make internal components register their own doctor checks from component packages. It removes the central run-list edit requirement, not the doctor package ownership model.
- This spec does not change the diagnostic metadata source of truth. It adds a consistency test rather than merging doctor check metadata and explain metadata.
- The spec is in `design` status and needs user approval before implementation begins.

## RFC Documentation

No RFC comments are required. This work does not implement protocol behavior.

## Implementation Summary

### What Was Implemented

- Added an unexported doctor check registry in `cmd/ze/doctor/registry.go` with explicit phase, order, component, dependency, platform, diagnostic-code, and check-function metadata.
- Wired `runChecks` to execute registered checks in pre-config, missing-config, and post-config phases while preserving missing-config and parse-failure early returns.
- Migrated the plugin binary check group through `cmd/ze/doctor/check_plugins.go`; the direct `checkPlugins(result.Plugins)` append was removed from `runChecks`.
- Added registry ordering, duplicate-name, duplicate-code, unknown-phase, diagnostic metadata, and production runner wiring tests.
- Updated future-agent guidance in `ai/rules/doctor-checks.md`, `ai/patterns/registration.md`, and `ai/CODE-TO-DOCS.md`.
- Excluded the root build-tag-only package from `ZE_PACKAGES` so full verification skips `tools.go`.

### Bugs Found/Fixed

- `make ze-lint-changed` flagged `registry.go` byte validation with `intrange`; fixed by using `for i := range len(value)`.
- Full `make ze-verify` initially failed because `tmp/go.mod` was missing, so `go list ./...` entered `tmp/qemu/gomodcache`; restored the local sentinel during verification, then left the tracked sentinel content unchanged.
- Full `make ze-verify` also exposed that the root package only contains build-tagged `tools.go`; `ZE_PACKAGES` now excludes the root module package before running unit tests.
- Full `make ze-verify` hit one plugin functional timeout on test 213; focused rerun `bin/ze-test bgp plugin 213` passed, and the final full verify passed.

### Documentation Updates

- `ai/rules/doctor-checks.md`: future runtime dependency checks must register doctor checks instead of appending central runner calls.
- `ai/patterns/registration.md`: doctor check registry added to Ze registration mechanisms.
- `ai/CODE-TO-DOCS.md`: new doctor registry and plugin registration files mapped to `docs/features.md`.
- `docs/guide/health-checks.md` and `docs/features.md`: behavior claims remain valid because `ze doctor` CLI syntax, JSON shape, diagnostic codes, and user-visible checks are unchanged.

### Deviations from Plan

- Added the `Makefile` root package exclusion because full verification could not pass with the build-tag-only root package selected for unit tests.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Introduce doctor check registry | Implemented | `cmd/ze/doctor/registry.go` | Registry validates metadata and sorts checks deterministically. |
| Migrate small non-risky check group | Implemented | `cmd/ze/doctor/check_plugins.go` | Plugin binary checks register as `plugin-binaries`. |
| Preserve deterministic ordering | Implemented | `cmd/ze/doctor/registry_test.go` | Phase, order, and name ordering covered, including negative and max int order values. |
| Keep platform-specific checks constrained | Implemented | `cmd/ze/doctor/registry.go` | Platform metadata is required and enforced; no new Linux code was added. |
| Tie check codes to diagnostic metadata | Implemented | `TestDoctorRegisteredCheckCodesHaveMetadata` | Registered codes must resolve via `diagnostic.Lookup`. |
| Preserve doctor JSON behavior | Implemented | `test/ui/doctor-plugin-*.ci` | Final functional run passed both plugin doctor tests. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Implemented | `TestRunChecksExecutesRegisteredPluginCheck`; `cmd/ze/doctor/doctor.go:181` | Production runner executes registered post-config checks. |
| AC-2 | Implemented | `TestDoctorCheckRegistryOrdersByPhaseOrderName` | Unsorted registrations return sorted by phase, order, then name. |
| AC-3 | Implemented | `TestDoctorCheckRegistryRejectsDuplicateName` | Duplicate check names are rejected. |
| AC-4 | Implemented | `TestDoctorCheckRegistryRejectsDuplicateCodeWithinCheck` | Duplicate codes within one check are rejected; shared fixture code across checks remains accepted. |
| AC-5 | Implemented | `TestDoctorRegisteredCheckCodesHaveMetadata` | Registered `doctor-*` codes must have diagnostic metadata. |
| AC-6 | Implemented | `bin/ze-test ui --pattern doctor-plugin` | `doctor-plugin-external-builtin` still appears with warning severity. |
| AC-7 | Implemented | `bin/ze-test ui --pattern doctor-plugin` | Internal built-in plugin config remains clean. |
| AC-8 | Implemented | `doctor-plugin-external-builtin.ci`; `doctor-plugin-internal-clean.ci` | Existing JSON envelope and fields remain exercised through `ze doctor --json`. |
| AC-9 | Implemented | `TestDoctorMissingConfig`; `go test -race ./cmd/ze/doctor` | Missing config still emits `doctor-config-missing` and returns early. |
| AC-10 | Implemented | `TestDoctorInvalidConfig`; `go test -race ./cmd/ze/doctor` | Parse failure still emits `doctor-config-parse` and returns early. |
| AC-11 | Implemented | `ai/rules/doctor-checks.md`; `ai/patterns/registration.md` | Future check path says to register checks, not edit the central append list. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDoctorCheckRegistryOrdersByPhaseOrderName` | Added and passed | `cmd/ze/doctor/registry_test.go` | AC-2 and order boundaries. |
| `TestDoctorCheckRegistryRejectsDuplicateName` | Added and passed | `cmd/ze/doctor/registry_test.go` | AC-3. |
| `TestDoctorCheckRegistryRejectsDuplicateCodeWithinCheck` | Added and passed | `cmd/ze/doctor/registry_test.go` | AC-4. |
| `TestDoctorCheckRegistryRejectsUnknownPhase` | Added and passed | `cmd/ze/doctor/registry_test.go` | Closed phase boundary. |
| `TestDoctorRegisteredCheckCodesHaveMetadata` | Added and passed | `cmd/ze/doctor/registry_test.go` | AC-5. |
| `TestRunChecksExecutesRegisteredPluginCheck` | Added and passed | `cmd/ze/doctor/doctor_test.go` | AC-1 production runner wiring. |
| `doctor-plugin-external-builtin` | Existing and passed | `test/ui/doctor-plugin-external-builtin.ci` | AC-6 and AC-8. |
| `doctor-plugin-internal-clean` | Existing and passed | `test/ui/doctor-plugin-internal-clean.ci` | AC-7 and AC-8. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/doctor/registry.go` | Created | Registry types, validation, sorting, phase filtering, platform gating. |
| `cmd/ze/doctor/registry_test.go` | Created | Ordering, duplicate validation, metadata consistency tests. |
| `cmd/ze/doctor/check_plugins.go` | Created | Plugin binary check registration wrapper. |
| `cmd/ze/doctor/doctor.go` | Modified | Runner calls registered phase hooks and no longer appends `checkPlugins(result.Plugins)`. |
| `cmd/ze/doctor/doctor_test.go` | Modified | Added production runner wiring test. |
| `ai/rules/doctor-checks.md` | Modified | Contributor rule now points to registry registration. |
| `ai/patterns/registration.md` | Modified | Doctor check registry documented as a registration mechanism. |
| `ai/CODE-TO-DOCS.md` | Modified | Source mapping updated for new doctor files. |
| `Makefile` | Modified | Full verification package list excludes build-tag-only root package. |
| `tmp/go.mod` | Unchanged | Accidental sentinel-content drift was reverted to match `scripts/evidence/qemu-run.py`; it is no longer part of Commit A. |

### Audit Summary

- **Total items:** 21.
- **Implemented:** 21.
- **Partial:** 0.
- **Skipped:** 0.
- **Changed:** 10 files plus learned summary, counter, and learned index.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| New checks do not edit the central run list | Source review and unit test | `cmd/ze/doctor/doctor.go:181` calls `runDoctorChecks`; search found no `checkPlugins(result.Plugins)` append; `TestRunChecksExecutesRegisteredPluginCheck` passed. |
| Registered checks execute through user entry point | Functional test | `bin/ze-test ui --pattern doctor-plugin` passed 2/2, covering `doctor-plugin-external-builtin` and `doctor-plugin-internal-clean`. |
| Diagnostic metadata is complete | Unit test | `TestDoctorRegisteredCheckCodesHaveMetadata` passed in `go test -race ./cmd/ze/doctor` and final `make ze-verify`. |
| JSON behavior preserved | Functional test | Existing doctor plugin UI tests passed through `ze doctor --json`; JSON expectations still assert `diagnostics`, warning severity, and advisory presence or absence. |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NONE | Reviewer agent reported no BLOCKER, ISSUE, or NOTE findings. | Requested files | No action required. |
| 2 | BLOCKER | `registeredDoctorChecks` had no production caller and was only used by tests. | `cmd/ze/doctor/registry.go:70` | Removed the production helper and changed the metadata test to read `defaultDoctorCheckRegistry.checks()` directly; no feature code removed. |

### Fixes applied

- Removed test-only `registeredDoctorChecks` from production code; the registered doctor checks remain wired through `runDoctorChecks` from `runChecks`.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NONE | Fresh reviewer pass after removing `registeredDoctorChecks` found no BLOCKER, ISSUE, or NOTE findings. | Current patch | No action required. |

### Final status

- [x] Review gate shows 0 BLOCKER and 0 ISSUE.
- [x] All NOTEs recorded above or explicitly none.

## Pre-Commit Verification

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/doctor/registry.go` | Yes | `read` observed `¶cmd/ze/doctor/registry.go#713E`. |
| `cmd/ze/doctor/registry_test.go` | Yes | `read` observed `¶cmd/ze/doctor/registry_test.go#8FF9`. |
| `cmd/ze/doctor/check_plugins.go` | Yes | `read` observed `¶cmd/ze/doctor/check_plugins.go#31CB`. |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Registered check executes from production runner | `TestRunChecksExecutesRegisteredPluginCheck` passed; `doctor.go:181` calls `runDoctorChecks(doctorCheckPhasePostConfig, checkCtx)`. |
| AC-2 | Deterministic ordering | `TestDoctorCheckRegistryOrdersByPhaseOrderName` passed. |
| AC-3 | Duplicate check names rejected | `TestDoctorCheckRegistryRejectsDuplicateName` passed. |
| AC-4 | Duplicate codes within a check rejected | `TestDoctorCheckRegistryRejectsDuplicateCodeWithinCheck` passed. |
| AC-5 | Registered doctor codes have metadata | `TestDoctorRegisteredCheckCodesHaveMetadata` passed. |
| AC-6 | External built-in plugin advisory preserved | `bin/ze-test ui --pattern doctor-plugin` PASS 106 `doctor-plugin-external-builtin`. |
| AC-7 | Internal plugin remains clean | `bin/ze-test ui --pattern doctor-plugin` PASS 107 `doctor-plugin-internal-clean`. |
| AC-8 | JSON shape preserved | Functional tests still assert `diagnostics`, warning severity, and diagnostic code through `ze doctor --json`. |
| AC-9 | Missing-config flow preserved | `TestDoctorMissingConfig` passed in `go test -race ./cmd/ze/doctor`. |
| AC-10 | Parse-failure flow preserved | `TestDoctorInvalidConfig` passed in `go test -race ./cmd/ze/doctor`. |
| AC-11 | Future check rule updated | `ai/rules/doctor-checks.md` and `ai/patterns/registration.md` now reference doctor check registration. |

### Wiring Verified (end-to-end)

| Entry Point | Functional Test | Verified |
|-------------|-----------------|----------|
| `ze doctor --json external-builtin.conf` | `test/ui/doctor-plugin-external-builtin.ci` | PASS in `bin/ze-test ui --pattern doctor-plugin`. |
| `runChecks` post-config phase | `TestRunChecksExecutesRegisteredPluginCheck` | PASS in `go test -race ./cmd/ze/doctor`. |
| doctor registry metadata validation | `TestDoctorRegisteredCheckCodesHaveMetadata` | PASS in `go test -race ./cmd/ze/doctor`. |
| stale production helper removal | Search and review | `registeredDoctorChecks` has no remaining matches; clean reviewer pass confirmed no feature code was removed. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Doctor check rule references registry | `ai/rules/doctor-checks.md` | Yes, rule now requires registered checks and forbids new central append calls. |
| Registration pattern lists doctor registry | `ai/patterns/registration.md` | Yes, Doctor Check Registry section added. |
| Source map includes new doctor files | `ai/CODE-TO-DOCS.md` | Yes, `check_plugins.go` and `registry.go` map to `docs/features.md`. |
| User guide examples remain valid | `docs/guide/health-checks.md` | Yes, CLI syntax and JSON contract unchanged. |

## Checklist

### Goal Gates (MUST pass)

- [x] AC-1..AC-11 all demonstrated.
- [x] Wiring Test table complete and every row has a concrete test name.
- [x] Review gate clean with 0 BLOCKER and 0 ISSUE.
- [x] Targeted doctor package tests pass.
- [x] Changed UI functional doctor tests pass.
- [x] `make ze-lint-changed` passes after Go edits.
- [x] Feature code integrated in production runner.
- [x] Documentation Update Checklist answered with source evidence.
- [x] Rule and registration docs updated.
- [x] Critical Review passes.

### Quality Gates (SHOULD pass, defer only with user approval)

- [x] Implementation Audit complete.
- [x] Mistake Log escalation reviewed.
- [x] No unneeded exported symbols.

### Design

- [x] No premature abstraction beyond registry fields required by this spec.
- [x] No speculative dynamic discovery.
- [x] Single responsibility per new file.
- [x] Explicit registration metadata.
- [x] Minimal coupling to doctor package.

### TDD

- [x] Tests written.
- [x] Tests failed before implementation with undefined registry symbols.
- [x] Tests pass after implementation.
- [x] Boundary tests for phase and order validation.
- [x] Functional tests for end-to-end behavior.
- [x] Interop tests marked N/A with justification.
- [x] Goal Validation table filled with concrete evidence.

### Completion (BLOCKING before any commit)

- [x] Critical Review passes.
- [x] Partial or skipped items have user approval because there are none.
- [x] Implementation Summary filled.
- [x] Implementation Audit filled.
- [x] Learned summary written to `plan/learned/837-doctor-check-registry.md`.
- [x] Commit A script includes code, tests, docs, spec, learned summary, and counter bump.
- [x] Commit B script removes this spec only after Commit A preserves final spec state.