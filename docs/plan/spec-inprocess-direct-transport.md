# Spec: In-Process Plugin Direct Transport

**Status:** Skeleton — not ready for implementation.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugin/process.go` — deliverBatch(), startInternal()
3. `pkg/plugin/sdk/sdk.go` — callEngineRaw(), eventLoop(), Run()
4. `internal/plugin/inprocess.go` — GetInternalPluginRunner

## Task

Replace JSON + socket transport with direct Go function calls for in-process plugins. Keep socket startup (cold path, 5 round-trips). Switch to direct calls after Stage 5 for the runtime hot path. Expected: 40-60% reduction in IPC-related overhead for internal plugins.

**Root cause:** Profiling shows ~27% CPU in `syscall.rawsyscalln` + ~36% in goroutine scheduling. Each UPDATE requires 2 RPC round-trips per plugin (~12 JSON ops, ~8 pipe I/O ops, ~10 goroutine transitions). For internal plugins (`ze.pluginname`) running as goroutines with `net.Pipe()`, this overhead is unnecessary.

Buffered TCP writes and TCP buffer tuning were tried and reverted — the bottleneck is plugin IPC, not TCP writes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — plugin architecture, 5-stage protocol
  → Constraint: 5-stage startup protocol unchanged (runs over sockets)
  → Decision: Direct transport activates after Stage 5 only
- [ ] `docs/architecture/pool-architecture.md` — pool patterns relevant to batch delivery
  → Constraint: Cache consumer tracking must be preserved

**Key insights:**
- (to be completed during research phase)

## Current Behavior (MANDATORY)

**Source files read:** (must complete before implementation)
- [ ] `internal/plugin/process.go` — deliverBatch(), startInternal(), engine-side delivery + startup
- [ ] `internal/plugin/server_dispatch.go` — dispatchPluginRPC, handleUpdateRouteRPC
- [ ] `internal/plugin/inprocess.go` — GetInternalPluginRunner, wraps runner with net.Pipe
- [ ] `pkg/plugin/sdk/sdk.go` — callEngineRaw(), eventLoop(), Run(), plugin-side transport
- [ ] `pkg/plugin/rpc/conn.go` — CallRPC, CallBatchRPC, current socket transport
- [ ] `internal/plugin/rpc_plugin.go` — PluginConn.SendDeliverBatch, current delivery
- [ ] `internal/ipc/batch.go` — WriteBatchFrame, ParseBatchEvents, current framing

**Behavior to preserve:**
- Fork mode unchanged (external plugins use existing JSON+socket path)
- Plugin code unchanged (same `onEvent`, `UpdateRoute` API)
- Inter-plugin RPCs work (routed through engine dispatcher, direct on both sides)
- 5-stage startup protocol unchanged (runs over sockets)
- Cache consumer tracking preserved (`EventResult.CacheConsumer`)

**Behavior to change:**
- In-process plugin event delivery: socket → direct function call
- In-process plugin RPC dispatch: socket → direct function call
- JSON event strings kept (plugins expect `onEvent(string)` with JSON)

## Data Flow (MANDATORY)

### Entry Point
- Engine `deliverBatch()` sends events to plugins

### Transformation Path (current)
1. Engine formats events as JSON strings via `FormatMessage()`
2. Engine wraps in JSON-RPC envelope, NUL framing
3. Engine writes to net.Pipe (socket I/O)
4. Plugin SDK reads from net.Pipe, parses JSON-RPC envelope
5. Plugin SDK calls `onEvent(string)` with JSON string

### Transformation Path (after direct transport)
1. Engine formats events as JSON strings via `FormatMessage()` (unchanged)
2. Engine calls `bridge.DeliverEvents()` directly (no socket, no envelope)
3. Plugin SDK `onEvent(string)` called with JSON string (unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin (current) | JSON-RPC over net.Pipe + NUL framing | [ ] |
| Engine → Plugin (after) | Direct Go function call via DirectBridge | [ ] |
| Plugin → Engine (current) | JSON-RPC over MuxConn | [ ] |
| Plugin → Engine (after) | Direct Go function call via bridge.DispatchRPC | [ ] |

### Integration Points
- `BridgedConn` wraps `net.Conn` — discovered via type assertion in SDK `NewWithConn`
- `DirectBridge` stored on Process struct for engine-side access
- Both directions independently activatable

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Bridge via net.Conn wrapper | SDK discovers via type assertion in `NewWithConn`. No changes to runner signatures or plugin registration |
| Keep JSON event strings | Plugins expect `onEvent(string)` with JSON. Only transport eliminated, not serialization |
| Phased delivery | Event delivery first (biggest impact), then RPC. Each independently testable |
| Concurrency preserved | `deliveryLoop` goroutine structure stays. Direct calls run in calling goroutine |
| Startup over sockets | 5-stage startup is cold path, negligible overhead. Direct activates after Stage 5 |

## Hot Path Operations Eliminated

| Operation | Current per-UPDATE | After Direct |
|-----------|-------------------|--------------|
| json.Marshal | 4 | 1 (FormatMessage only) |
| json.Unmarshal | 8 | 1 (plugin event parse) |
| net.Pipe I/O | 8 ops | 0 |
| Goroutine transitions | ~10 | ~2 |
| SetWriteDeadline | 4 | 0 |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

## Files to Modify

- `internal/plugin/process.go` — `deliverBatch()`, `startInternal()` — engine-side delivery + startup
- `internal/plugin/server_dispatch.go` — `dispatchPluginRPC`, `handleUpdateRouteRPC` — extract for direct call
- `internal/plugin/inprocess.go` — `GetInternalPluginRunner` — wraps runner with BridgedConn
- `pkg/plugin/sdk/sdk.go` — `callEngineRaw()`, `eventLoop()`, `Run()` — plugin-side transport
- `pkg/plugin/rpc/conn.go` — `CallRPC`, `CallBatchRPC` — current socket transport (preserved)
- `internal/plugin/rpc_plugin.go` — `PluginConn.SendDeliverBatch` — current delivery (bypassed)
- `internal/ipc/batch.go` — `WriteBatchFrame`, `ParseBatchEvents` — current framing (bypassed)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| Plugin SDK docs | [ ] | `.claude/rules/plugin-design.md` — update invocation modes table |
| Functional test | [ ] | Existing functional tests verify behavioral parity |

## Files to Create

- `pkg/plugin/rpc/bridge.go` — DirectBridge struct, BridgedConn type, Bridger interface

## Implementation Steps

### Phase 1: DirectBridge + BridgedConn (Foundation)
1. Create `pkg/plugin/rpc/bridge.go` with `DirectBridge` struct (function pointers + ready channel), `BridgedConn` type (wraps `net.Conn`), `Bridger` interface (type assertion discovery)
2. Modify `internal/plugin/inprocess.go` — create `DirectBridge`, wrap `net.Conn` in `BridgedConn`
3. Modify `internal/plugin/process.go` — `startInternal()` stores `DirectBridge` on Process
4. Modify `pkg/plugin/sdk/sdk.go` — `NewWithConn` checks if engineConn implements `Bridger`, stores bridge

### Phase 2: Engine→Plugin Direct Event Delivery
5. Modify `internal/plugin/process.go` — `deliverBatch()` calls directly when bridge ready instead of `connB.SendDeliverBatch`. Preserve EventResult notification.
6. Modify `pkg/plugin/sdk/sdk.go` — after startup in `Run()`: register `onEvent` handler on bridge, signal ready. `eventLoop` accepts direct-delivered events alongside socket events.

### Phase 3: Plugin→Engine Direct RPC Dispatch
7. Modify `internal/plugin/server_dispatch.go` — extract handler logic into functions callable from socket dispatch and directly
8. Modify `internal/plugin/process.go` — after startup: set `DispatchRPC` on bridge using extracted handler
9. Modify `pkg/plugin/sdk/sdk.go` — `callEngineRaw()` calls directly when bridge has `DispatchRPC`

### Phase 4: Testing + Verification
10. Unit tests for DirectBridge: event round-trip, RPC round-trip, error propagation
11. Run existing functional tests (behavioral parity with socket transport)
12. Benchmark: socket vs direct for deliver-batch throughput
13. Profiling comparison: pprof before vs after

### Failure Routing

| Failure | Route To |
|---------|----------|
| Direct delivery drops events | Check EventResult notification path |
| RPC timeout after switch | Check bridge ready signaling |
| Existing tests fail | Bridge integration issue — check BridgedConn wrapping |
| Benchmark shows no improvement | Profile to identify remaining bottleneck |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Buffered TCP writes (bufio.Writer) | Bottleneck is plugin IPC, not TCP writes | Direct transport bridge |
| TCP buffer tuning (16MB SO_SNDBUF) | Same — wrong bottleneck | Direct transport bridge |

## Design Insights

- ~90-95% IPC overhead eliminated for internal plugins (both directions combined)
- JSON event strings kept for plugin compatibility — only transport layer eliminated
- BridgedConn type assertion pattern keeps SDK and runner signatures unchanged
- Each phase (event delivery, RPC dispatch) is independently testable and deployable

## Implementation Summary

### What Was Implemented
- (pending — spec is deferred)

### Documentation Updates
- (pending)

### Deviations from Plan
- (pending)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC defined and demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/`
- [ ] Spec included in commit
