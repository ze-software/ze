# Spec: bgp-as-plugin -- BGP as a Config-Driven Plugin

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 2/4 |
| Updated | 2026-04-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `cmd/ze/hub/main.go` -- current reactor/engine wiring
3. `internal/component/iface/register.go` -- iface plugin pattern (target pattern)
4. `internal/component/bgp/reactor/reactor.go` -- reactor responsibilities
5. `internal/component/plugin/server/startup.go` -- plugin startup phases
6. `plan/learned/524-fib-config-autoload.md` -- ConfigRoots auto-loading mechanism

## Task

Make BGP a config-driven plugin, identical in lifecycle to iface. If `bgp { }` is not in config, BGP does not load. If it is, BGP loads via ConfigRoots auto-loading. This allows ze to run as an interface-only manager, a FIB-only route installer, or a full BGP daemon depending on config.

Currently, the BGP reactor is always created and registered as a subsystem in `cmd/ze/hub/main.go`. The reactor hosts the plugin server, config reload coordinator, and event dispatcher. Making BGP optional requires extracting these roles into the engine so they run independently of BGP.

### User-Facing Config (no change)

```
bgp {
    peer upstream {
        ...
    }
}
```

BGP loads automatically. Remove the `bgp { }` section and BGP doesn't load.

### Architecture Change

**Current:**
```
Engine
  +-- BGP Subsystem (reactor -- always created)
        +-- Plugin Server (hosted by reactor)
        +-- Config Reload (hosted by reactor)
        +-- Event Dispatcher (hosted by reactor)
        +-- Peer FSMs
```

**Target:**
```
Engine
  +-- Plugin Server (engine-level, always runs)
  +-- Config Reload (engine-level, always runs)
  +-- BGP Plugin (loaded via ConfigRoots when bgp { } in config)
        +-- Reactor (owns peers, FSM, event dispatch)
        +-- Wire Layer
```

### Scope

**In scope:**

| Area | Description |
|------|-------------|
| Plugin server extraction | Move plugin server from reactor to engine. Runs independently of BGP. |
| Config reload extraction | Move config reload coordinator from reactor to engine. |
| BGP as plugin | BGP registers with ConfigRoots: ["bgp"]. RunEngine creates reactor, wires peers. |
| ConfigureBus for BGP | BGP gets Bus via ConfigureBus callback, not hardcoded `reactor.SetBus(b)` |
| hub/main.go cleanup | Remove hardcoded reactor creation. Engine creates Bus, starts plugin server, auto-loads plugins from config. |
| Engine without BGP | `interface { }` only config: engine starts, iface loads, no BGP. |

**Out of scope:**

| Area | Reason |
|------|--------|
| Plugin SDK changes | BGP plugin uses existing SDK 5-stage protocol |
| Wire format changes | No protocol changes |
| Config syntax changes | `bgp { }` stays the same |
| External plugin compat | External plugins still connect via hub server |

### Key Challenges

| Challenge | Detail |
|-----------|--------|
| Plugin server ownership | Currently reactor creates and owns the plugin server. Must move to engine. |
| Config reload | Currently reactor.Reload() re-parses config and reconciles peers. Must split: engine handles plugin lifecycle, BGP handles peer reconciliation. |
| ProcessSpawner | Currently reactor.SetProcessSpawner(pm). The plugin manager must be engine-level. |
| Event dispatch | EventDispatcher bridges reactor to plugins. Must still work when BGP is a plugin. |
| Startup ordering | The plugin server starts plugins. If BGP is a plugin, its startup goes through the same 5-stage protocol as other plugins. |
| Chaos testing | Chaos wrappers inject into reactor. Must still work when reactor is inside a plugin. |
| GR marker | Read/write at hub level for restart detection. Must still work. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- engine, subsystem, plugin boundaries
  -> Constraint: subsystems own external I/O (TCP/FSM), plugins do not
  -> Decision: BGP transitions from subsystem to plugin with ConfigRoots

### Key Source Files
- [ ] `cmd/ze/hub/main.go` -- current startup sequence (reactor, engine, subsystem, bus)
- [ ] `internal/component/bgp/reactor/reactor.go` -- reactor responsibilities, startAPIServer
- [ ] `internal/component/bgp/subsystem/subsystem.go` -- BGP subsystem adapter
- [ ] `internal/component/plugin/server/startup.go` -- plugin startup phases
- [ ] `internal/component/iface/register.go` -- target pattern (plugin with ConfigRoots)
- [ ] `internal/core/engine/engine.go` -- engine responsibilities
- [ ] `internal/component/bgp/config/loader_create.go` -- config parsing, reactor creation

### Incremental Approach

This is a large refactor. Suggested phases:

**Phase 1: Extract plugin server to engine.** Move plugin server creation from `reactor.startAPIServer()` to the engine. The engine starts the plugin server. The reactor registers as a "BGP handler" with the plugin server instead of owning it. All plugin auto-loading (config paths, families, events, send types) stays in the plugin server. This is the critical structural change.

**Phase 2: Extract config reload to engine.** The reload coordinator moves to engine-level. BGP registers a reload callback for the `bgp` subtree. The engine handles plugin start/stop on config change. BGP handles peer reconciliation.

**Phase 3: BGP as plugin.** BGP registers via `registry.Register()` with `ConfigRoots: ["bgp"]`, `ConfigureBus`, `RunEngine`. The `runEngine` function creates the reactor, wires peers from config. hub/main.go creates only the engine and bus -- no direct reactor creation.

**Phase 4: Cleanup.** Remove `subsystem.NewBGPSubsystem`, remove `reactor.SetBus`/`SetProcessSpawner` direct calls. All wiring goes through plugin callbacks.

### Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config has `bgp { peer ... }` + `interface { }` | Both BGP and iface load. Full functionality. |
| AC-2 | Config has only `interface { }` (no bgp section) | Engine starts, iface loads, no BGP. No errors. |
| AC-3 | Config has only `fib { kernel { } }` | Engine starts, fib-kernel + sysrib load. No BGP, no iface. |
| AC-4 | Config has only `bgp { peer ... }` (no interface) | BGP loads, iface does not. |
| AC-5 | Empty config (no sections) | Engine starts, nothing loads. Clean idle. |
| AC-6 | Add `bgp { }` to config via editor, commit | BGP auto-loads at reload time. |
| AC-7 | Remove `bgp { }` from config via editor, commit | BGP auto-stops at reload time. |
| AC-8 | All existing BGP functional tests pass | No regression. |
| AC-9 | `ze show bgp summary` works when BGP is loaded | CLI dispatch reaches BGP plugin. |
| AC-10 | `ze show bgp summary` returns clear error when BGP not loaded | Not a crash, not silent. |

### Risk

This is the most invasive architectural change in ze. The reactor is coupled to nearly everything. The phased approach mitigates risk: each phase is independently testable and shippable. Phase 1 (plugin server extraction) is the riskiest because it changes the ownership model for the plugin lifecycle.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main.go` -- reactor always created, subsystem always registered
- [ ] `internal/component/bgp/reactor/reactor.go` -- reactor owns plugin server, config reload, event dispatch
- [ ] `internal/component/bgp/subsystem/subsystem.go` -- thin adapter wrapping reactor as ze.Subsystem
- [ ] `internal/core/engine/engine.go` -- engine starts subsystems, owns Bus

**Behavior to preserve:**
- All existing BGP functionality when `bgp { }` is in config
- Plugin 5-stage handshake protocol
- Config reload with verify/apply phases
- GR marker read/write at hub level
- Chaos testing injection

**Behavior to change:**
- Reactor no longer always created -- only when `bgp { }` in config
- Plugin server moves from reactor to engine
- Config reload coordinator moves from reactor to engine
- BGP registers as a plugin with ConfigRoots: ["bgp"]

## Data Flow (MANDATORY)

### Entry Point
- Config file parsed by YANG loader
- Engine created with Bus and plugin server
- Plugins auto-loaded based on ConfigRoots

### Transformation Path
1. Config parsed into Tree
2. Engine starts plugin server (no reactor needed)
3. CollectContainerPaths finds "bgp" -> matches BGP plugin's ConfigRoots
4. BGP plugin auto-loaded via runPluginPhase
5. BGP RunEngine creates reactor, wires peers from config
6. Reactor starts peer FSMs, event dispatch

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Engine | Tree parsed, plugin server started | [ ] |
| Engine -> BGP Plugin | ConfigRoots auto-load, 5-stage handshake | [ ] |
| BGP Plugin -> Reactor | RunEngine creates reactor internally | [ ] |
| Reactor -> Plugin Server | Event dispatch, command routing | [ ] |

### Integration Points
- `internal/core/engine/engine.go` -- owns plugin server and config reload
- `cmd/ze/hub/main.go` -- creates engine only, not reactor
- BGP plugin register.go -- ConfigRoots: ["bgp"], RunEngine creates reactor

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with only `interface { }` | -> | Engine starts, no BGP | `test/parse/no-bgp.ci` |
| Config with `bgp { }` added at reload | -> | BGP auto-loads | `test/reload/bgp-autoload.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestEngineStartsWithoutBGP` | `internal/core/engine/engine_test.go` | Engine runs with no subsystems |
| `TestBGPPluginAutoLoads` | `internal/component/plugin/server/startup_test.go` | ConfigRoots "bgp" triggers BGP load |
| `TestBGPPluginAutoStops` | `internal/component/plugin/server/startup_test.go` | Removing bgp section stops BGP |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario |
|------|----------|-------------------|
| `test-no-bgp` | `test/parse/no-bgp.ci` | Config without bgp loads cleanly |
| `test-bgp-autoload` | `test/reload/bgp-autoload.ci` | Adding bgp section at reload starts BGP |

## Files to Modify

- `cmd/ze/hub/main.go` -- remove hardcoded reactor creation
- `internal/core/engine/engine.go` -- own plugin server and config reload
- `internal/component/bgp/reactor/reactor.go` -- extract plugin server, add ConfigRoots registration
- `internal/component/bgp/subsystem/subsystem.go` -- remove or convert to plugin adapter

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/register.go` | BGP plugin registration with ConfigRoots: ["bgp"] |
| `test/parse/no-bgp.ci` | Functional test: engine without BGP |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | BGP YANG unchanged |
| CLI commands | Yes | Commands return error when BGP not loaded |
| Functional test | Yes | `test/parse/no-bgp.ci` |
| Plugin registration | Yes | `internal/component/bgp/register.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- BGP is now config-driven |
| 2 | Config syntax changed? | No | bgp { } syntax unchanged |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | No | -- |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- BGP as plugin |
| 6 | Has a user guide page? | No | -- |
| 7 | Wire format changed? | No | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No | -- |
| 10 | Test infrastructure changed? | No | -- |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- modular daemon |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- BGP as plugin |

## Implementation Steps

### Phase 1: Extract plugin server to engine
1. Move plugin server creation from reactor.startAPIServer to engine
2. Engine starts plugin server independently of BGP
3. All plugin auto-loading stays in plugin server
4. Reactor registers as BGP handler with plugin server

### Phase 2: Extract config reload to engine
1. Reload coordinator moves to engine level
2. BGP registers reload callback for bgp subtree
3. Engine handles plugin start/stop on config change

### Phase 3: BGP as plugin
1. BGP registers via registry.Register with ConfigRoots: ["bgp"]
2. RunEngine creates reactor, wires peers from config
3. hub/main.go creates only engine and bus

### Phase 4: Cleanup
1. Remove subsystem.NewBGPSubsystem
2. Remove reactor.SetBus/SetProcessSpawner direct calls
3. All wiring through plugin callbacks

### Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| No BGP regression | All existing .ci tests pass |
| Engine without BGP | Starts cleanly with interface-only config |
| Plugin server independent | Plugin startup works without reactor |
| Config reload works | Both BGP and non-BGP config changes |
| GR marker | Still read/written at hub level |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Engine starts without BGP | `test/parse/no-bgp.ci` passes |
| BGP auto-loads from config | All existing BGP .ci tests pass |
| No regression | `make ze-verify` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Plugin isolation | BGP plugin cannot access engine internals it shouldn't |
| Config-driven loading | No way to force-load BGP without config |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Plugin server extraction breaks plugins | Phase 1 -- fix server ownership |
| Config reload breaks | Phase 2 -- fix reload coordinator |
| BGP tests fail | Phase 3 -- fix plugin wiring |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-bgp-as-plugin.md`
- [ ] **Summary included in commit**
