# Spec: arch-9-plugin-manager-wiring

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-arch-8-config-provider-wiring |
| Phase | - |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/plugin-manager-wiring.md` — two-phase startup design
4. `internal/component/plugin/server/startup.go` — runPluginPhase (lines 202-258 = process spawning, 260+ = handshake)
5. `internal/component/plugin/manager/manager.go` — current stub
6. `cmd/ze/hub/main.go` — nopPluginManager to replace

## Task

Decompose plugin startup into two phases. PluginManager owns process spawning (Phase 1). Server owns protocol handshake (Phase 2). This replaces the last nop stub in the Engine.

### The Split

`runPluginPhase` (startup.go) currently does both phases in one function:
- Lines 202-258: create ProcessManager, TLS acceptor, fork processes → moves to PluginManager
- Lines 260-328: tier computation, 5-stage protocol, async handlers → stays in Server

### Why Two Phases

Engine starts plugins before subsystems: `PluginManager.StartAll()` then `BGPSubsystem.Start()`. But Server (which runs the 5-stage protocol) is created inside the reactor (a subsystem). Two phases solve this:
- Phase 1 (PluginManager.StartAll, before subsystem): spawn processes — no Server needed
- Phase 2 (Server.RunHandshake, during reactor start): 5-stage protocol — Server exists

### Auto-Load

Auto-load discovers plugins for unclaimed families/events/send-types AFTER explicit plugins complete Stage 1 (registration). So auto-load is Phase 2 work — it calls `PluginManager.SpawnMore()` to create new processes, then runs handshake on them.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin-manager-wiring.md` — two-phase startup design with diagrams
  → Decision: Phase 1 = process spawning (PluginManager), Phase 2 = handshake (Server)
  → Decision: auto-load stays in Server, calls PluginManager.SpawnMore for new processes
  → Constraint: TLS acceptor setup needs HubConfig from reactor config
- [ ] `plan/spec-arch-0-system-boundaries.md` — umbrella arch spec
  → Decision: PluginManager owns process management
  → Constraint: Engine starts plugins before subsystems

**Key insights:**
- `runPluginPhase` lines 202-258 are pure process management (no Server dependency except HubConfig)
- `runPluginPhase` lines 260+ are handshake (needs Server: registry, subscriptions, dispatcher)
- Auto-load depends on Stage 1 results (PluginRegistry) — can't move to Phase 1
- ProcessManager is created per auto-load phase (explicit, families, events, send-types)
- PluginManager must support `SpawnMore` for mid-handshake auto-load

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/startup.go` — `runPluginPhase` creates ProcessManager (line 208), TLS acceptor (214-250), forks processes (253), computes tiers (260-270), runs 5-stage per tier (278-318), starts async handlers (323-325)
  → Constraint: TLS acceptor needs s.config.Hub (HubConfig)
  → Constraint: ProcessManager.StartWithContext needs s.ctx
  → Constraint: new ProcessManager per auto-load phase (overwrites s.procManager)
- [ ] `internal/component/plugin/server/startup_autoload.go` — auto-load uses s.config.ConfiguredFamilies and s.registry.LookupFamily() — depends on Stage 1 results
  → Constraint: auto-load runs AFTER explicit plugins' handshake
- [ ] `internal/component/plugin/manager/manager.go` — stub Manager. StartAll marks running, no real work.
  → Constraint: Manager in package `plugin` (import alias needed)
- [ ] `internal/component/plugin/process/manager.go` — ProcessManager: NewProcessManager(configs), StartWithContext, Stop, GetProcess, SetAcceptor
  → Constraint: ProcessManager takes []plugin.PluginConfig at construction
- [ ] `cmd/ze/hub/main.go` — nopPluginManager stub, Engine.NewEngine with stub
  → Constraint: HubConfig available in reactor config (from CreateReactorFromTree)

**Behavior to preserve:**
- 5-stage protocol semantics and tier barrier synchronization
- Auto-load discovery for unclaimed families/events/send-types
- TLS acceptor for external plugin connect-back
- Per-phase ProcessManager creation pattern
- DirectBridge, subscriptions, command registration wiring
- Async RPC handlers after all tiers complete

**Behavior to change:**
- ProcessManager created by PluginManager, not Server
- TLS acceptor setup in PluginManager, not Server
- Process fork/start in PluginManager.StartAll, not Server.runPluginPhase
- Server.runPluginPhase receives processes instead of creating them
- nopPluginManager replaced with real Manager
- Process stop/cleanup via PluginManager.StopAll, not Server.cleanup

## Data Flow (MANDATORY)

### Entry Points

| Source | Entry | Format |
|--------|-------|--------|
| Engine.Start | PluginManager.StartAll(ctx, bus, config) | Context + interfaces |
| reactor.StartWithContext | Server.StartWithContext → handshake | Server internal |

### Transformation Path

1. Engine.Start → PluginManager.StartAll(ctx, bus, config)
2. PluginManager creates ProcessManager from explicit plugin configs
3. PluginManager sets up TLS acceptor if external plugins exist
4. PluginManager forks/starts all processes → returns
5. Engine.Start → BGPSubsystem.Start → reactor.StartWithContext
6. Reactor creates Server, calls Server.StartWithContext
7. Server gets processes from PluginManager (passed via reactor)
8. Server runs 5-stage handshake per tier
9. Server discovers auto-load plugins → calls PluginManager.SpawnMore
10. Server runs handshake for auto-loaded plugins
11. Server signals reactor ready

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine → PluginManager | StartAll(ctx, bus, config) | [ ] |
| PluginManager → ProcessManager | NewProcessManager + StartWithContext | [ ] |
| Reactor → Server | Server.StartWithContext (internal) | [ ] |
| Server → PluginManager | GetProcessManager(), SpawnMore() | [ ] |

### Integration Points
- `internal/component/plugin/manager/manager.go` — PluginManager creates ProcessManager, owns process lifecycle
- `internal/component/plugin/server/startup.go` — Server.runPluginPhase receives processes from PluginManager
- `internal/component/bgp/reactor/reactor.go` — reactor passes PluginManager ref to Server
- `cmd/ze/hub/main.go` — Engine created with real PluginManager

### Architectural Verification
- [ ] No bypassed layers (Engine → PluginManager → processes, Server → handshake)
- [ ] No import cycles (manager/ imports process/, server/ imports process/ — no manager↔server)
- [ ] PluginManager owns process lifecycle, Server owns protocol wiring

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine.Start with real PluginManager | → | PluginManager.StartAll spawns processes | `TestPluginManagerStartAll` |
| Server receives processes from PM | → | 5-stage handshake completes | `test/plugin/announce.ci` |
| Auto-load during handshake | → | PM.SpawnMore creates additional processes | `test/plugin/add-remove.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PluginManager.StartAll with plugin configs | ProcessManager created, processes spawned |
| AC-2 | External plugins in config | TLS acceptor created by PluginManager |
| AC-3 | Server.StartWithContext after PM.StartAll | Server reads processes from PluginManager, runs handshake |
| AC-4 | Auto-load discovers unclaimed families | Server calls PM.SpawnMore, new processes handshaked |
| AC-5 | PluginManager.StopAll | All processes stopped |
| AC-6 | nopPluginManager removed | cmd/ze/hub/main.go uses real Manager |
| AC-7 | `make ze-verify` | All existing tests pass |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginManagerStartAll` | `internal/component/plugin/manager/manager_test.go` | StartAll creates ProcessManager | |
| `TestPluginManagerSpawnMore` | `internal/component/plugin/manager/manager_test.go` | SpawnMore adds processes | |
| `TestPluginManagerStopAll` | `internal/component/plugin/manager/manager_test.go` | StopAll stops ProcessManager | |
| `TestPluginManagerSatisfiesInterface` | `internal/component/plugin/manager/manager_test.go` | var _ ze.PluginManager = (*Manager)(nil) | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-announce` | `test/plugin/announce.ci` | Plugin started via PluginManager, UPDATE sent | |
| `test-add-remove` | `test/plugin/add-remove.ci` | Plugin started, routes added/removed | |

## Files to Modify

- `pkg/ze/plugin.go` — add PluginManager methods (SpawnMore, GetProcessManager, SetHubConfig)
- `internal/component/plugin/manager/manager.go` — real StartAll with process spawning
- `internal/component/plugin/server/startup.go` — runPluginPhase splits: process spawning extracted, handshake receives processes
- `internal/component/plugin/server/server.go` — Server receives PluginManager reference
- `internal/component/bgp/reactor/reactor.go` — pass PluginManager to Server
- `cmd/ze/hub/main.go` — replace nopPluginManager, pass HubConfig to PluginManager

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin-manager-wiring.md` — update after implementation |

## Files to Create

No new files.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Implement | Phases 1-3 below |
| 4-12 | Standard verification + review |

### Implementation Phases

1. **Phase 1: PluginManager process spawning** — Extract ProcessManager creation, TLS acceptor, and process forking from `runPluginPhase` into PluginManager.StartAll. Add SpawnMore for auto-load. Add GetProcessManager for Server.
   - Tests: `TestPluginManagerStartAll`, `TestPluginManagerSpawnMore`, `TestPluginManagerStopAll`, `TestPluginManagerSatisfiesInterface`
   - Files: `pkg/ze/plugin.go`, `manager.go`

2. **Phase 2: Wire Server to receive processes from PluginManager** — Server.runPluginPhase uses PluginManager's ProcessManager instead of creating its own. Reactor passes PluginManager reference to Server. Auto-load calls PluginManager.SpawnMore.
   - Files: `startup.go`, `server.go`, `reactor.go`
   - Verify: functional tests pass

3. **Phase 3: Wire hub + remove stub** — Replace nopPluginManager in cmd/ze/hub/main.go with real Manager. Pass HubConfig.
   - Files: `cmd/ze/hub/main.go`
   - Verify: `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify |
|-------|---------------|
| Completeness | Every AC-N has implementation |
| Correctness | runPluginPhase no longer creates ProcessManager — uses PluginManager's |
| Correctness | Auto-load calls SpawnMore, handshakes new processes |
| Correctness | TLS acceptor created in PluginManager, not Server |
| No import cycle | manager/ does not import server/ |
| No duplication | Process spawning code exists in ONE place (PluginManager) |
| No layering | nopPluginManager fully removed |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Real PluginManager in hub | `grep 'pluginmgr.NewManager' cmd/ze/hub/main.go` |
| No nop stubs | `grep -c 'nopPluginManager' cmd/ze/hub/main.go` returns 0 |
| ProcessManager in PluginManager | `grep 'ProcessManager' internal/component/plugin/manager/manager.go` |
| SpawnMore exists | `grep 'SpawnMore' internal/component/plugin/manager/manager.go` |
| `make ze-verify` passes | Run and paste output |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| TLS | Acceptor cert generation unchanged, token auth preserved |
| Process count | Bounded by config, no unbounded spawning |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import cycle | Rethink package placement |
| Process not found by Server | Check PM reference passing through reactor |
| Auto-load spawns wrong plugins | Compare registry snapshot logic |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| StartupHooks with RunPluginStartup | Server not created when PluginManager.StartAll fires | Two-phase: spawn first, handshake after Server exists |
| 11-method StartupCallbacks | Interleaved call sites don't map to clean interface | Server keeps handshake internally, PluginManager owns spawning |

## Design Insights

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Architecture docs updated
- [ ] Critical Review passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-arch-9-plugin-manager-wiring.md`
- [ ] Summary included in commit
