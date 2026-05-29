# Spec: Unified Update Backend

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/748-cpe-6-self-update.md` - self-update design history
4. `plan/learned/714-cpe-5-firmware-update.md` - update checker design history
5. `plan/learned/580-gokrazy-0-umbrella.md` - gokrazy platform design history
6. `internal/component/config/system/update.go` - passive update checker
7. `internal/component/config/system/selfupdate.go` - self-update logic
8. `internal/component/cmd/show/update.go` - show system update handler
9. `internal/component/cmd/update/firmware.go` - firmware command handlers
10. `cmd/ze/hub/main_system.go` - hub backend selection
11. `internal/component/host/platform_linux.go` - platform detection
12. `internal/component/gokrazy/gokrazy.go` - gokrazy web proxy

## Task

Replace the two global singletons (`UpdateChecker`, `SelfUpdater`) with a single
update backend interface that carries explicit backend identity (`ze-self-update`
or `gokrazy-ab`). On gokrazy, surface "updates handled by gokrazy" clearly and
probe gokrazy management endpoints for reachability. On normal Linux, wrap the
existing Ze self-update code with no behavior change.

This is a small-to-medium refactor: the existing update code stays intact, wrapped
by a new abstraction layer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall component isolation
  -> Constraint: components are independent; the update backend must not import gokrazy internals
- [ ] `ai/patterns/registration.md` - registration pattern for new backends
  -> Constraint: backends register via init(); hub selects the active one

### Learned Summaries
- [ ] `plan/learned/748-cpe-6-self-update.md` - SelfUpdater design decisions
  -> Decision: SelfUpdater is separate from UpdateChecker because the state machine is different
  -> Decision: hub selects between them based on config (auto-apply/restart triggers SelfUpdater)
- [ ] `plan/learned/714-cpe-5-firmware-update.md` - UpdateChecker design decisions
  -> Decision: implemented as system config extension, uses report bus
  -> Decision: version comparison is lexicographic (date-based versioning)
- [ ] `plan/learned/580-gokrazy-0-umbrella.md` - gokrazy platform decisions
  -> Constraint: ze is a self-contained gokrazy appliance; gokrazy manages the image

**Key insights:**
- Two globals today: `activeChecker *UpdateChecker` and `activeSelfUpdater *SelfUpdater`, each with their own mutex.
- Hub in `main_system.go:240` selects between them based on config (auto-apply/restart -> SelfUpdater, else UpdateChecker).
- `show system update` calls `ActiveExtendedUpdateStatus()` which falls back from SelfUpdater to UpdateChecker. No backend field in output.
- All firmware commands (`update system firmware *`) go through `ActiveSelfUpdaterInstance()` (3 callers: `show/update.go:78`, `update/firmware.go:39`, `selfupdate.go:1130`) and return "not configured" when nil.
- Platform detection already exists: `host.DetectPlatform()` returns `PlatformInfo` with `Type` field; `PlatformGokrazy` is type 1.
- Gokrazy web proxy exists at `/gokrazy/` in `internal/component/gokrazy/gokrazy.go`, proxying to Unix socket `/run/gokrazy-http.sock`.
- No gokrazy endpoint reports "update available" directly. Useful read-only endpoints: `GET /update/features`, `GET /` with `Accept: application/json`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/update.go` (310L) - UpdateChecker: periodic version check, report bus, global `activeChecker`
  -> Constraint: `ValidateUpdateCheckURL` enforces HTTPS (HTTP only for localhost)
  -> Constraint: `isNewer` is lexicographic, guards against non-numeric prefixes
- [ ] `internal/component/config/system/selfupdate.go` (1155L) - SelfUpdater: download/verify/stage/restart, global `activeSelfUpdater`
  -> Constraint: SelfUpdater has ManualCheck, ManualDownload, ManualApply, ManualRestart, Rollback
  -> Constraint: `ActiveExtendedUpdateStatus()` falls back to UpdateChecker if no SelfUpdater
- [ ] `internal/component/cmd/show/update.go` (101L) - show system update handler, builds map from `ActiveExtendedUpdateStatus()`
  -> Constraint: status field derived from download-status cascade; no backend field
- [ ] `internal/component/cmd/update/firmware.go` (143L) - firmware subcommands, all call `ActiveSelfUpdaterInstance()`
  -> Constraint: returns `"update checker not configured"` when SelfUpdater is nil
- [ ] `cmd/ze/hub/main_system.go:232-280` - `updateStopper` interface, `startUpdateChecker` selects backend
  -> Constraint: interface has `Stop()` and `Status()` methods
- [ ] `internal/component/host/platform_linux.go` (168L) - `DetectPlatform()` returns `PlatformInfo` with `Type`
  -> Constraint: `PlatformGokrazy` detected via socket, /perm+read-only-root, or /user/gokrazy
- [ ] `internal/component/gokrazy/gokrazy.go` (162L) - reverse proxy to gokrazy management socket
  -> Constraint: reads password from /perm/gokr-pw.txt, /etc/gokr-pw.txt, /gokr-pw.txt
- [ ] `cmd/ze/doctor/doctor.go:1932-1947` - self-update writable check
  -> Constraint: checks `auto-apply` config, warns if binary parent not writable
- [ ] `internal/component/config/system/schema/ze-system-conf.yang:325-383` - update-check YANG
  -> Constraint: description says "Firmware version check and self-update"
- [ ] `internal/component/cmd/update/schema/ze-cli-update-cmd.yang` - firmware CLI YANG
  -> Constraint: descriptions say "Firmware self-update operations"

**Behavior to preserve:**
- Ze self-update on normal Linux: no change to download/verify/stage/restart logic
- UpdateChecker on normal Linux: no change to periodic check logic
- YANG config structure (`system { update-check { ... } }`)
- CLI command paths (`update system firmware check/download/apply/restart/rollback`)
- `show system update` output fields (all existing fields preserved)
- Report bus integration (RaiseWarning/ClearWarning with source/code/subject)
- URL validation (HTTPS required, HTTP only for localhost)
- History persistence (ze-update-history.json)

**Behavior to change:**
- `show system update` output adds `backend` field with value `"ze-self-update"` or `"gokrazy-ab"`
- `show system update` on gokrazy returns status `"managed by gokrazy"` and message explaining external management
- Firmware commands on gokrazy return structured "unsupported: updates managed by gokrazy" instead of "not configured"
- Doctor check: skip writable-binary warning on gokrazy; warn if update-check config present on gokrazy (ignored)
- YANG descriptions updated from "Firmware self-update" to platform-neutral wording
- Two globals replaced by one `activeBackend` with backend identity

## Data Flow (MANDATORY)

### Entry Point
- Config reload: `startUpdateChecker(sc)` in hub, reads `system.update-check` config tree
- Platform detection: `host.DetectPlatform()` called at startup, result cached
- CLI commands: RPC dispatch to firmware handlers or show handler

### Transformation Path
1. Hub startup: `DetectPlatform()` -> `PlatformInfo.Type`
2. Hub config load: `ExtractUpdateCheckFromMap(tree)` -> `UpdateCheckConfig`
3. Backend selection: if `PlatformGokrazy` -> `gokrazyBackend`; else -> existing logic (UpdateChecker or SelfUpdater)
4. Backend wraps existing type (SelfUpdater/UpdateChecker) or gokrazy status probe
5. CLI handlers call backend interface methods instead of global singletons
6. `show system update` adds `backend` field from backend identity

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub -> Backend | Interface method calls | [ ] |
| Backend -> UpdateChecker/SelfUpdater | Delegation (wrapper) | [ ] |
| Backend -> Gokrazy | HTTP probe to Unix socket | [ ] |
| CLI -> Backend | Global `ActiveBackend()` accessor | [ ] |

### Integration Points
- `startUpdateChecker` in `cmd/ze/hub/main_system.go` - creates and starts backend
- `handleShowSystemUpdate` in `internal/component/cmd/show/update.go` - queries backend
- `handleFirmware*` in `internal/component/cmd/update/firmware.go` - dispatches to backend
- `checkWritableDestinations` in `cmd/ze/doctor/doctor.go` - doctor checks
- `gokrazy.Handler` in `internal/component/gokrazy/gokrazy.go` - reuse for status probe

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `startUpdateChecker(sc)` on gokrazy | -> | `gokrazyBackend.Start()` | `TestBackendSelectionGokrazy` |
| `startUpdateChecker(sc)` on Linux | -> | `zeSelfUpdateBackend.Start()` | `TestBackendSelectionLinux` |
| `show system update` RPC | -> | backend `.Status()` with `backend` field | `TestShowSystemUpdateBackendField` |
| `update system firmware check` on gokrazy | -> | returns unsupported | `TestFirmwareCheckGokrazyUnsupported` |
| `ze doctor` on gokrazy with update-check config | -> | warns config ignored | `TestDoctorGokrazyUpdateCheckWarning` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Platform is gokrazy, update-check config present | Backend selected is `gokrazy-ab` |
| AC-2 | Platform is plain-linux/systemd/container, auto-apply config | Backend selected is `ze-self-update` (wraps SelfUpdater) |
| AC-3 | Platform is plain-linux/systemd/container, no auto-apply | Backend selected is `ze-self-update` (wraps UpdateChecker) |
| AC-4 | `show system update` on any platform | Output includes `"backend": "<name>"` field |
| AC-5 | `show system update` on gokrazy | Status is `"managed by gokrazy"`, message explains external management |
| AC-6 | `show system update` on gokrazy, gokrazy reachable | Output includes gokrazy reachability and features |
| AC-7 | `update system firmware check` on gokrazy | Returns structured response: unsupported, updates managed by gokrazy |
| AC-8 | `update system firmware download/apply/restart/rollback` on gokrazy | Same structured unsupported response as AC-7 |
| AC-9 | `ze doctor` on gokrazy with `auto-apply: true` | Skips writable-binary warning |
| AC-10 | `ze doctor` on gokrazy with `update-check { url ... }` present | Warns that Ze self-update config is ignored on gokrazy |
| AC-11 | `ze doctor` on plain Linux with `auto-apply: true` | Existing writable-binary check unchanged |
| AC-12 | YANG descriptions for update-check and firmware commands | Platform-neutral wording (not "self-update" exclusively) |
| AC-13 | Normal Linux self-update flow | Behavior identical to current (no regression) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBackendSelectionGokrazy` | `internal/component/config/system/backend_test.go` | AC-1: gokrazy platform -> gokrazy-ab backend | |
| `TestBackendSelectionSelfUpdate` | `internal/component/config/system/backend_test.go` | AC-2: auto-apply -> ze-self-update (SelfUpdater) | |
| `TestBackendSelectionChecker` | `internal/component/config/system/backend_test.go` | AC-3: no auto-apply -> ze-self-update (UpdateChecker) | |
| `TestBackendStatusIncludesName` | `internal/component/config/system/backend_test.go` | AC-4: Status() output has backend field | |
| `TestGokrazyBackendStatus` | `internal/component/config/system/backend_test.go` | AC-5: gokrazy status returns managed-by-gokrazy | |
| `TestGokrazyProbeReachable` | `internal/component/config/system/backend_test.go` | AC-6: gokrazy probe with fake HTTP server | |
| `TestGokrazyProbeUnreachable` | `internal/component/config/system/backend_test.go` | AC-6: gokrazy probe when socket absent | |
| `TestGokrazyFirmwareUnsupported` | `internal/component/config/system/backend_test.go` | AC-7, AC-8: firmware ops return unsupported | |
| `TestShowSystemUpdateBackendField` | `internal/component/cmd/show/update_test.go` | AC-4: show output includes backend key | |
| `TestFirmwareCheckGokrazyUnsupported` | `internal/component/cmd/update/firmware_test.go` | AC-7: check on gokrazy returns unsupported | |
| `TestDoctorGokrazySkipsWritable` | `cmd/ze/doctor/doctor_test.go` | AC-9: no writable warning on gokrazy | |
| `TestDoctorGokrazyWarnsIgnoredConfig` | `cmd/ze/doctor/doctor_test.go` | AC-10: warns ignored self-update config | |
| `TestDoctorLinuxWritableUnchanged` | `cmd/ze/doctor/doctor_test.go` | AC-11: existing behavior preserved | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A: no new numeric inputs introduced.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-system-update-backend` | `test/plugin/show-system-update-backend.ci` | User runs `show system update`, sees backend field | |

### Interop Tests
N/A: no wire protocol changes.

### Future (if deferring any tests)
- Gokrazy integration test on real gokrazy image (requires gokrazy test harness, deferred with user approval)
- Mutating gokrazy update proxy tests (deferred: mutating endpoints explicitly out of scope)

## Files to Modify

- `internal/component/config/system/update.go` - remove global activeChecker (absorbed into backend)
- `internal/component/config/system/selfupdate.go` - remove global activeSelfUpdater (absorbed into backend)
- `internal/component/cmd/show/update.go` - add backend field, gokrazy-specific status
- `internal/component/cmd/update/firmware.go` - dispatch through backend instead of ActiveSelfUpdaterInstance()
- `cmd/ze/hub/main_system.go` - backend selection logic (platform + config -> backend)
- `cmd/ze/doctor/doctor.go` - platform-aware self-update doctor checks
- `internal/component/config/system/schema/ze-system-conf.yang` - update descriptions
- `internal/component/cmd/update/schema/ze-cli-update-cmd.yang` - update descriptions

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | Description updates only, no new leaves |
| YANG validation constraints | [ ] | N/A |
| YANG custom validators | [ ] | N/A |
| CLI commands/flags | [ ] | N/A: existing commands, changed dispatch |
| CLI grammar | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/plugin/show-system-update-backend.ci` |
| Pipe completeness | [ ] | N/A: existing commands |
| Env var registration | [ ] | N/A |
| Doctor check for runtime dependencies | [x] | Modify existing checks in `cmd/ze/doctor/doctor.go` |
| Prometheus counters/metrics | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - mention platform-aware update backend |
| 2 | Config syntax changed? | [ ] | N/A - config structure unchanged |
| 3 | CLI command added/changed? | [ ] | N/A - commands unchanged, behavior change on gokrazy |
| 4 | API/RPC added/changed? | [ ] | N/A - output gains `backend` field (additive) |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - update backend abstraction |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [ ] | N/A |

## Files to Create

- `internal/component/config/system/backend.go` - UpdateBackend interface + backend identity type + global accessor
- `internal/component/config/system/backend_gokrazy.go` - gokrazyBackend: status probe, unsupported firmware ops
- `internal/component/config/system/backend_ze.go` - zeBackend: wraps UpdateChecker or SelfUpdater
- `internal/component/config/system/backend_test.go` - backend selection and behavior tests
- `test/plugin/show-system-update-backend.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Backend interface + wiring (MANDATORY FIRST)** -- define interface, register global, write failing wiring tests
   - Tests: `TestBackendSelectionGokrazy`, `TestBackendSelectionSelfUpdate`, `TestBackendSelectionChecker`
   - Files: `backend.go` (interface + global), `backend_ze.go` (stub), `backend_gokrazy.go` (stub)
   - Verify: interface compiles, wiring tests fail because backends are stubs

2. **Phase: Ze backend wrapper** -- wrap UpdateChecker and SelfUpdater behind the interface
   - Tests: `TestBackendStatusIncludesName`, `TestBackendSelectionSelfUpdate`, `TestBackendSelectionChecker`
   - Files: `backend_ze.go`, `update.go` (remove global), `selfupdate.go` (remove global)
   - Verify: existing Ze update behavior passes through wrapper; no regression

3. **Phase: Gokrazy backend** -- implement status probe and unsupported firmware responses
   - Tests: `TestGokrazyBackendStatus`, `TestGokrazyProbeReachable`, `TestGokrazyProbeUnreachable`, `TestGokrazyFirmwareUnsupported`
   - Files: `backend_gokrazy.go`
   - Verify: gokrazy backend returns correct status and unsupported responses

4. **Phase: Rewire callers** -- update hub, show handler, firmware handlers, doctor
   - Tests: `TestShowSystemUpdateBackendField`, `TestFirmwareCheckGokrazyUnsupported`, `TestDoctorGokrazySkipsWritable`, `TestDoctorGokrazyWarnsIgnoredConfig`
   - Files: `cmd/ze/hub/main_system.go`, `show/update.go`, `update/firmware.go`, `doctor/doctor.go`
   - Verify: all callers use backend interface; gokrazy paths return correct responses

5. **Phase: YANG + docs** -- update descriptions to platform-neutral wording
   - Tests: AC-12 (visual review of YANG descriptions)
   - Files: `ze-system-conf.yang`, `ze-cli-update-cmd.yang`
   - Verify: descriptions no longer say "self-update" exclusively

6. **Functional tests** -- create after feature works
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Gokrazy backend returns structured unsupported, not error strings |
| Naming | Backend names are `"ze-self-update"` and `"gokrazy-ab"` exactly |
| Data flow | Hub creates backend; callers query it via global accessor |
| No regression | Existing Ze self-update tests pass unchanged |
| Doctor platform | Doctor checks are platform-aware; gokrazy skips writable, warns ignored config |
| YANG descriptions | Updated to platform-neutral language |
| Rule: no-layering | Old globals (`activeChecker`, `activeSelfUpdater`) fully removed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `backend.go` exists with `UpdateBackend` interface | `grep 'type UpdateBackend interface' internal/component/config/system/backend.go` |
| `backend_gokrazy.go` exists | `ls internal/component/config/system/backend_gokrazy.go` |
| `backend_ze.go` exists | `ls internal/component/config/system/backend_ze.go` |
| `show system update` includes `backend` field | `grep '"backend"' internal/component/cmd/show/update.go` |
| Old globals removed | `grep -c 'activeChecker\b' internal/component/config/system/update.go` returns 0 |
| Gokrazy firmware returns unsupported | `grep 'managed by gokrazy' internal/component/config/system/backend_gokrazy.go` |
| Doctor check platform-aware | `grep 'PlatformGokrazy' cmd/ze/doctor/doctor.go` |
| YANG descriptions updated | `grep -c 'self-update' internal/component/config/system/schema/ze-system-conf.yang` reduced |
| Functional test exists | `ls test/plugin/show-system-update-backend.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Gokrazy probe responses: validate HTTP status, limit body size, handle timeouts |
| Socket access | Gokrazy probe uses same socket path constant as proxy; no user-controlled paths |
| Auth injection | Gokrazy probe reuses existing password-reading code; no new auth paths |
| Error leakage | Error messages from gokrazy probe must not expose socket path or credentials |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Single interface wrapping both UpdateChecker and SelfUpdater | Separate interfaces per type; mode flag on existing types | One interface simplifies callers (show, firmware, hub) to a single dispatch point. The backend identity replaces the type-switch. |
| Backend identity as string constant | Enum type; auto-derived from struct type name | String is what appears in JSON output; enum adds indirection for two values. |
| Gokrazy backend probes HTTP, not Unix socket directly | Import gokrazy management library | HTTP probe reuses existing proxy infrastructure; no new dependencies. Socket is already proxied. |
| Status-only gokrazy support; no mutating endpoints | Full proxy of gokrazy update API | Mutating endpoints are partition/image operations, not Ze binary updates. Risk of confusion. Explicit future work. |
| Platform detection at hub startup, not per-request | Per-request detection; config flag | Platform doesn't change at runtime. One check at startup is simpler and correct. |

## Known Limitations
- No "update available" detection for gokrazy (no such gokrazy endpoint exists)
- No proxying of gokrazy mutating update endpoints (PUT /update/root, POST /reboot, etc.)
- Gokrazy status probe only checks reachability and features, not pending A/B state

## Implementation Summary

### What Was Implemented
- [to be filled]

### Bugs Found/Fixed
- [to be filled]

### Documentation Updates
- [to be filled]

### Deviations from Plan
- [to be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Backend abstraction unifies update callers | Unit test | `TestBackendSelectionGokrazy`, `TestBackendSelectionSelfUpdate` |
| show system update includes backend identity | Unit + functional test | `TestShowSystemUpdateBackendField`, `test-show-system-update-backend.ci` |
| Gokrazy returns clear unsupported for firmware ops | Unit test | `TestGokrazyFirmwareUnsupported` |
| Doctor is platform-aware | Unit test | `TestDoctorGokrazySkipsWritable`, `TestDoctorGokrazyWarnsIgnoredConfig` |
| No regression on normal Linux | Existing test suite | `make ze-unit-test` passes |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [to be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/798-unified-update-backend.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-unified-update-backend.md`
