# Spec: doctor-appliance-1-coherence

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | - |
| Phase | 1 of 2 (phase 2: spec-doctor-appliance-2-health) |
| Updated | 2026-05-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/doctor-checks.md` - doctor check conventions
4. `cmd/ze/doctor/doctor.go` - existing doctor implementation
5. `internal/component/host/platform_linux.go` - platform detection

## Task

Make each `ze doctor` check platform-aware so it can detect configuration
coherence gaps specific to the detected platform. Today, checks run
independently without knowing whether the platform is gokrazy, systemd,
container, or plain Linux. This leads to silent failures when two independent
configs conspire against each other (e.g., gokrazy config excludes its NTP,
Ze config disables Ze NTP, so nothing syncs the clock).

Thread `*host.PlatformInfo` through `runChecks()` to the checks that need it.
Each check decides its own severity based on platform context.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/doctor-checks.md` - doctor check conventions
  -> Constraint: every new doctor code must be registered in codes.go with title, description, examples
  -> Constraint: every new check needs unit test + functional test
- [ ] `docs/architecture/core-design.md` - component isolation
  -> Decision: components are independent unless they explicitly depend on each other
  -> Constraint: doctor is in cmd/ze/doctor/, not in internal/; it imports host package

### Learned Summaries
- [ ] `plan/learned/577-gokrazy-2-ntp.md` - NTP plugin design
  -> Decision: Ze NTP defaults persist-path to /perm/ze/timefile (gokrazy-specific)
  -> Constraint: YANG schema has `enabled false` default, so NTP is opt-in
- [ ] `plan/learned/576-gokrazy-1-dhcp-wiring.md` - DHCP config wiring
  -> Decision: resolv.conf defaults to /tmp/resolv.conf for gokrazy read-only rootfs
  -> Constraint: /etc/resolv.conf is correct for systemd platforms

**Key insights:**
- Platform detection already exists in `host.DetectPlatform()` and is called by `checkPlatform()`, but the result is discarded
- Gokrazy means Ze owns all system services; missing NTP/DHCP config is an error, not a warning
- Systemd/container platforms may have external NTP (chrony, timesyncd); missing Ze NTP is a warning
- Several config defaults are gokrazy-optimized (/perm/ze/timefile, /tmp/resolv.conf) and wrong elsewhere

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/doctor/doctor.go` - main doctor implementation (1951 lines)
  -> Constraint: `checkPlatform()` at line 218 signature is `func() []diagnostic.Diagnostic`; PlatformInfo built inside, discarded
  -> Constraint: `runChecks()` at line 106 calls `checkPlatform()` at line 119 before config load, then tree-dependent checks after line 146
  -> Constraint: `checkSystemdServiceInstall()` at line 296 signature is `func() []diagnostic.Diagnostic`; runs unconditionally
  -> Constraint: `checkNTPClient()` at line 1567 signature is `func(tree *config.Tree) []diagnostic.Diagnostic`; only checks when NTP enabled
  -> Constraint: `checkWritableDestinations()` at line 1776 signature is `func(tree *config.Tree) []diagnostic.Diagnostic`; checks NTP persist-path, BFD persist-dir, DNS resolv-conf-path, archive locations, self-update binary dir
- [ ] `cmd/ze/doctor/checks_linux.go` - Linux-specific checks (585 lines)
  -> Constraint: `checkNTPClockPrivilege()` at line 443 already checks CAP_SYS_TIME for NTP
  -> Constraint: all Linux-only check functions must have matching stubs in checks_other.go
  -> Constraint: uses `readFilePath` var (replaceable for tests) to read /proc paths
- [ ] `cmd/ze/doctor/checks_other.go` - non-Linux stubs returning nil (67 lines)
  -> Constraint: one stub per Linux-only function, all return nil
- [ ] `internal/component/host/platform_linux.go` - platform detection (168 lines)
  -> Constraint: `DetectPlatform()` is a method on `Detector` struct; default exported func at inventory.go:515 delegates to `defaultDetector`
  -> Constraint: PlatformInfo fields: Type, ReadOnlyRoot, PermAvailable, SystemdAvailable, GokrazyUpdateSocket, GokrazyUIAvailable, RebootAllowed, PersistentStorageWritable, FDLimitSoftCurrent, FDLimitHardMax, FDLimitRaisable
- [ ] `internal/component/host/inventory.go` - PlatformInfo struct at line 360
  -> Constraint: PlatformType is uint8 enum: Unknown=0, Gokrazy=1, Systemd=2, Container=3, PlainLinux=4, Darwin=5
- [ ] `internal/core/diagnostic/codes.go` - diagnostic code registry (433 lines)
  -> Constraint: codes registered in `builtinCodes` slice of `CodeMeta` with Code, Title, Description, Examples, optional RelatedCodes

**Behavior to preserve:**
- All existing check functions continue to work unchanged when platform is nil (backward compat during transition)
- `checkPlatform()` still emits its existing diagnostics (platform-detect, platform-unknown, platform-perm, platform-container-ro)
- `checkClockSkew()` still probes NTP pool independently (it's about current clock state, not config)
- `checkNTPClient()` still checks server reachability when NTP is enabled
- `checkWritableDestinations()` still probes writable paths
- `checkSystemdServiceInstall()` still validates unit file when present on systemd
- JSON and text output format unchanged

**Behavior to change:**
- `checkPlatform()` additionally returns `*host.PlatformInfo` for use by subsequent checks
- `checkSystemdServiceInstall()` skips on gokrazy/container platforms (irrelevant)
- `checkNTPClient()` additionally warns when NTP disabled AND platform has no external clock sync
- `checkWritableDestinations()` warns about platform-mismatched defaults (gokrazy defaults on systemd, etc.)
- New: `checkResolvConfPath()` warns when resolv-conf-path default doesn't match platform
- New: `checkMachineID()` warns when `/etc/machine-id` is absent on platforms that need it (Linux-only)

## Data Flow (MANDATORY)

### Entry Point
- `ze doctor [--json] [config-file]` invokes `Run()` which calls `runChecks(configPath)`

### Transformation Path
1. `runChecks()` calls modified `checkPlatform()` which returns `(*host.PlatformInfo, []diagnostic.Diagnostic)`
2. `runChecks()` loads config to get `*config.Tree`
3. `runChecks()` passes `platform` to checks that need context-aware behavior
4. Each check internally decides severity based on `platform.Type`
5. Diagnostics collected into `[]diagnostic.Diagnostic` and returned

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| host package -> doctor package | Import `host.DetectPlatform()`, use `*host.PlatformInfo` (already imported) | [ ] |
| config package -> doctor package | Existing: `config.LoadConfig()`, `config.Tree` | [ ] |

### Integration Points
- `host.DetectPlatform()` - already called in `checkPlatform()`; keep the result instead of discarding
- `config.Tree` - already used by tree-dependent checks; no change
- `diagnostic.Diagnostic` - existing return type; no change
- `diagnostic/codes.go` - add new codes

### Architectural Verification
- [ ] No bypassed layers (platform info flows through runChecks, not a global)
- [ ] No unintended coupling (doctor already imports host package)
- [ ] No duplicated functionality (extends existing checks, doesn't recreate)
- [ ] Zero-copy preserved where applicable (PlatformInfo is a pointer, passed by reference)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor --json <config>` | -> | coherence checks in `runChecks()` | `test/ui/doctor-platform-coherence.ci` |
| `runChecks()` | -> | modified `checkPlatform()` returning PlatformInfo | `TestCheckPlatformReturnsPlatformInfo` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Gokrazy platform, Ze NTP disabled, no NTP servers configured | Error diagnostic `doctor-clock-no-sync` emitted |
| AC-2 | Systemd platform, Ze NTP disabled | Warning diagnostic `doctor-clock-no-sync` emitted (external NTP possible) |
| AC-3 | Darwin/unknown platform, Ze NTP disabled | No clock-sync diagnostic emitted |
| AC-4 | Gokrazy platform, Ze NTP enabled with servers | No clock-sync diagnostic emitted |
| AC-5 | Gokrazy platform, `checkSystemdServiceInstall()` | Systemd check skipped (no diagnostic about missing unit) |
| AC-6 | Container platform, `checkSystemdServiceInstall()` | Systemd check skipped |
| AC-7 | Systemd platform, `checkSystemdServiceInstall()` | Systemd check runs as before |
| AC-8 | NTP persist-path is `/perm/ze/timefile` on systemd platform | Warning diagnostic `doctor-config-platform-mismatch` |
| AC-9 | NTP persist-path is `/perm/ze/timefile` on gokrazy platform | No mismatch diagnostic |
| AC-10 | DNS resolv-conf-path is `/tmp/resolv.conf` on systemd platform | Warning diagnostic `doctor-config-platform-mismatch` |
| AC-11 | DNS resolv-conf-path is `/etc/resolv.conf` on gokrazy platform | Warning diagnostic `doctor-config-platform-mismatch` (read-only root) |
| AC-12 | DNS resolv-conf-path is `/tmp/resolv.conf` on gokrazy platform | No mismatch diagnostic |
| AC-13 | Gokrazy platform, `/etc/machine-id` does not exist | Warning diagnostic `doctor-machine-id-missing` |
| AC-14 | Systemd platform, `/etc/machine-id` exists | No machine-id diagnostic |
| AC-15 | Platform is nil (e.g., detection failed) | All coherence-enhanced checks degrade to current behavior (no crash, no new diagnostics) |
| AC-16 | `ze doctor --json` output includes new diagnostic codes in same format | JSON output unchanged structurally |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckPlatformReturnsPlatformInfo` | `cmd/ze/doctor/doctor_test.go` | checkPlatform() returns usable PlatformInfo alongside diagnostics | |
| `TestCheckNTPCoherenceGokrazyNoNTP` | `cmd/ze/doctor/doctor_test.go` | AC-1: gokrazy + NTP disabled = error | |
| `TestCheckNTPCoherenceSystemdNoNTP` | `cmd/ze/doctor/doctor_test.go` | AC-2: systemd + NTP disabled = warning | |
| `TestCheckNTPCoherenceDarwinNoNTP` | `cmd/ze/doctor/doctor_test.go` | AC-3: darwin + NTP disabled = skip | |
| `TestCheckNTPCoherenceGokrazyNTPEnabled` | `cmd/ze/doctor/doctor_test.go` | AC-4: gokrazy + NTP enabled = no diagnostic | |
| `TestCheckSystemdServiceSkipsGokrazy` | `cmd/ze/doctor/doctor_test.go` | AC-5: gokrazy skips systemd check | |
| `TestCheckSystemdServiceSkipsContainer` | `cmd/ze/doctor/doctor_test.go` | AC-6: container skips systemd check | |
| `TestCheckSystemdServiceRunsOnSystemd` | `cmd/ze/doctor/doctor_test.go` | AC-7: systemd runs normally | |
| `TestCheckPersistPathMismatchSystemd` | `cmd/ze/doctor/doctor_test.go` | AC-8: /perm path on systemd = warning | |
| `TestCheckPersistPathMatchGokrazy` | `cmd/ze/doctor/doctor_test.go` | AC-9: /perm path on gokrazy = no warning | |
| `TestCheckResolvConfMismatchSystemd` | `cmd/ze/doctor/doctor_test.go` | AC-10: /tmp/resolv.conf on systemd = warning | |
| `TestCheckResolvConfMismatchGokrazy` | `cmd/ze/doctor/doctor_test.go` | AC-11: /etc/resolv.conf on gokrazy = warning | |
| `TestCheckResolvConfMatchGokrazy` | `cmd/ze/doctor/doctor_test.go` | AC-12: /tmp/resolv.conf on gokrazy = no warning | |
| `TestCheckMachineIDMissingGokrazy` | `cmd/ze/doctor/checks_linux_test.go` | AC-13: gokrazy + no machine-id = warning | |
| `TestCheckMachineIDPresentSystemd` | `cmd/ze/doctor/checks_linux_test.go` | AC-14: systemd + machine-id present = no warning | |
| `TestCheckCoherenceNilPlatform` | `cmd/ze/doctor/doctor_test.go` | AC-15: nil platform = no crash, no new diagnostics | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this feature.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-platform-coherence` | `test/ui/doctor-platform-coherence.ci` | `ze doctor --json` with crafted config shows coherence diagnostics | |

### Interop Tests
N/A - not a protocol feature.

### Future
- None deferred.

## Files to Modify
- `cmd/ze/doctor/doctor.go` - modify `checkPlatform()` to return `(*host.PlatformInfo, []diagnostic.Diagnostic)`; modify `runChecks()` to capture and thread platform; modify `checkSystemdServiceInstall()` to accept platform and skip on gokrazy/container; modify `checkNTPClient()` to accept platform and add clock-sync gap detection; modify `checkWritableDestinations()` to accept platform and warn about path mismatches; add `checkResolvConfPath()` for resolv-conf-path vs platform
- `cmd/ze/doctor/checks_linux.go` - add `checkMachineID(platform)` implementation
- `cmd/ze/doctor/checks_other.go` - add `checkMachineID()` stub
- `cmd/ze/doctor/doctor_test.go` - unit tests for all platform coherence behaviors
- `cmd/ze/doctor/checks_linux_test.go` - unit tests for `checkMachineID`
- `internal/core/diagnostic/codes.go` - register `doctor-clock-no-sync`, `doctor-config-platform-mismatch`, `doctor-machine-id-missing`

## Files to Create
- `test/ui/doctor-platform-coherence.ci` - functional test for platform coherence diagnostics

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/ui/doctor-platform-coherence.ci` |
| Pipe completeness | No | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | Yes | This IS the doctor check feature |
| Prometheus counters/metrics | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - mention platform-aware doctor checks |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |

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

1. **Phase: Wiring** -- modify `checkPlatform()` signature, thread PlatformInfo through `runChecks()`
   - Tests: `TestCheckPlatformReturnsPlatformInfo`, `TestCheckCoherenceNilPlatform`
   - Files: `doctor.go` (`checkPlatform` return type, `runChecks` plumbing)
   - Verify: existing tests still pass; new test verifies PlatformInfo is captured and usable

2. **Phase: NTP coherence** -- add clock-sync gap detection to `checkNTPClient()`
   - Tests: `TestCheckNTPCoherenceGokrazyNoNTP`, `TestCheckNTPCoherenceSystemdNoNTP`, `TestCheckNTPCoherenceDarwinNoNTP`, `TestCheckNTPCoherenceGokrazyNTPEnabled`
   - Files: `doctor.go` (`checkNTPClient` gains platform parameter)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Systemd service skip** -- make `checkSystemdServiceInstall()` platform-aware
   - Tests: `TestCheckSystemdServiceSkipsGokrazy`, `TestCheckSystemdServiceSkipsContainer`, `TestCheckSystemdServiceRunsOnSystemd`
   - Files: `doctor.go` (`checkSystemdServiceInstall` gains platform parameter)
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Path coherence** -- add persist-path and resolv-conf-path mismatch detection
   - Tests: `TestCheckPersistPathMismatchSystemd`, `TestCheckPersistPathMatchGokrazy`, `TestCheckResolvConfMismatchSystemd`, `TestCheckResolvConfMismatchGokrazy`, `TestCheckResolvConfMatchGokrazy`
   - Files: `doctor.go` (`checkWritableDestinations` gains platform, new `checkResolvConfPath`)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Machine ID** -- add machine-id check (Linux-only)
   - Tests: `TestCheckMachineIDMissingGokrazy`, `TestCheckMachineIDPresentSystemd`
   - Files: `checks_linux.go` (new `checkMachineID`), `checks_other.go` (stub)
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Diagnostic codes** -- register all new codes in codes.go
   - Files: `internal/core/diagnostic/codes.go`
   - Verify: `grep -c 'doctor-' internal/core/diagnostic/codes.go` count increases by 3

7. **Functional tests** -- create .ci test
   - Files: `test/ui/doctor-platform-coherence.ci`
   - Verify: `ze doctor --json` with test config produces expected codes

8. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Severity tiers match: error on gokrazy, warning on systemd/container, skip on darwin/unknown |
| Naming | Diagnostic codes use `doctor-` prefix, kebab-case |
| Data flow | PlatformInfo flows through runChecks as a local, never stored as a global |
| Nil safety | Every check that receives platform handles nil gracefully (AC-15) |
| Stub parity | Every Linux-only check has a matching stub in checks_other.go |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `checkPlatform()` returns PlatformInfo | `grep 'PlatformInfo' cmd/ze/doctor/doctor.go` |
| `doctor-clock-no-sync` code registered | `grep 'doctor-clock-no-sync' internal/core/diagnostic/codes.go` |
| `doctor-config-platform-mismatch` code registered | `grep 'doctor-config-platform-mismatch' internal/core/diagnostic/codes.go` |
| `doctor-machine-id-missing` code registered | `grep 'doctor-machine-id-missing' internal/core/diagnostic/codes.go` |
| `checkMachineID` in checks_linux.go | `grep 'checkMachineID' cmd/ze/doctor/checks_linux.go` |
| `checkMachineID` stub in checks_other.go | `grep 'checkMachineID' cmd/ze/doctor/checks_other.go` |
| Functional test exists | `ls test/ui/doctor-platform-coherence.ci` |
| All unit tests pass | `go test ./cmd/ze/doctor/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | PlatformInfo comes from `host.DetectPlatform()` (trusted internal). Config tree comes from parsed config (validated by YANG). No external input. |
| Path traversal | Machine-id check reads fixed path `/etc/machine-id`, no user input in path construction. |
| Information leakage | Diagnostic messages report path existence, not file contents. Consistent with existing checks. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, revisit design |
| Functional test fails | Check AC; if AC wrong, redesign; if AC correct, fix implementation |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Standalone `checkPlatformCoherence()` function | User pointed out each check should be independently context-aware, not a separate cross-cutting layer | Thread platform through individual checks |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Doctor checks should have been platform-aware from the start. Each check is an independent unit that should know its execution context. The pattern of "detect platform, discard result, then run checks blind" is an anti-pattern that allowed coherence gaps to go undetected.

## Core Insight

Platform coherence is not a separate concern layered on top of existing checks.
It is the responsibility of each check to understand its context and adjust behavior.
The architectural fix is threading platform info through the check pipeline.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Thread platform through individual checks | Standalone `checkPlatformCoherence()` function | Each check is independent and context-aware; coherence logic belongs where the domain knowledge lives |
| Error severity on gokrazy, warning on systemd | Uniform severity across platforms | Gokrazy means Ze owns everything; missing NTP is a definite problem. Systemd may have external NTP. |
| Nil-safe platform parameter | Required non-nil platform | Graceful degradation when platform detection fails; existing behavior preserved |

## Known Limitations
- Cannot detect external NTP (chrony, timesyncd) on systemd platforms, so the `doctor-clock-no-sync` warning may be a false positive. Acceptable as a warning (not error).
- Machine-id check is Linux-only; non-Linux stubs return nil.
- Phase 2 (spec-doctor-appliance-2-health) will add broader appliance probes: management socket reachability, log backend sanity, DHCP path validation, etc.

## RFC Documentation
N/A - not a protocol feature.

## Implementation Summary

### What Was Implemented
- `checkPlatform()` now returns `*host.PlatformInfo` alongside existing diagnostics, and `runChecks()` threads that pointer through the doctor checks that need platform context.
- `checkSystemdServiceInstall()` skips irrelevant systemd unit checks on gokrazy and container platforms while preserving nil-platform and systemd behavior.
- `checkNTPClient()` now reports `doctor-clock-no-sync` when Ze NTP is disabled on platforms where clock sync needs explicit attention: error on gokrazy, warning on systemd/container/plain Linux, no diagnostic on Darwin/unknown/nil.
- `checkWritableDestinations()` now warns when the gokrazy `/perm` NTP persist path is used on non-gokrazy Linux platforms.
- Added `checkResolvConfPath()` for DNS resolv.conf path/platform mismatches.
- Added Linux-only `checkMachineID()` and non-Linux stub, warning on missing or empty `/etc/machine-id` for gokrazy/systemd.
- Added private Linux-only `ze.test.doctor.machine-id-path` override so functional tests can deterministically exercise missing machine-id behavior without depending on the host.
- Registered `doctor-clock-no-sync`, `doctor-config-platform-mismatch`, and `doctor-machine-id-missing` in `internal/core/diagnostic/codes.go`.
- Added unit, functional, and QEMU integration coverage for the coherence behavior.

### Bugs Found/Fixed
- QEMU/root execution exposed a false-negative risk in NTP privilege tests because UID 0 always bypasses `CAP_SYS_TIME` parsing. Added the `currentUID` test seam so privilege tests stay deterministic.
- Lint caught an exhaustive-switch issue in `checkResolvConfPath()` after the platform switch was added. Fixed by making the default branch return nil explicitly.
- Final audit found missing explicit coverage for unknown platform in AC-3 and the nil-platform machine-id path in AC-15. Added `TestCheckNTPCoherenceUnknownNoNTP` and extended `TestCheckCoherenceNilPlatform`.

### Documentation Updates
- Updated `docs/features.md` System Readiness row to mention platform-aware doctor checks for missing clock sync, platform-mismatched persistence/DNS paths, and machine-id presence.

### Deviations from Plan
- Added private env override `ze.test.doctor.platform` to make `ze doctor --json` functional tests deterministic across developer hosts and CI platforms.
- Added `cmd/ze/doctor/checks_integration_linux_test.go` for QEMU coverage of the Linux-only machine-id path. The planned file list did not include this extra integration test, but `ai/rules/qemu-testing.md` requires QEMU coverage for Linux-only code.
- Updated `test/ui/doctor-service-systemd.ci` to force the systemd platform so the new platform-aware skip logic does not make the existing service test host-dependent.
- Added `test/ui/doctor-machine-id.ci` after `/ze-review` found that `doctor-machine-id-missing` had unit and QEMU integration coverage but no user-entrypoint functional test.
- `make ze-verify` failed on unrelated existing issues outside the doctor changes. Targeted doctor verification (`go test -race ./cmd/ze/doctor`, `ze-test ui`, QEMU integration) passed.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Thread `*host.PlatformInfo` through `runChecks()` | Done | `runChecks()` in `doctor.go` | Platform is local state, not a global. |
| `checkPlatform()` returns platform plus diagnostics | Done | `checkPlatform()` in `doctor.go` | Existing diagnostics preserved. |
| Checks decide severity from platform context | Done | `clockNoSyncSeverity()` in `doctor.go` | Maps gokrazy to error, standard Linux to warning. |
| Skip systemd service check on gokrazy/container | Done | `checkSystemdServiceInstall()` in `doctor.go` | Nil platform still runs old behavior. |
| Detect platform-mismatched writable/DNS paths | Done | `checkNTPPersistPath()`, `checkResolvConfPath()` in `doctor.go` | Uses shared `doctor-config-platform-mismatch` diagnostic. |
| Add Linux machine-id check with non-Linux stub | Done | `checkMachineID()` in `checks_linux.go` + `checks_other.go` | Warns only for gokrazy/systemd and nil-safe. |
| Register new doctor diagnostic codes | Done | `codes.go`: `doctor-clock-no-sync`, `doctor-config-platform-mismatch`, `doctor-machine-id-missing` | Three new entries in `builtinCodes`. |
| Expose behavior through `ze doctor --json` | Done | `test/ui/doctor-platform-coherence.ci` | Functional test asserts new JSON diagnostics through the CLI entry point. |
| Expose machine-id diagnostic through `ze doctor --json` | Done | `test/ui/doctor-machine-id.ci` | Linux-only functional test asserts `doctor-machine-id-missing` through the CLI entry point. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCheckNTPCoherenceGokrazyNoNTP`, `test/ui/doctor-platform-coherence.ci` | Gokrazy + Ze NTP disabled emits `doctor-clock-no-sync` error. |
| AC-2 | Done | `TestCheckNTPCoherenceSystemdNoNTP` | Systemd + Ze NTP disabled emits `doctor-clock-no-sync` warning. |
| AC-3 | Done | `TestCheckNTPCoherenceDarwinNoNTP`, `TestCheckNTPCoherenceUnknownNoNTP` | Darwin and unknown platforms emit no clock-sync diagnostic. |
| AC-4 | Done | `TestCheckNTPCoherenceGokrazyNTPEnabled` | Gokrazy with enabled Ze NTP and server emits no clock-sync gap. |
| AC-5 | Done | `TestCheckSystemdServiceSkipsGokrazy` | Gokrazy skips systemd unit reads. |
| AC-6 | Done | `TestCheckSystemdServiceSkipsContainer` | Container skips systemd unit reads. |
| AC-7 | Done | `TestCheckSystemdServiceRunsOnSystemd`, `test/ui/doctor-service-systemd.ci` | Systemd platform still validates installed unit files. |
| AC-8 | Done | `TestCheckPersistPathMismatchSystemd` | `/perm/ze/timefile` on systemd emits platform mismatch warning. |
| AC-9 | Done | `TestCheckPersistPathMatchGokrazy` | `/perm/ze/timefile` on gokrazy emits no platform mismatch. |
| AC-10 | Done | `TestCheckResolvConfMismatchSystemd` | `/tmp/resolv.conf` on systemd emits platform mismatch warning. |
| AC-11 | Done | `TestCheckResolvConfMismatchGokrazy`, `test/ui/doctor-platform-coherence.ci` | `/etc/resolv.conf` on gokrazy emits platform mismatch warning. |
| AC-12 | Done | `TestCheckResolvConfMatchGokrazy` | `/tmp/resolv.conf` on gokrazy emits no mismatch. |
| AC-13 | Done | `TestCheckMachineIDMissingGokrazy`, `TestCheckMachineIDIntegration`, `test/ui/doctor-machine-id.ci` | Missing `/etc/machine-id` on gokrazy emits warning. |
| AC-14 | Done | `TestCheckMachineIDPresentSystemd` | Non-empty `/etc/machine-id` on systemd emits no diagnostic. |
| AC-15 | Done | `TestCheckCoherenceNilPlatform` | Nil platform produces no new coherence diagnostics and no crash. |
| AC-16 | Done | `test/ui/doctor-platform-coherence.ci` | `ze doctor --json` keeps the existing `diagnostics` JSON structure and includes new codes. |

### Tests from TDD Plan
| Test | Status | File | Notes |
|------|--------|------|-------|
| `TestCheckPlatformReturnsPlatformInfo` | Done | `doctor_test.go` | Verifies forced systemd platform is returned. |
| `TestCheckNTPCoherenceGokrazyNoNTP` | Done | `doctor_test.go` | AC-1. |
| `TestCheckNTPCoherenceSystemdNoNTP` | Done | `doctor_test.go` | AC-2. |
| `TestCheckNTPCoherenceDarwinNoNTP` | Done | `doctor_test.go` | AC-3 Darwin. |
| `TestCheckNTPCoherenceUnknownNoNTP` | Added | `doctor_test.go` | AC-3 unknown. Added during audit. |
| `TestCheckNTPCoherenceGokrazyNTPEnabled` | Done | `doctor_test.go` | AC-4. |
| `TestCheckSystemdServiceSkipsGokrazy` | Done | `doctor_test.go` | AC-5. |
| `TestCheckSystemdServiceSkipsContainer` | Done | `doctor_test.go` | AC-6. |
| `TestCheckSystemdServiceRunsOnSystemd` | Done | `doctor_test.go` | AC-7. |
| `TestCheckPersistPathMismatchSystemd` | Done | `doctor_test.go` | AC-8. |
| `TestCheckPersistPathMatchGokrazy` | Done | `doctor_test.go` | AC-9. |
| `TestCheckResolvConfMismatchSystemd` | Done | `doctor_test.go` | AC-10. |
| `TestCheckResolvConfMismatchGokrazy` | Done | `doctor_test.go` | AC-11. |
| `TestCheckResolvConfMatchGokrazy` | Done | `doctor_test.go` | AC-12. |
| `TestCheckMachineIDMissingGokrazy` | Done | `checks_linux_test.go` | AC-13. |
| `TestCheckMachineIDPresentSystemd` | Done | `checks_linux_test.go` | AC-14. |
| `TestCheckCoherenceNilPlatform` | Done | `doctor_test.go` | AC-15. Extended to include `checkMachineID(nil)`. |
| `doctor-platform-coherence` | Done | `test/ui/doctor-platform-coherence.ci` | Functional `ze doctor --json` coverage. |
| `doctor-machine-id` | Added | `test/ui/doctor-machine-id.ci` | Added after review for `doctor-machine-id-missing` coverage. |
| `TestCheckMachineIDIntegration` | Added | `checks_integration_linux_test.go` | QEMU integration for Linux-only machine-id. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/doctor/doctor.go` | Done | Platform threading, NTP clock-sync, systemd skip, path mismatch, DNS path, env test seam. |
| `cmd/ze/doctor/checks_linux.go` | Done | Linux `checkMachineID()` and `currentUID` test seam. |
| `cmd/ze/doctor/checks_other.go` | Done | Non-Linux `checkMachineID()` stub. |
| `cmd/ze/doctor/doctor_test.go` | Done | Unit tests for AC-1 through AC-12 and AC-15 plus code inventory. |
| `cmd/ze/doctor/checks_linux_test.go` | Done | Unit tests for AC-13 and AC-14. |
| `internal/core/diagnostic/codes.go` | Done | Registered three new doctor codes. |
| `test/ui/doctor-platform-coherence.ci` | Done | New functional test. |
| `test/ui/doctor-machine-id.ci` | Changed | Added after `/ze-review` to satisfy functional test gate for `doctor-machine-id-missing`. |
| `cmd/ze/doctor/checks_integration_linux_test.go` | Changed | Extra QEMU integration test required by Linux-only code rule. |
| `test/ui/doctor-service-systemd.ci` | Changed | Existing service test forced to systemd platform. |
| `docs/features.md` | Changed | User-facing feature documentation updated. |

### Audit Summary
- **Total items:** 55
- **Done:** 50
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| NTP coherence gap detected on gokrazy | Unit test + functional test | `TestCheckNTPCoherenceGokrazyNoNTP`; `doctor-platform-coherence.ci` asserts `doctor-clock-no-sync`; targeted `go test` and `ze-test ui` passed. |
| Systemd check skipped on gokrazy | Unit test | `TestCheckSystemdServiceSkipsGokrazy` asserts service unit file is not read; targeted `go test` passed. |
| Path mismatch detected cross-platform | Unit test + functional test | `TestCheckPersistPathMismatchSystemd`, `TestCheckResolvConfMismatchSystemd`; `doctor-platform-coherence.ci` asserts `doctor-config-platform-mismatch`. |
| Machine-id absence detected | Unit test + functional test + QEMU integration test | `TestCheckMachineIDMissingGokrazy`; `doctor-machine-id.ci`; `TestCheckMachineIDIntegration`; QEMU doctor package and UI run passed. |
| Nil platform safe | Unit test | `TestCheckCoherenceNilPlatform` covers NTP, writable path, DNS path, and machine-id nil-platform paths. |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | AC-3 explicitly covered Darwin but not unknown platform. | `cmd/ze/doctor/doctor_test.go` | Added `TestCheckNTPCoherenceUnknownNoNTP`. |
| 2 | NOTE | AC-15 nil-platform test did not include the new machine-id check. | `cmd/ze/doctor/doctor_test.go` | Extended `TestCheckCoherenceNilPlatform` with `checkMachineID(nil)`. |

### Fixes applied
- Added audit-gap unit assertions listed in Run 1.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `checkMachineID` is production-wired, but no `.ci` functional test exercises `ze doctor --json` exposing `doctor-machine-id-missing`. `TestCheckMachineIDIntegration` calls the function directly, which does not satisfy the user-entrypoint functional test gate. | `checkMachineID` in `checks_linux.go`, `runChecks` in `doctor.go` | Add a functional test with a private machine-id path override. |
| 2 | NOTE | The new machine-id functional test is Linux-only and skips on Darwin, as expected for Linux-only `checkMachineID` wiring. | `test/ui/doctor-machine-id.ci` | No action needed. QEMU Linux run executes the test. |

### Fixes applied after Run 2
- Added private Linux-only env override `ze.test.doctor.machine-id-path` in `cmd/ze/doctor/checks_linux.go`.
- Added `TestCheckMachineIDPathOverride` in `cmd/ze/doctor/checks_linux_test.go`.
- Added Linux-only functional test `test/ui/doctor-machine-id.ci`.
- Re-ran `/ze-review`; result: 0 BLOCKER, 0 ISSUE, one Darwin-skip NOTE recorded above.

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ui/doctor-platform-coherence.ci` | Yes | `ls -la cmd/ze/doctor/checks_integration_linux_test.go test/ui/doctor-platform-coherence.ci` returned file entry, size 574. |
| `cmd/ze/doctor/checks_integration_linux_test.go` | Yes | `ls -la cmd/ze/doctor/checks_integration_linux_test.go test/ui/doctor-platform-coherence.ci` returned file entry, size 625. |
| `test/ui/doctor-machine-id.ci` | Yes | File added and executed in QEMU via `go run ./cmd/ze-test ui doctor-machine-id`. |

### AC Verified (grep/test)

All 16 ACs verified by targeted `go test -race ./cmd/ze/doctor` (PASS), `ze-test ui` functional tests (PASS), and QEMU integration for Linux-only paths (PASS). Each AC maps 1:1 to a named unit test in the TDD Plan table above; the AC table in the Acceptance Criteria section records which test demonstrates each AC.

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze doctor --json <config>` | `test/ui/doctor-platform-coherence.ci` | Yes. Line 14 invokes `ze doctor --json platform-coherence.conf`; lines 16-18 assert new codes and JSON `diagnostics`. `ze-test ui` passed. |
| `ze doctor --json <config>` | `test/ui/doctor-machine-id.ci` | Yes. Line 12 invokes `ze doctor --json empty.conf`; lines 14-15 assert `doctor-machine-id-missing` and JSON `diagnostics`. QEMU `ze-test ui doctor-machine-id` passed. |
| `ze doctor --json <config>` | `test/ui/doctor-service-systemd.ci` | Yes. Existing service readiness path still runs when forced to systemd; `ze-test ui` passed. |

### Repo-Level Verification
| Command | Result | Notes |
|---------|--------|-------|
| `go test -race ./cmd/ze/doctor` | PASS | Full doctor package with race detector. |
| `ze-test ui doctor-platform-coherence doctor-service-systemd` | PASS | Functional tests (2/2 locally; `doctor-machine-id` skipped on Darwin). |
| QEMU integration (`go test -tags integration ./cmd/ze/doctor` + `ze-test ui doctor-machine-id`) | PASS | Linux machine-id unit + functional test in VM. |
| `git diff --check` | PASS | No whitespace errors. |
| `make ze-verify` | FAIL | Unrelated pre-existing issues outside doctor. |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`cmd/ze/doctor/`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
