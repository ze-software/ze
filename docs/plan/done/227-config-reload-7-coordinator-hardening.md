# Spec: config-reload-7-coordinator-hardening

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugin/reload.go` — reloadConfig(), verify phase, apply phase
4. `internal/plugin/reload_test.go` — existing coordinator tests
5. `internal/plugin/bgp.go` — handleDaemonReload()

**Parent spec:** `spec-reload-lifecycle-tests.md` (umbrella)
**Depends on:** `spec-config-reload-2-coordinator.md` (coordinator exists), `spec-config-reload-3-sighup.md` (SIGHUP wiring exists)

## Task

Harden the reload coordinator against plugin failures and wire all reload entry points through the coordinator. This spec addresses gaps identified in the critical review of sub-spec 2.

Six changes:

1. **Crashed plugin detection during verify:** When `connB == nil` during verify phase, treat it as a verify error instead of silently skipping. A plugin that registered `WantsConfigRoots` but has no connection is broken — the coordinator must not silently proceed.

2. **Crashed plugin detection during apply:** When `connB == nil` during apply phase, log an error (as today) but also collect it. After apply phase completes, if any plugin was unreachable, return a warning-level error so the caller knows the reload was partial.

3. **Process-alive check before apply:** After all plugins pass verify and before entering apply, re-check that all affected plugin processes still have a live `connB`. If any crashed between verify and apply, abort the reload with an error. This prevents sending apply to a subset of plugins when another has died.

4. **Apply error aggregation:** Currently apply errors are logged but the coordinator returns `nil` (success). Collect apply errors and return them as a combined error. The caller (and ultimately the user) must know if apply was rejected by a plugin.

5. **Wire `handleDaemonReload` through coordinator:** The `ze bgp daemon reload` RPC handler currently calls `ctx.Reactor.Reload()` — the direct path that bypasses the coordinator entirely. When `HasConfigLoader()` is true, it should use the coordinator path (`ReloadFromDisk`) so plugins participate in the reload. Falls back to direct `Reload()` when no config loader is set.

6. **Document crash handling:** Add a "Plugin Crash During Reload" section to the coordinator code (comments in reload.go) documenting the expected behavior for each phase: verify (error), between-verify-and-apply (abort), apply (error aggregation).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor role, plugin communication

### Source Files (MUST read)
- [ ] `internal/plugin/reload.go` — reloadConfig() coordinator logic, verify phase (line 144-168), apply phase (line 170-203)
- [ ] `internal/plugin/reload_test.go` — existing 13 coordinator tests
- [ ] `internal/plugin/bgp.go` — handleDaemonReload() (line 69), bgpCommands table (line 16)
- [ ] `internal/plugin/process.go` — ConnB() (line 151), Process struct
- [ ] `internal/plugin/types.go` — ReactorInterface (line 215), Reload() method

**Key insights:**
- `connB == nil` means the plugin process has died or its socket was closed — always an error during reload
- The verify phase already accumulates errors in `verifyErrors []string` — crashed plugins fit naturally
- The apply phase currently logs errors but returns nil — changing this to return an error is the fix
- `handleDaemonReload` calls `ctx.Reactor.Reload()` which bypasses the coordinator entirely
- `Server.ReloadFromDisk()` is the coordinator entry point for disk-based reload — already exists
- The `CommandContext` struct has a `Server` field that provides access to the coordinator

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/reload.go` — verify phase silently skips connB==nil (line 150-152), apply phase logs errors but returns nil (line 196-201), no process-alive check before apply
- [ ] `internal/plugin/bgp.go` — handleDaemonReload calls ctx.Reactor.Reload() (line 71), bypasses coordinator
- [ ] `internal/plugin/reload_test.go` — no tests for crashed plugin during reload, no tests for apply error propagation

**Behavior to preserve:**
- Successful verify→apply flow unchanged
- No-change detection (empty diff) unchanged
- Per-root filtering unchanged
- Wildcard root handling unchanged
- Concurrent reload rejection unchanged
- SIGHUP signal wiring unchanged (already correct)

**Behavior to change:**
- `connB == nil` during verify → verify error (currently: silent skip)
- `connB == nil` during apply → error collected (currently: silent skip)
- Apply errors → returned to caller (currently: logged, return nil)
- Process-alive check added before apply phase (currently: none)
- `handleDaemonReload` → uses coordinator when available (currently: always direct Reload)

## Data Flow (MANDATORY)

### Entry Point — Three Paths (after this spec)

**Path A (SIGHUP, coordinator active):** SIGHUP → `HasConfigLoader()` true → `server.ReloadFromDisk()` → coordinator → plugins verify/apply → reactor verify/apply

**Path B (SIGHUP, fallback):** SIGHUP → `HasConfigLoader()` false → `adapter.Reload()` → direct reactor reload

**Path C (RPC, coordinator active):** `ze bgp daemon reload` → `handleDaemonReload()` → `HasConfigLoader()` true → `server.ReloadFromDisk()` → coordinator → plugins verify/apply → reactor verify/apply

**Path D (RPC, fallback):** `ze bgp daemon reload` → `handleDaemonReload()` → `HasConfigLoader()` false → `ctx.Reactor.Reload()` → direct reactor reload

### Transformation Path (coordinator changes only)
1. Verify phase: for each affected plugin, check connB is not nil (error if nil), send ConfigVerify
2. **NEW:** After verify passes, re-check all connB are still alive (abort if any nil)
3. Apply phase: for each affected plugin, check connB is not nil (collect error if nil), send ConfigApply, collect errors
4. **CHANGED:** Return combined apply errors to caller instead of nil
5. Reactor apply + SetConfigTree (unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RPC handler → Coordinator | `handleDaemonReload` calls `server.ReloadFromDisk()` when available | [ ] |
| Coordinator → Plugin | ConfigVerify/ConfigApply RPCs via connB | [ ] |
| Coordinator → Reactor | VerifyConfig/ApplyConfigDiff via ReactorInterface | [ ] |

### Integration Points
- `handleDaemonReload` in bgp.go — gains access to Server for coordinator path
- `CommandContext` in command.go — already has `Server *Server` field (or needs it added)
- `reloadConfig()` in reload.go — verify, pre-apply check, apply error aggregation

### Architectural Verification
- [ ] No bypassed layers (all reload paths go through coordinator when available)
- [ ] No unintended coupling (handleDaemonReload uses existing Server methods)
- [ ] No duplicated functionality (reuses ReloadFromDisk, no new coordinator logic)
- [ ] Zero-copy preserved where applicable (N/A)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReloadVerifyCrashedPlugin` | `internal/plugin/reload_test.go` | connB==nil during verify → verify error returned | |
| `TestReloadApplyCrashedPlugin` | `internal/plugin/reload_test.go` | connB==nil during apply → error returned (not nil) | |
| `TestReloadApplyErrorReturned` | `internal/plugin/reload_test.go` | Plugin apply rejection → error returned to caller | |
| `TestReloadProcessDiedBetweenVerifyAndApply` | `internal/plugin/reload_test.go` | Process dies after verify, before apply → reload aborted | |
| `TestDaemonReloadUsesCoordinator` | `internal/plugin/handler_test.go` | handleDaemonReload uses ReloadFromDisk when HasConfigLoader true | |
| `TestDaemonReloadFallsBackToReactor` | `internal/plugin/handler_test.go` | handleDaemonReload uses Reactor.Reload when HasConfigLoader false | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing reload tests | `test/reload/*.ci` | Existing reload behavior unchanged | |
| All existing plugin tests | `test/plugin/*.ci` | Plugin behavior unchanged | |

### Future (if deferring any tests)
- Functional test for plugin crash during reload — requires daemon orchestration + plugin crash simulation

## Files to Modify
- `internal/plugin/reload.go` — verify phase: error on connB==nil; pre-apply alive check; apply phase: error aggregation + return
- `internal/plugin/bgp.go` — handleDaemonReload: use coordinator when HasConfigLoader is true

## Files to Create
- None — all changes are in existing files. New tests go in existing test files.

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write test: verify phase crashes** — Test that when a plugin's connB is nil during verify, the coordinator returns a verify error mentioning the plugin name.
   → **Review:** Does the test set up a Process with nil connB? Does it assert the error message?

2. **Run test** — Verify FAIL (paste output)
   → **Review:** Does it fail for the RIGHT reason (silent skip returning nil)?

3. **Implement verify crash detection** — In the verify loop (reload.go ~line 148-152), when connB is nil, append to verifyErrors instead of continuing silently.
   → **Review:** Is the error message clear? Does it include the plugin name?

4. **Run test** — Verify PASS (paste output)
   → **Review:** Did the existing 13 tests still pass?

5. **Write test: apply error returned** — Test that when a plugin rejects apply (status "error"), the coordinator returns an error instead of nil.
   → **Review:** Does the test verify the running config IS still updated (apply errors are non-fatal for SetConfigTree)?

6. **Run test** — Verify FAIL (paste output)
   → **Review:** Does it fail because apply errors are currently swallowed?

7. **Implement apply error aggregation** — Collect apply errors in a slice. After the apply loop, if errors exist, log them and return a combined error. Still call SetConfigTree and reactor ApplyConfigDiff — the point is to inform the caller, not abort mid-apply.
   → **Review:** Does SetConfigTree still happen? Does reactor ApplyConfigDiff still happen? Only the return value changes.

8. **Run test** — Verify PASS (paste output)
   → **Review:** All existing tests still pass?

9. **Write test: apply phase connB==nil** — Test that when connB becomes nil during apply, the error is collected and returned.
   → **Review:** Similar to verify crash test but in apply phase.

10. **Implement apply phase connB==nil handling** — In the apply loop, when connB is nil, add to apply errors instead of silent continue.

11. **Write test: process died between verify and apply** — Test where process connB is set to nil after verify completes but before apply starts. The coordinator should abort.
    → **Review:** This requires a way to close connB between phases. Consider using a mock that closes after N calls.

12. **Implement pre-apply alive check** — After verify succeeds and before the apply loop, iterate affected plugins and check connB is not nil. If any is nil, return error without entering apply.
    → **Review:** Is this check sufficient? connB could die during apply too — but that's handled by step 10.

13. **Write test: handleDaemonReload coordinator path** — Mock a Server with HasConfigLoader returning true. Verify that handleDaemonReload calls ReloadFromDisk instead of Reactor.Reload.
    → **Review:** Does the CommandContext have access to Server?

14. **Implement handleDaemonReload coordinator path** — In bgp.go, handleDaemonReload checks if the CommandContext has a Server with HasConfigLoader. If yes, calls server.ReloadFromDisk. If no, falls back to ctx.Reactor.Reload().
    → **Review:** Is the Server accessible from CommandContext? If not, what needs to change?

15. **Write test: handleDaemonReload fallback** — Mock with HasConfigLoader returning false. Verify Reactor.Reload is called.

16. **Add crash handling documentation** — Add comments in reload.go documenting crash behavior per phase.

17. **Verify all** — `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? All tests pass?

## RFC Documentation

N/A — no protocol changes.

## Implementation Summary

### What Was Implemented

**Change 1 — Verify crash detection:** `reload.go:158-161` — connB==nil during verify appends to verifyErrors with plugin name.

**Change 2 — Apply crash detection:** `reload.go:204-207` — connB==nil during apply appends to applyErrors with plugin name.

**Change 3 — Pre-apply alive check:** `reload.go:179-192` — re-checks all affected plugin connB after verify phase. If any nil, aborts with error listing dead plugins.

**Change 4 — Apply error aggregation:** `reload.go:251-253` — apply errors collected in slice, returned as combined error. Apply RPC failures and rejections both logged AND collected.

**Change 5 — Reactor apply error collection:** `reload.go:241-243` — reactor ApplyConfigDiff error also collected in applyErrors (was only logged).

**Change 6 — handleDaemonReload coordinator path:** `bgp.go:73-96` — checks `ctx.Server != nil && ctx.Server.HasConfigLoader()`, uses `ReloadFromDisk(ctx.Server.Context())` when available, falls back to `ctx.Reactor.Reload()`.

**Change 7 — Crash handling documentation:** `reload.go:144-155` — consolidated comment block documenting all three crash detection points.

**Wiring changes:**
- `command.go:109` — Added `Server *Server` field to CommandContext
- `server.go:48` — Wired `Server: s` in wrapHandler
- `server.go:923` — Wired `Server: s` in handleUpdateRouteRPC
- `server.go:224-226` — Added `Context()` accessor to Server

### Bugs Found/Fixed
- handleDaemonReload used `context.Background()` instead of server context — fixed to use `ctx.Server.Context()`
- Apply error logging was removed when adding applyErrors collection — restored both logging and collection
- TestReloadApplyCrashedPlugin name was misleading (tested verify, not apply) — renamed to TestReloadVerifyCrashedPluginMultiple

### Investigation → Test Rule
- beforeVerifyRsp hook pattern discovered for deterministic inter-phase testing: hook runs before verify response is sent, blocking the coordinator while test mutates state

### Design Insights
- Only niling the connB pointer (not closing the connection) is necessary for inter-phase tests — closing breaks the in-flight verify response read
- SetConfigTree must still be called after apply errors because the reactor has already applied and the config tree must reflect the new state

### Documentation Updates
- None — no architectural changes, only behavioral hardening of existing coordinator

### Deviations from Plan
- **TestReloadApplyCrashedPlugin renamed** to TestReloadVerifyCrashedPluginMultiple — the test nils connB before reload, so verify catches it, not apply. Name was misleading.
- **TestDaemonReloadNoServer added** — bonus test not in spec, validates fallback when Server is nil
- **Server.Context() accessor added** — needed for handleDaemonReload to use proper server context instead of context.Background()
- **Reactor apply error added to applyErrors** — critical review found reactor ApplyConfigDiff error was only logged, not returned

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| connB==nil during verify → error | ✅ Done | `reload.go:158-161` | Appends to verifyErrors with plugin name |
| connB==nil during apply → error collected | ✅ Done | `reload.go:204-207` | Appends to applyErrors with plugin name |
| Apply errors returned to caller | ✅ Done | `reload.go:251-253` | Combined error after SetConfigTree |
| Pre-apply alive check | ✅ Done | `reload.go:179-192` | Re-checks all connB, aborts if any nil |
| handleDaemonReload uses coordinator | ✅ Done | `bgp.go:74-82` | Uses ReloadFromDisk when HasConfigLoader true |
| handleDaemonReload fallback to Reload | ✅ Done | `bgp.go:84-93` | Falls back to ctx.Reactor.Reload() |
| Crash handling documented in comments | ✅ Done | `reload.go:144-155` | Three crash detection points documented |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReloadVerifyCrashedPlugin | ✅ Done | `reload_test.go:632` | Single crashed plugin, verify error |
| TestReloadApplyCrashedPlugin | 🔄 Changed | `reload_test.go:702` | Renamed to TestReloadVerifyCrashedPluginMultiple — tests verify, not apply |
| TestReloadApplyErrorReturned | ✅ Done | `reload_test.go:669` | Apply rejection returns error, config still updated |
| TestReloadProcessDiedBetweenVerifyAndApply | ✅ Done | `reload_test.go:737` | Uses beforeVerifyRsp hook for deterministic inter-phase |
| TestDaemonReloadUsesCoordinator | ✅ Done | `handler_test.go:2541` | Coordinator path, config loader set |
| TestDaemonReloadFallsBackToReactor | ✅ Done | `handler_test.go:2575` | Fallback path, no config loader |
| TestDaemonReloadNoServer (bonus) | ✅ Done | `handler_test.go:2604` | Nil Server fallback |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/reload.go` | ✅ Modified | Verify crash, pre-apply check, apply error aggregation, crash docs, reactor error collection |
| `internal/plugin/bgp.go` | ✅ Modified | handleDaemonReload coordinator path with fallback |
| `internal/plugin/command.go` | ✅ Modified | Added Server field to CommandContext |
| `internal/plugin/server.go` | ✅ Modified | Context() accessor, Server wiring in wrapHandler + handleUpdateRouteRPC |
| `internal/plugin/reload_test.go` | ✅ Modified | 4 new tests (1 renamed) + mockPluginResponder enhanced |
| `internal/plugin/handler_test.go` | ✅ Modified | 3 new tests + mockReactorReload |

### Audit Summary
- **Total items:** 17
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (TestReloadApplyCrashedPlugin renamed — documented in Deviations)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (fixes specific identified gaps, no new abstractions)
- [x] No speculative features (all changes address concrete review findings)
- [x] Single responsibility (coordinator handles errors, handler routes to coordinator)
- [x] Explicit behavior (errors returned, not swallowed)
- [x] Minimal coupling (uses existing Server.ReloadFromDisk, no new interfaces)
- [x] Next-developer test (clear error messages, documented crash handling)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during implementation)
- [x] Implementation complete
- [x] Tests PASS (all 18 reload tests + 3 handler tests pass)
- [x] Boundary tests cover all numeric inputs (N/A)
- [x] Feature code integrated into codebase (`internal/*`)
- [x] Functional tests verify end-user behavior (existing tests cover regression)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (100% — 96 editor + 4 reload tests)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (N/A)
- [x] RFC references added to code (N/A)
- [x] RFC constraint comments added (N/A)

### Completion (after tests pass - see Completion Checklist)
- [x] Architecture docs updated with learnings (none needed)
- [x] Implementation Audit completed (all items have status + location)
- [x] All Partial/Skipped items have user approval (none)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
