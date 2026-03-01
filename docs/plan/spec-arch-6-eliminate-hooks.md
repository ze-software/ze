# Spec: arch-6 — Eliminate BGPHooks (BLOCKED)

## Status: BLOCKED

This spec documents the final phase of the architecture restructuring. It is **blocked** because eliminating BGPHooks requires the reactor to be wired to the Bus as a Subsystem — which is integration work that modifies existing production code paths, not standalone component building.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-arch-0-system-boundaries.md` — umbrella spec
3. `internal/plugin/types.go` — BGPHooks struct, ReactorLifecycle interface
4. `internal/plugins/bgp/server/hooks.go` — NewBGPHooks constructor

## Task

Eliminate the `BGPHooks` callback injection pattern from `internal/plugin/`. The BGP subsystem should publish events to the Bus directly instead of through `BGPHooks`. After this phase, `internal/plugin/` has zero BGP-specific code.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-arch-0-system-boundaries.md` — umbrella spec, Phase 6 description
  → Decision: BGPHooks disappear — BGP subsystem publishes to bus directly
  → Constraint: `internal/plugin/` must have zero BGP-specific code after this phase
- [ ] `pkg/ze/bus.go` — Bus interface for event delivery
  → Constraint: Bus is content-agnostic, payload is always `[]byte`
- [ ] `pkg/ze/subsystem.go` — Subsystem interface the reactor must implement
  → Constraint: Start(ctx, bus, config) provides Bus reference

### Source Files (existing patterns to follow)
- [ ] `internal/plugin/types.go` — BGPHooks struct (7 callbacks), ReactorLifecycle (17 methods)
  → Constraint: BGPHooks uses `any`-typed parameters to break import cycle
- [ ] `internal/plugin/server_events.go` — 6 delegating wrapper methods
  → Constraint: Each method null-checks bgpHooks then delegates
- [ ] `internal/plugins/bgp/server/hooks.go` — NewBGPHooks constructor
  → Constraint: Creates one JSONEncoder, closes over it in each hook function
- [ ] `internal/plugins/bgp/server/events.go` — actual event dispatch logic
  → Constraint: onMessageReceived, onPeerStateChange, etc. do the real work

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/types.go` — BGPHooks struct with OnMessageReceived, OnMessageBatchReceived, OnPeerStateChange, OnPeerNegotiated, OnMessageSent, BroadcastValidateOpen, CodecRPCHandler
- [ ] `internal/plugin/server_events.go` — 6 methods delegating to bgpHooks
- [ ] `internal/plugin/server_dispatch.go` — 2 CodecRPCHandler call sites
- [ ] `internal/plugins/bgp/server/hooks.go` — NewBGPHooks creates closures around JSONEncoder
- [ ] `internal/plugins/bgp/server/events.go` — event dispatch implementations
- [ ] `internal/plugins/bgp/reactor/reactor.go` — single creation point for BGPHooks

**Behavior to preserve:**
- All BGP events (UPDATE, state change, negotiated, sent) delivered to plugins
- CodecRPC handling (decode-nlri, encode-nlri, etc.)
- BroadcastValidateOpen RPC
- DirectBridge optimization for in-process plugins

**Behavior to change:**
- Event delivery path: BGPHooks callbacks → Bus publish
- `internal/plugin/` has zero BGP-specific code

## Data Flow (MANDATORY)

### Entry Point
- Reactor receives BGP message on wire
- Reactor formats event as payload bytes
- Reactor calls bus.Publish() instead of Server.OnMessageReceived()

### Transformation Path
1. Wire read → message parse → WireUpdate (existing, unchanged)
2. Format payload as `[]byte` (JSON/text/binary)
3. `bus.Publish("bgp/update", payload, metadata)` (replaces BGPHooks call)
4. Bus matches subscriptions, delivers to consumers
5. Plugin receives via `consumer.Deliver(events)`

### Boundaries Crossed

| Boundary | Mechanism | Content |
|----------|-----------|---------|
| Reactor → Bus | `bus.Publish()` | Opaque `[]byte` payload + string metadata |
| Bus → Plugin | `consumer.Deliver()` | Same opaque payload |

### Integration Points
- Reactor must implement `ze.Subsystem` to receive Bus reference
- Plugins must subscribe to Bus topics instead of plugin.Server subscriptions
- CodecRPC handlers must be reachable without BGPHooks indirection

### Architectural Verification
- [ ] No import cycles — Bus breaks the cycle between plugin and bgp/server
- [ ] No BGP-specific code in `internal/plugin/`
- [ ] DirectBridge optimization preserved (Bus internal detail)

## Blocking Dependencies

| Dependency | Why |
|-----------|-----|
| Reactor implements `ze.Subsystem` | Reactor must receive `ze.Bus` at `Start()` |
| Reactor uses Bus for event delivery | Events must flow through Bus, not through `Server.OnMessageReceived()` |
| Import cycle resolution | `internal/plugins/bgp/server` imports `internal/plugin`; eliminating hooks without the Bus would create a reverse import cycle |

## Current Architecture (BGPHooks)

BGPHooks exists to break an import cycle. The `any`-typed parameters exist because `internal/plugins/bgp/server` imports `internal/plugin`, so `internal/plugin` cannot import `internal/plugins/bgp/server`.

## Target Architecture (Bus-based)

With the Bus, the reactor publishes directly to the Bus. Neither side imports the other. The Bus is the intermediary.

## What Needs to Change

| Component | Current | Target |
|-----------|---------|--------|
| `internal/plugin/types.go` | `BGPHooks` struct with 7 callbacks | Remove entirely |
| `internal/plugin/server.go` | `bgpHooks` field, `NewServer` assignment | Remove field |
| `internal/plugin/server_events.go` | 6 methods delegating to hooks | Remove or replace with Bus publish |
| `internal/plugin/server_dispatch.go` | `CodecRPCHandler` via hooks | Direct codec function registry |
| `internal/plugins/bgp/server/hooks.go` | `NewBGPHooks()` constructor | Remove entirely |
| `internal/plugins/bgp/server/events.go` | Event dispatch implementations | Move to reactor Bus publish calls |
| `internal/plugins/bgp/reactor/reactor.go` | Creates BGPHooks in `startAPIServer()` | Remove BGPHooks, use Bus |

## Files Referencing BGPHooks (10 files)

| File | References |
|------|-----------|
| `internal/plugin/types.go` | Type definition + ServerConfig field |
| `internal/plugin/server.go` | Struct field + NewServer |
| `internal/plugin/server_events.go` | 6 delegating methods |
| `internal/plugin/server_dispatch.go` | 2 CodecRPCHandler calls |
| `internal/plugins/bgp/server/hooks.go` | NewBGPHooks constructor |
| `internal/plugins/bgp/server/events.go` | Event dispatch logic |
| `internal/plugins/bgp/reactor/reactor.go` | Creation + injection |
| `internal/plugin/server_test.go` | Test with inline hooks |
| `internal/plugin/server_rpc_test.go` | Test for CodecRPCHandler |
| `internal/plugin/validate_open_test.go` | Test helper for BroadcastValidateOpen |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Reactor publishes to Bus | → | Plugin receives event without BGPHooks | Blocked — requires reactor Subsystem wiring |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP UPDATE received | Plugin receives via Bus, not via BGPHooks |
| AC-2 | `internal/plugin/` grep for BGP | Zero references to BGP types or concepts |
| AC-3 | `internal/plugin/types.go` | No BGPHooks struct |
| AC-4 | All existing functional tests | Pass unchanged |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| Blocked | — | Requires reactor Subsystem wiring | Blocked |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Blocked | — | Requires reactor Subsystem wiring | Blocked |

## Files to Modify

- `internal/plugin/types.go` — remove BGPHooks struct
- `internal/plugin/server.go` — remove bgpHooks field
- `internal/plugin/server_events.go` — remove or replace hook delegation
- `internal/plugin/server_dispatch.go` — replace CodecRPCHandler via hooks
- `internal/plugins/bgp/server/hooks.go` — remove entirely
- `internal/plugins/bgp/server/events.go` — move to reactor Bus publish
- `internal/plugins/bgp/reactor/reactor.go` — remove BGPHooks, use Bus

## Files to Create

- None — this is a removal/restructuring phase

## Implementation Steps

1. **Prerequisite:** Reactor implements `ze.Subsystem` and receives Bus at Start
2. **Prerequisite:** Plugins subscribe to Bus topics
3. **Replace event delivery** — reactor publishes to Bus instead of calling Server event methods
4. **Remove BGPHooks** — delete struct, field, constructor, delegating methods
5. **Migrate CodecRPC** — move handler registry to non-cyclic location
6. **Verify** — all functional tests pass

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import cycle | Verify Bus breaks the cycle |
| Event delivery regression | Check Bus subscription matching |
| CodecRPC broken | Verify handler registry migration |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec
