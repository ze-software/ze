# Spec: reactor-interface-split

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/types.go` - ReactorInterface definition (68 methods)
4. `internal/plugin/server.go` - mixed generic+BGP server (1833 lines)
5. `internal/plugins/bgp/reactor/reactor.go` - Reactor struct implementation
6. `docs/plan/spec-plugin-restructure.md` - predecessor spec (context for deferred items)

## Task

Split the monolithic `ReactorInterface` (68 methods) into a generic lifecycle interface and a BGP-specific interface. Extract ~500 lines of BGP-specific code from `server.go` into a BGP callback layer. This unblocks the remaining file moves deferred from `spec-plugin-restructure.md`.

**Why now:** The plugin restructure spec completed Phases 1-4 (renames, directory moves, type extraction, sub-packages). The remaining Phase 4 items (handler/, update/, validate/) and Phase 5 (server split) are all blocked by the same root cause: `ReactorInterface` mixes generic lifecycle with BGP operations, creating circular imports when BGP code tries to move to `internal/plugins/bgp/`.

**Scope:**
1. Split `ReactorInterface` into `ReactorLifecycle` + `BGPReactor`
2. Extract BGP formatting callbacks from `server.go` (~500 lines)
3. Move BGP handler files to `internal/plugins/bgp/handler/` (unblocked by split)
4. Move BGP errors from `errors.go` to `route/` (unblocked by reduced coupling)

**Out of scope:**
- Moving `update_text.go`/`update_wire.go` — still coupled to `CommandContext`/`Response` (needs shared API types extraction, separate spec)
- Moving `validate_open.go` — still coupled to `Server`/`ProcessManager`
- Moving `json.go`/`text.go` — still coupled to server types

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor role in system
  → Decision: Reactor is the central event loop; Server manages plugin processes
  → Constraint: Engine↔Plugin boundary is JSON over pipes, never direct calls
- [ ] `.claude/rules/plugin-design.md` - plugin registration and lifecycle
  → Constraint: 5-stage protocol is generic; BGP formatting is protocol-specific

### Source Files (MUST read before implementing)
- [ ] `internal/plugin/types.go` - ReactorInterface (68 methods, lines 73-318)
  → Constraint: 16 generic methods, 52 BGP-specific methods
- [ ] `internal/plugin/server.go` - Server struct (1833 lines, ~75% generic)
  → Constraint: BGP callbacks (OnMessageReceived, OnPeerStateChange, etc.) are ~500 lines
- [ ] `internal/plugins/bgp/reactor/reactor.go` - Reactor implementation
  → Constraint: Implements all 68 methods, stores `api *plugin.Server`
- [ ] `internal/plugin/command.go` - CommandContext.Reactor() returns ReactorInterface
  → Constraint: Handler files access reactor through this accessor
- [ ] `docs/plan/spec-plugin-restructure.md` - predecessor spec, deferred items

**Key insights:**
- ReactorInterface is 76% BGP-specific (52/68 methods)
- server.go is 75% generic infrastructure (1300/1833 lines)
- Hub does NOT reference ReactorInterface — it's purely a BGP reactor concern
- No type assertions on ReactorInterface anywhere — always passed as parameter
- RPCProviders injection mechanism already exists (from plugin-restructure Phase 5)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/types.go` - single `ReactorInterface` with 68 methods used by all handler files
- [ ] `internal/plugin/server.go` - `reactor ReactorInterface` field, BGP formatting mixed with generic lifecycle
- [ ] `internal/plugin/command.go` - `CommandContext.Reactor() ReactorInterface` accessor used by all handlers
- [ ] `internal/plugins/bgp/reactor/reactor.go` - `Reactor` struct implements all 68 methods, embeds `APISyncState`

**Behavior to preserve:**
- All existing tests must continue passing
- Plugin 5-stage startup protocol unchanged
- JSON output format (ze-bgp) unchanged
- RPC dispatch behavior unchanged
- Handler behavior unchanged (same methods called, same arguments)

**Behavior to change:**
- `ReactorInterface` splits into two interfaces
- Handler files that use BGP methods access them through `BGPReactor` instead
- Server stores `ReactorLifecycle` (generic); BGP layer holds `BGPReactor` reference
- BGP formatting callbacks registered via hooks instead of direct methods on Server

## Data Flow (MANDATORY)

### Entry Point
- Server created via `NewServer(config, reactor)` — reactor implements both interfaces
- Handler files access reactor via `ctx.Reactor()` in `CommandContext`

### Transformation Path
1. `NewServer` receives full reactor, stores lifecycle portion
2. BGP callback layer registered via hooks (OnMessageReceived, etc.)
3. Handler files that need BGP methods use a BGP-specific accessor
4. Generic server lifecycle (startup, RPC dispatch) uses only lifecycle interface

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server → Reactor (lifecycle) | `ReactorLifecycle` interface (16 methods) | [ ] |
| Server → Reactor (BGP) | `BGPReactor` interface (52 methods) via accessor | [ ] |
| Server → BGP formatting | Callback hooks registered at init | [ ] |

### Integration Points
- `CommandContext.Reactor()` — needs to return appropriate interface for each handler
- `Reactor` struct — implements both interfaces (no change to implementation)
- RPCProviders — already plumbed, used when handlers move to `handler/`

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make test` after interface split | All tests pass, zero failures |
| AC-2 | `make lint` after interface split | Zero lint issues |
| AC-3 | `make functional` after interface split | All functional tests pass |
| AC-4 | BGP handler files in `handler/` package | Handlers compile and work from new package |
| AC-5 | server.go has no BGP imports | Generic server is protocol-agnostic |
| AC-6 | `internal/plugin/errors.go` deleted | BGP errors live in `route/` |

## Design

### Interface Split

**ReactorLifecycle** (stays in `internal/plugin/types.go`, 16 methods):

| Category | Methods |
|----------|---------|
| Introspection | `Peers()`, `Stats()`, `GetPeerProcessBindings()`, `GetPeerCapabilityConfigs()` |
| Lifecycle | `Stop()` |
| Configuration | `Reload()`, `VerifyConfig()`, `ApplyConfigDiff()`, `AddDynamicPeer()`, `RemovePeer()`, `TeardownPeer()`, `GetConfigTree()`, `SetConfigTree()` |
| Startup coordination | `SignalAPIReady()`, `AddAPIProcessCount()`, `SignalPluginStartupComplete()`, `SignalPeerAPIReady()` |

**BGPReactor** (moves to `internal/plugins/bgp/types/reactor.go`, 52 methods):

| Category | Methods | Count |
|----------|---------|-------|
| Route announce | `AnnounceRoute`, `AnnounceFlowSpec`, `AnnounceVPLS`, `AnnounceL2VPN`, `AnnounceL3VPN`, `AnnounceLabeledUnicast`, `AnnounceMUPRoute`, `AnnounceNLRIBatch`, `AnnounceEOR`, `AnnounceWatchdog` | 10 |
| Route withdraw | `WithdrawRoute`, `WithdrawFlowSpec`, `WithdrawVPLS`, `WithdrawL2VPN`, `WithdrawL3VPN`, `WithdrawLabeledUnicast`, `WithdrawMUPRoute`, `WithdrawNLRIBatch`, `WithdrawWatchdog` | 9 |
| BGP messages | `SendBoRR`, `SendEoRR`, `SendRawMessage` | 3 |
| RIB operations | `RIBInRoutes`, `RIBOutRoutes`, `RIBStats`, `ClearRIBIn`, `ClearRIBOut`, `FlushRIBOut` | 6 |
| Transactions | `BeginTransaction`, `CommitTransaction`, `CommitTransactionWithLabel`, `RollbackTransaction`, `InTransaction`, `TransactionID` | 6 |
| Commit | `SendRoutes` | 1 |
| Watchdog routes | `AddWatchdogRoute`, `RemoveWatchdogRoute` | 2 |
| UPDATE cache | `ForwardUpdate`, `DeleteUpdate`, `RetainUpdate`, `ReleaseUpdate`, `ListUpdates` | 5 |

### Server BGP Extraction

BGP-specific methods to extract from server.go (~500 lines):

| Method | Lines | Extraction Target |
|--------|-------|-------------------|
| `OnMessageReceived` | ~30 | BGP callback hook |
| `OnPeerStateChange` | ~25 | BGP callback hook |
| `OnPeerNegotiated` | ~20 | BGP callback hook |
| `OnMessageSent` | ~30 | BGP callback hook |
| `formatMessageForSubscription` | ~30 | BGP callback hook |
| `formatSentMessageForSubscription` | ~10 | BGP callback hook |
| `messageTypeToEventType` | ~15 | BGP callback hook |
| `handleDecodeMPReachRPC` | ~40 | BGP decode registration |
| `handleDecodeMPUnreachRPC` | ~30 | BGP decode registration |
| `handleDecodeUpdateRPC` | ~35 | BGP decode registration |
| `decodeMPNLRIs` | ~25 | BGP decode helper |
| `formatNLRIsAsJSON` | ~15 | BGP decode helper |
| `formatDecodeUpdateJSON` | ~70 | BGP decode helper |
| `EncodeNLRI` | ~35 | BGP codec API |
| `DecodeNLRI` | ~30 | BGP codec API |
| `BroadcastValidateOpen` | ~10 | BGP callback hook |
| `GetDecodeFamilies` | ~5 | BGP query |

### Callback Hook Pattern

Generic server defines hook slots. BGP layer registers implementations at server creation.

| Hook | Purpose | Registered By |
|------|---------|---------------|
| `OnFormatMessage` | Format BGP message for plugin subscription | BGP callback layer |
| `OnPeerEvent` | Handle peer state change / negotiation | BGP callback layer |
| `OnMessageEvent` | Process incoming/outgoing BGP message | BGP callback layer |
| `OnValidateOpen` | Broadcast OPEN validation to plugins | BGP callback layer |

### Handler Move (Unblocked)

After the interface split, handler files can move to `internal/plugins/bgp/handler/` because:
- They import `plugin.BGPReactor` → becomes `bgptypes.BGPReactor` (already in `internal/plugins/bgp/types/`)
- They import `plugin.CommandContext` → stays as `plugin.CommandContext` (generic type)
- RPCProviders injection already plumbed (from plugin-restructure spec)
- 245+ type references shrink significantly since BGP types are now in `bgptypes`

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorImplementsLifecycle` | `internal/plugin/types_test.go` | Reactor satisfies ReactorLifecycle | |
| `TestReactorImplementsBGPReactor` | `internal/plugins/bgp/types/reactor_test.go` | Reactor satisfies BGPReactor | |
| `TestServerAcceptsLifecycle` | `internal/plugin/server_test.go` | Server works with ReactorLifecycle | |
| `TestHandlerAccessesBGPReactor` | `internal/plugins/bgp/handler/*_test.go` | Handlers get BGPReactor via accessor | |
| All existing handler tests | `internal/plugin/*_test.go` → `handler/*_test.go` | Existing behavior preserved | |

### Boundary Tests
Not applicable — structural refactoring, no numeric fields.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing functional tests | `test/` | All BGP functionality unchanged | |

### Future
- Tests verifying non-BGP protocol could plug in (aspirational)

## Files to Modify
- `internal/plugin/types.go` - replace ReactorInterface with ReactorLifecycle (16 methods)
- `internal/plugin/server.go` - remove BGP methods, add callback hooks, store ReactorLifecycle
- `internal/plugin/command.go` - update CommandContext accessor for BGP reactor
- `internal/plugin/mock_reactor_test.go` - split mock into lifecycle + BGP parts
- `internal/plugins/bgp/reactor/reactor.go` - register BGP callbacks on server creation
- All handler files (`bgp.go`, `cache.go`, `commit.go`, `raw.go`, `refresh.go`, `rib_handler.go`) - update reactor access pattern, then move to `handler/`
- `internal/plugin/errors.go` - delete (move errors to route/)
- `internal/plugins/bgp/route/route.go` - absorb errors from errors.go

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | No new RPCs |
| RPC count in architecture docs | No | No new RPCs |
| CLI commands/flags | No | No new commands |
| CLI usage/help text | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | Existing tests cover all behavior |

## Files to Create
- `internal/plugins/bgp/types/reactor.go` - BGPReactor interface (52 methods)
- `internal/plugins/bgp/handler/` - moved handler files (bgp.go, cache.go, commit.go, raw.go, refresh.go, rib_handler.go)
- `internal/plugins/bgp/handler/*_test.go` - moved handler tests

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Split ReactorInterface** - Define ReactorLifecycle in types.go, BGPReactor in bgptypes
   → **Review:** Both interfaces cover all 68 methods? No method lost?

2. **Run tests** - Verify compilation and test pass with both interfaces
   → **Review:** All tests pass? No interface satisfaction errors?

3. **Update Server** - Server stores ReactorLifecycle, add BGP accessor
   → **Review:** Generic server has zero BGP imports?

4. **Extract BGP callbacks** - Move formatting methods to callback hooks
   → **Review:** Callbacks registered correctly? Events still reach plugins?

5. **Move handler files** - Move to `internal/plugins/bgp/handler/`
   → **Review:** RPCProviders injection working? All handlers accessible?

6. **Move errors** - Delete errors.go, merge into route/
   → **Review:** All error references updated?

7. **Verify all** - `make lint && make test && make functional`
   → **Review:** Zero issues? All tests deterministic?

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Interface not satisfied | Compile error on Reactor | Step 1 — check method signatures match |
| Circular import | `import cycle not allowed` | Step 3 — check BGP imports in generic server |
| Handler test fails | Missing reactor method | Step 3 — verify BGP accessor returns correct type |
| Functional test fails | Plugin doesn't receive events | Step 4 — callback registration broken |

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

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-6 all demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Feature code integrated into codebase

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make lint` passes
- [ ] Architecture docs updated with new structure
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### 🏗️ Design
- [ ] No premature abstraction (splitting real mixed interface)
- [ ] No speculative features (no new protocol support)
- [ ] Single responsibility (lifecycle vs BGP operations)
- [ ] Explicit behavior (callback registration, not implicit)
- [ ] Minimal coupling (generic server protocol-agnostic)
- [ ] Next-developer test (clear where to add non-BGP protocol)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Feature code integrated
- [ ] Functional tests verify end-user behavior

### Documentation
- [ ] Required docs read
- [ ] Architecture docs updated with new structure

### Completion
- [ ] Implementation Audit completed
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
