# Spec: arch-8-config-provider-wiring

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-arch-7-subsystem-wiring |
| Phase | - |
| Updated | 2026-03-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/provider.go` — ConfigProvider implementation
4. `internal/component/bgp/config/loader.go` — LoadReactorWithPlugins (to be decomposed)
5. `cmd/ze/hub/main.go` — startup path (currently uses nop stubs)
6. `internal/component/bgp/server/event_dispatcher.go` — EOR notification target

## Task

Three deliverables:

1. **Wire real ConfigProvider** — replace `stubConfigProvider()` in `cmd/ze/hub/main.go` with the real `config.Provider`, populated from the parsed config tree
2. **Decompose LoadReactorWithPlugins** — split config loading from reactor creation so ConfigProvider can serve as the config authority
3. **EOR Bus notification** — EventDispatcher publishes `bgp/eor` to Bus when End-of-RIB markers are detected

### Design Decision: ConfigProvider Populated from Existing Parse

The existing config pipeline (YANG parser → Tree → `map[string]any`) stays. After parsing, the config tree is stored in the ConfigProvider via a new `SetRoot(name, tree)` method. The ConfigProvider becomes the authority — reactor and future consumers read from it instead of carrying their own `configTree` copy.

### Design Decision: EOR via EventDispatcher

The EventDispatcher gets a Bus reference. When `onEORReceived` fires (EOR detected inside UPDATE delivery), it publishes `bgp/eor` to Bus. This keeps the notification at the detection point — no callback chain back to reactor.

### Design Decision: Incremental Decomposition

`LoadReactorWithPlugins` is split into two functions:
- `LoadConfig(store, input, configPath, cliPlugins)` → returns parsed tree + plugin list
- `CreateReactor(tree, plugins, configDir, configPath, store)` → returns reactor

The caller (`cmd/ze/hub/main.go`) calls `LoadConfig`, populates ConfigProvider, then calls `CreateReactor`. This is the first step toward the full arch-0 decomposition where PluginManager owns plugin lifecycle.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/subsystem-wiring.md` — current Engine/Bus/Subsystem wiring
  → Constraint: Bus is notification layer, EventDispatcher is data path
  → Decision: reactor publishes Bus notifications alongside EventDispatcher calls
- [ ] `plan/spec-arch-0-system-boundaries.md` — target architecture
  → Decision: ConfigManager is central authority for config
  → Constraint: subsystems and plugins read config via ConfigProvider interface

### Completed Phases
- [ ] `plan/learned/326-arch-4-config-manager.md` — ConfigProvider built
  → Decision: Provider stores roots as `map[string]map[string]any`
  → Decision: Watch() returns channel for reload notifications

**Key insights:**
- ConfigProvider.Load() reads JSON from disk — we need SetRoot() for pre-parsed trees
- Config tree is already `map[string]any` from `tree.ToMap()` — fits Provider.roots directly
- EventDispatcher imports `pkg/ze` (via plugin types) — adding Bus reference is safe
- EOR is detected in events.go onMessageReceived, not visible to reactor

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/provider.go` — Provider struct with roots map, Load/Get/Watch/Schema/RegisterSchema. Load reads JSON from file. No way to set a root from pre-parsed tree.
  → Constraint: Provider.Load() is JSON-only — need SetRoot() for YANG-parsed trees
  → Constraint: Provider.Get() returns shallow copy — safe for concurrent access
- [ ] `internal/component/bgp/config/loader.go` — LoadReactorWithPlugins bundles: collect YANG, parse config, extract plugins, merge CLI plugins, expand deps, create reactor, wire reload. CreateReactorFromTree builds reactor.Config, wires SSH, authz, metrics, pprof.
  → Constraint: plugins provide YANG schemas that affect config parsing (steps coupled)
  → Constraint: CreateReactorFromTree wires SSH, authz, metrics — not just reactor
- [ ] `cmd/ze/hub/main.go` — runBGPInProcess uses nopConfigProvider and nopPluginManager stubs. Engine.Start calls BGPSubsystem.Start.
  → Constraint: chaos wrappers, GR marker, privilege drop ordering preserved
- [ ] `internal/component/bgp/server/event_dispatcher.go` — wraps pluginserver.Server + JSONEncoder. OnEORReceived delegates to onEORReceived in events.go. No Bus reference.
  → Constraint: EventDispatcher currently has no Bus field
- [ ] `internal/component/bgp/server/events.go` — onMessageReceived detects EOR at line 189, calls onEORReceived. EOR detection is inside UPDATE delivery, not visible to reactor.
  → Constraint: EOR detection happens 3 calls deep from reactor

**Behavior to preserve:**
- YANG-based config parsing pipeline
- Plugin YANG schema collection from global registry
- Config tree structure (`map[string]any` keyed by root)
- All existing tests (no behavior change from user perspective)
- EventDispatcher data path unchanged
- SSH, authz, metrics, pprof wiring in CreateReactorFromTree

**Behavior to change:**
- ConfigProvider populated with real config tree (not nop stub)
- LoadReactorWithPlugins split into LoadConfig + CreateReactor
- EventDispatcher publishes `bgp/eor` Bus notification when EOR detected
- Reactor's configTree sourced from ConfigProvider (single source of truth)

## Data Flow (MANDATORY)

### Entry Points

| Source | Entry | Format |
|--------|-------|--------|
| Config file | `cmd/ze/hub/main.go` reads file | Raw bytes |
| UPDATE wire | EOR detected in events.go | RawMessage with WireUpdate |

### Transformation Path

**Config path (changed):**
1. `cmd/ze/hub/main.go` reads config file — unchanged
2. `LoadConfig(store, input, configPath, cliPlugins)` — new: returns tree + plugins
3. `configProvider.SetRoot("bgp", tree.ToMap()["bgp"])` — new: populate provider
4. `CreateReactor(tree, plugins, configDir, configPath, store)` — extracted from LoadReactorWithPlugins
5. Engine.Start() passes configProvider to BGPSubsystem.Start()

**EOR notification path (new):**
1. UPDATE received → onMessageReceived → EOR detected
2. onEORReceived → deliver to plugins (unchanged)
3. EventDispatcher publishes `bgp/eor` to Bus — new

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config file → ConfigProvider | SetRoot() with parsed tree | [ ] |
| ConfigProvider → Subsystem | BGPSubsystem.Start(ctx, bus, config) | [ ] |
| EventDispatcher → Bus | bus.Publish("bgp/eor", metadata) | [ ] |

### Integration Points
- `internal/component/config/provider.go` — new SetRoot() method
- `internal/component/bgp/config/loader.go` — split LoadReactorWithPlugins
- `cmd/ze/hub/main.go` — wire real ConfigProvider
- `internal/component/bgp/server/event_dispatcher.go` — Bus field + EOR publish

### Architectural Verification
- [ ] No bypassed layers (config flows through ConfigProvider)
- [ ] No unintended coupling (Provider has no BGP knowledge)
- [ ] No duplicated functionality (reuses existing parse pipeline)
- [ ] ConfigProvider is single source of truth for config tree

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine.Start with real ConfigProvider | → | BGPSubsystem receives ConfigProvider with bgp root | `TestEnginePassesConfigToSubsystem` |
| ConfigProvider.Get("bgp") | → | Returns parsed BGP config tree | `TestConfigProviderServesBGPTree` |
| EOR detected in UPDATE | → | Bus notification published on bgp/eor | `TestEORBusNotification` |
| Config with BGP section | → | Reactor starts via Engine with real config | All 273 existing functional tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ConfigProvider.SetRoot("bgp", tree) then Get("bgp") | Returns the BGP config subtree |
| AC-2 | Engine.Start() with real ConfigProvider | BGPSubsystem.Start receives ConfigProvider with populated bgp root |
| AC-3 | LoadConfig() called with valid config | Returns parsed tree and plugin list, no reactor created |
| AC-4 | CreateReactor() called with tree + plugins | Returns reactor, identical to current LoadReactorWithPlugins behavior |
| AC-5 | EOR marker detected in received UPDATE | EventDispatcher publishes notification to `bgp/eor` Bus topic with peer and family metadata |
| AC-6 | No EOR in UPDATE | No `bgp/eor` Bus notification |
| AC-7 | Existing LoadReactor() callers | Continue to work (LoadReactor wraps LoadConfig + CreateReactor) |
| AC-8 | nopConfigProvider and nopPluginManager | Removed from cmd/ze/hub/main.go |
| AC-9 | `make ze-verify` | All existing tests pass |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProviderSetRoot` | `internal/component/config/provider_test.go` | SetRoot stores tree, Get retrieves it | |
| `TestProviderSetRootOverwrite` | `internal/component/config/provider_test.go` | SetRoot overwrites existing root | |
| `TestProviderSetRootNotifiesWatchers` | `internal/component/config/provider_test.go` | SetRoot sends ConfigChange to Watch channel | |
| `TestLoadConfigReturnsTreeAndPlugins` | `internal/component/bgp/config/loader_test.go` | LoadConfig returns tree + plugins without creating reactor | |
| `TestCreateReactorFromLoadConfig` | `internal/component/bgp/config/loader_test.go` | CreateReactor with LoadConfig output produces working reactor | |
| `TestEnginePassesConfigToSubsystem` | `internal/component/bgp/subsystem/subsystem_test.go` | BGPSubsystem.Start receives ConfigProvider, can Get("bgp") | |
| `TestConfigProviderServesBGPTree` | `internal/component/bgp/subsystem/subsystem_test.go` | ConfigProvider.Get("bgp") returns non-empty tree in running subsystem | |
| `TestEORBusNotification` | `internal/component/bgp/server/event_dispatcher_test.go` | EventDispatcher publishes bgp/eor to Bus when EOR detected | |
| `TestNoEORBusNotificationForNonEOR` | `internal/component/bgp/server/event_dispatcher_test.go` | No bgp/eor published for normal UPDATEs | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric inputs.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-announce` | `test/plugin/announce.ci` | Config parsed, reactor starts via Engine, UPDATE sent to peer — proves full startup path with real ConfigProvider | |
| `test-add-remove` | `test/plugin/add-remove.ci` | Config parsed, reactor starts, routes added and removed — proves reactor created correctly from decomposed LoadConfig + CreateReactor | |

### Future (if deferring any tests)
- ConfigProvider.Watch() integration with SIGHUP reload — deferred until Engine.Reload wires through ConfigProvider
- PluginManager replacing pluginserver.Server — Spec B

## Files to Modify

- `internal/component/config/provider.go` — add SetRoot() method
- `internal/component/bgp/config/loader.go` — split LoadReactorWithPlugins into LoadConfig + CreateReactor
- `cmd/ze/hub/main.go` — replace nop stubs with real ConfigProvider, use LoadConfig + CreateReactor
- `internal/component/bgp/server/event_dispatcher.go` — add Bus field, SetBus method, publish bgp/eor
- `internal/component/bgp/server/events.go` — call Bus publish in onEORReceived
- `internal/component/bgp/subsystem/subsystem.go` — pass Bus to EventDispatcher in Start()

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
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/subsystem-wiring.md` — ConfigProvider wiring, EOR Bus |

## Files to Create

No new files — all changes are to existing files.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan — check what exists |
| 3. Implement | Phases 1-3 below |
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

1. **Phase: ConfigProvider.SetRoot + wiring** — Add SetRoot to Provider, split LoadReactorWithPlugins, wire real ConfigProvider in hub/main.go, remove nop stubs
   - Tests: `TestProviderSetRoot`, `TestProviderSetRootOverwrite`, `TestProviderSetRootNotifiesWatchers`, `TestLoadConfigReturnsTreeAndPlugins`, `TestCreateReactorFromLoadConfig`, `TestEnginePassesConfigToSubsystem`, `TestConfigProviderServesBGPTree`
   - Files: `provider.go`, `loader.go`, `main.go`, `subsystem.go`
   - Verify: tests fail → implement → tests pass

2. **Phase: EOR Bus notification** — Add Bus field to EventDispatcher, publish bgp/eor when EOR detected
   - Tests: `TestEORBusNotification`, `TestNoEORBusNotificationForNonEOR`
   - Files: `event_dispatcher.go`, `events.go`, `subsystem.go`
   - Verify: tests fail → implement → tests pass

3. **Phase: Cleanup + verification** — Remove nop stubs from hub/main.go, verify all tests
   - Full verification: `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | ConfigProvider.Get("bgp") returns same tree as old reactor.configTree |
| Correctness | LoadConfig + CreateReactor produces identical reactor to LoadReactorWithPlugins |
| Correctness | EOR Bus notification fires only for actual EOR markers, not normal UPDATEs |
| Backward compat | LoadReactor() still works (wraps new functions) |
| Rule: no-layering | nop stubs fully removed, real implementations used |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Provider.SetRoot exists | `grep 'func.*Provider.*SetRoot' internal/component/config/provider.go` |
| LoadConfig function exists | `grep 'func LoadConfig' internal/component/bgp/config/loader.go` |
| Real ConfigProvider in hub | `grep 'config.NewProvider' cmd/ze/hub/main.go` |
| No nop stubs in hub | `grep -c 'nopConfigProvider\|nopPluginManager' cmd/ze/hub/main.go` returns 0 |
| EOR Bus publish | `grep 'bgp/eor' internal/component/bgp/server/` |
| `make ze-verify` passes | Run and paste output |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | SetRoot: nil tree should not panic |
| Resource exhaustion | Watch channels bounded (cap 1, drain pattern) — unchanged |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| LoadReactor callers break | Phase 1 — ensure wrapper function preserved |
| EOR fires for non-EOR | Phase 2 — check IsEOR() condition |
| Existing functional tests fail | Phase 1 — CreateReactor must be identical to old code path |
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

EOR: RFC 4724 Section 2 — already documented in events.go.

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
- [ ] AC-1..AC-9 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-arch-8-config-provider-wiring.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
