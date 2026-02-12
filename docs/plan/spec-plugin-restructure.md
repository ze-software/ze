# Spec: plugin-restructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - plugin architecture
4. `internal/plugin/types.go` - mixed types (the hardest file)
5. `internal/plugin/server.go` - mixed server (the largest file)
6. `internal/plugin/command.go` - RPC aggregation

## Task

Separate BGP-specific code from generic plugin infrastructure in `internal/plugin/`.

Currently `internal/plugin/` contains 43 non-test Go files, all `package plugin`. Of these, 26 are BGP-specific, 14 are generic infrastructure, and 4 are mixed. The existing `internal/plugin/bgp/` subdirectory (reactor, attribute, nlri, etc.) is also BGP-specific.

Goals:
1. Rename BGP extension plugins to `bgp-<name>` pattern for clarity
2. Move `internal/plugin/bgp/` engine into `internal/plugins/bgp/`
3. Move BGP-specific flat files into sub-packages under `internal/plugins/bgp/`
4. Split mixed files (`types.go`, `server.go`, `command.go`, `subscribe.go`) at the generic/BGP boundary
5. `internal/plugin/` becomes purely generic plugin infrastructure

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
- [ ] `.claude/rules/plugin-design.md` - plugin registration and lifecycle

### Source Files (MUST read before implementing each phase)
- [ ] `internal/plugin/types.go` - mixed generic+BGP types, ReactorInterface
- [ ] `internal/plugin/server.go` - mixed generic+BGP server (47K)
- [ ] `internal/plugin/command.go` - RPC dispatch aggregation
- [ ] `internal/plugin/subscribe.go` - event constants and subscription manager
- [ ] `internal/plugin/registration.go` - plugin stage types and registry
- [ ] `internal/plugin/process.go` - process lifecycle (generic)
- [ ] `internal/plugin/rpc_plugin.go` - typed RPC IPC (generic)

**Key insights:**
- All 43 files share `package plugin` — moving files to subdirectories creates new packages, breaking unexported symbol access
- `ReactorInterface` in `types.go` has ~40 methods mixing generic lifecycle with BGP-specific route operations
- `server.go` is 47K mixing plugin startup protocol with BGP message formatting
- The reactor (`internal/plugin/bgp/reactor/`) imports 35 types from `internal/plugin` — many are BGP-specific types that should move with the BGP code
- `internal/hub/` imports only 5 generic types (SubsystemManager, SchemaRegistry, Hub, Schema, SubsystemConfig)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/types.go` - defines Response, ServerConfig, PluginConfig (generic) AND RouteSpec, FlowSpecRoute, WireUpdate, ReactorInterface (BGP-specific) in one file
- [ ] `internal/plugin/server.go` - plugin startup stages 1-5, config/registry delivery (generic) AND OnMessageReceived, formatMessageForSubscription, BroadcastValidateOpen (BGP-specific)
- [ ] `internal/plugin/command.go` - Dispatcher, Handler, CommandContext (generic) AND BgpPluginRPCs() aggregation (BGP-specific)
- [ ] `internal/plugin/subscribe.go` - SubscriptionManager (generic) AND EventUpdate/EventOpen constants (BGP-specific)

**Behavior to preserve:**
- All existing tests must continue passing
- Plugin registration protocol (5-stage) unchanged
- Plugin registry (internal/plugin/registry/) stays as leaf package
- CLI framework (internal/plugin/cli/) stays as leaf package
- Auto-generated all.go mechanism unchanged
- JSON output format (ze-bgp) unchanged
- Text output format unchanged

**Behavior to change:**
- Plugin extension directory names: `evpn` → `bgp-evpn`, etc.
- Plugin registration names: `"evpn"` → `"bgp-evpn"`, etc.
- Import paths: `internal/plugin/bgp/` → `internal/plugins/bgp/`
- Import paths: BGP-specific flat files get new package paths under `internal/plugins/bgp/`

## Data Flow (MANDATORY)

### Entry Point
- Plugin system is initialized by `cmd/ze/main.go` and `internal/hub/hub.go`
- Plugin server created via `plugin.NewServer()` with `ServerConfig`
- Reactor implements `ReactorInterface` and is passed to server

### Transformation Path
1. Hub creates SubsystemManager + SchemaRegistry (generic)
2. Server starts, spawns plugin processes (generic)
3. 5-stage protocol completes (generic)
4. BGP messages arrive → Server formats them → dispatches to plugins (mixed)
5. Plugin commands arrive → Server dispatches via RPC handlers (mixed)
6. Route commands parsed → RouteSpec/FlowSpecRoute created → Reactor called (BGP-specific)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub → Plugin Server | SubsystemManager, SchemaRegistry types | [ ] |
| Server → Reactor | ReactorInterface (40+ methods) | [ ] |
| Server → Plugins | JSON/text formatting via subscribe/format | [ ] |
| Config → Plugin | RouteNextHop, YANG functions | [ ] |
| CLI → Plugin | ParseRouteAttributes, RouteSpec types | [ ] |

### Integration Points
- `internal/plugin/bgp/reactor/` imports 35 symbols from `internal/plugin`
- `internal/hub/` imports 5 symbols from `internal/plugin`
- `internal/config/` imports 10 symbols from `internal/plugin`
- `cmd/ze/` imports 28 symbols from `internal/plugin`

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Design

### Target Directory Structure

```
internal/plugins/
  bgp/                    # BGP protocol engine (moved from internal/plugin/bgp/)
    attribute/            # existing
    capability/           # existing
    context/              # existing
    fsm/                  # existing
    message/              # existing
    nlri/                 # existing
    reactor/              # existing
    rib/                  # existing
    wire/                 # existing
    schema/               # existing
    server/               # NEW: BGP event handling extracted from server.go
    format/               # NEW: text/json message formatting
    route/                # NEW: route parsing and keywords
    update/               # NEW: update text/wire building
    types/                # NEW: BGP-specific types from types.go
  bgp-evpn/              # was evpn/
  bgp-flowspec/          # was flowspec/
  bgp-gr/                # was gr/
  bgp-hostname/          # was hostname/
  bgp-llnh/              # was llnh/
  bgp-ls/                # was bgpls/
  bgp-rib/               # was rib/
  bgp-role/              # was role/
  bgp-rr/                # was rr/
  bgp-vpn/               # was vpn/

internal/plugin/          # Generic plugin infrastructure ONLY
  all/                    # auto-generated imports (unchanged)
  cli/                    # CLI framework (unchanged)
  registry/               # central registry (unchanged)
  # Generic files stay here:
  handler.go              # RPCRegistration type
  hub.go                  # command routing via YANG
  inprocess.go            # internal plugin runner
  pending.go              # in-flight request tracking
  plugin.go               # plugin help/command RPCs
  process.go              # process lifecycle
  registry.go             # command registry
  reload.go               # config reload coordination
  resolve.go              # plugin resolution
  rpc_plugin.go           # typed RPC IPC
  schema.go               # YANG schema management
  server.go               # generic plugin server (lifecycle only)
  session.go              # session ready/ping/bye
  socketpair.go           # socket pair creation
  startup_coordinator.go  # stage sync
  subsystem.go            # forked subsystem management
  system.go               # system RPCs
  types.go                # generic types only (Response, ServerConfig, etc.)
```

### Go Package Naming

Directory names with hyphens are valid in Go. The package declaration uses a valid identifier:

| Directory | Package Name | Registration Name |
|-----------|-------------|-------------------|
| `bgp-evpn/` | `package bgpevpn` | `"bgp-evpn"` |
| `bgp-flowspec/` | `package bgpflowspec` | `"bgp-flowspec"` |
| `bgp-gr/` | `package bgpgr` | `"bgp-gr"` |
| `bgp-hostname/` | `package bgphostname` | `"bgp-hostname"` |
| `bgp-llnh/` | `package bgpllnh` | `"bgp-llnh"` |
| `bgp-ls/` | `package bgpls` | `"bgp-ls"` |
| `bgp-rib/` | `package bgprib` | `"bgp-rib"` |
| `bgp-role/` | `package bgprole` | `"bgp-role"` |
| `bgp-rr/` | `package bgprr` | `"bgp-rr"` |
| `bgp-vpn/` | `package bgpvpn` | `"bgp-vpn"` |

### ReactorInterface Split

The biggest design challenge. Currently one interface with ~40 methods. Split into:

| Interface | Location | Methods |
|-----------|----------|---------|
| `ReactorLifecycle` | `internal/plugin/types.go` | `Stop()`, `Reload()`, `Peers()`, `Stats()`, `VerifyConfig()`, `ApplyConfigDiff()`, `AddDynamicPeer()`, `RemovePeer()`, `TeardownPeer()`, `SignalAPIReady()`, `AddAPIProcessCount()`, `SignalPluginStartupComplete()`, `SignalPeerAPIReady()`, `GetPeerProcessBindings()`, `GetPeerCapabilityConfigs()`, `GetConfigTree()`, `SetConfigTree()` |
| `BGPReactor` | `internal/plugins/bgp/types/` | `AnnounceRoute()`, `WithdrawRoute()`, `AnnounceFlowSpec()`, `WithdrawFlowSpec()`, `AnnounceVPLS()`, `AnnounceL2VPN()`, `AnnounceL3VPN()`, `AnnounceLabeledUnicast()`, `AnnounceMUPRoute()`, `AnnounceNLRIBatch()`, `WithdrawNLRIBatch()`, `AnnounceEOR()`, `SendBoRR()`, `SendEoRR()`, `ForwardUpdate()`, `DeleteUpdate()`, `RetainUpdate()`, `ReleaseUpdate()`, `ListUpdates()`, `RIBInRoutes()`, `RIBOutRoutes()`, `RIBStats()`, `ClearRIBIn()`, `ClearRIBOut()`, `FlushRIBOut()`, `SendRawMessage()`, `SendRoutes()`, all watchdog/transaction methods |

The generic server stores `ReactorLifecycle`. BGP-specific code type-asserts to `BGPReactor` when needed, or the BGP server layer wraps the generic server and holds the full reactor reference.

### Circular Import Prevention

The risk: generic `server.go` calls BGP formatting code, while BGP formatting imports generic types.

Solution: **callback registration pattern**

The generic server defines callback hooks. The BGP layer registers implementations at init time:

| Hook | Purpose | Currently in |
|------|---------|-------------|
| `OnFormatMessage` | Format BGP message for subscription | `server.go` (formatMessageForSubscription) |
| `OnPeerStateChange` | Handle peer state change event | `server.go` (OnPeerStateChange) |
| `OnPeerNegotiated` | Handle negotiated capabilities event | `server.go` (OnPeerNegotiated) |
| `OnMessageReceived` | Process incoming BGP message | `server.go` (OnMessageReceived) |
| `OnValidateOpen` | Broadcast OPEN validation to plugins | `validate_open.go` |
| `OnDecodeNLRI` | Decode NLRI for a family | `server.go` |
| `OnEncodeNLRI` | Encode NLRI from args | `server.go` |

The BGP layer registers these callbacks when the server is created, injecting BGP behavior without the generic server importing BGP packages.

## Phases

### Phase 1: Rename BGP extension plugins

Mechanical rename of 10 plugin directories + update registration names, imports, generator script, functional tests, and docs. No architectural changes.

| Current | New Directory | New Package | New Name |
|---------|--------------|-------------|----------|
| `internal/plugins/evpn/` | `internal/plugins/bgp-evpn/` | `bgpevpn` | `"bgp-evpn"` |
| `internal/plugins/flowspec/` | `internal/plugins/bgp-flowspec/` | `bgpflowspec` | `"bgp-flowspec"` |
| `internal/plugins/gr/` | `internal/plugins/bgp-gr/` | `bgpgr` | `"bgp-gr"` |
| `internal/plugins/hostname/` | `internal/plugins/bgp-hostname/` | `bgphostname` | `"bgp-hostname"` |
| `internal/plugins/llnh/` | `internal/plugins/bgp-llnh/` | `bgpllnh` | `"bgp-llnh"` |
| `internal/plugins/bgpls/` | `internal/plugins/bgp-ls/` | `bgpls` | `"bgp-ls"` |
| `internal/plugins/rib/` | `internal/plugins/bgp-rib/` | `bgprib` | `"bgp-rib"` |
| `internal/plugins/role/` | `internal/plugins/bgp-role/` | `bgprole` | `"bgp-role"` |
| `internal/plugins/rr/` | `internal/plugins/bgp-rr/` | `bgprr` | `"bgp-rr"` |
| `internal/plugins/vpn/` | `internal/plugins/bgp-vpn/` | `bgpvpn` | `"bgp-vpn"` |

Impacted areas:
- All `register.go` files (package name + Registration.Name)
- All plugin `.go` files (package declaration)
- Import paths in `internal/plugin/text.go`, `update_text.go`, `json_test.go`, `update_text_test.go`
- `scripts/gen-plugin-imports.go` (discovery path)
- `internal/plugin/all/all.go` (regenerated)
- `internal/plugin/all/all_test.go` (expected plugin names)
- Config files in `test/` (plugin references)
- Functional test `.ci` files (plugin names)
- `.claude/rules/plugin-design.md` (documentation paths)

### Phase 2: Move BGP engine directory

Move `internal/plugin/bgp/` → `internal/plugins/bgp/`. Purely mechanical path update.

Files that import `internal/plugin/bgp/...`:
- All 26 BGP-specific flat files in `internal/plugin/` (attribute, message, nlri, context, rib, wire imports)
- `internal/plugin/types.go` (imports attribute, message, nlri, rib)
- `cmd/ze/bgp/encode.go`
- `internal/config/loader.go`
- Various test files

This is a large sed + verify operation, similar to the `internal/plugin/<name>` → `internal/plugins/<name>` move already completed.

### Phase 3: Split types.go

Split `internal/plugin/types.go` into generic and BGP-specific parts.

**Stay in `internal/plugin/types.go`** (generic):
- `Response`, `ResponseWrapper`, `WrapResponse`, `NewResponse`, `NewErrorResponse`
- `PeerInfo`, `PeerCapabilityConfig`, `ReactorStats`
- `PluginConfig`, `ServerConfig`
- `ContentConfig`, `WireEncoding`, `ParseWireEncoding`
- `PeerProcessBinding`, `StateChangeReceiver`
- `RIBStatsInfo`
- `DynamicPeerConfig`
- Status constants, format constants
- `ReactorLifecycle` interface (new, generic subset)

**Move to `internal/plugins/bgp/types/`** (BGP-specific):
- `RouteSpec`, `FlowSpecRoute`, `VPLSRoute`, `L2VPNRoute`, `L3VPNRoute`, `LabeledUnicastRoute`, `MUPRouteSpec`
- `FlowSpecActions`
- `RouteNextHop`, `NextHopExplicit`, `NextHopSelf`, `NextHopUnset`, `NewNextHopExplicit`, `NewNextHopSelf`
- `NLRIGroup`, `UpdateTextResult`, `NLRIBatch`
- `RawMessage`, `WireUpdate` reference
- `TransactionResult`
- `LargeCommunity` alias
- AFI/SAFI name constants
- MUP route type constants
- `BGPReactor` interface (new, BGP-specific subset)

### Phase 4: Move BGP-specific flat files

Move 23 clearly-BGP files from `internal/plugin/` into sub-packages under `internal/plugins/bgp/`:

| Sub-package | Files | Responsibility |
|-------------|-------|----------------|
| `internal/plugins/bgp/format/` | `json.go`, `text.go`, `format_buffer.go`, `decode.go` | Message formatting (text/JSON/decode) |
| `internal/plugins/bgp/route/` | `route.go`, `route_keywords.go` | Route parsing for all families |
| `internal/plugins/bgp/update/` | `update_text.go`, `update_wire.go` | "update text/hex" command building |
| `internal/plugins/bgp/wireu/` | `wire_update.go`, `wire_update_split.go`, `wire_extract.go`, `mpwire.go`, `nexthop.go` | WireUpdate, MP wire, next-hop (if not in types) |
| `internal/plugins/bgp/filter/` | `filter.go` | Attribute/NLRI filtering |
| `internal/plugins/bgp/handler/` | `bgp.go`, `cache.go`, `commit.go`, `commit_manager.go`, `raw.go`, `refresh.go`, `rib_handler.go` | BGP RPC handlers |
| `internal/plugins/bgp/errors/` | `errors.go` | BGP-specific error types |
| `internal/plugins/bgp/validate/` | `validate_open.go` | OPEN validation broadcast |

Each sub-package has its own `package` declaration and exports the types/functions needed by other packages.

### Phase 5: Split mixed files

Split `server.go`, `command.go`, `subscribe.go`, `registration.go` at the generic/BGP boundary:

**server.go** split:
- Generic stays: plugin startup, stage transitions, config/registry delivery, client management, RPC dispatch loop
- BGP moves to `internal/plugins/bgp/server/`: message formatting, event dispatch, OPEN validation, NLRI decode/encode
- Connection: generic server defines callback hooks, BGP server registers implementations

**command.go** split:
- Generic stays: `Dispatcher`, `Handler`, `CommandContext`, `AllBuiltinRPCs()`, `SystemPluginRPCs()`, `PluginLifecycleRPCs()`
- BGP moves: `BgpPluginRPCs()` aggregation moves to BGP handler package
- Connection: RPC registration becomes dynamic (BGP layer registers its RPCs with the dispatcher)

**subscribe.go** split:
- Generic stays: `SubscriptionManager`, `Subscription`, `PeerFilter`, matching logic
- BGP moves: `EventUpdate`, `EventOpen`, etc. constants, `validBgpEvents` map
- Connection: event type validation becomes pluggable (BGP layer registers valid event types)

**registration.go**:
- Mostly generic (PluginStage, PluginRegistration, PluginCapabilities, PluginRegistry)
- `InjectedCapability` has BGP semantics but the struct is protocol-agnostic (code + bytes)
- Likely stays as-is with minor cleanup

## 🧪 TDD Test Plan

### Unit Tests

Each phase has its own verification:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| Phase 1: `TestAllPluginsRegistered` | `internal/plugin/all/all_test.go` | All 10 renamed plugins register | |
| Phase 1: `TestFamilyMappings` | `internal/plugin/all/all_test.go` | Families map to renamed plugins | |
| Phase 1: `TestCapabilityMappings` | `internal/plugin/all/all_test.go` | Capabilities map to renamed plugins | |
| Phase 2: existing reactor tests | `internal/plugins/bgp/reactor/*_test.go` | Reactor works with new import path | |
| Phase 3: existing types tests | `internal/plugin/types_test.go` | Generic types still work | |
| Phase 3: new BGP types tests | `internal/plugins/bgp/types/*_test.go` | BGP types work from new location | |
| Phase 4: existing route tests | moved test files | Route parsing works from new package | |
| Phase 5: server lifecycle tests | `internal/plugin/server_test.go` | Generic server lifecycle works | |
| Phase 5: BGP server tests | `internal/plugins/bgp/server/*_test.go` | BGP event handling works | |

### Boundary Tests

Not applicable — this is a structural refactoring, not adding numeric fields.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing functional tests | `test/` | All BGP functionality unchanged | |
| Plugin name in config | `test/plugin/` | Config uses `bgp-evpn` not `evpn` | |

### Future
- Integration tests verifying non-BGP protocol could plug into generic infrastructure (aspirational, no concrete protocol yet)

## Files to Modify

Phase 1:
- `internal/plugins/*/register.go` - package name + Registration.Name for all 10 plugins
- `internal/plugins/*/*.go` - package declarations
- `internal/plugin/text.go`, `update_text.go` - import paths for evpn/flowspec/vpn
- `internal/plugin/json_test.go`, `update_text_test.go` - import paths
- `scripts/gen-plugin-imports.go` - discovery patterns
- `internal/plugin/all/all.go` - regenerated
- `internal/plugin/all/all_test.go` - expected names
- `.claude/rules/plugin-design.md` - documentation

Phase 2:
- ~30 files importing `internal/plugin/bgp/...` - path update to `internal/plugins/bgp/...`

Phase 3:
- `internal/plugin/types.go` - remove BGP types, keep generic types
- `internal/plugins/bgp/types/*.go` - new package with BGP types
- All files referencing moved types - import updates

Phase 4:
- 23 BGP-specific files - move to sub-packages under `internal/plugins/bgp/`
- All files referencing moved functions/types - import updates

Phase 5:
- `internal/plugin/server.go` - extract BGP code
- `internal/plugin/command.go` - extract BGP RPC aggregation
- `internal/plugin/subscribe.go` - extract BGP event constants

## Files to Create

Phase 3:
- `internal/plugins/bgp/types/types.go` - BGP-specific types
- `internal/plugins/bgp/types/reactor.go` - BGPReactor interface

Phase 4:
- Sub-package directories under `internal/plugins/bgp/` (format/, route/, update/, wireu/, filter/, handler/, errors/, validate/)

Phase 5:
- `internal/plugins/bgp/server/server.go` - BGP event handling

## Implementation Steps

Phases 1 and 2 are mechanical (git mv + sed). Phases 3-5 are architectural and require careful interface design.

Each phase is independently committable and testable. The system works correctly after each phase.

1. **Phase 1: Rename BGP extension plugins**
   - git mv each plugin directory
   - Update package declarations and Registration.Name
   - Update all import paths
   - Regenerate all.go
   - Update all_test.go expected names
   - Update functional tests and config files
   → **Review:** Do all tests pass? Are all references updated?

2. **Phase 2: Move BGP engine directory**
   - git mv `internal/plugin/bgp/` → `internal/plugins/bgp/`
   - Update all import paths (~30 files)
   → **Review:** Do all tests pass? No stale imports?

3. **Phase 3: Split types.go**
   - Create `internal/plugins/bgp/types/` package
   - Move BGP-specific types (RouteSpec, FlowSpecRoute, etc.)
   - Split ReactorInterface into ReactorLifecycle + BGPReactor
   - Update all importers
   → **Review:** Interface split clean? No import cycles?

4. **Phase 4: Move BGP-specific flat files**
   - Create sub-packages (format/, route/, update/, etc.)
   - Move files, update package declarations
   - Export needed symbols
   - Update all importers
   → **Review:** Package boundaries clean? No circular imports?

5. **Phase 5: Split mixed files**
   - Extract BGP code from server.go via callback hooks
   - Extract BgpPluginRPCs() via dynamic registration
   - Extract event constants via pluggable validation
   - Update all importers
   → **Review:** Generic server truly protocol-agnostic?

6. **Verify all** - `make lint && make test && make functional` after each phase

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [To be filled]

### Design Insights
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Rename BGP extensions to bgp-<name> | ✅ Done | `internal/plugins/bgp-*/` | Committed b42ead8 |
| Move internal/plugin/bgp/ to internal/plugins/bgp/ | ✅ Done | `internal/plugins/bgp/` | 181 files, pure path rename |
| Move BGP-specific flat files to sub-packages | | | |
| Split types.go into generic + BGP | | | |
| Split server.go into generic + BGP | | | |
| internal/plugin/ becomes generic-only | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestAllPluginsRegistered with new names | ✅ Done | `internal/plugin/all/all_test.go` | Phase 1 |
| All existing tests pass per phase | ✅ Done | `make verify` | Phase 1+2 green |
| Functional tests pass per phase | ✅ Done | 243/243 pass | Phase 1+2 green |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| 10 plugin directories renamed | ✅ Done | Committed b42ead8 |
| internal/plugin/bgp/ moved | ✅ Done | 181 files changed |
| internal/plugins/bgp/types/ created | | |
| BGP sub-packages created | | |
| Mixed files split | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### 🏗️ Design
- [x] No premature abstraction (splitting real mixed code, not speculative)
- [x] No speculative features (no new protocol support, just clean separation)
- [x] Single responsibility (each sub-package has clear purpose)
- [x] Explicit behavior (callback registration, not implicit coupling)
- [x] Minimal coupling (generic server doesn't import BGP packages)
- [x] Next-developer test (clear where BGP code vs generic infra lives)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Feature code integrated
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Architecture docs updated with new structure

### Completion
- [ ] Implementation Audit completed
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
