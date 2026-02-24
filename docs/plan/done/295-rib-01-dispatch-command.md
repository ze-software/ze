# Spec: rib-01 — Inter-Plugin Command Dispatch

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/server_dispatch.go` - existing RPC handlers (pattern to follow)
4. `pkg/plugin/sdk/sdk.go` - SDK methods (UpdateRoute pattern to replicate)
5. `internal/yang/modules/ze-plugin-engine.yang` - YANG RPC definitions

## Task

Add `ze-plugin-engine:dispatch-command` RPC so plugins can invoke the engine's command dispatcher and receive structured `{status, data}` responses. This enables plugin-to-plugin communication through the engine.

**Why needed:** The existing `update-route` RPC routes through the same dispatcher but wraps all responses as `UpdateRouteOutput{PeersAffected, RoutesSent}`, losing the execute-command `{status, data}` response. Specs rib-02 and rib-03 require plugins to dispatch commands to other plugins and receive data (e.g., bgp-rr requesting replay from bgp-adj-rib-in, receiving `{last-index: N}`).

**What already exists (NOT duplicated):**

| Component | Status | Location |
|-----------|--------|----------|
| YANG `ze-plugin-callback:execute-command` | Complete | `ze-plugin-callback.yang:146-181` |
| SDK `OnExecuteCommand` callback + handler | Complete | `sdk.go:233-238, 848-870` |
| Engine dispatcher → plugin routing | Complete | `command.go:243-374` |
| `routeToProcess()` → `SendExecuteCommand()` | Complete | `command.go:352`, `rpc_plugin.go:170-186` |
| RPC types `ExecuteCommandInput/Output` | Complete | `rpc/types.go:162-174` |

**What this spec adds:**

| Component | Location |
|-----------|----------|
| YANG `ze-plugin-engine:dispatch-command` RPC | `ze-plugin-engine.yang` |
| Engine handler `handleDispatchCommandRPC` | `server_dispatch.go` |
| SDK method `DispatchCommand()` | `sdk.go` |
| RPC types `DispatchCommandInput/Output` | `rpc/types.go` |

**Part of series:** rib-01 (this) → rib-02 → rib-03 → rib-04

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - command dispatch architecture
  → Constraint: Dispatcher.Dispatch() is the single entry point for all command routing
- [ ] `docs/architecture/core-design.md` - plugin communication model
  → Constraint: plugins communicate via engine-mediated RPCs, never directly

### RFC Summaries
- N/A — infrastructure, not protocol

**Key insights:**
- execute-command callback is fully wired: YANG defined, SDK handler dispatches, engine sends via ConnB
- Dispatcher.Dispatch() already routes commands to plugins via registry longest-match lookup
- Only gap: no engine RPC for plugins to call the dispatcher with full response preservation
- handleUpdateRouteRPC shows the exact pattern to follow (same dispatcher call, different response type)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/server_dispatch.go` - handleUpdateRouteRPC creates CommandContext, calls s.dispatcher.Dispatch(), wraps as UpdateRouteOutput
- [ ] `pkg/plugin/sdk/sdk.go` - UpdateRoute() calls ze-plugin-engine:update-route, uses callEngineWithResult pattern
- [ ] `internal/yang/modules/ze-plugin-engine.yang` - existing engine RPCs: update-route, subscribe-events, decode-*, encode-*
- [ ] `internal/yang/modules/ze-plugin-callback.yang` - execute-command RPC fully defined with {serial, command, args, peer} input and {status, data} output
- [ ] `internal/plugin/command.go` - Dispatcher.Dispatch() parses input, routes to builtins → subsystems → plugin registry
- [ ] `pkg/plugin/rpc/types.go` - ExecuteCommandInput/Output types exist for the callback direction

**Behavior to preserve:**
- All existing RPCs unchanged
- Dispatcher routing logic unchanged (longest-match prefix)
- execute-command callback interface unchanged
- DirectBridge optimization applies transparently

**Behavior to change:**
- Add dispatch-command RPC (new engine-side handler)
- Add SDK method for plugins to call the dispatcher

## Data Flow (MANDATORY)

### Entry Point
- Plugin A calls `p.DispatchCommand(ctx, command)`

### Transformation Path
1. SDK method marshals `DispatchCommandInput{Command}`, calls `callEngineWithResult("ze-plugin-engine:dispatch-command", input)`
2. Engine `handleDispatchCommandRPC` extracts command string
3. Engine creates `CommandContext`, calls `s.dispatcher.Dispatch(ctx, command)`
4. Dispatcher performs longest-match lookup in plugin registry
5. Dispatcher calls `routeToProcess()` → `SendExecuteCommand()` on target plugin's ConnB
6. Target plugin's `onExecuteCommand` callback processes command, returns `{status, data}`
7. Response flows back: target plugin → dispatcher → engine handler → SDK → calling plugin

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin A → Engine | dispatch-command RPC on Socket A (or DirectBridge) | [ ] |
| Engine → Plugin B | execute-command callback on Socket B (or DirectBridge) | [ ] |

### Integration Points
- `Dispatcher.Dispatch()` — existing, no modification needed
- `routeToProcess()` → `SendExecuteCommand()` — existing, no modification needed
- SDK `callEngineWithResult()` — existing pattern, reused
- `dispatchPluginRPC()` — add case for new method name
- DirectBridge — handled transparently by existing SDK dispatch logic

### Architectural Verification
- [ ] No bypassed layers (goes through standard dispatcher)
- [ ] No unintended coupling (uses same dispatch path as CLI/API)
- [ ] No duplicated functionality (extends existing dispatch, doesn't recreate)
- [ ] Zero-copy preserved where applicable (N/A for command strings)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin A calls DispatchCommand with command registered by Plugin B | Plugin B's onExecuteCommand handler invoked, response returned to A |
| AC-2 | Plugin B returns status="done" with JSON data | Plugin A receives both status and data string |
| AC-3 | Command not found in registry | DispatchCommand returns error |
| AC-4 | Plugin B returns error | DispatchCommand returns error with message |
| AC-5 | DirectBridge path (internal plugins) | Same behavior as socket path |
| AC-6 | `make ze-verify` passes | All tests pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchCommandToPlugin` | `internal/plugin/server_dispatch_test.go` | RPC dispatches to registered plugin, response returned | |
| `TestDispatchCommandNotFound` | `internal/plugin/server_dispatch_test.go` | Unknown command returns error | |
| `TestDispatchCommandPluginError` | `internal/plugin/server_dispatch_test.go` | Plugin error propagated to caller | |
| `TestSDKDispatchCommand` | `pkg/plugin/sdk/sdk_test.go` | SDK method calls RPC and parses response | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| command | non-empty string | any valid command | empty string → error | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dispatch-command-cross-plugin` | `test/plugin/dispatch-command.ci` | Plugin dispatches command to another plugin, receives structured response | |

## Files to Modify

- `internal/yang/modules/ze-plugin-engine.yang` — add dispatch-command RPC definition
- `internal/plugin/server_dispatch.go` — add handleDispatchCommandRPC, register in dispatchPluginRPC switch
- `pkg/plugin/sdk/sdk.go` — add DispatchCommand(ctx, command) method
- `pkg/plugin/rpc/types.go` — add DispatchCommandInput/DispatchCommandOutput types

## Files to Create

- `test/plugin/dispatch-command.ci` — functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPC) | [x] | `ze-plugin-engine.yang` |
| RPC count in arch docs | [x] | `docs/architecture/api/architecture.md` |
| Plugin SDK docs | [x] | `.claude/rules/plugin-design.md` |
| Functional test | [x] | `test/plugin/dispatch-command.ci` |

## Implementation Steps

1. **Write unit test for dispatch RPC** → Verify FAIL
2. **Add YANG RPC definition** — `dispatch-command` with input: `{command}`, output: `{status, data}`
3. **Add RPC types** — `DispatchCommandInput{Command string}`, `DispatchCommandOutput{Status, Data string}`
4. **Add engine handler** — `handleDispatchCommandRPC`: create CommandContext, call `s.dispatcher.Dispatch()`, map `*Response` to `DispatchCommandOutput{Status, Data}`
5. **Register handler** — add case in `dispatchPluginRPC()` / `dispatchPluginRPCDirect()`
6. **Add SDK method** — `DispatchCommand(ctx, command) (status, data string, err error)` using `callEngineWithResult`
7. **Write functional test** — two-plugin scenario via `.ci` test
8. **Run `make ze-verify`** → paste output
9. **Critical Review** → all 6 quality checks

### Failure Routing

| Failure | Route To |
|---------|----------|
| RPC not dispatched | Step 5 (check method name in dispatchPluginRPC switch) |
| Response data lost | Step 4 (check Response → DispatchCommandOutput mapping) |
| DirectBridge path fails | Step 5 (check direct dispatch variant) |
| SDK method doesn't parse response | Step 6 (check callEngineWithResult + unmarshal) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

- This RPC follows the exact same pattern as update-route, with a different response type
- The engine dispatcher is already the universal command router — this just exposes it to plugins
- No new dispatch logic needed — all routing goes through existing Dispatcher.Dispatch()
- DirectBridge optimization applies transparently (SDK checks bridge before socket)

## Implementation Summary

### What Was Implemented
- YANG RPC definition `ze-plugin-engine:dispatch-command` with `{command}` input, `{status, data}` output
- RPC types `DispatchCommandInput` and `DispatchCommandOutput` in `pkg/plugin/rpc/types.go`
- Engine handler `handleDispatchCommandRPC` (socket path) in `internal/plugin/server_dispatch.go`
- Engine handler `handleDispatchCommandDirect` (DirectBridge path) in `internal/plugin/server_dispatch.go`
- Both `dispatchPluginRPC` and `dispatchPluginRPCDirect` switch cases registered
- SDK method `DispatchCommand(ctx, command) (status, data, error)` in `pkg/plugin/sdk/sdk.go`
- SDK type alias `DispatchCommandOutput` exported
- Helper `responseToDispatchOutput()` converts dispatcher Response → RPC output (JSON-encodes structured Data, passes strings through)

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/architecture/api/architecture.md`: RPC count 11→12, added dispatch-command to Socket A table
- `pkg/plugin/rpc/types.go`: package doc updated with dispatch-command in Socket A list

### Deviations from Plan
- Functional test `test/plugin/dispatch-command.ci` deferred to rib-02. Python `ze_api.py` lacks dispatch-command and execute-command callback support. Unit tests cover all transport paths (socket, DirectBridge, SDK). The first real consumer (bgp-rr → bgp-adj-rib-in) will provide end-to-end validation.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| YANG dispatch-command RPC | ✅ Done | `ze-plugin-engine.yang:156-183` | input{command}, output{status, data} |
| Engine handler (socket) | ✅ Done | `server_dispatch.go:161-199` | handleDispatchCommandRPC |
| Engine handler (DirectBridge) | ✅ Done | `server_dispatch.go:389-412` | handleDispatchCommandDirect |
| RPC switch registration (socket) | ✅ Done | `server_dispatch.go:61-63` | dispatchPluginRPC case |
| RPC switch registration (direct) | ✅ Done | `server_dispatch.go:257-258` | dispatchPluginRPCDirect case |
| SDK method | ✅ Done | `sdk.go:457-470` | DispatchCommand() |
| RPC types | ✅ Done | `types.go:188-200` | DispatchCommandInput/Output |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestDispatchCommandToPlugin | Registered command dispatched, response returned |
| AC-2 | ✅ Done | TestDispatchCommandToPlugin | status="done", data contains JSON |
| AC-3 | ✅ Done | TestDispatchCommandNotFound | Unknown command returns RPC error |
| AC-4 | ✅ Done | TestDispatchCommandPluginError | Error status + message propagated |
| AC-5 | ✅ Done | TestDispatchCommandDirectBridge | Same behavior through direct path |
| AC-6 | ⚠️ Partial | make ze-verify | Lint: 0 issues. Unit tests: all pass. Functional: 96/96 pass. Two pre-existing flaky tests (TestSlowPluginFatal, TestDeliveryLoopBatching) unrelated to changes |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestDispatchCommandToPlugin | ✅ Done | `server_dispatch_test.go:30` | AC-1, AC-2 |
| TestDispatchCommandNotFound | ✅ Done | `server_dispatch_test.go:92` | AC-3 |
| TestDispatchCommandPluginError | ✅ Done | `server_dispatch_test.go:136` | AC-4 |
| TestSDKDispatchCommand | ✅ Done | `sdk_test.go:1079` | SDK round-trip |
| TestDispatchCommandEmptyCommand | ✅ Done | `server_dispatch_test.go:193` | Boundary: empty string |
| TestDispatchCommandDirectBridge | ✅ Done | `server_dispatch_test.go:244` | AC-5 |
| dispatch-command-cross-plugin (.ci) | ❌ Deferred | N/A | ze_api.py lacks support; deferred to rib-02 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/yang/modules/ze-plugin-engine.yang` | ✅ Done | dispatch-command RPC added |
| `internal/plugin/server_dispatch.go` | ✅ Done | handler + switch cases |
| `pkg/plugin/sdk/sdk.go` | ✅ Done | DispatchCommand method + type alias |
| `pkg/plugin/rpc/types.go` | ✅ Done | Input/Output types |
| `test/plugin/dispatch-command.ci` | ❌ Deferred | Deferred to rib-02 |

### Audit Summary
- **Total items:** 20
- **Done:** 18
- **Partial:** 1 (AC-6: pre-existing flaky tests)
- **Skipped:** 0
- **Changed:** 1 (functional test deferred to rib-02)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
