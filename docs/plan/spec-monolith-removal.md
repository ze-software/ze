# Spec: monolith-removal — Decompose internal/plugin/

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-arch-0-system-boundaries.md` — umbrella architecture spec
3. `internal/plugin/types.go` — shared interfaces (ReactorLifecycle, PluginConfig)
4. `internal/plugin/server.go` — current Server orchestrator
5. `internal/component/` — new component packages that replace parts of the monolith

## Task

Decompose `internal/plugin/` (~31 source files, ~18K LOC) into focused sub-packages. The monolith currently bundles process management, 5-stage startup protocol, RPC dispatch, command handlers, config reload, schema registry, and shared types into a single Go package. This makes it difficult to understand responsibility boundaries and creates coupling that blocks the arch-0 integration work (Phases 5–6).

**Not in scope:** wiring the new `internal/component/` skeletons to production code paths (that's Phase 5–6 of arch-0). This spec only breaks the monolith into smaller packages with clear boundaries. All existing tests must continue to pass — pure restructuring, no behavior changes.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-arch-0-system-boundaries.md` — component boundaries, 5-stage protocol
  → Decision: 5 components (Engine, Bus, ConfigProvider, PluginManager, Subsystem)
  → Constraint: Plugin infrastructure manages lifecycle and message routing
- [ ] `docs/architecture/api/process-protocol.md` — multi-process coordination
  → Constraint: Process lifecycle, respawn, socket pair creation
- [ ] `docs/architecture/core-design.md` — system architecture
  → Constraint: Plugin Infrastructure layer description

### Source Files (existing patterns to follow)
- [ ] `internal/plugin/types.go` — ReactorLifecycle (16 methods), PluginConfig, WireEncoding interfaces
  → Constraint: ReactorLifecycle is the BGP-specific coupling point
- [ ] `internal/plugin/server.go` — Server struct, orchestrates everything
  → Constraint: Server depends on ProcessManager, StartupCoordinator, schema, pending, reload
- [ ] `internal/plugin/process.go` — Process struct, subprocess spawning
  → Constraint: Tightly coupled to process_manager.go and process_delivery.go
- [ ] `internal/plugin/server_startup.go` — 5-stage protocol implementation
  → Constraint: Imports registration types, socket pairs, RPC connections

**Key insights:**
- ReactorLifecycle (16 methods) in types.go is the primary BGP coupling — Server calls reactor methods directly
- Server is the central orchestrator — decomposing it requires extracting its dependencies first
- Process management (process.go + process_manager.go + process_delivery.go) is a self-contained unit
- The 4 leaf packages (registry/, cli/, bgp/shared/, all/) are already well-separated

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/types.go` (311 LOC) — ReactorLifecycle, PluginConfig, WireEncoding, EventFormatter
- [ ] `internal/plugin/server.go` (430 LOC) — Server struct, NewServer, component composition
- [ ] `internal/plugin/server_dispatch.go` (521 LOC) — command routing, response handling
- [ ] `internal/plugin/server_startup.go` (673 LOC) — 5-stage protocol orchestration
- [ ] `internal/plugin/server_startup_text.go` (248 LOC) — text-mode 5-stage variant
- [ ] `internal/plugin/server_events.go` (82 LOC) — NLRI encode/decode routing
- [ ] `internal/plugin/server_client.go` (104 LOC) — per-client connection handler
- [ ] `internal/plugin/process.go` (690 LOC) — Process struct, lifecycle, subprocess spawning
- [ ] `internal/plugin/process_manager.go` (272 LOC) — multi-process coordination, respawn
- [ ] `internal/plugin/process_delivery.go` (218 LOC) — per-process event delivery goroutine
- [ ] `internal/plugin/startup_coordinator.go` (244 LOC) — multi-plugin barrier sync
- [ ] `internal/plugin/registration.go` (382 LOC) — stage data types, validators
- [ ] `internal/plugin/command.go` (446 LOC) — dispatch router, RPC aggregation
- [ ] `internal/plugin/system.go` (398 LOC) — 11 system-level RPCs
- [ ] `internal/plugin/plugin.go` (104 LOC) — 4 plugin lifecycle RPCs
- [ ] `internal/plugin/session.go` (67 LOC) — 3 session RPCs
- [ ] `internal/plugin/subscribe.go` (359 LOC) — event subscription parsing
- [ ] `internal/plugin/handler.go` (23 LOC) — constants, type stubs
- [ ] `internal/plugin/schema.go` (342 LOC) — RPC method index per plugin
- [ ] `internal/plugin/pending.go` (234 LOC) — in-flight command tracking
- [ ] `internal/plugin/reload.go` (443 LOC) — verify→apply config reload
- [ ] `internal/plugin/resolve.go` (144 LOC) — plugin string resolution
- [ ] `internal/plugin/hub.go` (208 LOC) — command routing hub
- [ ] `internal/plugin/socketpair.go` (155 LOC) — Unix socket pair creation
- [ ] `internal/plugin/rpc_plugin.go` (256 LOC) — typed RPC wrapper
- [ ] `internal/plugin/inprocess.go` (96 LOC) — internal plugin runner registry
- [ ] `internal/plugin/rib_handler.go` (163 LOC) — RIB metadata handler
- [ ] `internal/plugin/events.go` (50 LOC) — event namespace constants
- [ ] `internal/plugin/validator.go` (25 LOC) — port/family validators
- [ ] `internal/plugin/subsystem.go` (386 LOC) — subsystem startup orchestration
- [ ] `internal/plugin/subsystem_text.go` (127 LOC) — text-mode subsystem protocol

**Behavior to preserve:**
- All existing test expectations
- Plugin 5-stage startup protocol (JSON and text modes)
- Process lifecycle (start, stop, respawn, wait)
- RPC dispatch and response routing
- Config reload verify→apply protocol
- Socket pair creation for inter-process communication

**Behavior to change:**
- None — pure package reorganization

## Data Flow (MANDATORY)

### Entry Point
- Plugin startup: `Server.Start()` → `ProcessManager.Start()` → per-process 5-stage
- RPC commands: stdin text/JSON → `Server.dispatch()` → handler → response
- Events: reactor → `Server.Broadcast()` → per-process delivery goroutine
- Config reload: `Server.Reload()` → verify→apply per plugin

### Transformation Path
1. Server creates ProcessManager with plugin configs
2. ProcessManager spawns Process per plugin (fork or goroutine)
3. StartupCoordinator synchronizes 5-stage across all processes
4. After stage 5, Server routes events to delivery goroutines
5. RPC requests from plugins dispatched through command.go to handlers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server ↔ ProcessManager | Direct method calls | [ ] |
| Process ↔ 5-stage protocol | Socket pair + RPC | [ ] |
| Server ↔ Command handlers | Direct method calls via dispatch | [ ] |
| Server ↔ ReactorLifecycle | Interface (16 methods) — BGP coupling | [ ] |

### Integration Points
- `ReactorLifecycle` interface in types.go binds Server to BGP reactor
- `PluginConfig` in types.go shared between Server, ProcessManager, Process
- `registration.go` types shared between startup protocol and Server

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Target Package Structure

The monolith decomposes into focused packages under `internal/plugin/`:

| Package | Files | LOC | Responsibility |
|---------|-------|-----|----------------|
| `internal/plugin/` (root) | types.go, events.go, validator.go, resolve.go, inprocess.go, hub.go | ~530 | Shared types, interfaces, constants |
| `internal/plugin/process/` | process.go, process_manager.go, process_delivery.go | ~1,180 | Process lifecycle, respawn, event delivery |
| `internal/plugin/startup/` | server_startup.go, server_startup_text.go, startup_coordinator.go, registration.go | ~1,550 | 5-stage protocol orchestration |
| `internal/plugin/server/` | server.go, server_dispatch.go, server_events.go, server_client.go, subsystem.go, subsystem_text.go | ~1,650 | Server core, dispatch, subsystem orchestration |
| `internal/plugin/handler/` | command.go, system.go, plugin.go, session.go, subscribe.go, handler.go | ~1,420 | RPC command implementations |
| `internal/plugin/rpc/` | rpc_plugin.go, pending.go, socketpair.go | ~645 | RPC connections, request tracking, socket pairs |
| `internal/plugin/reload/` | reload.go | ~443 | Config reload protocol |
| `internal/plugin/schema/` | schema.go, rib_handler.go | ~505 | RPC method registry, RIB metadata |
| (unchanged) `registry/` | existing | ~583 | Central plugin registration |
| (unchanged) `cli/` | existing | ~265 | Shared CLI framework |
| (unchanged) `all/` | existing | ~35 | Blank imports |

**Total: ~8,800 LOC in 8 new sub-packages + ~530 LOC remaining in root**

## Phased Execution

Pure restructuring proceeds bottom-up: extract leaves first (no dependents), then intermediate packages, then the Server core last.

### Phase A — Extract leaf packages (no dependents within monolith)

| Step | Move to | Files | Why leaf |
|------|---------|-------|----------|
| A-1 | `plugin/rpc/` | socketpair.go, rpc_plugin.go, pending.go | No in-package dependents except Server/Process |
| A-2 | `plugin/schema/` | schema.go, rib_handler.go | Only consumed by Server and handlers |

### Phase B — Extract cohesive units

| Step | Move to | Files | Dependency |
|------|---------|-------|------------|
| B-1 | `plugin/process/` | process.go, process_manager.go, process_delivery.go | Imports rpc/ types, root types |
| B-2 | `plugin/startup/` | server_startup.go, server_startup_text.go, startup_coordinator.go, registration.go | Imports process/, rpc/, root types |
| B-3 | `plugin/reload/` | reload.go | Imports process/, rpc/ |

### Phase C — Extract handlers and Server

| Step | Move to | Files | Dependency |
|------|---------|-------|------------|
| C-1 | `plugin/handler/` | command.go, system.go, plugin.go, session.go, subscribe.go, handler.go | Imports process/, rpc/, schema/, root types |
| C-2 | `plugin/server/` | server.go, server_dispatch.go, server_events.go, server_client.go, subsystem.go, subsystem_text.go | Imports all sub-packages |

### Phase D — Clean root package

| Step | Action |
|------|--------|
| D-1 | Root retains only: types.go, events.go, validator.go, resolve.go, inprocess.go, hub.go |
| D-2 | Review ReactorLifecycle — document BGP coupling for Phase 6 elimination |
| D-3 | Update all `// Design:` and `// Related:` references |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| All existing functional tests | → | Same code, new package paths | `make ze-functional-test` passes unchanged |
| All existing unit tests | → | Same logic, updated imports | `go test ./internal/plugin/...` passes |
| Plugin 5-stage startup | → | startup/ package | Existing `TestCompleteProtocol*` tests |
| Process respawn | → | process/ package | Existing `TestRespawn*` tests |
| RPC dispatch | → | handler/ + server/ | Existing `TestCommand*`, `TestServer*` tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make test-all` after each phase | All tests pass — zero regressions |
| AC-2 | Each sub-package | Imports only from root or lower-layer sub-packages (no circular deps) |
| AC-3 | Root package after completion | Contains only shared types and constants (~530 LOC) |
| AC-4 | `go build ./...` | Clean compilation, no import cycles |
| AC-5 | External importers of `internal/plugin` | `internal/plugin` types still accessible (re-exported if moved) |
| AC-6 | Each sub-package | Single cohesive responsibility |
| AC-7 | File count per sub-package | No sub-package exceeds 6 source files |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| All existing `internal/plugin/*_test.go` | Same locations, updated imports | No regressions | |
| `TestNoImportCycles` | `internal/plugin/import_test.go` | No circular dependencies between sub-packages | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing functional tests | `test/` | Unchanged behavior | |

## Files to Modify

**Every `.go` file in `internal/plugin/`** — files move to sub-packages, imports updated.

**External importers** (49 files from grep survey):
- `internal/plugins/bgp/handler/` — imports `internal/plugin` for types
- `internal/plugins/bgp/server/` — imports `internal/plugin` for Server, events
- `internal/plugins/bgp/reactor/` — imports `internal/plugin` for types
- `internal/config/` — imports `internal/plugin` for PluginConfig
- `internal/hub/` — imports `internal/plugin` for Server
- `cmd/ze/` — imports `internal/plugin` for CLI dispatch

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A — no new RPCs |
| RPC count in architecture docs | No | N/A — no new RPCs |
| CLI commands/flags | No | N/A — no CLI changes |
| Plugin SDK docs | No | N/A — no SDK changes |
| `// Design:` references | Yes | All moved files |
| `// Related:` references | Yes | All moved files + siblings |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugin/process/` (dir) | Process lifecycle package |
| `internal/plugin/startup/` (dir) | 5-stage protocol package |
| `internal/plugin/server/` (dir) | Server core package |
| `internal/plugin/handler/` (dir) | RPC command handlers package |
| `internal/plugin/rpc/` (dir) | RPC connections and tracking |
| `internal/plugin/reload/` (dir) | Config reload protocol |
| `internal/plugin/schema/` (dir) | RPC method registry |

Note: `internal/plugin/handler/` will conflict with `internal/plugins/bgp/handler/`. Despite same last segment, full import paths differ. If confusing, alternative name: `internal/plugin/command/`.

## Implementation Steps

Each phase is a separate commit. Each commit must pass `make test-all`.

1. **Phase A-1:** Move socketpair.go, rpc_plugin.go, pending.go → `plugin/rpc/`. Update imports. Run tests.
2. **Phase A-2:** Move schema.go, rib_handler.go → `plugin/schema/`. Update imports. Run tests.
3. **Phase B-1:** Move process files → `plugin/process/`. Update imports. Run tests.
4. **Phase B-2:** Move startup files → `plugin/startup/`. Update imports. Run tests.
5. **Phase B-3:** Move reload.go → `plugin/reload/`. Update imports. Run tests.
6. **Phase C-1:** Move command handler files → `plugin/handler/`. Update imports. Run tests.
7. **Phase C-2:** Move server files → `plugin/server/`. Update imports. Run tests.
8. **Phase D:** Clean root, update `// Design:` and `// Related:` references. Final `make test-all`.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import cycle | Identify coupled types, move shared type to root package |
| Test compilation error | Update test imports to new package paths |
| External importer breaks | Re-export moved types from root via type alias or wrapper |
| `// Design:` hook fails | Update design references in moved files |

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Import cycles between sub-packages | Bottom-up extraction — leaves first, orchestrators last |
| External importers break | Root re-exports moved types until external importers are updated |
| ReactorLifecycle couples server/ to BGP | Keep in root (types.go) — Phase 6 eliminates it |
| handler/ name conflicts with bgp/handler/ | Use `command/` as alternative package name if needed |
| Large number of external importers (49 files) | Phase each commit, fix importers incrementally |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.

## Design Insights

- The monolith exists because Go's flat package model made it easy to add files without thinking about boundaries. The 5-stage protocol, process management, and RPC handling are genuinely separate concerns that ended up in one package through incremental growth.
- ReactorLifecycle (16 methods) is the primary BGP coupling point. It should NOT be moved into a sub-package — it stays in root until Phase 6 of arch-0 eliminates it entirely.
- The 4 existing leaf packages (registry/, cli/, bgp/shared/, all/) prove the pattern works. This spec extends it to the remaining 31 files.
