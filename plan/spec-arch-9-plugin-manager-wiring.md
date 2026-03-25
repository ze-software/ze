# Spec: arch-9-plugin-manager-wiring

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-arch-8-config-provider-wiring |
| Phase | 1/4 |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/plugin-manager-wiring.md` — target architecture with Mermaid diagrams
4. `internal/component/plugin/server/server.go` — Server struct (god object being decomposed)
5. `internal/component/plugin/server/startup.go` — 5-stage protocol (to move)
6. `internal/component/plugin/manager/manager.go` — current PluginManager stub
7. `pkg/ze/plugin.go` — PluginManager interface
8. `cmd/ze/hub/main.go` — startup path (nopPluginManager to replace)

## Task

Extract plugin lifecycle management from `pluginserver.Server` into `PluginManager`, replacing the last nop stub in the Engine. The Server becomes an API surface (event delivery, command dispatch, subscriptions) that queries PluginManager for process and registry state.

The migration uses a **StartupHooks** interface as the seam: PluginManager drives the 5-stage protocol, Server provides wiring at specific points within each stage.

### Critical Review Finding: Real Interface Surface

The 5-stage handler (`handleProcessStartupRPC`, 220 lines) calls 9+ Server methods at interleaved points within stages, not at clean stage boundaries. The hooks interface must match the actual call sites:

| Call site | Within stage | Server method | Purpose |
|-----------|-------------|---------------|---------|
| Stage 1 complete | 1 | `RegisterInRegistry(reg)` | Validate registration, detect conflicts |
| Stage 1 complete | 1 | `RegisterCacheConsumer(name, unordered)` | Track cache participation |
| Between 1→2 | 2 | `GetConfigForPlugin(proc)` | Config subtree for plugin |
| Stage 3 complete | 3 | `AddCapabilities(caps)` | Store for OPEN injection |
| Between 3→4 | 4 | `GetRegistryForPlugin(proc)` | Command info list |
| Stage 5 ready | 5 | `RegisterSubscriptions(proc, subs)` | Event subscription setup |
| Stage 5 ready | 5 | `WireBridgeDispatch(proc)` | DirectBridge callback wiring |
| Stage 5 ready | 5 | `RegisterCommands(proc, defs)` | Dispatcher command setup |
| Stage 5 done | 5 | `SignalAPIReady()` | Notify reactor one plugin is ready |
| All tiers done | post | `StartAsyncHandlers(procs)` | Start runtime RPC goroutines |
| All phases done | post | `SignalStartupComplete()` | Notify reactor all plugins done |

### Design Decision: Full Extraction

`handleProcessStartupRPC` and supporting functions (`stageTransition`, `progressThroughStages`, `runPluginPhase`, `runPluginStartup`, `deliverConfigRPC`, `deliverRegistryRPC`) move from Server to PluginManager. Server implements StartupHooks with the 11 methods above.

### Design Decision: ProcessManager per Phase

`runPluginPhase` creates a new ProcessManager per auto-load phase (explicit → families → events → send-types). Each phase overwrites `s.procManager`. PluginManager must handle this: either create per-phase or accumulate processes.

Four implementation phases:

1. **Define StartupHooks interface** — 11 methods, Server implements, no behavior change
2. **Move ProcessManager + startup functions** — `handleProcessStartupRPC`, `stageTransition`, `progressThroughStages`, `runPluginPhase` move to PluginManager
3. **Move auto-load + phase sequencing** — `runPluginStartup`, auto-load discovery move to PluginManager
4. **Move registry + capabilities + wire hub** — PluginManager owns PluginRegistry, CapabilityInjector. Remove nopPluginManager from hub/main.go

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin-manager-wiring.md` — target architecture with diagrams
  → Decision: StartupCallbacks is the seam between PluginManager and Server
  → Decision: Server queries PluginManager for process/registry state
  → Constraint: 5-stage protocol ordering and tier barriers must be preserved
- [ ] `docs/architecture/subsystem-wiring.md` — Engine/Bus/Subsystem wiring
  → Constraint: Bus is notification layer, PluginManager receives Bus at StartAll
- [ ] `plan/spec-arch-0-system-boundaries.md` — umbrella arch spec
  → Decision: PluginManager owns 5-stage protocol, process management, DirectBridge
  → Constraint: PluginManager receives Bus + ConfigProvider at StartAll

### Completed Phases
- [ ] `plan/learned/325-arch-3-plugin-manager.md` — PluginManager built as stub
  → Decision: Manager struct with minimal tracking, full implementation deferred
- [ ] `plan/learned/328-arch-6-eliminate-hooks.md` — EventDispatcher replaces BGPHooks
  → Constraint: EventDispatcher calls Server methods for subscription/process queries

**Key insights:**
- Server currently owns ProcessManager, StartupCoordinator, PluginRegistry, CapabilityInjector
- `handleProcessStartupRPC` (220 lines) calls 9+ Server methods at interleaved points — not clean stage boundaries
- `runPluginPhase` creates a NEW ProcessManager per auto-load phase (overwrites previous)
- Auto-load in startup_autoload.go discovers plugins for unclaimed families/events/send-types
- Tier execution: plugins grouped by dependency, tier N completes before tier N+1 starts
- After all tiers: async `handleSingleProcessCommandsRPC` goroutines start per process — Server-owned
- `wireBridgeDispatch` and command registration are deeply Server-internal (Dispatcher access)
- StartupHooks decouples protocol execution (PluginManager) from Server wiring (11 methods)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/server.go` — Server struct with 15+ fields. Constructor takes ServerConfig + ReactorLifecycle. Start() creates ProcessManager, runs startup phases, starts async RPC handlers. Stop() cancels context + cleanup.
  → Constraint: Server.Start() creates ProcessManager from config.Plugins
  → Constraint: Server holds reactor ref for config delivery, peer queries, cache consumer registration
  → Constraint: Server.Dispatcher() used by EventDispatcher for command routing
- [ ] `internal/component/plugin/server/startup.go` — startPluginTier() runs 5-stage protocol per tier. progressThroughStages() drives one plugin. stageTransition() handles each stage. Config delivery at Stage 2, capability collection at Stage 3, command/subscription wiring at Stage 5.
  → Constraint: stageTransition calls s.reactor methods (GetConfigTree, RegisterCacheConsumer)
  → Constraint: Stage 5 wires DirectBridge, subscriptions, commands — all Server-owned
  → Constraint: barrier synchronization via StartupCoordinator per tier
- [ ] `internal/component/plugin/server/startup_autoload.go` — 4-phase auto-load: explicit, families, events, send-types. Uses registry.Snapshot() to find plugins for unclaimed resources.
  → Constraint: auto-load creates additional tiers after explicit plugins
  → Constraint: auto-load needs ConfiguredFamilies, ConfiguredCustomEvents, ConfiguredCustomSendTypes from reactor config
- [ ] `internal/component/plugin/server/startup_coordinator.go` — per-tier barrier synchronization. StageComplete/WaitForStage pattern. Tracks stage progress per plugin index.
  → Constraint: coordinator is created per tier with plugin count
  → Constraint: PluginFailed aborts the tier
- [ ] `internal/component/plugin/process/manager.go` — ProcessManager creates and tracks plugin processes. StartWithContext forks/goroutines. Stop sends SIGTERM. Wait blocks until done.
  → Constraint: ProcessManager created from []ProcessConfig (derived from reactor config)
  → Constraint: ProcessManager.SetAcceptor for TLS external plugin connect-back
- [ ] `internal/component/plugin/manager/manager.go` — current PluginManager stub. Register/StartAll/StopAll minimal. Stores bus/config refs.
  → Constraint: StartAll currently marks plugins as running, nothing else
- [ ] `pkg/ze/plugin.go` — PluginManager interface: Register, StartAll, StopAll, Plugin, Plugins, Capabilities
  → Constraint: StartAll signature is (ctx, Bus, ConfigProvider) error

**Behavior to preserve:**
- 5-stage protocol semantics (declaration → configure → capabilities → registry → ready)
- Tier-based barrier synchronization (tier N done before tier N+1)
- Auto-load discovery for unclaimed families/events/send-types
- DirectBridge optimization for in-process plugins
- Per-process subscription registration
- Command registration in Dispatcher
- Cache consumer tracking
- Respawn limits and disabled process handling
- TLS acceptor for external plugin connect-back
- Config delivery from ConfigProvider (now real, not stub)

**Behavior to change:**
- PluginManager creates/owns ProcessManager (not Server)
- PluginManager drives 5-stage protocol (not Server.Start)
- PluginManager owns PluginRegistry and CapabilityInjector (not Server)
- Server receives process/registry references from PluginManager
- nopPluginManager stub in cmd/ze/hub/main.go replaced with real implementation
- StartupCallbacks interface defines the seam

## Data Flow (MANDATORY)

### Entry Points

| Source | Entry | Format |
|--------|-------|--------|
| Engine.Start() | PluginManager.StartAll(ctx, bus, config) | Context + interfaces |
| Plugin process | Socket A: registration RPC | JSON over mux connection |
| Config | ConfigProvider.Get("bgp") | map[string]any |

### Transformation Path

**Startup (changed):**
1. Engine.Start() calls PluginManager.StartAll(ctx, bus, config)
2. PluginManager creates ProcessManager from registered plugin configs
3. PluginManager creates StartupCoordinator per tier
4. For each tier: drive 5-stage protocol, call StartupCallbacks at each stage
5. Server (implementing StartupCallbacks) wires subscriptions, commands, DirectBridge
6. PluginManager signals OnStartupComplete, Server signals reactor

**Event delivery (unchanged):**
1. Reactor → EventDispatcher → Server.Subscriptions().GetMatching() → per-process delivery
2. Server queries PluginManager.ProcessManager() for process references

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine → PluginManager | StartAll(ctx, bus, config) | [ ] |
| PluginManager → Server | StartupCallbacks methods | [ ] |
| Server → PluginManager | ProcessManager(), Plugin(), Capabilities() queries | [ ] |
| PluginManager → Processes | 5-stage protocol over socket | [ ] |

### Integration Points
- `pkg/ze/plugin.go` — StartupCallbacks interface added
- `internal/component/plugin/manager/manager.go` — real implementation
- `internal/component/plugin/server/server.go` — implements StartupCallbacks, receives ProcessManager
- `cmd/ze/hub/main.go` — wire real PluginManager

### Architectural Verification
- [ ] No bypassed layers (startup goes through PluginManager, not Server directly)
- [ ] No unintended coupling (PluginManager has no BGP knowledge)
- [ ] No duplicated functionality (startup logic moves, not copied)
- [ ] Tier ordering and barrier semantics preserved

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine.Start() with real PluginManager | → | PluginManager.StartAll() creates ProcessManager | `TestEngineStartsPluginManager` |
| PluginManager.StartAll() with plugins | → | 5-stage protocol completes via StartupCallbacks | `TestPluginManagerDrivesStartup` |
| Server implements StartupCallbacks | → | OnReady wires subscriptions + commands | `TestServerImplementsCallbacks` |
| Config with plugin section | → | Plugins started via PluginManager | `test/plugin/announce.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | StartupCallbacks interface defined | Server can implement it without changing public API |
| AC-2 | PluginManager.StartAll() called by Engine | ProcessManager created, processes started |
| AC-3 | 5-stage protocol driven by PluginManager | Each stage completes, callbacks fire at correct points |
| AC-4 | Tier barrier synchronization | Tier N plugins all reach stage X before any reaches X+1 |
| AC-5 | Auto-load for unclaimed families | Plugins discovered and loaded in additional tiers |
| AC-6 | Server.OnReady called at Stage 5 | Subscriptions, commands, DirectBridge wired |
| AC-7 | Server queries PluginManager | ProcessManager(), Plugin(), Capabilities() return correct data |
| AC-8 | nopPluginManager removed | cmd/ze/hub/main.go uses real PluginManager |
| AC-9 | `make ze-verify` | All existing tests pass |
| AC-10 | Config delivery at Stage 2 | Plugin receives config from ConfigProvider (real, not stub) |
| AC-11 | Capability injection at Stage 3 | Capabilities stored in PluginManager, queryable |
| AC-12 | External plugin TLS connect-back | ProcessManager.SetAcceptor still works |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartupCallbacksInterface` | `internal/component/plugin/server/server_test.go` | Server satisfies StartupCallbacks interface | |
| `TestPluginManagerStartAll` | `internal/component/plugin/manager/manager_test.go` | StartAll creates ProcessManager, drives 5-stage | |
| `TestPluginManagerStopAll` | `internal/component/plugin/manager/manager_test.go` | StopAll stops ProcessManager | |
| `TestPluginManagerProcessQuery` | `internal/component/plugin/manager/manager_test.go` | Plugin(name) returns correct process | |
| `TestPluginManagerCapabilities` | `internal/component/plugin/manager/manager_test.go` | Capabilities() returns collected caps from Stage 3 | |
| `TestTierBarrier` | `internal/component/plugin/manager/manager_test.go` | Tier N completes before tier N+1 starts | |
| `TestAutoLoadFamilies` | `internal/component/plugin/manager/manager_test.go` | Unclaimed families trigger auto-load | |
| `TestOnReadyWiresSubscriptions` | `internal/component/plugin/server/server_test.go` | OnReady registers subscriptions in SubscriptionManager | |
| `TestOnReadyWiresCommands` | `internal/component/plugin/server/server_test.go` | OnReady registers commands in Dispatcher | |
| `TestEngineStartsPluginManager` | `internal/component/bgp/subsystem/subsystem_test.go` | Engine.Start passes real PluginManager | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric inputs.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-announce` | `test/plugin/announce.ci` | Plugin started via PluginManager, sends UPDATE | |
| `test-add-remove` | `test/plugin/add-remove.ci` | Plugin started, routes added/removed | |

### Future (if deferring any tests)
- External plugin TLS connect-back test — requires TLS infrastructure in test harness
- Respawn limit test — requires process crash simulation

## Files to Modify

- `pkg/ze/plugin.go` — add StartupCallbacks interface
- `internal/component/plugin/manager/manager.go` — real StartAll/StopAll implementation
- `internal/component/plugin/server/server.go` — implement StartupCallbacks, receive ProcessManager from PluginManager
- `internal/component/plugin/server/startup.go` — extract stage logic into PluginManager-callable functions
- `cmd/ze/hub/main.go` — replace nopPluginManager with real PluginManager

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | `.claude/rules/plugin-design.md` — document StartupCallbacks |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin-manager-wiring.md` — update after implementation |

## Files to Create

No new files — all changes are to existing files.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan — check what exists |
| 3. Implement | Phases 1-4 below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: StartupHooks interface** — Define 11-method interface in `pkg/ze/plugin.go`. Server implements it by wrapping existing methods. No behavior change — Server still drives startup.
   - Tests: `TestStartupHooksInterface`
   - Files: `pkg/ze/plugin.go`, `server.go`
   - Verify: compile + existing tests pass

2. **Phase 2: Move startup functions to PluginManager** — Move `handleProcessStartupRPC`, `stageTransition`, `progressThroughStages`, `runPluginPhase` from Server to PluginManager. These functions call StartupHooks instead of `s.` Server methods. Server.Start delegates to PluginManager.
   - Tests: `TestPluginManagerStartAll`, `TestPluginManagerStopAll`, `TestPluginManagerProcessQuery`
   - Files: `manager.go`, `startup.go`, `server.go`
   - Verify: all tests pass, all functional tests pass

3. **Phase 3: Move auto-load + phase sequencing** — Move `runPluginStartup`, `getUnclaimedFamilyPlugins`, `getUnclaimedEventTypePlugins`, `getUnclaimedSendTypePlugins` to PluginManager. PluginManager owns the 4-phase sequence (explicit → families → events → send-types) and accumulates ProcessManagers across phases.
   - Tests: `TestAutoLoadFamilies`, `TestTierBarrier`, `TestMultiPhaseProcessAccumulation`
   - Files: `manager.go`, `startup.go`, `startup_autoload.go`, `server.go`
   - Verify: all tests pass, all functional tests pass

4. **Phase 4: Move registry + capabilities + wire hub** — PluginManager owns PluginRegistry and CapabilityInjector. Server no longer creates them. Remove nopPluginManager from hub/main.go.
   - Tests: `TestPluginManagerCapabilities`, `TestEngineStartsPluginManager`
   - Files: `manager.go`, `server.go`, `cmd/ze/hub/main.go`
   - Verify: `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | 5-stage protocol ordering preserved exactly — compare against original handleProcessStartupRPC line by line |
| Correctness | Tier barrier synchronization preserved — coordinator per-tier, indices per-tier |
| Correctness | ProcessManager-per-phase pattern preserved — each auto-load phase creates new PM |
| Correctness | Auto-load discovers same plugins as before — registry snapshot logic unchanged |
| Correctness | Async handlers start AFTER all tiers complete, not per-tier |
| Correctness | wireBridgeDispatch and command registration happen BEFORE ready OK response |
| Data flow | StartupHooks called at correct interleaved points within stages (11 call sites) |
| Rule: no-layering | Old startup functions in Server deleted, not duplicated |
| Rule: no-layering | nopPluginManager fully removed |
| Import cycle | PluginManager must not import Server — only uses StartupHooks interface |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| StartupCallbacks interface defined | `grep 'StartupCallbacks' pkg/ze/plugin.go` |
| Server implements StartupCallbacks | `grep 'StartupCallbacks' internal/component/plugin/server/server.go` |
| PluginManager.StartAll real implementation | `grep 'ProcessManager\|StartupCoordinator' internal/component/plugin/manager/manager.go` |
| No nop stubs in hub | `grep -c 'nopPluginManager' cmd/ze/hub/main.go` returns 0 |
| `make ze-verify` passes | Run and paste output |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Plugin registration: validate name uniqueness, family format |
| Resource exhaustion | Process count bounded by config (no unbounded spawning) |
| Privilege | Plugin processes inherit privilege drop (unchanged) |
| TLS | External plugin connect-back auth preserved |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Stage ordering wrong | Phase 3 — verify callback timing against startup.go |
| Tier barrier broken | Phase 3 — verify coordinator usage matches original |
| Auto-load different | Phase 4 — compare registry snapshots |
| Functional test fails | Check if startup sequence changed — compare with original |
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

## RFC Documentation

No RFC work — internal architecture refactoring.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
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
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-arch-9-plugin-manager-wiring.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
