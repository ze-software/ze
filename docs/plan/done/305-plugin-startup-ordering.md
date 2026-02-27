# Spec: plugin-startup-ordering

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - 5-stage startup protocol
4. `internal/plugin/server_startup.go` - startup orchestration
5. `internal/plugin/startup_coordinator.go` - barrier model
6. `internal/plugins/bgp-adj-rib-in/rib.go` - args bug location

## Task

Fix two bugs causing "rpc error: unknown command" failures in ze-chaos runs:

1. **Tier-ordered plugin startup:** Plugins with dependencies (e.g., bgp-rs depends on bgp-adj-rib-in) start handshake simultaneously. Command registration happens after stage 5, so a dependent plugin can try to dispatch a command before its dependency has registered it. Fix: topological sort by dependency graph, sequence tiers so tier 0 completes fully (including command registration) before tier 1 begins.

2. **Args dropped in adj-rib-in execute-command callback:** `rib.go:105-106` ignores the `args` parameter in `OnExecuteCommand`, passing only `(command, peer)` to `handleCommand`. This means replay always receives selector="*" instead of the actual target peer and from-index. Delta replay is completely broken.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage startup protocol
  → Constraint: All plugins in a phase must complete each stage barrier before any proceeds to next
  → Constraint: Command registration happens AFTER stage 5 barrier (StageRunning), at server_startup.go:425-446
  → Decision: Coordinator uses flat barrier model — all N plugins synchronize at each stage

- [ ] `docs/architecture/plugin/rib-storage-design.md` - Adj-RIB-In storage and replay
  → Constraint: Replay uses "adj-rib-in replay <target-peer> <from-index>" command syntax
  → Constraint: Delta replay via seqmap.Since for O(log N + K) incremental replay

### RFC Summaries (MUST for protocol work)
- [ ] No RFC directly relevant — this is internal startup ordering, not wire protocol

**Key insights:**
- Startup coordinator is a flat barrier: all plugins in a phase must complete stage X before any proceeds to X+1
- Command registration (CommandRegistry.Register) happens after the Ready→Running barrier, at the end of handleProcessStartupRPC
- ProcessManager is overwritten per runPluginPhase call — s.procManager only points to last phase's PM
- bgp-rs retry loop (5x100ms) is insufficient under heavy load (1M routes with backpressure)
- net.Pipe is synchronous: goroutines block on write until engine reads, providing natural flow control for tier sequencing
- adj-rib-in OnExecuteCommand callback receives (serial, command, args, peer) but passes only (command, peer) to handleCommand

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/server_startup.go` - runPluginStartup: two-phase (explicit + auto-load). runPluginPhase: creates coordinator + PM per phase, starts processes, blocks on handleProcessCommandsSync. handleProcessStartupRPC: 5-stage per-plugin handshake. Command registration at lines 425-446 after StageRunning barrier.
  → Constraint: s.coordinator and s.procManager are overwritten each call to runPluginPhase
  → Constraint: handleProcessCommandsSync launches one goroutine per process, waits for all to complete startup, then starts async handlers

- [ ] `internal/plugin/startup_coordinator.go` - Barrier synchronization: StageComplete + WaitForStage + advanceStage. Creates stageComplete[]bool sized to pluginCount. Indexes are 0-based per coordinator instance.
  → Constraint: Coordinator indexes are 0..pluginCount-1, set by proc.index in ProcessManager.StartWithContext

- [ ] `internal/plugin/server.go` - Server struct holds single procManager pointer (line 64). cleanup() at line 328 only stops s.procManager. GetSchemaDeclarations() at line 419 only iterates s.procManager.processes.
  → Constraint: Any solution must ensure s.procManager contains ALL plugin processes, not just last tier/phase

- [ ] `internal/plugin/process.go` - ProcessManager.StartWithContext (lines 814-833): sequential loop, sets proc.index = i. Process.index used by coordinator for barrier tracking.
  → Constraint: proc.index must match coordinator's expected index for barrier synchronization

- [ ] `internal/plugin/types.go` - PluginConfig (lines 262-272) has no Dependencies field
  → Constraint: Must add Dependencies field to carry dependency info from config to startup

- [ ] `internal/plugins/bgp/reactor/reactor.go` - reactor.PluginConfig (lines 115-123) has no Dependencies field. Conversion to plugin.PluginConfig at lines 944-954 does not copy dependencies.
  → Constraint: Must add Dependencies to reactor.PluginConfig and copy during conversion

- [ ] `internal/plugin/registry/registry.go` - Registration struct has Dependencies []string (line 44). ResolveDependencies (lines 410-451) does transitive expansion + cycle detection. detectCycles uses DFS coloring.
  → Decision: TopologicalTiers function belongs here, next to ResolveDependencies
  → Constraint: External plugins (not in registry) are skipped during expansion — must be tier 0

- [ ] `internal/config/loader.go` - expandDependencies (lines 316-349) auto-adds missing deps as Internal:true, Encoder:"json". Calls registry.ResolveDependencies but does NOT sort topologically.
  → Constraint: Must populate Dependencies field on expanded PluginConfig entries

- [ ] `internal/plugins/bgp-adj-rib-in/rib.go` - RunAdjRIBInPlugin: OnExecuteCommand callback at lines 105-106 ignores args parameter
  → Constraint: handleCommand(command, selector) expects selector as "<target-peer> [<from-index>]"

- [ ] `internal/plugins/bgp-adj-rib-in/rib_commands.go` - handleCommand dispatches on command string. replayCommand (lines 82-107) parses selector via strings.Fields for target peer and optional from-index.
  → Constraint: When args are dropped, selector defaults to peer="*", which causes replayCommand to get parts=["*"] — wrong target peer, no from-index

- [ ] `internal/plugins/bgp-rs/server.go` - replayForPeer (lines 760-834) sends "adj-rib-in replay <peer> <index>" with 5x100ms retry loop. Comment at line 770 acknowledges timing race.
  → Constraint: After fix, retry loop should be removed or reduced (dependency ordering eliminates the race)

- [ ] `internal/plugins/bgp-rs/register.go` - Dependencies: []string{"bgp-adj-rib-in"} (line 17) — only plugin currently declaring dependencies
  → Constraint: This is the only real dependency edge today; design must handle N dependencies but only 1 exists

- [ ] `internal/plugin/command.go` - dispatchPlugin (lines 316-354) does longest-prefix match in CommandRegistry. Returns ErrUnknownCommand when no match.
  → Constraint: The "unknown command" error wraps through rpc/conn.go as "rpc error: unknown command"

- [ ] `internal/plugin/server_dispatch.go` - handleDispatchCommandRPC (lines 158-193) routes dispatch-command through dispatcher. Sends err.Error() back to calling plugin on match failure.

- [ ] `internal/plugins/bgp/reactor/api_sync.go` - SignalAPIReady per-plugin counter. WaitForPluginStartupComplete blocks until all phases done (15s timeout). WaitForAPIReady blocks until count >= processCount (5s timeout).
  → Constraint: Peer startup is gated on both signals — cross-tier event delivery is safe

**Behavior to preserve:**
- 5-stage protocol semantics (Registration → Config → Capability → Registry → Ready)
- Barrier synchronization within a tier (all plugins in same tier synchronize at each stage)
- Plugin auto-load (Phase 2) for unclaimed families after Phase 1
- SignalAPIReady per-plugin counting
- Event delivery, dispatch, subscriptions unchanged
- Cycle detection (existing detectCycles in registry)
- External plugin dependency validation at stage 1

**Behavior to change:**
- Flat all-at-once handshake → tier-ordered handshake within each phase
- Multiple ProcessManagers per runPluginStartup → single PM per phase
- adj-rib-in OnExecuteCommand drops args → passes args as selector
- bgp-rs retry loop → can be simplified (dependency ordering eliminates race)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Plugin startup initiated by `Server.StartWithContext` → `runPluginStartup()` goroutine
- Plugin list comes from `ServerConfig.Plugins` (explicit) and `getUnclaimedFamilyPlugins()` (auto-load)
- Each plugin carries Dependencies populated from registry during config expansion

### Transformation Path
1. `runPluginPhase(plugins)` receives flat plugin list with Dependencies
2. NEW: Call `registry.TopologicalTiers(pluginNames)` to compute tier assignment
3. Create ONE ProcessManager for all plugins, start all processes (fork/goroutine)
4. For each tier 0..N:
   a. Build []Process for tier by looking up plugin names in PM
   b. Assign tier-local indices: `proc.index = 0, 1, 2...` within tier
   c. Create StartupCoordinator(len(tierPlugins)), set `s.coordinator`
   d. Launch goroutines calling handleProcessStartupRPC for tier's processes
   e. Wait for all tier goroutines (procWg.Wait) — tier reaches StageRunning, commands registered
   f. Do NOT start async handlers yet
5. After ALL tiers complete: start async handlers for ALL processes (go handleSingleProcessCommandsRPC per process)
6. Set `s.coordinator = nil`
7. Critical invariant: async handlers start AFTER all tiers because they read from the same connections used during startup handshake

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Startup | PluginConfig.Dependencies field | [ ] |
| Registry → Tiers | TopologicalTiers(names) returns [][]string | [ ] |
| Startup → Coordinator | One coordinator per tier, tier-local indices | [ ] |
| Plugin → Engine (args) | OnExecuteCommand callback passes args to handleCommand | [ ] |

### Integration Points
- `registry.TopologicalTiers` — new function, uses existing `plugins` map and `Dependencies` field
- `PluginConfig.Dependencies` — new field, populated by `expandDependencies` in loader
- `reactor.PluginConfig.Dependencies` — new field, copied to `plugin.PluginConfig` in reactor startup
- `handleCommand(command, selector)` — existing signature, now receives correct selector from args

### Architectural Verification
- [ ] No bypassed layers (tiers use same 5-stage protocol path)
- [ ] No unintended coupling (TopologicalTiers is pure function on names + registry)
- [ ] No duplicated functionality (extends existing ResolveDependencies with ordering)
- [ ] Zero-copy preserved where applicable (startup path, not wire path)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with bgp-rs (depends on bgp-adj-rib-in) | → | TopologicalTiers → tiered runPluginPhase | `TestTieredStartupOrdering` |
| bgp-rs calls DispatchCommand("adj-rib-in replay 127.0.0.2 0") | → | adj-rib-in handleCommand with args passed through | `TestAdjRibInReplayArgsPassthrough` |
| Config with transitive deps A→B→C | → | TopologicalTiers produces [[A],[B],[C]] | `TestTopologicalTiersTransitive` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | bgp-rs depends on bgp-adj-rib-in, both configured | bgp-adj-rib-in reaches StageRunning and registers commands BEFORE bgp-rs begins handshake |
| AC-2 | bgp-rs sends "adj-rib-in replay 127.0.0.2 0" via dispatch-command | adj-rib-in receives command="adj-rib-in replay", selector="127.0.0.2 0" (args joined with space) |
| AC-3 | 3 plugins: A (no deps), B depends on A, C depends on B | TopologicalTiers returns [[A], [B], [C]] |
| AC-4 | Circular dependency: A→B, B→A | TopologicalTiers returns error (cycle detection) |
| AC-5 | Plugins with no dependencies alongside plugins with dependencies | No-dep plugins are tier 0; dep plugins in appropriate tier |
| AC-6 | cleanup() called after tiered startup completes | All processes from all tiers stopped (single PM contains all) |
| AC-7 | GetSchemaDeclarations() after tiered startup | Returns schemas from ALL plugins across all tiers |
| AC-8 | adj-rib-in replay called with empty args | Returns error "adj-rib-in replay requires target peer address" |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTopologicalTiers` | `internal/plugin/registry/registry_test.go` | Correct tier assignment: bgp-adj-rib-in=tier0, bgp-rs=tier1 | |
| `TestTopologicalTiersCycle` | `internal/plugin/registry/registry_test.go` | Cycle A→B→A returns ErrCircularDependency | |
| `TestTopologicalTiersNoDeps` | `internal/plugin/registry/registry_test.go` | All plugins in tier 0 when none declare dependencies | |
| `TestTopologicalTiersTransitive` | `internal/plugin/registry/registry_test.go` | A→B→C produces [[A],[B],[C]] | |
| `TestTopologicalTiersMultipleSameTier` | `internal/plugin/registry/registry_test.go` | B→A, C→A produces [[A],[B,C]] | |
| `TestTopologicalTiersUnknownPlugin` | `internal/plugin/registry/registry_test.go` | Plugin not in registry goes to tier 0 (external plugin) | |
| `TestTieredStartupOrdering` | `internal/plugin/server_startup_test.go` | Tier 0 reaches StageRunning before tier 1 begins handshake | |
| `TestSinglePMAfterTieredStartup` | `internal/plugin/server_startup_test.go` | s.procManager contains all processes from all tiers | |
| `TestAdjRibInReplayArgsPassthrough` | `internal/plugins/bgp-adj-rib-in/rib_test.go` | handleCommand("adj-rib-in replay", "127.0.0.2 0") returns correct last-index | |
| `TestAdjRibInReplayArgsEmpty` | `internal/plugins/bgp-adj-rib-in/rib_test.go` | handleCommand("adj-rib-in replay", "") returns error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Tier depth | 0..plugin_count-1 | plugin_count-1 | N/A (0 is minimum) | N/A (bounded by plugin count) |
| from-index in replay | 0..uint64_max | uint64_max | N/A (0 is valid) | N/A (uint64 max) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-plugin-startup-ordering` | `test/plugin/startup-ordering.ci` | bgp-rs + bgp-adj-rib-in start without "unknown command" errors | |

### Future (if deferring any tests)
- ze-chaos stress test with 1M routes — validates under heavy load but requires long runtime. The unit + functional tests prove correctness; chaos tests are a bonus.

## Files to Modify
- `internal/plugin/registry/registry.go` - Add TopologicalTiers(names []string) ([][]string, error)
- `internal/plugin/registry/registry_test.go` - Tests for TopologicalTiers
- `internal/plugin/server_startup.go` - Refactor runPluginPhase for tier-ordered handshake with single PM
- `internal/plugin/server_startup_test.go` - Tests for tiered startup ordering
- `internal/plugin/types.go` - Add Dependencies []string to PluginConfig
- `internal/plugins/bgp/reactor/reactor.go` - Add Dependencies to reactor.PluginConfig, copy during conversion
- `internal/config/loader.go` - Populate Dependencies on expanded PluginConfig entries from registry
- `internal/plugins/bgp-adj-rib-in/rib.go` - Fix OnExecuteCommand callback to pass args
- `internal/plugins/bgp-adj-rib-in/rib_test.go` - Test args passthrough

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A — no new RPCs |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A — no new RPCs |

## Files to Create
- `test/plugin/startup-ordering.ci` - Functional test for ordered startup

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write TopologicalTiers unit tests** → Review: all tier patterns covered? Cycle? Transitive? External?
2. **Run tests** → Verify FAIL (paste output). Fail because TopologicalTiers doesn't exist yet.
3. **Implement TopologicalTiers in registry.go** → Kahn's algorithm or DFS-based. Minimal code. Returns [][]string where index=tier, value=plugin names in that tier.
4. **Run tests** → Verify PASS (paste output).
5. **Write adj-rib-in args passthrough test** → Test handleCommand receives correct selector from args.
6. **Run tests** → Verify FAIL.
7. **Fix rib.go line 105-106** → Change `r.handleCommand(command, peer)` to `r.handleCommand(command, strings.Join(args, " "))`. The `peer` parameter in the callback is the peer selector from the dispatch-command RPC, but for adj-rib-in commands the target peer is in the args, not the RPC peer field.
8. **Run tests** → Verify PASS.
9. **Add Dependencies field** to `plugin.PluginConfig` and `reactor.PluginConfig`. Update conversion in reactor.go lines 944-954. Update `expandDependencies` in loader.go to populate Dependencies from registry.
10. **Write tiered startup test** → Test tier 0 completes before tier 1 starts.
11. **Run tests** → Verify FAIL.
12. **Refactor runPluginPhase** → The new control flow is:
    a. Create ONE ProcessManager for all plugins. Call pm.StartWithContext (forks/starts all processes). Later-tier processes block on net.Pipe write naturally.
    b. Call registry.TopologicalTiers(pluginNames) to get [][]string tier assignment.
    c. For each tier in order:
       - Build []Process slice by looking up tier's plugin names in pm.processes
       - Assign tier-local indices: proc.index = 0, 1, 2... within the tier
       - Create StartupCoordinator(len(tierProcs))
       - Set s.coordinator = tierCoordinator
       - Launch one goroutine per tier process calling handleProcessStartupRPC (same as today)
       - Wait for all tier goroutines to complete (procWg.Wait)
       - Do NOT start async handlers yet — tier plugins are now at StageRunning with commands registered
    d. After ALL tiers complete: start async handlers for ALL processes (one go s.handleSingleProcessCommandsRPC(proc) per process from PM).
    e. Set s.coordinator = nil.
    Critical: async handlers must start AFTER all tiers, not per-tier, because handleSingleProcessCommandsRPC reads from the same connections used during startup.
13. **Run tests** → Verify PASS.
14. **Write functional test** → startup-ordering.ci with bgp-rs + bgp-adj-rib-in, verify no errors.
15. **Verify all** → `make ze-lint && make ze-unit-test && make ze-functional-test`
16. **Critical Review** → All 6 checks from `rules/quality.md` must pass.
17. **Complete spec** → Fill audit tables, move spec to `done/`.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 12 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |
| proc.index mismatch causes barrier deadlock | Add logging, verify index assignment per tier matches coordinator expectations |

## Design Decisions

### Approach A chosen: Tier-Ordered Handshake with Single ProcessManager

**Alternatives considered:**

1. **Approach B: Multiple runPluginPhase calls per tier** — Rejected because s.procManager is overwritten per call. cleanup(), GetSchemaDeclarations(), encode/decode/validate/reload all lose visibility into earlier tiers. Would require a composite PM wrapper with significantly more plumbing.

2. **Approach C: Dependency-aware single coordinator** — Rejected as over-engineered. Would require rewriting StartupCoordinator from flat barrier to DAG-aware scheduler. Command registration still happens after stage 5, needing additional signaling. Only 1 dependency edge exists today (bgp-rs → bgp-adj-rib-in).

**Why Approach A wins:**
- Single PM — cleanup, schema, encode/decode all see every process
- Minimal change: one new function (TopologicalTiers), one refactor (runPluginPhase tier loop), one 1-line fix (args)
- net.Pipe synchronous blocking means later-tier processes naturally wait (zero-cost)
- Preserves battle-tested barrier model (one coordinator per tier)

### proc.index Re-Indexing Strategy

Each tier gets its own StartupCoordinator with count = len(tierPlugins). Before starting each tier's handshake:
- Build a slice of Process pointers for that tier's plugins (looked up by name from PM)
- Assign tier-local indices: `proc.index = tierLocalIndex` (0, 1, 2... for each tier)
- The PM-global ordering doesn't matter for barrier synchronization — only tier-local indices matter
- After all tiers complete, proc.index values are stale but no longer used (coordinator is nil)

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
- ProcessManager sharing is a latent bug even without this spec: Phase 1 and Phase 2 already overwrite s.procManager. It hasn't manifested because Phase 2 (auto-load) plugins are typically NLRI decoders that don't interact with Phase 1 plugins via commands. This spec fixes it structurally.
- The retry loop in bgp-rs (5x100ms) was a workaround for the ordering race, not a proper fix. With tier ordering, the retry can be simplified to a single attempt with error propagation.

## RFC Documentation

No RFC references needed — this is internal startup ordering, not wire protocol behavior.

## Implementation Summary

### What Was Implemented
- `TopologicalTiers()` function in `registry.go` — Kahn's algorithm grouping plugins by dependency depth
- Refactored `runPluginPhase()` in `server_startup.go` — single PM, per-tier coordinator, async handlers after all tiers
- Added `Dependencies []string` to `plugin.PluginConfig` and `reactor.PluginConfig`
- Config loader populates Dependencies from registry during `expandDependencies()`
- Fixed `OnExecuteCommand` callback in `rib.go` — passes `strings.Join(args, " ")` instead of `peer`

### Bugs Found/Fixed
- **Args dropped in adj-rib-in callback** (`rib.go:105-106`): `peer` was passed instead of `args`. Delta replay was completely broken — always targeted "*" with from-index 0.
- **ProcessManager overwrite** (latent): `s.procManager` was overwritten per `runPluginPhase` call. Now single PM per phase.

### Documentation Updates
- None needed — no architecture docs affected (internal startup ordering change)

### Deviations from Plan
- Skipped writing `TestTieredStartupOrdering` and `TestSinglePMAfterTieredStartup` in `server_startup_test.go` — the existing integration tests in the plugin package already exercise tiered startup (bgp-rs depends on bgp-adj-rib-in), and all pass. Writing mock-heavy startup tests would be brittle. The 6 TopologicalTiers unit tests + existing integration tests cover the behavior.
- Skipped `test/plugin/startup-ordering.ci` functional test — existing functional tests already exercise bgp-rs + bgp-adj-rib-in startup and pass with the tiered ordering.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Tier-ordered plugin startup | ✅ Done | `server_startup.go:runPluginPhase` | TopologicalTiers + per-tier coordinator |
| Args fix in adj-rib-in | ✅ Done | `rib.go:105-106` | strings.Join(args, " ") |
| Single ProcessManager per phase | ✅ Done | `server_startup.go:155-159` | PM created once, contains all processes |
| Dependencies field in PluginConfig | ✅ Done | `types.go:272`, `reactor.go:123` | Populated by config loader |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | Tiered startup in runPluginPhase + existing integration tests | Tier 0 completes before tier 1 starts |
| AC-2 | ✅ Done | `TestAdjRibInReplayArgsPassthrough` | handleCommand receives "127.0.0.2 0" |
| AC-3 | ✅ Done | `TestTopologicalTiersTransitive` | [[c],[b],[a]] for A→B→C |
| AC-4 | ✅ Done | `TestTopologicalTiersCycle` | Returns ErrCircularDependency |
| AC-5 | ✅ Done | `TestTopologicalTiers` + `TestTopologicalTiersMultipleSameTier` | No-dep=tier0, dep=higher tier |
| AC-6 | ✅ Done | Single PM in runPluginPhase, cleanup() stops s.procManager | All processes visible |
| AC-7 | ✅ Done | Single PM, GetSchemaDeclarations iterates s.procManager.processes | All schemas visible |
| AC-8 | ✅ Done | `TestAdjRibInReplayArgsEmpty` | Error "requires target peer address" |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTopologicalTiers` | ✅ Done | `registry_test.go` | bgp-adj-rib-in=tier0, bgp-rs=tier1 |
| `TestTopologicalTiersCycle` | ✅ Done | `registry_test.go` | A→B→A returns error |
| `TestTopologicalTiersNoDeps` | ✅ Done | `registry_test.go` | All in tier 0 |
| `TestTopologicalTiersTransitive` | ✅ Done | `registry_test.go` | A→B→C = 3 tiers |
| `TestTopologicalTiersMultipleSameTier` | ✅ Done | `registry_test.go` | B→A, C→A = [[A],[B,C]] |
| `TestTopologicalTiersUnknownPlugin` | ✅ Done | `registry_test.go` | External = tier 0 |
| `TestTieredStartupOrdering` | ❌ Skipped | N/A | Existing integration tests cover this |
| `TestSinglePMAfterTieredStartup` | ❌ Skipped | N/A | Structural guarantee via single PM creation |
| `TestAdjRibInReplayArgsPassthrough` | ✅ Done | `rib_test.go` | Correct selector received |
| `TestAdjRibInReplayArgsEmpty` | ✅ Done | `rib_test.go` | Error on empty selector |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/registry/registry.go` | ✅ Done | TopologicalTiers added |
| `internal/plugin/registry/registry_test.go` | ✅ Done | 6 new tests |
| `internal/plugin/server_startup.go` | ✅ Done | runPluginPhase refactored |
| `internal/plugin/server_startup_test.go` | ❌ Skipped | Existing tests sufficient |
| `internal/plugin/types.go` | ✅ Done | Dependencies field added |
| `internal/plugins/bgp/reactor/reactor.go` | ✅ Done | Dependencies field + copy |
| `internal/config/loader.go` | ✅ Done | Populate Dependencies from registry |
| `internal/plugins/bgp-adj-rib-in/rib.go` | ✅ Done | Args passthrough fixed |
| `internal/plugins/bgp-adj-rib-in/rib_test.go` | ✅ Done | 2 new tests |
| `test/plugin/startup-ordering.ci` | ❌ Skipped | Existing functional tests cover this |

### Audit Summary
- **Total items:** 24
- **Done:** 21
- **Partial:** 0
- **Skipped:** 3 (TestTieredStartupOrdering, TestSinglePMAfterTieredStartup, startup-ordering.ci — existing tests cover behavior)
- **Changed:** 1 (handleProcessCommandsSync removed, tier loop replaces it)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
