# Spec: vrf-0 -- VRF Support (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/bgp/reactor/reactor.go` -- Reactor struct and constructor
4. `internal/component/bgp/plugins/rib/rib.go` -- RIB plugin
5. `internal/component/plugin/server/` -- plugin server and hub
6. Child specs: `spec-vrf-1-*` through `spec-vrf-N-*`

## Task

Add VRF (Virtual Routing and Forwarding) support to Ze. A VRF is a full isolation domain: each VRF instance has its own BGP reactor (peers, FSM, TCP listeners), its own RIB, its own interface bindings, and its own hub. The VRF plugin is an orchestrator that creates and manages these isolated stacks.

### Design Decisions (agreed with user)

| Decision | Detail |
|----------|--------|
| VRF is a full stack | Each VRF gets its own reactor, peers, RIB, interfaces -- not just a RIB namespace |
| Universal abstraction | The current global stack is the unnamed/default VRF. Named VRFs are additional instances. Ze can run VRF-only with no non-VRF path |
| Process model | Goroutine-based, in-process. Enables cross-VRF communication (route leaking) |
| Plugin naming | Colon-separated: `bgp-rib:vrf-red`, `bgp-rib:vrf-blue` |
| Lifecycle | Config at startup + dynamic create/delete at runtime |
| CLI surface | `show vrf <name> <command>` -- VRF intercepts, extracts name, forwards to the correct instance |
| YANG wrapping | VRF plugin takes the YANG schemas of its child modules and wraps them in `vrf <name> { ... }`. Default VRF keeps unwrapped YANG |
| Hub instantiation | Each VRF gets its own hub instance. VRF plugin is a hub-of-hubs |
| RIB independence | RIB code works unchanged -- it connects to whichever hub it is given. With VRF plugin: per-VRF hub. Without: global hub directly |
| Kernel integration | Deferred to FIB module (future spec). VRF is BGP-level isolation initially |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| Reactor multi-instance | Refactor reactor to be instantiable N times in the same process (eliminate global state) |
| Hub multi-instance | Hub and ProcessManager support multiple independent instances with derived plugin names |
| VRF plugin | Orchestrator that creates/destroys per-VRF stacks (reactor + hub + RIB + plugins) |
| VRF config | YANG schema for VRF definition, per-VRF peer config, per-VRF interface binding |
| VRF CLI | `show vrf <name> <command>` routing, VRF-aware command dispatch |
| Dynamic lifecycle | Runtime VRF create/delete with graceful peer drain and RIB cleanup |
| Metrics scoping | Per-VRF metric labels or per-VRF metric registries |
| Cross-VRF route leaking | Interface for route import/export between VRF RIB instances |

**Out of Scope:**

| Area | Reason |
|------|--------|
| FIB / kernel VRF devices | Requires FIB module not yet built. Separate spec set |
| Route-target based VRF assignment | Phase 2 -- config-based peer-to-VRF mapping first |
| MPLS/VPNv4 label distribution | Requires MPLS infrastructure |
| Inter-VRF forwarding plane | Requires FIB module |

### Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-vrf-1-hub-multi.md` | Hub and ProcessManager support multiple instances. Colon-separated naming (`bgp-rib:vrf-red`). Plugin startup per hub | - |
| 2 | `spec-vrf-2-plugin.md` | VRF plugin: orchestrator that creates per-VRF stacks (reactor + hub + RIB). Config-based VRF definition. Startup/shutdown lifecycle | vrf-1 |
| 3 | `spec-vrf-3-yang-cli.md` | YANG wrapping (`vrf <name> { ... }`), CLI command routing (`show vrf <name> ...`), VRF-aware dispatch | vrf-2 |
| 4 | `spec-vrf-4-dynamic.md` | Runtime VRF create/delete. Graceful drain. Hot reconfiguration | vrf-3 |
| 5 | `spec-vrf-5-cross-vrf.md` | Route leaking between VRF RIB instances. Import/export policy | vrf-4 |

Note: Reactor multi-instance was analyzed and requires no code changes -- all global state is safe to share across VRF instances. The reactor is already instantiable N times.

Phases are strictly ordered. Each phase must be complete before the next begins.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- overall architecture, reactor, plugin model
  -> Constraint: reactor is the central event loop; plugins connect via hub
- [ ] `.claude/rules/plugin-design.md` -- plugin registration, 5-stage protocol, proximity principle
  -> Constraint: registration via `init()` in `register.go`, blank import is only coupling
- [ ] `.claude/rules/goroutine-lifecycle.md` -- goroutine patterns
  -> Constraint: long-lived workers only, no per-event goroutines in hot paths

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4364.md` -- BGP/MPLS IP VPNs (L3VPN)
  -> Constraint: VRF is per-PE, route distinguisher makes routes unique across VRFs
- [ ] `rfc/short/rfc4271.md` -- BGP-4: session management, UPDATE processing
  -> Constraint: each BGP session belongs to exactly one routing context

**Key insights:**
- VRF is a universal abstraction -- the current Ze without VRF is equivalent to a single unnamed VRF
- Each VRF is a goroutine-isolated stack sharing the same OS process
- The VRF plugin is an orchestrator, not a data-plane component
- Cross-VRF route leaking is possible because all VRFs share process memory (goroutine model)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor.go` -- Reactor struct, `New()` constructor, event loop
- [ ] `internal/component/bgp/reactor/session.go` -- global `bufMux4K`, `bufMux64K` buffer pools
- [ ] `internal/component/bgp/reactor/forward_pool.go` -- global `fwdWriteDeadlineNs` atomic
- [ ] `internal/component/bgp/reactor/forward_build.go` -- global `modBufPool` sync.Pool
- [ ] `internal/component/bgp/reactor/received_update.go` -- global `msgIDCounter` atomic
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- RIBManager, RunRIBPlugin entry point
- [ ] `internal/component/bgp/plugins/rib/register.go` -- RIB plugin registration
- [ ] `internal/component/plugin/server/hub.go` -- Hub, SchemaRegistry, command routing
- [ ] `internal/component/plugin/process/manager.go` -- ProcessManager, `map[string]*Process`
- [ ] `internal/component/hub/hub.go` -- Orchestrator, `NewOrchestrator()`

**Behavior to preserve:**
- Single-instance Ze works exactly as today (unnamed default VRF)
- RIB plugin code unchanged -- connects to whichever hub it receives
- Plugin 5-stage startup protocol unchanged per hub instance
- DirectBridge zero-copy event delivery unchanged
- All existing config, CLI, YANG unchanged for non-VRF deployments

**Behavior to change:**
- Reactor global state (buffer pools, counters, deadlines) moved to per-instance
- ProcessManager supports derived plugin names (`bgp-rib:vrf-red`)
- Hub instantiable as independent instances with separate plugin sets
- New VRF plugin orchestrates per-VRF stack lifecycle
- CLI gains `vrf <name>` prefix for VRF-scoped commands
- YANG schemas wrapped in `vrf <name> {}` container for named VRFs

## Data Flow (MANDATORY)

### Entry Points

| Entry | Format | VRF impact |
|-------|--------|------------|
| BGP wire (TCP) | Binary BGP messages | Per-VRF reactor has own TCP listeners, own peer set |
| CLI command | Text (`show vrf red rib status`) | VRF plugin extracts name, routes to correct instance |
| Config file | YANG-modeled tree | VRF block contains nested peer/RIB/interface config |
| Plugin events | JSON/structured events over DirectBridge | Per-VRF hub delivers events to per-VRF plugins only |
| Runtime API | `vrf add red` / `vrf delete red` | VRF plugin creates/destroys stacks dynamically |

### Transformation Path

1. Config parse: `vrf <name> { bgp { peer { ... } } }` extracted per VRF
2. VRF plugin receives per-VRF config blocks
3. VRF plugin creates per-VRF hub + reactor + RIB plugin instances
4. Each reactor starts its own TCP listeners, manages its own peers
5. Each reactor's events flow to its own hub, delivered to its own RIB instance
6. CLI commands with `vrf <name>` prefix routed to the correct VRF's command handlers

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> VRF plugin | VRF config block extracted by config parser | [ ] |
| VRF plugin -> per-VRF reactor | VRF plugin calls reactor constructor with per-VRF config | [ ] |
| VRF plugin -> per-VRF hub | VRF plugin creates hub, passes to reactor/plugins | [ ] |
| CLI -> VRF plugin -> per-VRF handler | Command dispatch strips VRF prefix, forwards remainder | [ ] |
| Cross-VRF (future) | Goroutine-safe channel or function call between RIB instances | [ ] |

### Integration Points
- `Reactor.New()` -- must accept hub reference instead of assuming global
- `ProcessManager` -- must support derived names and multiple instances
- CLI dispatch -- must recognize `vrf <name>` prefix and route accordingly
- Config parser -- must support `vrf { ... }` blocks containing nested config
- YANG schema registry -- must support per-VRF schema wrapping

### Architectural Verification
- [ ] No bypassed layers (VRF routes through hub, not direct cross-VRF calls)
- [ ] No unintended coupling (VRF instances share only the process, not state)
- [ ] No duplicated functionality (VRF reuses existing reactor, RIB, hub code)
- [ ] Zero-copy preserved (per-VRF DirectBridge, per-VRF buffer pools)

## Reactor Global State Analysis

The following global state in `internal/component/bgp/reactor/` was analyzed for VRF multi-instance support. This is the scope of `spec-vrf-1-reactor-multi.md`.

### All Global State Is Safe to Share

| Global | File | Type | Reason |
|--------|------|------|--------|
| `bufMux4K` | `session.go:53` | Pool + budget | Same process, same memory, global budget is correct |
| `bufMux64K` | `session.go:58` | Pool + budget | Same as above |
| `modBufPool` | `forward_build.go:18` | `sync.Pool` | Stateless buffer pool, no contention concern |
| `fwdWriteDeadlineNs` | `forward_pool.go:53` | `atomic.Int64` | Global tuning knob, not per-VRF |
| `msgIDCounter` | `received_update.go:20` | `atomic.Uint64` | Monotonic process-wide sequence number, non-contiguous IDs per VRF is fine |
| Loggers | Various | Lazy loggers | Read-only, safe to share |
| Error sentinels | Various | `var Err*` | Read-only, safe to share |

### Already Per-Instance (no change needed)

| Component | Status |
|-----------|--------|
| `Reactor.peers` map | Per-instance |
| `Reactor.listeners` map | Per-instance |
| `Reactor.fwdPool` | Per-instance |
| `Reactor.recentUpdates` | Per-instance |
| `Reactor.eventDispatcher` | Per-instance |
| `Reactor.config` | Per-instance (injected) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `vrf red { bgp { peer { ... } } }` | -> | VRF plugin creates reactor+RIB for VRF red | `test/plugin/vrf-basic.ci` |
| `show vrf red rib status` CLI command | -> | VRF dispatches to red's RIB handler | `test/plugin/vrf-cli.ci` |
| BGP UPDATE on VRF red's peer | -> | VRF red's reactor processes, delivers to VRF red's RIB | `test/plugin/vrf-update.ci` |
| `vrf add blue` runtime command | -> | VRF plugin creates new stack dynamically | `test/plugin/vrf-dynamic.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Reactor constructed twice in same process | Both instances operate independently, no shared mutable state |
| AC-2 | Config with two named VRFs | Two independent reactor+RIB stacks running, each with own peers |
| AC-3 | Config with no VRF block | Ze operates exactly as today (unnamed default, no VRF plugin needed) |
| AC-4 | `show vrf red rib status` | Command reaches VRF red's RIB, returns red's routes only |
| AC-5 | `show rib status` (no VRF prefix) | Command reaches default RIB (backwards compatible) |
| AC-6 | BGP UPDATE on VRF red peer | Processed by VRF red's reactor, stored in VRF red's RIB only |
| AC-7 | BGP UPDATE on VRF blue peer | Does not appear in VRF red's RIB |
| AC-8 | `vrf add green` at runtime | New reactor+RIB stack created, peers can be configured |
| AC-9 | `vrf delete red` at runtime | Peers drained gracefully, RIB cleared, resources freed |
| AC-10 | VRF plugin wraps child YANG | `vrf red { rib { ... } }` appears in CLI schema, commands dispatch correctly |
| AC-11 | Two VRFs with same peer IP on different ports | No conflict -- each reactor has own listener and peer map |
| AC-12 | Metrics from VRF red | Labeled with VRF name, distinguishable from VRF blue metrics |

## Design Insights

- All globals are safe to share: pools, budgets, deadlines, message ID counter. Same process, same memory, no isolation needed.
- The reactor is already multi-instance ready. No refactoring needed -- just instantiate it N times.
- The goroutine model means VRF deletion must carefully drain all goroutines (reactor event loop, forward pool workers, RIB event handler) before freeing resources.
- YANG wrapping is the key CLI enabler -- it makes VRF-scoped commands appear naturally in the schema tree without modifying any child module's YANG.
