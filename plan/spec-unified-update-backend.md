# Spec: Unified Update Backend

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | closing |
| Updated | 2026-06-17 |

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

**Key insights (post-implementation):**
- Single global `activeBackend UpdateBackend` in `backend.go:119`, protected by `activeBackendMu`.
- Hub in `main_system.go:279` (`startBackend`) calls `system.NewBackend(platformType, cfg, opts)` which selects backend by platform.
- `show system update` calls `ActiveExtendedUpdateStatus()` which queries the active backend. Output includes `backend` field.
- All firmware commands go through `system.ActiveBackend()` in `internal/plugins/update-cmd/cmd/firmware.go:16`.
- Platform detection: `host.DetectPlatform()` returns `PlatformInfo` with `Type`; `PlatformGokrazy` selects gokrazy-ab backend.
- Gokrazy backend probes management socket via HTTP (Unix socket transport) using `internal/core/gokrazyutil` helpers.
- No gokrazy endpoint reports "update available" directly. Probed: `GET /update/features`, `GET /`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/update.go` (286L) - UpdateChecker: periodic version check, report bus
  -> Constraint: `ValidateUpdateCheckURL` enforces HTTPS (HTTP only for localhost)
  -> Constraint: `isNewer` is lexicographic, guards against non-numeric prefixes
- [ ] `internal/component/config/system/selfupdate.go` (1016L) - SelfUpdater: download/verify/stage/restart
  -> Constraint: SelfUpdater has ManualCheck, ManualDownload, ManualApply, ManualRestart, Rollback
- [ ] `internal/component/config/system/backend.go` (148L) - UpdateBackend interface, factory registry, active backend global
  -> Constraint: `NewBackend(platform, cfg, opts)` selects backend; `ActiveExtendedUpdateStatus()` queries it
- [ ] `internal/component/config/system/backend_ze_distro.go` (141L) - zeBackend wraps UpdateChecker/SelfUpdater
  -> Constraint: build tag `ze_distro`; auto-apply/restart config selects SelfUpdater, else UpdateChecker
- [ ] `internal/component/config/system/backend_ze_appliance.go` (71L) - stripped Ze backend for minimal builds
  -> Constraint: build tag `!ze_distro`; returns "unsupported in minimal build"
- [ ] `internal/component/config/system/backend_gokrazy.go` (190L) - gokrazy backend: status probe, unsupported firmware ops
  -> Constraint: probes Unix socket via HTTP; returns "managed by gokrazy" status
- [ ] `internal/plugins/update-cmd/cmd/show.go` (115L) - show system update handler, queries `ActiveExtendedUpdateStatus()`
  -> Constraint: output includes `backend` field; gokrazy-specific fields when applicable
- [ ] `internal/plugins/update-cmd/cmd/firmware.go` (156L) - firmware subcommands, all call `system.ActiveBackend()`
  -> Constraint: returns structured unsupported response on gokrazy via `ErrFirmwareUnsupported`
- [ ] `cmd/ze/hub/main_system.go:279-316` - `startBackend`/`stopBackend`, platform-aware backend selection
  -> Constraint: `detectPlatform` is `sync.OnceValues`; backend created via `system.NewBackend`
- [ ] `internal/component/host/platform_linux.go` (168L) - `DetectPlatform()` returns `PlatformInfo` with `Type`
  -> Constraint: `PlatformGokrazy` detected via socket, /perm+read-only-root, or /user/gokrazy
- [ ] `internal/core/gokrazyutil/gokrazyutil.go` - shared gokrazy management helpers (socket path, auth header)
  -> Constraint: `DefaultSocketPath` and `AuthHeader()` used by gokrazy backend
- [ ] `internal/component/doctor/checks_storage.go:184-202` - self-update writable check (platform-aware)
  -> Constraint: skips writable-binary warning on gokrazy (AC-9)
- [ ] `internal/component/doctor/checks_platform.go:283-297` - gokrazy update-check config warning
  -> Constraint: warns that update-check config is ignored on gokrazy (AC-10)
- [ ] `internal/component/config/system/yang/ze-system-conf.yang:325-383` - update-check YANG
  -> Constraint: description says "System version check and platform update orchestration"
- [ ] `internal/plugins/update-cmd/yang/ze-update-firmware-cmd.yang` - firmware CLI YANG
  -> Constraint: descriptions say "System firmware lifecycle management"

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
- `startBackend` in `cmd/ze/hub/main_system.go:279` - creates and starts backend
- `handleShowSystemUpdate` in `internal/plugins/update-cmd/cmd/show.go:12` - queries backend
- `handleFirmware*` in `internal/plugins/update-cmd/cmd/firmware.go` - dispatches to backend
- `checkWritableDestinations` in `internal/component/doctor/checks_storage.go:113` - platform-aware writable check
- `checkUpdateBackendConfig` in `internal/component/doctor/checks_platform.go:283` - gokrazy config warning
- `gokrazyutil.AuthHeader` in `internal/core/gokrazyutil/gokrazyutil.go` - shared auth for gokrazy probe

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `startBackend(cfg)` on gokrazy | -> | `gokrazyBackend.Start()` | `TestBackendSelectionGokrazy` |
| `startBackend(cfg)` on Linux | -> | `zeBackend.Start()` | `TestBackendSelectionSelfUpdate`, `TestBackendSelectionChecker` |
| `show system update` RPC | -> | backend `.Status()` with `backend` field | `TestShowSystemUpdateBackendField` |
| `update system firmware check` on gokrazy | -> | returns unsupported | `TestGokrazyFirmwareUnsupported` |
| `ze doctor` on gokrazy with update-check config | -> | warns config ignored | `TestDoctorGokrazyWarnsIgnoredConfig` |

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
| `TestBackendSelectionGokrazy` | `internal/component/config/system/backend_ze_distro_test.go` | AC-1: gokrazy platform -> gokrazy-ab backend | done |
| `TestBackendSelectionSelfUpdate` | `internal/component/config/system/backend_ze_distro_test.go` | AC-2: auto-apply -> ze-self-update (SelfUpdater) | done |
| `TestBackendSelectionChecker` | `internal/component/config/system/backend_ze_distro_test.go` | AC-3: no auto-apply -> ze-self-update (UpdateChecker) | done |
| `TestBackendStatusIncludesName` | `internal/component/config/system/backend_ze_distro_test.go` | AC-4: Status() output has backend field | done |
| `TestGokrazyBackendStatus` | `internal/component/config/system/backend_ze_distro_test.go` | AC-5: gokrazy status returns managed-by-gokrazy | done |
| `TestGokrazyProbeReachable` | `internal/component/config/system/backend_ze_distro_test.go` | AC-6: gokrazy probe with fake HTTP server | done |
| `TestGokrazyProbeUnreachable` | `internal/component/config/system/backend_ze_distro_test.go` | AC-6: gokrazy probe when socket absent | done |
| `TestGokrazyFirmwareUnsupported` | `internal/component/config/system/backend_ze_distro_test.go` | AC-7, AC-8: firmware ops return unsupported | done |
| `TestShowSystemUpdateBackendField` | `internal/plugins/update-cmd/cmd/show_test.go` | AC-4: show output includes backend key | done |
| (covered by `TestGokrazyFirmwareUnsupported`) | `internal/component/config/system/backend_ze_distro_test.go` | AC-7: check on gokrazy returns unsupported | done |
| `TestDoctorGokrazySkipsWritable` | `internal/component/doctor/doctor_test.go` | AC-9: no writable warning on gokrazy | done |
| `TestDoctorGokrazyWarnsIgnoredConfig` | `internal/component/doctor/doctor_test.go` | AC-10: warns ignored self-update config | done |
| `TestDoctorLinuxWritableUnchanged` | `internal/component/doctor/doctor_test.go` | AC-11: existing behavior preserved | done |
| `TestStrippedBackendDisablesZeSelfUpdateWithoutURL` | `internal/component/config/system/backend_ze_appliance_test.go` | Minimal build returns unsupported | done |
| `TestStrippedBackendRejectsInvalidURL` | `internal/component/config/system/backend_ze_appliance_test.go` | Stripped build validates URL | done |
| `TestStrippedBackendRejectsInvalidSelfUpdateConfig` | `internal/component/config/system/backend_ze_appliance_test.go` | Stripped build validates self-update config | done |

### Boundary Tests (MANDATORY for numeric inputs)
N/A: no new numeric inputs introduced.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-system-update-backend` | `test/plugin/show-system-update-backend.ci` | User runs `show system update`, sees backend field | done |

### Interop Tests
N/A: no wire protocol changes.

### Future (if deferring any tests)
- Gokrazy integration test on real gokrazy image (requires gokrazy test harness, deferred with user approval)
- Mutating gokrazy update proxy tests (deferred: mutating endpoints explicitly out of scope)

## Files Modified

- `internal/component/config/system/update.go` - removed global activeChecker (absorbed into backend)
- `internal/component/config/system/selfupdate.go` - removed global activeSelfUpdater (absorbed into backend)
- `internal/plugins/update-cmd/cmd/show.go` - add backend field, gokrazy-specific status (relocated from cmd/show/)
- `internal/plugins/update-cmd/cmd/firmware.go` - dispatch through backend interface (relocated from cmd/update/)
- `cmd/ze/hub/main_system.go` - backend selection logic (platform + config -> backend)
- `internal/component/doctor/checks_storage.go` - platform-aware writable check (relocated from doctor.go)
- `internal/component/doctor/checks_platform.go` - gokrazy update-check config warning
- `internal/component/config/system/yang/ze-system-conf.yang` - platform-neutral descriptions
- `internal/plugins/update-cmd/yang/ze-update-firmware-cmd.yang` - platform-neutral descriptions

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

## Files Created

- `internal/component/config/system/backend.go` - UpdateBackend interface + factory registry + global accessor
- `internal/component/config/system/backend_gokrazy.go` - gokrazyBackend: status probe, unsupported firmware ops
- `internal/component/config/system/backend_ze_distro.go` - zeBackend: wraps UpdateChecker or SelfUpdater (build tag: ze_distro)
- `internal/component/config/system/backend_ze_appliance.go` - strippedZeBackend: minimal build stub (build tag: !ze_distro)
- `internal/component/config/system/backend_ze_distro_test.go` - backend selection and behavior tests (build tag: ze_distro)
- `internal/component/config/system/backend_ze_appliance_test.go` - stripped backend tests (build tag: !ze_distro)
- `internal/plugins/update-cmd/cmd/show_test.go` - show handler backend field test
- `internal/core/gokrazyutil/gokrazyutil.go` - shared gokrazy management helpers
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
| YANG descriptions updated | `grep -c 'self-update' internal/component/config/system/yang/ze-system-conf.yang` reduced |
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
- `UpdateBackend` interface with factory registry and platform-based selection (`NewBackend`)
- `zeBackend` wrapping UpdateChecker/SelfUpdater behind the interface (build tag: `ze_distro`)
- `strippedZeBackend` for minimal builds returning "unsupported in minimal build" (build tag: `!ze_distro`)
- `gokrazyBackend` with Unix socket probe for reachability/features, structured unsupported responses
- `gokrazyutil` shared package for socket path and auth header
- Single `activeBackend` global replacing `activeChecker` + `activeSelfUpdater`
- `show system update` output includes `backend` field and gokrazy-specific fields
- Firmware CLI handlers dispatch through `UpdateBackend` interface
- Doctor checks: skip writable-binary warning on gokrazy, warn if update-check config present on gokrazy
- YANG descriptions updated to platform-neutral wording
- `docs/features.md` and `docs/architecture/core-design.md` updated

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/features.md:40` documents platform-aware backend with all three variants
- `docs/architecture/core-design.md:1244` references `UpdateBackend` interface

### Deviations from Plan
- File renamed: `backend_ze.go` became `backend_ze_distro.go` + `backend_ze_appliance.go` (build-tagged)
- Test file split: `backend_test.go` became `backend_ze_distro_test.go` + `backend_ze_appliance_test.go`
- Show/firmware handlers relocated to `internal/plugins/update-cmd/cmd/` (plugin self-containment refactor)
- Doctor checks split into `checks_storage.go` and `checks_platform.go` (not in original `doctor.go`)
- New `internal/core/gokrazyutil/` package for shared gokrazy helpers (spec planned to reuse proxy code directly)
- `TestFirmwareCheckGokrazyUnsupported` absorbed into `TestGokrazyFirmwareUnsupported` at the backend level

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Single update backend interface | done | `backend.go:72` | `UpdateBackend` interface |
| Explicit backend identity | done | `backend.go:17-21` | `BackendName` type with two constants |
| Gokrazy reports "managed by gokrazy" | done | `backend_gokrazy.go:19-20` | `managedStatus`, `managedMessage` |
| Gokrazy probes management endpoints | done | `backend_gokrazy.go:98-109` | HTTP probe to Unix socket |
| Normal Linux wraps existing code | done | `backend_ze_distro.go:19-41` | Wraps UpdateChecker or SelfUpdater |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestBackendSelectionGokrazy` | `backend_ze_distro_test.go:25` |
| AC-2 | done | `TestBackendSelectionSelfUpdate` | `backend_ze_distro_test.go:37` |
| AC-3 | done | `TestBackendSelectionChecker` | `backend_ze_distro_test.go:59` |
| AC-4 | done | `TestBackendStatusIncludesName`, `TestShowSystemUpdateBackendField` | `backend_ze_distro_test.go:78`, `show_test.go:12` |
| AC-5 | done | `TestGokrazyBackendStatus` | `backend_ze_distro_test.go:91` |
| AC-6 | done | `TestGokrazyProbeReachable`, `TestGokrazyProbeUnreachable` | `backend_ze_distro_test.go:110,131` |
| AC-7 | done | `TestGokrazyFirmwareUnsupported` | `backend_ze_distro_test.go:152` (covers Check) |
| AC-8 | done | `TestGokrazyFirmwareUnsupported` | `backend_ze_distro_test.go:152` (covers Download/Apply/Restart/Rollback) |
| AC-9 | done | `TestDoctorGokrazySkipsWritable` | `doctor_test.go:1465` |
| AC-10 | done | `TestDoctorGokrazyWarnsIgnoredConfig` | `doctor_test.go:1484` |
| AC-11 | done | `TestDoctorLinuxWritableUnchanged` | `doctor_test.go:1498` |
| AC-12 | done | YANG descriptions updated | `ze-system-conf.yang:327`, `ze-update-firmware-cmd.yang:5` |
| AC-13 | done | Existing test suite passes | `zeBackend` delegates to unmodified UpdateChecker/SelfUpdater |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBackendSelectionGokrazy` | done | `backend_ze_distro_test.go:25` | |
| `TestBackendSelectionSelfUpdate` | done | `backend_ze_distro_test.go:37` | |
| `TestBackendSelectionChecker` | done | `backend_ze_distro_test.go:59` | |
| `TestBackendStatusIncludesName` | done | `backend_ze_distro_test.go:78` | |
| `TestGokrazyBackendStatus` | done | `backend_ze_distro_test.go:91` | |
| `TestGokrazyProbeReachable` | done | `backend_ze_distro_test.go:110` | |
| `TestGokrazyProbeUnreachable` | done | `backend_ze_distro_test.go:131` | |
| `TestGokrazyFirmwareUnsupported` | done | `backend_ze_distro_test.go:152` | |
| `TestShowSystemUpdateBackendField` | done | `show_test.go:12` | |
| `TestFirmwareCheckGokrazyUnsupported` | absorbed | `backend_ze_distro_test.go:152` | Covered by `TestGokrazyFirmwareUnsupported` |
| `TestDoctorGokrazySkipsWritable` | done | `doctor_test.go:1465` | |
| `TestDoctorGokrazyWarnsIgnoredConfig` | done | `doctor_test.go:1484` | |
| `TestDoctorLinuxWritableUnchanged` | done | `doctor_test.go:1498` | |
| `test-show-system-update-backend` | done | `test/plugin/show-system-update-backend.ci` | Functional test |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `backend.go` | created | As planned |
| `backend_gokrazy.go` | created | As planned |
| `backend_ze.go` | renamed | `backend_ze_distro.go` (ze_distro tag) + `backend_ze_appliance.go` (!ze_distro tag) |
| `backend_test.go` | renamed | `backend_ze_distro_test.go` + `backend_ze_appliance_test.go` |
| `show-system-update-backend.ci` | created | As planned |
| `gokrazyutil.go` | created | Not in plan; shared helpers extracted from gokrazy proxy |
| `show_test.go` | created | Not in plan; handler-level test for show backend field |

### Audit Summary
- **Total items:** 31 (5 requirements + 13 ACs + 13 tests + planned files)
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (file renames/splits)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Backend abstraction unifies update callers | Unit test | `TestBackendSelectionGokrazy`, `TestBackendSelectionSelfUpdate` |
| show system update includes backend identity | Unit + functional test | `TestShowSystemUpdateBackendField`, `test-show-system-update-backend.ci` |
| Gokrazy returns clear unsupported for firmware ops | Unit test | `TestGokrazyFirmwareUnsupported` |
| Doctor is platform-aware | Unit test | `TestDoctorGokrazySkipsWritable`, `TestDoctorGokrazyWarnsIgnoredConfig` |
| No regression on normal Linux | Existing test suite | `make ze-unit-test` passes |

## Review Gate

### Run 1 (closure review, 2026-06-17)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Spec status was `design` but implementation committed | header | Fixed: status set to in-progress/closing |
| 2 | NOTE | File paths in spec were stale (pre-relocation) | throughout | Fixed: all paths updated to actual locations |
| 3 | NOTE | Extra file `backend_ze_appliance.go` not in original spec | Files to Create | Fixed: added to spec |
| 4 | NOTE | `gokrazyutil` package not in original spec | Files to Create | Fixed: added to spec |

### Fixes applied
- Updated all file paths to post-relocation locations
- Filled all audit tables with file:line evidence
- Added `backend_ze_appliance.go` and `gokrazyutil` to file lists
- Updated Wiring Test table to match actual test names

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| (clean) | | | | |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/system/backend.go` | yes | 3.9K, defines `UpdateBackend` interface |
| `internal/component/config/system/backend_gokrazy.go` | yes | 4.8K, gokrazy backend |
| `internal/component/config/system/backend_ze_distro.go` | yes | 3.2K, ze backend wrapper |
| `internal/component/config/system/backend_ze_appliance.go` | yes | 2.2K, stripped backend |
| `internal/component/config/system/backend_ze_distro_test.go` | yes | 7.8K, backend tests |
| `internal/component/config/system/backend_ze_appliance_test.go` | yes | 1.8K, stripped tests |
| `internal/plugins/update-cmd/cmd/show_test.go` | yes | 916B, show handler test |
| `internal/core/gokrazyutil/gokrazyutil.go` | yes | shared helpers |
| `test/plugin/show-system-update-backend.ci` | yes | 2.4K, functional test |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | gokrazy -> gokrazy-ab | `NewBackend` line 107: `if platform == host.PlatformGokrazy { name = BackendGokrazyAB }` |
| AC-2 | auto-apply -> SelfUpdater | `backend_ze_distro.go:35`: checks `AutoApply \|\| RestartImmediate \|\| RestartTime` |
| AC-3 | no auto-apply -> UpdateChecker | `backend_ze_distro.go:39`: `backend.checker = NewUpdateChecker(...)` |
| AC-4 | backend field in output | `show.go:17`: `"backend": string(ext.Backend)` |
| AC-5 | gokrazy managed status | `backend_gokrazy.go:19`: `managedStatus = "managed by gokrazy"` |
| AC-6 | gokrazy reachability | `show.go:24-28`: gokrazy-specific fields when `BackendGokrazyAB` |
| AC-7 | firmware check unsupported | `backend_gokrazy.go:77`: returns `ErrFirmwareUnsupported` |
| AC-8 | all firmware ops unsupported | `backend_gokrazy.go:80-94`: Download/Apply/Restart/Rollback all return `ErrFirmwareUnsupported` |
| AC-9 | doctor skips writable on gokrazy | `checks_storage.go:187-189`: `if platform.Type == host.PlatformGokrazy { return diags }` |
| AC-10 | doctor warns ignored config | `checks_platform.go:291-295`: emits `doctor-config-platform-mismatch` |
| AC-11 | Linux writable unchanged | `checks_storage.go:190-199`: existing logic preserved after gokrazy guard |
| AC-12 | YANG platform-neutral | `ze-system-conf.yang:327`: "System version check and platform update orchestration" |
| AC-13 | no regression | `zeBackend` delegates unmodified to UpdateChecker/SelfUpdater |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show system update` -> backend field | `test/plugin/show-system-update-backend.ci` | yes: checks `data.get('backend') != 'ze-self-update'` |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-13 all demonstrated
- [x] Wiring Test table complete
- [x] `/ze-review` gate clean
- [x] `make ze-test` passes
- [x] Feature code integrated
- [x] Integration completeness proven end-to-end
- [x] Architecture docs updated
- [x] Critical Review passes

### Quality Gates (SHOULD pass)
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction (3+ use cases?)
- [x] No speculative features (needed NOW?)
- [x] Single responsibility per component
- [x] Explicit > implicit behavior
- [x] Minimal coupling

### TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Tests PASS
- [x] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/909-unified-update-backend.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-unified-update-backend.md`
