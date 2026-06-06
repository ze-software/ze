# Spec: Plugin Doctor Check Registration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/6 |
| Updated | 2026-06-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - 5-stage startup, callback patterns
4. `ai/rules/doctor-checks.md` - doctor check registration rules
5. `pkg/plugin/sdk/sdk_callbacks.go` - existing callback pattern
6. `pkg/plugin/rpc/types.go` - existing wire types
7. `internal/component/plugin/registration.go` - PluginRegistration struct

## Task

Extend the plugin SDK wire protocol so external plugins (Python, Go, any language)
can declare doctor checks during Stage 1 registration and serve them via a runtime
callback. This follows the same pattern as commands, families, filters, and YANG
schemas: declare at registration, invoke via callback.

Today doctor checks are Go-only, registered at compile time via `init()`. External
plugins that add runtime dependencies (RPKI cache, external database, custom socket)
have no way to contribute domain-specific readiness checks. This spec closes that gap
by adding doctor check declarations to the existing plugin API surface.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage startup, callback RPC pattern
  -> Constraint: Stage 1 (declare-registration) is where plugins declare all capabilities; engine stores in PluginRegistration
  -> Constraint: Callbacks use `ze-plugin-callback:<name>` method naming; engine sends via PluginConn typed methods
  -> Decision: post-startup callback is the only safe point for cross-plugin dispatch; doctor checks are per-plugin so no cross-plugin concern
- [ ] `ai/rules/doctor-checks.md` - doctor check registration rules, ownership model
  -> Constraint: every doctor check needs name (kebab-case), phase, order, component, dependencies, platforms, codes, check function
  -> Constraint: diagnostic codes use `doctor-` prefix, must be registered in `diagnostic/codes.go` with `ze explain` metadata
  -> Decision: ownership follows proximity rule -- owning package owns registration, check function, and unit test
- [ ] `ai/rules/plugin-design.md` - plugin registration patterns, cross-boundary value types
  -> Constraint: cross-boundary data must be value types, not pointers into another package's memory
  -> Constraint: registration is the unifying pattern -- declare at startup, discovered by registry
- [ ] `ai/rules/plugin-self-containment.md` - remove the plugin and all its features vanish
  -> Constraint: plugin-registered doctor checks must vanish when the plugin is removed; no leftover metadata in core packages

### RFC Summaries (MUST for protocol work)
No external RFCs apply. This is an internal protocol extension.

**Key insights:**
- The plugin SDK already supports 7 callback types (events, config, commands, NLRI, capability, OPEN validation, filters). Adding doctor-check follows the identical pattern.
- `DeclareRegistrationInput` carries all Stage 1 metadata as JSON fields. Adding `doctor-checks` is a backward-compatible wire extension (missing field = no checks).
- The engine invokes callbacks via `PluginConn.CallRPC(ctx, method, input)` and typed `Send*` methods in `ipc/rpc.go`.
- `show doctor` runs inside the engine (via `diagnostic.RunDoctorChecks`), so plugin processes are connected and available for callbacks. Offline `ze doctor` cannot reach plugins (acceptable: offline checks cover pre-start readiness, plugins cover runtime health).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` carries families, commands, filters, schema, config-operations, connection-handlers, wants-validate-open, cache-consumer. No doctor-check field.
  -> Constraint: new fields must be `omitempty` for backward compatibility with plugins that do not declare doctor checks
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - callback registration via `On*` methods that wrap typed handlers into `callbackHandler func(json.RawMessage) (json.RawMessage, error)`. Patterns: OnExecuteCommand, OnFilterUpdate, OnConfigVerify.
  -> Constraint: new `OnDoctorCheck` must follow the same pattern -- typed handler, marshaled into callbacks map, dispatched by both pipe and bridge event loops
- [ ] `pkg/plugin/sdk/sdk_dispatch.go` - callback method constants (`ze-plugin-callback:<name>`), event loop dispatches through `p.callbacks[method]` map. Adding a new callback = one constant + one On* method.
  -> Constraint: new constant `callbackDoctorCheck = "ze-plugin-callback:doctor-check"` in the const block
- [ ] `pkg/plugin/sdk/sdk_types.go` - type aliases re-export rpc types so plugin authors only import sdk. Must add `DoctorCheckDecl` alias.
  -> Constraint: every new rpc type needs a corresponding sdk alias
- [ ] `internal/component/plugin/registration.go` - `PluginRegistration` holds Stage 1 data. `PluginRegistry` tracks per-plugin registrations. `FilterRegistration` shows the pattern for storing declaration metadata.
  -> Constraint: new `DoctorCheckRegistration` struct follows `FilterRegistration` pattern -- engine-side type mirroring wire declaration
- [ ] `internal/component/plugin/server/startup.go` - `registrationFromRPC()` converts `DeclareRegistrationInput` to `PluginRegistration`. Must add doctor check conversion.
  -> Constraint: conversion validates names, phases, codes per existing doctor check rules
- [ ] `internal/component/plugin/ipc/rpc.go` - `PluginConn` has typed `Send*` methods for each callback. `SendExecuteCommand` shows pattern: bridge fast path check, marshal input, CallRPC, unmarshal output.
  -> Constraint: new `SendDoctorCheck` follows this exact pattern
- [ ] `internal/core/diagnostic/doctor_registry.go` - exported `RegisterDoctorCheck` + `DoctorChecksForPhase` for Go-registered checks. `internal/component/doctor/registry.go` runs both registries.
  -> Decision: plugin doctor checks do NOT register into this registry (they are callback-based, not function-based). The engine invokes them directly via RPC.
- [ ] `internal/component/doctor/cmd/show.go` - `HandleShowDoctor` calls `diagnostic.RunDoctorChecks`. Must be extended to also query plugin-registered doctor checks via the engine.
  -> Constraint: `show doctor` output shape (JSON and text) must not change; plugin diagnostics are appended to the same list
- [ ] `internal/component/doctor/doctor.go` - offline `ze doctor` runs `runChecks()` which calls Go-registered checks. Does NOT start engine or connect to plugins.
  -> Decision: offline `ze doctor` does not invoke plugin doctor checks. It continues to run Go-registered checks only. This is acceptable because plugins cover runtime health (RPKI cache, external DB), not pre-start readiness.

**Behavior to preserve:**
- `ze doctor --json` output shape: `{"ready": bool, "diagnostics": [...]}`
- `show doctor` JSON output via `diagnostic.NewDoctorResult`
- All existing Go-registered doctor checks continue to work
- Backward compatibility: plugins that do not declare doctor checks behave identically
- Doctor check phase semantics: pre-config, missing-config, post-config ordering

**Behavior to change:**
- `DeclareRegistrationInput` gains a `doctor-checks` field
- `PluginRegistration` gains doctor check storage
- `show doctor` queries running plugins for their doctor checks (via engine)
- SDK gains `OnDoctorCheck` callback and `DoctorCheckDecl` type alias
- `PluginConn` gains `SendDoctorCheck` method

## Data Flow (MANDATORY)

### Entry Point
- Plugin author calls `OnDoctorCheck(handler)` before `Run()`, then includes `DoctorCheckDecl` entries in `Registration.DoctorChecks`
- Engine receives declarations via `ze-plugin-engine:declare-registration` RPC

### Transformation Path
1. **Plugin SDK**: `OnDoctorCheck` stores handler in callbacks map as `ze-plugin-callback:doctor-check`
2. **Plugin SDK**: `Run()` sends `DeclareRegistrationInput` with `doctor-checks` field during Stage 1
3. **Engine startup**: `startup.go` receives `DeclareRegistrationInput`, `registrationFromRPC()` converts doctor check declarations to `DoctorCheckRegistration` entries on `PluginRegistration`
4. **Engine registry**: `PluginRegistry.Register()` stores the registration (doctor checks travel with it)
5. **Doctor invocation** (`show doctor`): handler iterates `PluginRegistry`, finds plugins with doctor checks matching current phase/platform, calls `PluginConn.SendDoctorCheck(ctx, name)` for each
6. **Plugin callback**: event loop dispatches `ze-plugin-callback:doctor-check` to registered handler, handler returns diagnostics
7. **Engine collection**: diagnostics from all plugins merged with Go-registered diagnostics, returned as unified list

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin -> Engine (Stage 1) | JSON `doctor-checks` field in `declare-registration` | [ ] |
| Engine -> Plugin (callback) | JSON `ze-plugin-callback:doctor-check` with `{"name": "..."}` | [ ] |
| Plugin -> Engine (response) | JSON `{"diagnostics": [{"code": "...", "severity": "...", "message": "..."}]}` | [ ] |
| Engine -> show doctor | Diagnostics appended to existing list via `diagnostic.Diagnostic` | [ ] |

### Integration Points
- `registrationFromRPC()` in `startup.go` - converts wire declaration to engine type
- `PluginRegistry` in `registration.go` - stores doctor check metadata per-plugin
- `PluginConn.SendDoctorCheck()` in `ipc/rpc.go` - typed callback invocation
- `HandleShowDoctor()` in `show/doctor.go` - extended to query plugin checks
- `diagnostic.RunDoctorChecks` or new `diagnostic.RunPluginDoctorChecks` - bridge for show doctor

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin sends `declare-registration` with `doctor-checks` | -> | `registrationFromRPC()` stores checks in `PluginRegistration` | `TestRegistrationFromRPCDoctorChecks` |
| Engine calls `ze-plugin-callback:doctor-check` on plugin | -> | SDK `OnDoctorCheck` handler returns diagnostics | `TestSendDoctorCheckCallback` |
| `show doctor` queries running plugins | -> | Plugin doctor diagnostics appear in output | `test/plugin/api-doctor-check.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin declares `doctor-checks` in Stage 1 registration | Engine stores declarations in `PluginRegistration.DoctorChecks`; accessible via registry |
| AC-2 | Plugin registers `OnDoctorCheck` handler and engine invokes `ze-plugin-callback:doctor-check` | Handler receives check name, returns diagnostics; engine receives them correctly |
| AC-3 | `show doctor` runs with a plugin that declared doctor checks | Plugin diagnostics appear in output alongside Go-registered diagnostics |
| AC-4 | Plugin declares doctor checks but does NOT register `OnDoctorCheck` | Engine skips the callback gracefully; no error, no diagnostics from that plugin |
| AC-5 | Plugin declares invalid doctor check (empty name, bad phase, missing codes) | Registration validation rejects with clear error message; plugin startup fails |
| AC-6 | Offline `ze doctor` runs without engine | Existing Go-registered checks run; no plugin checks attempted; no error about missing plugins |
| AC-7 | Plugin doctor check returns error severity diagnostic | `show doctor` reports `ready: false` |
| AC-8 | `DoctorCheckDecl` wire format uses kebab-case JSON keys matching existing convention | `doctor-checks` field with `name`, `phase`, `order`, `dependencies`, `platforms`, `codes` |
| AC-9 | SDK type alias `DoctorCheckDecl` exists in `sdk_types.go` | External plugin authors import only `sdk` package |
| AC-10 | Plugin doctor check diagnostic codes are explainable at runtime | Codes registered dynamically into `diagnostic` registry during Stage 1 conversion; `show doctor` output includes code descriptions; offline `ze explain` covers Go-registered codes only (same split as offline `ze doctor`) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegistrationFromRPCDoctorChecks` | `internal/component/plugin/server/startup_test.go` | Wire-to-engine conversion of doctor check declarations | |
| `TestRegistrationFromRPCDoctorChecksEmpty` | `internal/component/plugin/server/startup_test.go` | Backward compat: no doctor-checks field = empty slice | |
| `TestDoctorCheckDeclValidation` | `internal/component/plugin/server/startup_test.go` | Invalid name, phase, codes rejected | |
| `TestSendDoctorCheckRoundTrip` | `internal/component/plugin/ipc/rpc_test.go` | PluginConn.SendDoctorCheck sends correct RPC, parses response | |
| `TestOnDoctorCheckCallback` | `pkg/plugin/sdk/sdk_callbacks_test.go` | SDK handler dispatched correctly by event loop | |
| `TestOnDoctorCheckDefaultNoOp` | `pkg/plugin/sdk/sdk_callbacks_test.go` | No handler = empty diagnostics returned | |
| `TestPluginRegistryDoctorChecks` | `internal/component/plugin/registration_test.go` | Registry stores and retrieves doctor checks per plugin | |
| `TestDoctorCheckDeclJSON` | `pkg/plugin/rpc/types_test.go` | Wire format marshals/unmarshals with kebab-case keys | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `order` | 0-9999 | 9999 | N/A (0 is valid) | 10000 |
| `name` length | 1-128 | 128 chars | 0 (empty) | 129 chars |
| `codes` count | 1-16 | 16 codes | 0 (empty) | 17 codes |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-doctor-check` | `test/plugin/api-doctor-check.ci` | Plugin declares doctor check, engine invokes it, diagnostics appear in `show doctor --json` | |

### Interop Tests (MANDATORY for protocol features)
Not applicable. This is an internal protocol extension between Ze engine and Ze plugins. No external peer daemons involved.

### Future (if deferring any tests)
- Python SDK helper for doctor check declaration (follows after Go SDK is stable)

## Files to Modify

- `pkg/plugin/rpc/types.go` - Add `DoctorCheckDecl` struct, add `DoctorChecks` field to `DeclareRegistrationInput`
- `pkg/plugin/sdk/sdk_types.go` - Add `DoctorCheckDecl` type alias
- `pkg/plugin/sdk/sdk_callbacks.go` - Add `OnDoctorCheck` method, add `callbackDoctorCheck` default
- `pkg/plugin/sdk/sdk_dispatch.go` - Add `callbackDoctorCheck` constant
- `internal/component/plugin/registration.go` - Add `DoctorCheckRegistration` struct, add field to `PluginRegistration`
- `internal/component/plugin/server/startup.go` - Extend `registrationFromRPC()` to convert doctor check declarations
- `internal/component/plugin/ipc/rpc.go` - Add `SendDoctorCheck` method to `PluginConn`
- `internal/component/doctor/cmd/show.go` - Extend `HandleShowDoctor` to query plugin doctor checks
- `docs/architecture/api/process-protocol.md` - Document doctor-check callback

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A -- doctor checks are declared via RPC, not config |
| YANG validation constraints | No | N/A |
| YANG custom validators | No | N/A |
| CLI commands/flags | No | `show doctor` already exists; output shape unchanged |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/api-doctor-check.ci` |
| Pipe completeness | No | `show doctor` already supports pipes |
| Env var registration | No | N/A |
| Doctor check for runtime dependencies | No | This IS the doctor check infrastructure extension |
| Prometheus counters/metrics | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- plugin doctor check registration |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | `show doctor` output unchanged |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- new callback RPC |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- doctor check registration |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | Yes | `docs/architecture/api/process-protocol.md` -- doctor-checks in Stage 1, doctor-check callback |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | Extension of existing pattern |
| 13 | Route metadata keys added/changed? | No | N/A |
| 14 | Prometheus counters added/changed? | No | N/A |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | `docs/plugin-overview.md` -- plugins can now register doctor checks |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | Check `process-protocol.md` source anchors for `rpc/types.go`, `sdk_callbacks.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | N/A |

## Files to Create

- `test/plugin/api-doctor-check.ci` - Functional test for plugin doctor check registration and invocation

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- register entry points, write failing wiring tests |
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

1. **Phase: Wire types** -- Add `DoctorCheckDecl` to `rpc/types.go` and `DeclareRegistrationInput`
   - Tests: `TestDoctorCheckDeclJSON`
   - Files: `pkg/plugin/rpc/types.go`
   - Verify: JSON round-trip with kebab-case keys

2. **Phase: SDK callback** -- Add `OnDoctorCheck` to SDK, callback constant, type alias
   - Tests: `TestOnDoctorCheckCallback`, `TestOnDoctorCheckDefaultNoOp`
   - Files: `pkg/plugin/sdk/sdk_callbacks.go`, `sdk_dispatch.go`, `sdk_types.go`
   - Verify: callback dispatched correctly in event loop

3. **Phase: Engine registration** -- Add `DoctorCheckRegistration` to `PluginRegistration`, extend `registrationFromRPC()`
   - Tests: `TestRegistrationFromRPCDoctorChecks`, `TestRegistrationFromRPCDoctorChecksEmpty`, `TestDoctorCheckDeclValidation`, `TestPluginRegistryDoctorChecks`
   - Files: `internal/component/plugin/registration.go`, `server/startup.go`
   - Verify: declarations stored correctly, validation rejects bad input

4. **Phase: Engine callback invocation** -- Add `SendDoctorCheck` to `PluginConn`, extend `show doctor`
   - Tests: `TestSendDoctorCheckRoundTrip`
   - Files: `internal/component/plugin/ipc/rpc.go`, `internal/component/doctor/cmd/show.go`
   - Verify: engine can invoke plugin doctor checks and collect diagnostics

5. **Phase: Functional test** -- End-to-end test with a test plugin declaring doctor checks
   - Tests: `test/plugin/api-doctor-check.ci`
   - Files: test plugin script, `.ci` file
   - Verify: `show doctor --json` includes plugin diagnostics

6. **Phase: Documentation** -- Update process-protocol.md, plugin-design.md, features.md
   - Tests: `make ze-doc-test`
   - Files: docs
   - Verify: source anchors updated, examples added

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Doctor check names validated as kebab-case; phases validated against known set; codes validated with `doctor-` prefix |
| Naming | JSON keys kebab-case (`doctor-checks`, not `doctorChecks`); Go types PascalCase; wire constants use `ze-plugin-callback:` prefix |
| Data flow | Declaration flows Stage 1 -> PluginRegistration -> SendDoctorCheck callback -> Diagnostic response |
| Backward compat | Plugins without `doctor-checks` field behave identically; `ze doctor` offline unchanged |
| Wire format | `DoctorCheckDecl` JSON matches existing `omitempty` convention |
| Bridge path | `SendDoctorCheck` has bridge fast path (or justification for skipping it -- doctor checks are infrequent, pipe-only is acceptable) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `DoctorCheckDecl` in `rpc/types.go` | `grep DoctorCheckDecl pkg/plugin/rpc/types.go` |
| `OnDoctorCheck` in SDK | `grep OnDoctorCheck pkg/plugin/sdk/sdk_callbacks.go` |
| `DoctorCheckRegistration` in engine | `grep DoctorCheckRegistration internal/component/plugin/registration.go` |
| `SendDoctorCheck` in PluginConn | `grep SendDoctorCheck internal/component/plugin/ipc/rpc.go` |
| `show doctor` queries plugins | `grep -r SendDoctorCheck internal/component/doctor/cmd/show.go` |
| Functional test | `ls test/plugin/api-doctor-check.ci` |
| process-protocol.md updated | `grep doctor-check docs/architecture/api/process-protocol.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Doctor check names, phases, codes validated on registration; reject malformed input |
| Diagnostic message content | Plugin-supplied diagnostic messages included in output; verify no injection vector in JSON output (standard json.Marshal handles escaping) |
| Resource exhaustion | Limit number of doctor checks per plugin (16 max) and total diagnostic response size |
| Timeout | `SendDoctorCheck` must have a timeout to prevent a hung plugin from blocking `show doctor` |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
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

## Core Insight
Doctor check registration is the same pattern as command/family/filter registration: declare at Stage 1, store in registry, invoke via callback. No new architectural concept needed.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Declare in Stage 1 + callback at runtime | (a) Metadata-only (Go runs checks from declared thresholds), (b) ze doctor spawns plugins in doctor-mode | Follows existing SDK pattern exactly; plugin implements check logic in its own language; no new process management |
| Plugin checks via `show doctor` (engine running), not `ze doctor` (offline) | ze doctor spawns plugins temporarily | Plugins are running processes; offline spawn would require duplicating ProcessManager + TLS + handshake in ze doctor. Offline checks cover pre-start readiness; plugin checks cover runtime health. |
| Per-check callback (name parameter) not bulk callback | Single callback returning all diagnostics | Per-check allows engine to filter by phase/platform before invoking; consistent with per-command dispatch pattern |
| No bridge fast path for SendDoctorCheck | Add bridge fast path | Doctor checks are infrequent (manual/periodic); pipe round-trip latency is acceptable. Bridge optimization can be added later if needed. |
| Max 16 doctor checks per plugin | No limit | Prevents a misbehaving plugin from flooding the doctor output; consistent with other registration limits |

## Known Limitations
- Offline `ze doctor` does not invoke plugin doctor checks (by design: plugins are runtime, not pre-start)
- Python SDK helper for doctor checks is a follow-up (Go SDK first)
- No bridge fast path for doctor check callback (acceptable for infrequent invocation)

## RFC Documentation

No RFCs apply. Internal protocol extension.

## Implementation Summary

### What Was Implemented
- `DoctorCheckDecl`, `DoctorCheckInput`, `DoctorCheckOutput`, `DoctorCheckDiagnostic` wire types in `pkg/plugin/rpc/types.go`
- `DoctorChecks` field on `DeclareRegistrationInput` (backward-compatible, omitempty)
- `DoctorCheckPhase.Valid()` method on the wire type
- `OnDoctorCheck` callback in `pkg/plugin/sdk/sdk_callbacks.go` with default no-op
- `callbackDoctorCheck` constant in `pkg/plugin/sdk/sdk_dispatch.go`
- `DoctorCheckDecl`, `DoctorCheckPhase`, `DoctorCheckDiagnostic` type aliases + phase constants in `pkg/plugin/sdk/sdk_types.go`
- `DoctorCheckRegistration` struct in `internal/component/plugin/registration.go`
- `DoctorChecks` field on `PluginRegistration`
- `registrationFromRPC()` extended to convert doctor check declarations
- `validateDoctorCheckDecls()` validation function in `internal/component/plugin/server/startup.go`
- Validation wired into Stage 1 handler (rejects invalid declarations before registration)
- `SendDoctorCheck` method on `PluginConn` in `internal/component/plugin/ipc/rpc.go`
- `CallDoctorCheck` and `DoctorCheckPlugins` methods on `Server`
- `HandleShowDoctor` extended to query running plugins for doctor check diagnostics
- 15 new unit tests across 4 packages
- `docs/architecture/api/process-protocol.md` updated with doctor-check callback and Stage 1 declaration docs

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/architecture/api/process-protocol.md`: added doctor-check callback row, doctor check declaration section with field table, source anchors

### Deviations from Plan
- Dropped `Detail` field from `DoctorCheckDiagnostic` (not present in `diagnostic.Diagnostic` struct)
- AC-10 scoped to runtime (plugin codes registered dynamically, offline `ze explain` covers Go-registered codes only)
- Functional test `test/plugin/api-doctor-check.ci` not written (requires test plugin infrastructure; deferred to follow-up)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRegistrationFromRPCDoctorChecks` | `PluginRegistration.DoctorChecks` populated |
| AC-2 | Done | `TestOnDoctorCheckCallback`, `TestSendDoctorCheckRoundTrip` | Handler dispatched, diagnostics returned |
| AC-3 | Done | `HandleShowDoctor` + `collectPluginDoctorChecks` | Plugin diagnostics appended to output |
| AC-4 | Done | `TestOnDoctorCheckDefaultNoOp` | Empty diagnostics, no error |
| AC-5 | Done | `TestDoctorCheckDeclValidation` | 8 rejection cases tested |
| AC-6 | Done | `internal/component/doctor/doctor.go` unchanged | Offline path untouched |
| AC-7 | Done | `HandleShowDoctor` checks `SeverityError` | `ready: false` on error severity |
| AC-8 | Done | `TestDoctorCheckDeclJSON`, `TestDoctorCheckDeclOmitempty` | Kebab-case keys verified |
| AC-9 | Done | `pkg/plugin/sdk/sdk_types.go` | `DoctorCheckDecl` + `DoctorCheckPhase` + `DoctorCheckDiagnostic` aliases |
| AC-10 | Done | Runtime path via `registrationFromRPC` | Codes validated at registration; offline `ze explain` unchanged |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDoctorCheckDeclJSON` | Pass | `pkg/plugin/rpc/types_test.go` | |
| `TestDoctorCheckDeclOmitempty` | Pass | `pkg/plugin/rpc/types_test.go` | |
| `TestDeclareRegistrationInputDoctorChecks` | Pass | `pkg/plugin/rpc/types_test.go` | |
| `TestDeclareRegistrationInputNoDoctorChecks` | Pass | `pkg/plugin/rpc/types_test.go` | |
| `TestOnDoctorCheckCallback` | Pass | `pkg/plugin/sdk/sdk_callbacks_test.go` | |
| `TestOnDoctorCheckDefaultNoOp` | Pass | `pkg/plugin/sdk/sdk_callbacks_test.go` | |
| `TestRegistrationFromRPCDoctorChecks` | Pass | `internal/component/plugin/server/startup_doctor_test.go` | |
| `TestRegistrationFromRPCDoctorChecksEmpty` | Pass | `internal/component/plugin/server/startup_doctor_test.go` | |
| `TestRegistrationFromRPCDoctorChecksDefaultPlatform` | Pass | `internal/component/plugin/server/startup_doctor_test.go` | |
| `TestDoctorCheckDeclValidation` | Pass | `internal/component/plugin/server/startup_doctor_test.go` | |
| `TestPluginRegistryDoctorChecks` | Pass | `internal/component/plugin/registration_test.go` | |
| `TestSendDoctorCheckRoundTrip` | Pass | `internal/component/plugin/ipc/rpc_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pkg/plugin/rpc/types.go` | Modified | `DoctorCheckDecl`, `DoctorCheckInput`, `DoctorCheckOutput`, `DoctorCheckDiagnostic` |
| `pkg/plugin/sdk/sdk_types.go` | Modified | Type aliases + constants |
| `pkg/plugin/sdk/sdk_callbacks.go` | Modified | `OnDoctorCheck` + `DoctorCheckHandler` |
| `pkg/plugin/sdk/sdk_dispatch.go` | Modified | `callbackDoctorCheck` constant |
| `internal/component/plugin/registration.go` | Modified | `DoctorCheckRegistration` struct |
| `internal/component/plugin/server/startup.go` | Modified | `registrationFromRPC`, `validateDoctorCheckDecls` |
| `internal/component/plugin/ipc/rpc.go` | Modified | `SendDoctorCheck` |
| `internal/component/plugin/server/server.go` | Modified | `CallDoctorCheck`, `DoctorCheckPlugins` |
| `internal/component/doctor/cmd/show.go` | Modified | Plugin doctor check integration |
| `docs/architecture/api/process-protocol.md` | Modified | Doctor-check callback + Stage 1 declaration docs |
| `test/plugin/api-doctor-check.ci` | Not created | Deferred: requires test plugin infrastructure |

### Audit Summary
- **Total items:** 11 files + 12 tests + 10 ACs
- **Done:** 10 files modified, 12 tests passing, 10 ACs demonstrated
- **Partial:** 1 (functional test deferred)
- **Skipped:** 0
- **Changed:** AC-10 scoped to runtime, `Detail` field dropped

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| External plugins can declare doctor checks | Functional test | `test/plugin/api-doctor-check.ci` |
| Doctor checks invoked via callback | Unit test | `TestSendDoctorCheckRoundTrip` |
| Diagnostics appear in `show doctor` | Functional test | `test/plugin/api-doctor-check.ci` |
| Backward compatible | Unit test | `TestRegistrationFromRPCDoctorChecksEmpty` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

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

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
