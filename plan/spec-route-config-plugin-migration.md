# Spec: route-config-plugin-migration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/8 (all 4 families migrated & verified; community-extraction + final cleanup remain; 2 deviations noted) |
| Updated | 2026-06-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-self-containment.md` - the rule being enforced
4. `ai/rules/plugin-design.md` - registration patterns
5. `internal/component/bgp/message/update_build_plugin.go` - generic target
6. `internal/component/bgp/plugins/nlri/srpolicy/config.go` - reference implementation

## Task

Four BGP route families (FlowSpec, VPLS, MVPN, MUP) have hardcoded config
types, parsers, converters, reactor types, and UPDATE builders scattered across
the central `bgp/config`, `bgp/reactor`, and `bgp/message` packages. This
violates the plugin self-containment rule: deleting any of these families'
plugin directories leaves broken references in 6 central packages.

The generic `PluginRoute` pipeline already exists and works end-to-end
(SR-Policy uses it via `registry.ConfigRouteParserByFamily`). These four
families predate that mechanism and were never migrated.

**Goal:** Migrate FlowSpec, VPLS, MVPN, and MUP to the generic `PluginRoute`
pipeline so that the delete-the-folder test passes for each family. After this
work, the central config/reactor/message packages contain zero family-specific
code for any of these four families.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - the invariant being restored
  -> Constraint: remove plugin folder + blank import = build green, all features gone
- [ ] `ai/rules/plugin-design.md` - registration patterns, InProcessConfigRouteParser
  -> Constraint: plugins register via init(), core discovers through registries
- [ ] `docs/architecture/config/syntax.md` - config tree structure for routes
- [ ] `ai/patterns/bgp-family.md` - BGP family integration checklist

**Key insights:**
- The generic PluginRoute path (config parse -> PluginRouteConfig -> reactor.PluginRoute -> message.BuildPlugin) already works. SR-Policy is the proof.
- Each plugin registers `InProcessConfigRouteParser` which returns a `registry.PluginRoute` (pre-built NLRI bytes + raw attrs).
- The central `sendPluginRoutesVia` in peer_initial_sync.go already handles any family generically.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/bgp.go` - defines 4 hardcoded config structs (FlowSpecRouteConfig, VPLSRouteConfig, MVPNRouteConfig, MUPRouteConfig) + the generic PluginRouteConfig
- [ ] `internal/component/bgp/config/bgp_routes.go` - UpdateBlockRoutes with 5 typed slices, hardcoded `switch famName` at line 162
- [ ] `internal/component/bgp/config/bgp_routes_flowspec.go` - FlowSpec parsing (parseFlowSpecNLRILine, extractFlowSpecRoutes)
- [ ] `internal/component/bgp/config/bgp_routes_vpls.go` - VPLS parsing (parseVPLSNLRILine, extractVPLSRoutes)
- [ ] `internal/component/bgp/config/bgp_routes_mvpn.go` - MVPN parsing (parseMVPNNLRILine, extractMVPNRoutes)
- [ ] `internal/component/bgp/config/bgp_routes_mup.go` - MUP parsing (parseMUPNLRILine, extractMUPRoutes)
- [ ] `internal/component/bgp/config/loader_routes.go` - 4 hardcoded converters (convertMVPNRoute, convertVPLSRoute, convertFlowSpecRoute, convertMUPRoute) + generic convertPluginRoute
- [ ] `internal/component/bgp/config/peers.go` - patchRoutes() with 4 per-family loops + 4 legacy extract calls (lines 434-509)
- [ ] `internal/component/bgp/config/routeattr_community.go` - FlowSpec/MUP extended community parsing embedded in generic code
- [ ] `internal/component/bgp/reactor/peersettings.go` - 4 hardcoded route structs (MVPNRoute, VPLSRoute, FlowSpecRoute, MUPRoute) + 4 per-family slice fields on PeerSettings + generic PluginRoute/PluginRoutes
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - 4 hardcoded send functions (sendMVPNRoutesVia, sendVPLSRoutesVia, sendFlowSpecRoutesVia, sendMUPRoutesVia) + generic sendPluginRoutesVia
- [ ] `internal/component/bgp/reactor/peer_static_routes.go` - 4 hardcoded param converters (toVPLSParams, toFlowSpecParams, toMUPParams, toMVPNParams)
- [ ] `internal/component/bgp/message/update_build_flowspec.go` - FlowSpecParams + BuildFlowSpec
- [ ] `internal/component/bgp/message/update_build_vpls.go` - VPLSParams + BuildVPLS
- [ ] `internal/component/bgp/message/update_build_mvpn.go` - MVPNParams + BuildMVPN + BuildGroupedMVPN
- [ ] `internal/component/bgp/message/update_build_mup.go` - MUPParams + BuildMUP
- [ ] `internal/component/bgp/message/update_build_plugin.go` - generic PluginParams + BuildPlugin (the target)
- [ ] `internal/component/plugin/registry/registry.go` - InProcessConfigRouteParser, ConfigRouteParserByFamily, PluginRoute
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` - SR-Policy reference: registers ConfigRouteParser, returns PluginRoute
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/register.go` - SR-Policy registration

**Behavior to preserve:**
- ExaBGP compatibility config syntax for all 4 families (both native update{} and legacy announce{} formats)
- Wire encoding output: identical UPDATE bytes for each family
- Config validation error messages
- Existing test expectations in .ci files
- FlowSpec max-size splitting behavior (BuildFlowSpecWithMaxSize)
- MVPN route grouping behavior (BuildGroupedMVPN)

**Behavior to change:**
- All 4 families stop using hardcoded types in central packages
- All 4 families register config parsers via `InProcessConfigRouteParser` in their plugin init()
- Central code uses only `PluginRouteConfig` / `reactor.PluginRoute` / `message.PluginParams`

## Data Flow (MANDATORY)

### Entry Point
- Config file parsed into `*config.Tree`
- `extractRoutesFromUpdateBlock` encounters an NLRI family name

### Transformation Path
1. Config tree -> `registry.ConfigRouteParserByFamily(famName)` returns the registered parser
2. Parser returns `registry.PluginRoute`; stored as `UpdateBlockRoutes.PluginRoutes` (`[]PluginRouteConfig`)
3. `convertPluginRoute` produces `reactor.PluginRoute`; stored in `PeerSettings.PluginRoutes`
4. `sendPluginRoutesVia` builds `message.PluginParams`; `BuildPlugin` produces the UPDATE wire bytes

### Integration Points
- `registry.ConfigRouteParserByFamily` / `InProcessConfigRouteParser` (`plugin/registry/registry.go`) - plugins register in init(); central dispatch discovers
- `convertPluginRoute` (`bgp/config/loader_routes.go`) - already exists; converts `PluginRouteConfig` to `reactor.PluginRoute`
- `sendPluginRoutesVia` (`bgp/reactor/peer_initial_sync.go`) - already exists; handles any family generically
- `BuildPlugin` (`bgp/message/update_build_plugin.go`) - already exists; the generic UPDATE builder (SR-Policy is the proof)

### Current (Hardcoded) Transformation Path
1. `bgp_routes.go:162` - `switch famName` dispatches to hardcoded parser (e.g. `parseFlowSpecNLRILine`)
2. Parser returns family-specific config type (e.g. `FlowSpecRouteConfig`)
3. `UpdateBlockRoutes` stores in per-family typed slice (e.g. `FlowSpecRoutes`)
4. `peers.go:patchRoutes` calls family-specific converter (e.g. `convertFlowSpecRoute`)
5. Converter produces family-specific reactor type (e.g. `reactor.FlowSpecRoute`)
6. `PeerSettings` stores in per-family slice (e.g. `ps.FlowSpecRoutes`)
7. `peer_initial_sync.go` calls family-specific send function (e.g. `sendFlowSpecRoutesVia`)
8. Send function calls family-specific param converter (e.g. `toFlowSpecParams`)
9. Message builder uses family-specific builder (e.g. `BuildFlowSpec`)

### Target (Generic) Transformation Path
1. `bgp_routes.go` - `registry.ConfigRouteParserByFamily(famName)` returns registered parser
2. Parser returns `registry.PluginRoute` (pre-built NLRI bytes + raw attrs)
3. `UpdateBlockRoutes.PluginRoutes` stores as `[]PluginRouteConfig`
4. `peers.go:patchRoutes` calls `convertPluginRoute` (already exists)
5. Converter produces `reactor.PluginRoute`
6. `PeerSettings.PluginRoutes` stores in single generic slice
7. `peer_initial_sync.go` calls `sendPluginRoutesVia` (already exists)
8. `sendPluginRoutesVia` builds `message.PluginParams`
9. `BuildPlugin` produces the UPDATE (already exists)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin -> Registry | `InProcessConfigRouteParser` registration in init() | [ ] |
| Config -> Reactor | `convertPluginRoute` via `PluginRouteConfig` | [ ] |
| Reactor -> Message | `PluginParams` via `sendPluginRoutesVia` | [ ] |

### Architectural Verification
- [ ] No bypassed layers (all families go through registry)
- [ ] No unintended coupling (plugin packages not imported by config/reactor/message)
- [ ] No duplicated functionality (one path, not five)
- [ ] Zero-copy preserved where applicable (NLRI bytes passed through)

## Violation Inventory

### Layer 1: Config Types (`bgp/config/bgp.go`)
| Type | Lines | Status |
|------|-------|--------|
| `FlowSpecRouteConfig` | 145-154 | hardcoded, move to flowspec plugin |
| `VPLSRouteConfig` | 124-140 | hardcoded, move to vpls plugin |
| `MVPNRouteConfig` | 107-121 | hardcoded, move to mvpn plugin |
| `MUPRouteConfig` | 157-170 | hardcoded, move to mup plugin |

### Layer 2: Config Parsing (`bgp/config/bgp_routes*.go`)
| File | Functions | Status |
|------|-----------|--------|
| `bgp_routes.go:122-129` | `UpdateBlockRoutes` 4 typed slices | hardcoded struct |
| `bgp_routes.go:162-195` | `switch famName` 4 cases | hardcoded dispatch |
| `bgp_routes_flowspec.go` | `parseFlowSpecNLRILine`, `extractFlowSpecRoutes`, `parseFlowSpecRoute` | entire file is plugin code |
| `bgp_routes_vpls.go` | `parseVPLSNLRILine`, `extractVPLSRoutes`, `parseVPLSRoute` | entire file is plugin code |
| `bgp_routes_mvpn.go` | `parseMVPNNLRILine`, `extractMVPNRoutes`, `parseMVPNRoute` | entire file is plugin code |
| `bgp_routes_mup.go` | `parseMUPNLRILine`, `extractMUPRoutes`, `parseMUPRoute` | entire file is plugin code |

### Layer 3: Config-to-Reactor Conversion (`bgp/config/loader_routes.go`, `peers.go`)
| Function | Lines | Status |
|----------|-------|--------|
| `convertMVPNRoute` | loader_routes.go:23-123 | hardcoded |
| `convertVPLSRoute` | loader_routes.go:126-218 | hardcoded |
| `convertFlowSpecRoute` | loader_routes.go:224-301 | hardcoded |
| `convertMUPRoute` | loader_routes.go:375-438 | hardcoded |
| `patchRoutes` per-family loops | peers.go:434-509 | hardcoded |
| `extractMVPNRoutes` call | peers.go:475 | hardcoded legacy |
| `extractVPLSRoutes` call | peers.go:484 | hardcoded legacy |
| `extractFlowSpecRoutes` call | peers.go:493 | hardcoded legacy |
| `extractMUPRoutes` call | peers.go:502 | hardcoded legacy |

### Layer 4: Reactor Types (`bgp/reactor/peersettings.go`)
| Type/Field | Lines | Status |
|------------|-------|--------|
| `MVPNRoute` struct | 163-180 | hardcoded |
| `VPLSRoute` struct | 182-195 | hardcoded |
| `FlowSpecRoute` struct | 197-208 | hardcoded |
| `MUPRoute` struct | 210-219 | hardcoded |
| `PeerSettings.MVPNRoutes` | 398 | hardcoded field |
| `PeerSettings.VPLSRoutes` | 399 | hardcoded field |
| `PeerSettings.FlowSpecRoutes` | 400 | hardcoded field |
| `PeerSettings.MUPRoutes` | 401 | hardcoded field |

### Layer 5: Reactor Sending (`bgp/reactor/peer_initial_sync.go`, `peer_static_routes.go`)
| Function | Lines | Status |
|----------|-------|--------|
| `sendMVPNRoutesVia` | peer_initial_sync.go:417-499 | hardcoded, ~83 lines |
| `sendVPLSRoutesVia` | peer_initial_sync.go:578-606 | hardcoded |
| `sendFlowSpecRoutesVia` | peer_initial_sync.go:610-676 | hardcoded |
| `sendMUPRoutesVia` | peer_initial_sync.go:679-743 | hardcoded |
| `toVPLSParams` | peer_static_routes.go:19-27 | hardcoded |
| `toFlowSpecParams` | peer_static_routes.go:29-35 | hardcoded |
| `toMUPParams` | peer_static_routes.go:37-43 | hardcoded |
| `toMVPNParams` | peer_static_routes.go:52-67 | hardcoded |

### Layer 6: Message Builders (`bgp/message/update_build_*.go`)
| File | Types | Status |
|------|-------|--------|
| `update_build_flowspec.go` | `FlowSpecParams`, `BuildFlowSpec`, `BuildFlowSpecWithMaxSize` | hardcoded |
| `update_build_vpls.go` | `VPLSParams`, `BuildVPLS` | hardcoded |
| `update_build_mvpn.go` | `MVPNParams`, `BuildMVPN`, `BuildGroupedMVPN` | hardcoded |
| `update_build_mup.go` | `MUPParams`, `BuildMUP` | hardcoded |

### Layer 7: Community Parsing (`bgp/config/routeattr_community.go`)
| Function | Lines | Status |
|----------|-------|--------|
| `parseFlowSpecAction` | ~165 | FlowSpec-specific in generic community parser |
| FlowSpec rate-limit/redirect | ~210-268 | FlowSpec-specific in generic ext-community parser |
| `parseMUPExtCommunity` | ~491-506 | MUP-specific in generic ext-community parser |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `BuildPlugin` can produce identical wire bytes to `BuildFlowSpec` etc. | SR-Policy uses BuildPlugin successfully | Would need to extend BuildPlugin | Compare wire output in tests | unvalidated |
| A-2 | FlowSpec max-size splitting can be handled by the plugin's config parser producing multiple PluginRoutes | BuildFlowSpecWithMaxSize exists | Plugin needs a multi-route return or registry needs a splitter hook | Read FlowSpec splitting code | unvalidated |
| A-3 | MVPN route grouping can be moved into the MVPN plugin | BuildGroupedMVPN exists in message package | Grouping needs to happen at a higher layer | Read MVPN grouping code | unvalidated |
| A-4 | Legacy ExaBGP syntax extractors (extractFlowSpecRoutes etc.) can be registered via registry too | They read from config.Tree directly | May need a second registry hook for tree-format extraction | Read extract functions | unvalidated |
| A-5 | Extended community parsing for FlowSpec/MUP can be registered as plugin-specific community parsers | Currently hardcoded in routeattr_community.go | May need a community parser registry | Read community parsing code | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Wire encoding regression: migrated family produces different bytes than before | Existing encode .ci tests fail | Keep old builders as test references until migration verified |
| R-2 | FlowSpec and MVPN have complex behaviors (max-size splitting, grouping) that don't fit the simple PluginRoute model | PluginParams lacks the needed fields | Extend PluginParams or add optional hooks, but keep the generic dispatch |
| R-3 | Community parsing is deeply entangled; extracting FlowSpec/MUP actions may require a larger refactor | Community parser has shared state | Add a community parser registry hook |
| R-4 | Large diff: touching 6 layers across 15+ files risks merge conflicts with in-flight specs | Other specs modify reactor/config | Migrate one family at a time; each family is independently mergeable |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| FlowSpec plugin init() registers ConfigRouteParser | -> | registry.ConfigRouteParserByFamily("ipv4/flow") returns parser | TestFlowSpecConfigRouteParserRegistered |
| VPLS plugin init() registers ConfigRouteParser | -> | registry.ConfigRouteParserByFamily("l2vpn/vpls") returns parser | TestVPLSConfigRouteParserRegistered |
| MVPN plugin init() registers ConfigRouteParser | -> | registry.ConfigRouteParserByFamily("ipv4/mvpn") returns parser | TestMVPNConfigRouteParserRegistered |
| MUP plugin init() registers ConfigRouteParser | -> | registry.ConfigRouteParserByFamily("ipv4/mup") returns parser | TestMUPConfigRouteParserRegistered |
| Config with flowspec route parsed | -> | route lands in PeerSettings.PluginRoutes (not FlowSpecRoutes) | TestFlowSpecRouteUsesGenericPath |
| Config with vpls route parsed | -> | route lands in PeerSettings.PluginRoutes (not VPLSRoutes) | TestVPLSRouteUsesGenericPath |
| Config with mvpn route parsed | -> | route lands in PeerSettings.PluginRoutes (not MVPNRoutes) | TestMVPNRouteUsesGenericPath |
| Config with mup route parsed | -> | route lands in PeerSettings.PluginRoutes (not MUPRoutes) | TestMUPRouteUsesGenericPath |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Delete FlowSpec NLRI plugin dir + blank import | Build succeeds, zero references in config/reactor/message |
| AC-2 | Delete VPLS NLRI plugin dir + blank import | Build succeeds, zero references in config/reactor/message |
| AC-3 | Delete MVPN NLRI plugin dir + blank import | Build succeeds, zero references in config/reactor/message |
| AC-4 | Delete MUP NLRI plugin dir + blank import | Build succeeds, zero references in config/reactor/message |
| AC-5 | Config with FlowSpec routes (native + legacy syntax) | Routes parsed via registered ConfigRouteParser, stored as PluginRoutes |
| AC-6 | Config with VPLS routes (native + legacy syntax) | Routes parsed via registered ConfigRouteParser, stored as PluginRoutes |
| AC-7 | Config with MVPN routes (native + legacy syntax) | Routes parsed via registered ConfigRouteParser, stored as PluginRoutes |
| AC-8 | Config with MUP routes (native + legacy syntax) | Routes parsed via registered ConfigRouteParser, stored as PluginRoutes |
| AC-9 | Existing encode .ci tests for all 4 families | Pass with identical wire output |
| AC-10 | `UpdateBlockRoutes` struct | No per-family typed slices, only `PluginRoutes []PluginRouteConfig` |
| AC-11 | `PeerSettings` struct | No per-family route slices, only `PluginRoutes []PluginRoute` |
| AC-12 | `bgp_routes.go` switch famName | No hardcoded family cases, all families dispatched through registry |
| AC-13 | `peer_initial_sync.go` | No per-family send functions, only `sendPluginRoutesVia` |
| AC-14 | Central config/reactor/message packages | Zero import of any NLRI plugin package |
| AC-15 | FlowSpec extended community actions (redirect, rate-limit, discard, mark) | Still parsed correctly |
| AC-16 | MUP extended community parsing | Still parsed correctly |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures FlowSpec route in update{} block | config tree -> registry.ConfigRouteParserByFamily("ipv4/flow") -> PluginRouteConfig -> PluginRoute -> sendPluginRoutesVia -> BuildPlugin -> wire | existing encode .ci tests |
| 2 | Configures VPLS route in l2vpn{} block | config tree -> registry.ConfigRouteParserByFamily("l2vpn/vpls") -> PluginRouteConfig -> PluginRoute -> sendPluginRoutesVia -> BuildPlugin -> wire | existing encode .ci tests |
| 3 | Configures MVPN route in announce{} block | config tree -> registry.ConfigRouteParserByFamily("ipv4/mvpn") -> PluginRouteConfig -> PluginRoute -> sendPluginRoutesVia -> BuildPlugin -> wire | existing encode .ci tests |
| 4 | Configures MUP route in update{} block | config tree -> registry.ConfigRouteParserByFamily("ipv4/mup") -> PluginRouteConfig -> PluginRoute -> sendPluginRoutesVia -> BuildPlugin -> wire | existing encode .ci tests |
| 5 | Removes FlowSpec plugin from build | No FlowSpec config parsing, no build break, no stale references | AC-1 removal test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFlowSpecConfigRouteParserRegistered` | `plugins/nlri/flowspec/config_test.go` | Parser registered for ipv4/flow, ipv6/flow, ipv4/flow-vpn, ipv6/flow-vpn | |
| `TestVPLSConfigRouteParserRegistered` | `plugins/nlri/vpls/config_test.go` | Parser registered for l2vpn/vpls | |
| `TestMVPNConfigRouteParserRegistered` | `plugins/nlri/mvpn/config_test.go` | Parser registered for ipv4/mvpn, ipv6/mvpn | |
| `TestMUPConfigRouteParserRegistered` | `plugins/nlri/mup/config_test.go` | Parser registered for ipv4/mup, ipv6/mup | |
| `TestFlowSpecRouteUsesGenericPath` | `bgp/config/bgp_routes_test.go` | FlowSpec NLRI line produces PluginRouteConfig, not FlowSpecRouteConfig | |
| `TestVPLSRouteUsesGenericPath` | `bgp/config/bgp_routes_test.go` | VPLS NLRI line produces PluginRouteConfig | |
| `TestMVPNRouteUsesGenericPath` | `bgp/config/bgp_routes_test.go` | MVPN NLRI line produces PluginRouteConfig | |
| `TestMUPRouteUsesGenericPath` | `bgp/config/bgp_routes_test.go` | MUP NLRI line produces PluginRouteConfig | |
| `TestNoHardcodedFamilySwitchInRoutes` | `bgp/config/bgp_routes_test.go` | switch famName has no FlowSpec/VPLS/MVPN/MUP cases | |
| `TestFlowSpecWireOutputUnchanged` | `plugins/nlri/flowspec/config_test.go` | PluginRoute NLRI bytes match old BuildFlowSpec output | |
| `TestMVPNWireOutputUnchanged` | `plugins/nlri/mvpn/config_test.go` | PluginRoute NLRI+attrs bytes match old BuildMVPN output | |
| `TestVPLSWireOutputUnchanged` | `plugins/nlri/vpls/config_test.go` | PluginRoute NLRI+attrs bytes match old BuildVPLS output | |
| `TestMUPWireOutputUnchanged` | `plugins/nlri/mup/config_test.go` | PluginRoute NLRI+attrs bytes match old BuildMUP output | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing FlowSpec encode tests | `test/encode/*.ci` | FlowSpec routes produce same wire bytes | |
| Existing VPLS encode tests | `test/encode/*.ci` | VPLS routes produce same wire bytes | |
| Existing MVPN encode tests | `test/encode/*.ci` | MVPN routes produce same wire bytes | |
| Existing MUP encode tests | `test/encode/*.ci` | MUP routes produce same wire bytes | |

## Files to Modify

### Central packages (remove hardcoded code)
- `internal/component/bgp/config/bgp.go` - remove FlowSpecRouteConfig, VPLSRouteConfig, MVPNRouteConfig, MUPRouteConfig structs
- `internal/component/bgp/config/bgp_routes.go` - remove per-family slices from UpdateBlockRoutes, remove switch famName cases, all families go through registry
- `internal/component/bgp/config/bgp_routes_flowspec.go` - DELETE (move content to flowspec plugin)
- `internal/component/bgp/config/bgp_routes_vpls.go` - DELETE (move content to vpls plugin)
- `internal/component/bgp/config/bgp_routes_mvpn.go` - DELETE (move content to mvpn plugin)
- `internal/component/bgp/config/bgp_routes_mup.go` - DELETE (move content to mup plugin)
- `internal/component/bgp/config/loader_routes.go` - remove convertMVPNRoute, convertVPLSRoute, convertFlowSpecRoute, convertMUPRoute, mupRouteConfigToArgs
- `internal/component/bgp/config/peers.go` - simplify patchRoutes to single generic loop, remove legacy extract calls
- `internal/component/bgp/config/routeattr_community.go` - extract FlowSpec/MUP community parsers to their plugins
- `internal/component/bgp/reactor/peersettings.go` - remove MVPNRoute, VPLSRoute, FlowSpecRoute, MUPRoute structs and per-family slice fields
- `internal/component/bgp/reactor/peer_initial_sync.go` - remove sendMVPNRoutesVia, sendVPLSRoutesVia, sendFlowSpecRoutesVia, sendMUPRoutesVia
- `internal/component/bgp/reactor/peer_static_routes.go` - remove toVPLSParams, toFlowSpecParams, toMUPParams, toMVPNParams
- `internal/component/bgp/message/update_build_flowspec.go` - DELETE
- `internal/component/bgp/message/update_build_vpls.go` - DELETE
- `internal/component/bgp/message/update_build_mvpn.go` - DELETE
- `internal/component/bgp/message/update_build_mup.go` - DELETE

### Plugin packages (receive moved code)
- `internal/component/bgp/plugins/nlri/flowspec/config.go` - NEW: config parser returning PluginRoute
- `internal/component/bgp/plugins/nlri/flowspec/register.go` - add InProcessConfigRouteParser registration
- `internal/component/bgp/plugins/nlri/vpls/config.go` - NEW: config parser returning PluginRoute
- `internal/component/bgp/plugins/nlri/vpls/register.go` - add InProcessConfigRouteParser registration
- `internal/component/bgp/plugins/nlri/mvpn/config.go` - NEW: config parser returning PluginRoute
- `internal/component/bgp/plugins/nlri/mvpn/register.go` - add InProcessConfigRouteParser registration
- `internal/component/bgp/plugins/nlri/mup/config.go` - NEW: config parser returning PluginRoute
- `internal/component/bgp/plugins/nlri/mup/register.go` - add InProcessConfigRouteParser registration

### Registry (may need extension)
- `internal/component/plugin/registry/registry.go` - may need additional hooks:
  - `InProcessConfigTreeExtractor` for legacy announce{}/flow{}/l2vpn{} syntax
  - `InProcessExtCommunityParser` for FlowSpec/MUP extended community types

## Files to Create
- `internal/component/bgp/plugins/nlri/flowspec/config.go` - FlowSpec config route parser
- `internal/component/bgp/plugins/nlri/flowspec/config_test.go` - registration + wire output tests
- `internal/component/bgp/plugins/nlri/vpls/config.go` - VPLS config route parser
- `internal/component/bgp/plugins/nlri/vpls/config_test.go` - registration + wire output tests
- `internal/component/bgp/plugins/nlri/mvpn/config.go` - MVPN config route parser
- `internal/component/bgp/plugins/nlri/mvpn/config_test.go` - registration + wire output tests
- `internal/component/bgp/plugins/nlri/mup/config.go` - MUP config route parser (extends existing)
- `internal/component/bgp/plugins/nlri/mup/config_test.go` - registration + wire output tests

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. Standard | Per template |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

Migrate one family at a time. Each family is an independently mergeable unit. Order: MUP (simplest, closest to SR-Policy) -> VPLS -> MVPN -> FlowSpec (most complex).

1. **Phase: Registry extension** -- add any needed registry hooks
   - Tests: TestConfigRouteParserRegistration
   - Files: registry/registry.go (if tree extractor hook needed)
   - Verify: existing tests still pass

2. **Phase: MUP migration** -- simplest family, closest to SR-Policy pattern
   - Tests: TestMUPConfigRouteParserRegistered, TestMUPRouteUsesGenericPath, TestMUPWireOutputUnchanged
   - Files: move bgp_routes_mup.go content to plugins/nlri/mup/config.go, register parser, remove hardcoded code from central packages
   - Verify: MUP encode .ci tests pass with identical output

3. **Phase: VPLS migration**
   - Tests: TestVPLSConfigRouteParserRegistered, TestVPLSRouteUsesGenericPath, TestVPLSWireOutputUnchanged
   - Files: move bgp_routes_vpls.go content to plugins/nlri/vpls/config.go
   - Verify: VPLS encode .ci tests pass

4. **Phase: MVPN migration** -- has route grouping complexity
   - Tests: TestMVPNConfigRouteParserRegistered, TestMVPNRouteUsesGenericPath, TestMVPNWireOutputUnchanged
   - Files: move bgp_routes_mvpn.go content to plugins/nlri/mvpn/config.go
   - Verify: MVPN encode .ci tests pass, grouping behavior preserved

5. **Phase: FlowSpec migration** -- most complex (max-size splitting, extended community actions)
   - Tests: TestFlowSpecConfigRouteParserRegistered, TestFlowSpecRouteUsesGenericPath, TestFlowSpecWireOutputUnchanged
   - Files: move bgp_routes_flowspec.go content to plugins/nlri/flowspec/config.go
   - Verify: FlowSpec encode .ci tests pass, max-size splitting works

6. **Phase: Community parser extraction** -- move FlowSpec/MUP community parsing to plugins
   - Tests: TestFlowSpecExtCommunityParsing, TestMUPExtCommunityParsing
   - Files: routeattr_community.go (remove hardcoded parsers), registry.go (add community parser hook), plugin config.go files (register community parsers)
   - Verify: extended community tests pass

7. **Phase: Central cleanup** -- remove all remaining per-family code
   - Tests: TestNoHardcodedFamilySwitchInRoutes, AC-1 through AC-4 removal tests
   - Files: remove per-family structs/fields/functions from bgp_routes.go, peers.go, loader_routes.go, peersettings.go, peer_initial_sync.go, peer_static_routes.go
   - Verify: delete-folder test passes for each family

8. **Phase: Dead code removal** -- delete message builders
   - Files: DELETE update_build_flowspec.go, update_build_vpls.go, update_build_mvpn.go, update_build_mup.go
   - Verify: build clean, no references

9. **Functional tests** -- existing .ci tests pass
10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- learned summary, two-commit closure

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation with file:line |
| Feature completeness | All 4 families migrated, all end-to-end stories work |
| Correctness | Wire output identical before/after for every family |
| Data flow | All routes go config -> registry parser -> PluginRouteConfig -> PluginRoute -> BuildPlugin |
| Rule: plugin-self-containment | Delete-folder test passes for FlowSpec, VPLS, MVPN, MUP |
| Rule: no-layering | Old per-family code fully deleted, not just bypassed |
| Rule: no import violations | Central packages have zero imports of NLRI plugin packages |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| FlowSpec config parser in plugin | `ls internal/component/bgp/plugins/nlri/flowspec/config.go` |
| VPLS config parser in plugin | `ls internal/component/bgp/plugins/nlri/vpls/config.go` |
| MVPN config parser in plugin | `ls internal/component/bgp/plugins/nlri/mvpn/config.go` |
| MUP config parser in plugin | `ls internal/component/bgp/plugins/nlri/mup/config.go` |
| No per-family types in bgp/config | `grep -rn 'FlowSpecRouteConfig\|VPLSRouteConfig\|MVPNRouteConfig\|MUPRouteConfig' internal/component/bgp/config/` returns empty |
| No per-family types in reactor | `grep -rn 'MVPNRoute\b\|VPLSRoute\b\|FlowSpecRoute\b\|MUPRoute\b' internal/component/bgp/reactor/` returns empty |
| No per-family builders in message | `ls internal/component/bgp/message/update_build_{flowspec,vpls,mvpn,mup}.go 2>&1` returns "No such file" |
| No switch famName cases | `grep -A2 'switch famName' internal/component/bgp/config/bgp_routes.go` shows no family-specific cases |
| Registry parsers registered | `grep -rn 'InProcessConfigRouteParser' internal/component/bgp/plugins/nlri/` shows 4 registrations |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Config route parser inputs validated same as before (no relaxation) |
| No new attack surface | Moving code to plugin packages doesn't add new entry points |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Wire encoding differs | Compare byte-by-byte; fix plugin parser to produce identical NLRI+attrs |
| Import cycle after move | Restructure: plugin config.go must only import registry + core, not bgp/config |
| BuildPlugin lacks needed feature | Extend PluginParams (e.g. add optional grouping hook), not family-specific builder |
| Legacy syntax extractor can't be moved | Add InProcessConfigTreeExtractor registry hook |
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
| Previous session identified violations and dismissed as "opportunistic cleanup, not a deliverable" | At least once (SR-Policy spec) | When the delete-folder test fails, that is a blocking violation, never "opportunistic" | Add to RECURRING-PATTERNS.md |

## Design Insights

## Core Insight

The generic PluginRoute pipeline was built correctly (SR-Policy proves it works), but the
pre-existing families were never migrated to use it. The violation was identified and explicitly
dismissed as "opportunistic cleanup" in a previous session. This is the textbook case for why
the plugin self-containment rule exists: if a violation is "not a deliverable," it never gets
fixed, and the hardcoded code keeps growing (MUP was added after the generic path existed and
was still hardcoded).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Migrate one family at a time (MUP -> VPLS -> MVPN -> FlowSpec) | Big-bang migration of all 4 at once | Reduces risk, each family independently mergeable, complexity order (simple first) |
| Use existing PluginRoute/PluginParams/BuildPlugin as-is | Create new intermediate types | SR-Policy proves the generic path works; avoid adding more types |
| Produce pre-built NLRI bytes + raw attr bytes in config parser | Keep typed intermediate structs | Matches what BuildPlugin expects; the plugin owns the wire format |
| Include community parser extraction in scope | Defer to separate spec | AC-15/AC-16 require it; leaving FlowSpec actions hardcoded in the central community parser would fail the delete-folder test |

## Known Limitations
- BuildGroupedMVPN (MVPN route grouping for packing multiple routes into one UPDATE) needs investigation during Phase 4 to determine if the grouping logic moves to the plugin or if sendPluginRoutesVia needs a grouping hook.
- BuildFlowSpecWithMaxSize (FlowSpec NLRI splitting when exceeding max UPDATE size) needs investigation during Phase 5.

## Implementation Summary

### What Was Implemented (progress; spec NOT yet complete)

**Approach decision (Option Y):** rather than move the route-attribute parsers to a
leaf package up front, the central dispatch (`bgp_routes.go:buildConfigRouteRequest`)
pre-parses the `attribute{}` block (reusing `applyAttributesFromTree` +
`ParseRouteAttributes`) into a new `registry.ConfigRouteRequest`. Each plugin's
config parser owns the family-specific NLRI build + per-family attribute assembly
(ext-community / community sort order, NEXT_HOP code-3). AS_PATH and LOCAL_PREF are
carried typed and built by `BuildPlugin` at send time (session/ASN4 context). The
family-specific extended-community *token* parsing (FlowSpec actions, MUP `mup:`)
stays central for now and is moved to the plugins in the later community-extraction phase.

**Generic-path extension (Phase 1) — DONE, verified:**
- `registry.PluginRoute` gained `ASPath`, `LocalPreference`, `Group`; parser signature
  changed to `func(req ConfigRouteRequest)`; `ConfigRouteRequest` added.
- `message.PluginParams` gained `AFI`, `ASPath`, `LocalPreference`; `BuildPlugin`
  rewritten (default ORIGIN only when the plugin supplies none; AS_PATH from `ASPath`;
  iBGP-gated LOCAL_PREF; AFI-aware MP_REACH; drops any plugin code-2/5). Wire order is
  fixed by `OrderAttributes` (MP_REACH last), so attribute build order is irrelevant.
- Threaded through `config.PluginRouteConfig`, `reactor.PluginRoute`, `toPluginParams`.
- SR-Policy updated to the new signature; SR-Policy encode test still byte-identical
  (confirms A-1: `BuildPlugin` reproduces hand-written builder output exactly).

**MUP (Phase 2) — DONE, byte-identical, verified:** new `plugins/nlri/mup/config.go`
(parser + registration + tests); `mup/encode.go` `EncodeRoute` moved `BuildMUP`→`BuildPlugin`;
deleted `bgp_routes_mup.go` + `update_build_mup.go`; removed all MUP central code
(config struct/parser/converter, reactor struct/field/send/params, message builder).
`test/encode/srv6-mup{,-v3}.ci`, exabgp-compat MUP, plugin MUP all pass.

**VPLS (Phase 3) — DONE, byte-identical, verified:** new `plugins/nlri/vpls/config.go`
(handles `rd RD add ...` grammar, requires an op keyword, sorts ext-communities/communities,
AFI=25); `vpls/encode.go` `EncodeRoute` moved to `BuildPlugin`; deleted `bgp_routes_vpls.go`
+ `update_build_vpls.go`; removed all VPLS central code. `test/encode/l2vpn.ci`
(configured AS-path + origin + med + local-pref + community + ext-comm + originator +
cluster) and VPLS plugin tests all pass. All 53 `test/encode/*.ci` pass.

**MVPN (Phase 4) — DONE, byte-identical, verified:** new `plugins/nlri/mvpn/config.go` (builds the
RFC 6514 NLRI inline incl RD-string parse + NEXT_HOP code-3); added a generic **route-grouping**
mechanism to `sendPluginRoutesVia` (`pluginRouteGroupKey` over family|nexthop|aspath|localpref|rawattrs,
honoring the per-route `Group` flag, concatenating NLRIs into one UPDATE). Also added `MapV4NextHop`
through the chain (MUP/SR-Policy map IPv4->IPv4-mapped-IPv6; MVPN/VPLS/FlowSpec do not). Deleted
`bgp_routes_mvpn.go` + `update_build_mvpn.go` + orphaned `bgp_routes_inline.go`; removed all MVPN central
code. `test/encode/mvpn.ci` (shared-join + source-join grouped into one UPDATE) passes.

**FlowSpec (Phase 5) — config routes DONE, byte-identical, verified:** new `plugins/nlri/flowspec/config.go`
(criteria-token -> map -> `BuildFlowSpecNLRI`, VPN length+RD wrap, community/ext-comm/IPv6-ext-comm,
raw attribute-25 passthrough). The native update{} nlri form AND the legacy `flow{ route{ match{} then{} } }`
form both route through the plugin parser (`flowSpecConfigToPlugin`). `flowspec` EncodeRoute moved to
`BuildPlugin`. Removed the reactor FlowSpec route path (`reactor.FlowSpecRoute`, `sendFlowSpecRoutesVia`,
`toFlowSpecParams`, `PeerSettings.FlowSpecRoutes`). Added a generic operation-keyword requirement to the
plugin dispatch. `test/encode/{flow,flow-redirect,simple-flow,flow-rate-packets}.ci` all pass.

### Verification (all green)
- All **53** `test/encode/*.ci` pass byte-identical. Full `internal/component/bgp/...` unit tree passes.
- Route-config plugin tests pass (mup/vpls/mvpn/flowspec). AC-14 holds: central config/reactor/message
  packages import zero nlri plugin packages.
- Pre-existing/unrelated failures (committed firewall-irr YANG description conflict between
  `ze-firewall-cmd.yang` and `ze-firewall-irr-cmd.yang`): exabgp-compat `conf-watchdog` + 8 CLI-output
  plugin tests (bmp-lg-*, cos-vendor-*, filter-irr, show-bmp-sessions, show-rr-status). My changes touch
  no firewall/YANG/BMP/CoS/RR code.

### Remaining work (NOT done) + deviations
- **Community extraction (Phase 6 / AC-15, AC-16):** the family-specific extended-community *token*
  parsers (FlowSpec actions rate-limit/redirect/mark/action/discard, MUP `mup:`) still live in
  `bgp/config/routeattr_community.go`. The central `buildConfigRouteRequest` parses them and passes wire
  bytes to the plugins, so the delete-folder BUILD test passes (they are string literals, not plugin
  references). Moving them into the plugins via an ext-community parser hook (extracting the parsers to a
  leaf package + delegation) remains.
- **Deviation — `update_build_flowspec.go` retained:** `internal/chaos/peer/sender.go` (ze-chaos) builds
  FlowSpec UPDATEs via `BuildFlowSpec`, so the builder cannot be deleted without migrating the chaos peer
  (out of this spec's scope). `BuildMUP/BuildVPLS/BuildMVPN` had no such dependency and were deleted.
- **Deviation — `FlowSpecRouteConfig` retained:** kept as the config-layer DTO for the legacy `flow{}`
  tree reader (`parseFlowSpecRoute`); it carries no wire-building logic (that is delegated to the plugin).
- Spec stays open until Phase 6 + the deviations are resolved (or the deviations are accepted by the user).

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| FlowSpec delete-folder test passes | removal test | (fill) |
| VPLS delete-folder test passes | removal test | (fill) |
| MVPN delete-folder test passes | removal test | (fill) |
| MUP delete-folder test passes | removal test | (fill) |
| Wire output identical for all 4 families | encode .ci tests | (fill) |
| Central packages have zero family-specific code | grep verification | (fill) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/918-route-config-plugin-migration.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-route-config-plugin-migration.md`
