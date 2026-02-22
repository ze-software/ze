# Spec: file-modularity

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/design-doc-references.md` - existing `// Design:` pattern
4. `.claude/hooks/require-design-ref.sh` - existing enforcement hook

## Task

Two related goals:

1. **Aggressive file splitting** — enforce one concern per file across the codebase. The previous spec (`done/221-file-splitting.md`) split 4 files mechanically. This spec covers the remaining 25+ files over 1000 lines and 14 files in the 600–1000 range that have multiple concerns.

2. **`// Related:` cross-reference comments** — after `// Design:`, each file lists its sibling files with topic annotations. This lets Claude load only the files needed for a task without scanning the whole package.

3. **Enforcement** — a hook that validates `// Related:` references point to files that actually exist, catching staleness from renames/deletions.

**Motivation:** Context window efficiency. When working on `bgp_peer.go`, Claude should know to also read `bgp_routes.go` and `bgp_util.go` without grepping the package. When `reactor.go` is 5439 lines, Claude must load all of it even when only the config reconciliation concern is relevant.

This is a pure mechanical refactor — no behavior changes. All existing tests must pass unchanged.

## Comment Format

```
// Design: docs/architecture/config/syntax.md — BGP config types
// Related: bgp_peer.go — peer parsing and process bindings
// Related: bgp_routes.go — route extraction and NLRI parsers
// Related: bgp_util.go — IP matching, duration parsing, utilities
```

Each related file gets its own `// Related:` line with a topic annotation.
Only added to files with strong coupling to siblings — not every file.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — understand package boundaries
  → Constraint: splitting is within packages, never across

### Prior Specs
- [ ] `docs/plan/done/221-file-splitting.md` — previous split work
  → Decision: Go compiles all files in a package together — splitting has zero semantic effect
  → Constraint: shared test helpers stay in base `_test.go` file
  → Constraint: file-local types must move with the functions that use them

### Rules
- [ ] `.claude/rules/design-doc-references.md` — existing `// Design:` pattern we're extending
- [ ] `.claude/hooks/require-design-ref.sh` — enforcement model to replicate

**Key insights:**
- Go packages are single compilation units — splitting files is semantically transparent
- `goimports` (auto-linter hook) handles import cleanup automatically
- Shared test helpers in `_test.go` files are package-scoped — they stay in the base test file
- The `// Design:` hook is a PreToolUse hook on Write/Edit that checks file content before allowing writes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/reactor/reactor.go` — 5439 lines, 5 concerns (lifecycle, route announce, wire helpers, config reconcile, peer ops)
- [ ] `internal/plugins/bgp/reactor/peer.go` — 2788 lines, 6 concerns (FSM, static routes, RIB routes, connection, initial sync, send)
- [ ] `internal/plugins/bgp/message/update_build.go` — 2499 lines, 10 concerns (one per address family + grouping)
- [ ] `internal/plugins/bgp/handler/update_text.go` — 2318 lines, 6 concerns (text attrs, NLRI, FlowSpec, VPLS, EVPN, dispatch)
- [ ] `internal/plugins/bgp/reactor/session.go` — 2007 lines, 7 concerns (pools, lifecycle, read loop, handlers, RFC 7606, negotiation, flow)
- [ ] `cmd/ze/bgp/decode.go` — 1932 lines, 8 concerns (dispatch, OPEN, UPDATE, MP, extcomm, BGP-LS, human format)
- [ ] `internal/config/parser.go` — 1577 lines, 6 concerns (Tree data structure, tokenizer, leaf/container, list, freeform, flex)
- [ ] `internal/plugins/bgp/route/route.go` — 1535 lines, 7 concerns (splitting, attrs, community, labeled, VPN, FlowSpec, MUP)
- [ ] `cmd/ze-chaos/main.go` — 1501 lines, 6 concerns (orchestrator, peer loop, schedulers, reporting, replay)
- [ ] `internal/test/runner/runner.go` — 1498 lines, 7 concerns (lifecycle, per-test, orchestrated, HTTP, JSON, logging, output)
- [ ] `internal/test/peer/peer.go` — 1478 lines, 5 concerns (struct+run, open gen, checker, expect files, actions)
- [ ] `internal/plugin/server.go` — 1447 lines, 5 concerns (lifecycle, startup, dispatch, client mgmt, event callbacks)
- [ ] `internal/config/routeattr.go` — 1437 lines, 6 concerns (community, RD, ASPath, RawAttr, PrefixSID, aggregation)
- [ ] `internal/plugins/bgp-nlri-flowspec/plugin.go` — 1415 lines, 6 concerns (entry, CLI, decode, format, encode, operator parse)
- [ ] `internal/plugins/bgp-nlri-flowspec/types.go` — 1381 lines, 5 concerns (FlowSpec struct, prefix component, numeric component, parsing, constants)
- [ ] `internal/exabgp/migrate.go` — 1377 lines, 7 concerns (orchestrator, neighbor, capability, routes, family, process, serializer)
- [ ] `internal/plugins/bgp-nlri-ls/types.go` — 1216 lines, 5 concerns (constants, descriptors, node, link, prefix)
- [ ] `internal/test/runner/record.go` — 1138 lines, 4 concerns (structs, collection, discovery+parse, directive parsers)
- [ ] `cmd/ze-chaos/web/state.go` — 1062 lines, 5 concerns (types, RingBuffer, ActiveSet, histogram, RouteMatrix)
- [ ] `internal/plugins/bgp-rib/rib.go` — 1055 lines, 5 concerns (manager, dispatch, wire helpers, commands, replay)
- [ ] `internal/config/loader.go` — 1034 lines, 5 concerns (parseTree, LoadReactor*, CreateReactorFromTree, converters, prefix helpers)
- [ ] `internal/exabgp/bridge.go` — 1028 lines, 4 concerns (ze→exabgp JSON, exabgp→ze commands, startup protocol, Bridge struct)
- [ ] `cmd/ze/config/main.go` — 1024 lines, 5 concerns (edit, check, migrate, fmt, dump subcommands)
- [ ] `cmd/ze-chaos/peer/simulator.go` — 1014 lines, 4 concerns (simulator, wire protocol, reconnect, withdraw)
- [ ] `.claude/hooks/require-design-ref.sh` — existing enforcement hook pattern to replicate

**Behavior to preserve:** ALL existing behavior. This is purely moving code between files in the same package and adding comments.

**Behavior to change:** None — pure file reorganization + comment addition.

## Data Flow (MANDATORY)

Not applicable — no data flow changes. Files are reorganized within existing packages.

### Entry Point
- No new entry points. All existing entry points remain unchanged.

### Transformation Path
- No transformations change. Code moves between files in the same Go package.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | No boundaries crossed — same package | [ ] |

### Integration Points
- All existing integration points preserved — same package, same exported API.

### Architectural Verification
- [ ] No bypassed layers — same package, same compilation unit
- [ ] No unintended coupling — splitting reduces coupling by organizing by concern
- [ ] No duplicated functionality — pure move
- [ ] Zero-copy preserved — no encoding changes

---

## Survey: Files Over 1000 Lines (25 files)

### Phase 1 — Critical (>2000 lines)

#### `internal/plugins/bgp/reactor/reactor.go` — 5439 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Reactor struct + lifecycle | `Reactor`, `New`, `Run`, `Stop`, `Wait` | ~300 | `reactor.go` (keep) |
| `reactorAPIAdapter` route announce | `handleAnnounce*` for unicast, VPN, labeled, MVPN, VPLS, FlowSpec, EVPN, MUP, NLRIBatch | ~2500 | `reactor_announce.go` |
| Wire-encoding helpers | `writeOriginAttr`, `writeASPathAttr`, `writeNextHopAttr`, `writeMEDAttr`, etc. | ~800 | `reactor_wire.go` |
| Config reconcile/reload | `parsePeersFromTree`, `reconcilePeers`, `ApplyConfigDiff` | ~600 | `reactor_config_reconcile.go` |
| Peer teardown/pause/resume/refresh RPCs | `teardown*`, `pause*`, `resume*`, `refresh*` | ~400 | `reactor_peer_ops.go` |

#### `internal/plugins/bgp/reactor/peer.go` — 2788 lines, 6 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Peer struct + FSM state machine | `Peer`, `run`, `runOnce`, FSM transitions | ~500 | `peer.go` (keep) |
| Static route building | `toStaticRouteUnicastParams`, `buildStaticRouteUpdateNew`, etc. | ~600 | `peer_static_routes.go` |
| RIB route building | `buildRIBRouteUpdate` | ~400 | `peer_rib_routes.go` |
| Connection management | Collision resolution, accept/inbound | ~300 | `peer_connection.go` |
| Initial route sending | `sendInitialRoutes`, full initial sync | ~500 | `peer_initial_sync.go` |
| Withdraw/split/update sending | Send path helpers | ~400 | `peer_send.go` |

#### `internal/plugins/bgp/message/update_build.go` — 2499 lines, 10 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| UpdateBuilder struct + alloc | `UpdateBuilder`, `New`, common helpers | ~200 | `update_build.go` (keep) |
| BuildUnicast + MP-Reach unicast | `BuildUnicast`, `BuildUnicastGrouped` | ~300 | `update_build_unicast.go` |
| BuildVPN + VPN NLRI encoding | `BuildVPN`, `BuildVPNGrouped` | ~250 | `update_build_vpn.go` |
| BuildLabeledUnicast | `BuildLabeledUnicast`, `BuildLabeledUnicastGrouped` | ~250 | `update_build_labeled.go` |
| BuildMVPN | `BuildMVPN`, `BuildMVPNGrouped` | ~200 | `update_build_mvpn.go` |
| BuildVPLS | `BuildVPLS`, `BuildVPLSGrouped` | ~200 | `update_build_vpls.go` |
| BuildFlowSpec | `BuildFlowSpec`, `BuildFlowSpecGrouped` | ~200 | `update_build_flowspec.go` |
| BuildEVPN | `BuildEVPN`, `BuildEVPNGrouped` | ~200 | `update_build_evpn.go` |
| BuildMUP | `BuildMUP`, `BuildMUPWithdraw`, grouped | ~250 | `update_build_mup.go` |
| Size-limited grouping helpers | Shared grouped/size-limited logic | ~150 | `update_build_grouped.go` |

#### `internal/plugins/bgp/handler/update_text.go` — 2318 lines, 6 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Text attribute parsing | `parsedAttrs`, `parseCommonAttributeText` | ~300 | `update_text.go` (keep) |
| NLRI section parsing | `parseNLRISection`, `parseINETNLRI`, `parseVPNNLRI`, `parseLabeledNLRI` | ~400 | `update_text_nlri.go` |
| FlowSpec text parsing | `parseFlowSpecSection`, `parseFlowSpecComponents` | ~350 | `update_text_flowspec.go` |
| VPLS text parsing | VPLS-specific parse functions | ~200 | `update_text_vpls.go` |
| EVPN text parsing | `parseEVPNSection`, MAC/ESI helpers | ~400 | `update_text_evpn.go` |
| RPC handlers | `handleUpdate`, `handleUpdateText`, `DispatchNLRIGroups` | ~300 | `update_text_dispatch.go` |

#### `internal/plugins/bgp/reactor/session.go` — 2007 lines, 7 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Buffer pools | `readBufPool4K`, `readBufPool64K`, `buildBufPool` | ~50 | `session.go` (keep) |
| Session struct + lifecycle | `Session`, `Start`, `Stop`, `Connect`, `Accept` | ~300 | `session.go` (keep) |
| BGP message read loop | `ReadAndProcess`, `readAndProcessMessage`, `processMessage` | ~300 | `session_read.go` |
| Message handlers per type | `handleOpen`, `handleKeepalive`, `handleUpdate`, `handleNotification`, `handleRouteRefresh` | ~500 | `session_handlers.go` |
| RFC 7606 enforcement | `enforceRFC7606`, `validateUpdateFamilies` | ~200 | `session_validation.go` |
| Capability negotiation | `negotiateWith`, `sendOpen` | ~300 | `session_negotiate.go` |
| Pause/resume flow control | Flow control functions | ~200 | `session_flow.go` |

#### `cmd/ze/bgp/decode.go` — 1932 lines, 8 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Top-level dispatch | `cmdDecode` | ~50 | `decode.go` (keep) |
| Plugin invocation modes | subprocess, in-process, internal, path | ~200 | `decode.go` (keep) |
| OPEN message decoding | OPEN + capability-to-JSON | ~250 | `decode_open.go` |
| UPDATE message decoding | path attribute parsing (`parsePathAttributesZe`, `parseASPathZe`) | ~400 | `decode_update.go` |
| MP_REACH/MP_UNREACH parsing | `buildMPReachZe`, `buildMPUnreachZe` | ~250 | `decode_mp.go` |
| Extended community parsing | Extended community decode functions | ~150 | `decode_extcomm.go` |
| BGP-LS attribute parsing | `parseBGPLSAttribute`, `parseSRv6EndXSID`, `parseSRMPLSAdjSID` | ~300 | `decode_bgpls.go` |
| Human-format rendering | `formatOpenHuman`, `formatUpdateHuman`, `formatAttributesHuman` | ~300 | `decode_human.go` |

### Phase 2 — High Priority (1000–2000 lines)

#### `internal/config/parser.go` — 1577 lines, 6 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Tree data structure | `get`, `set`, `containers`, `lists` | ~400 | `tree.go` |
| Parser struct + tokenizer | `Parser`, `tokenize`, `next`, `peek` | ~200 | `parser.go` (keep) |
| Leaf/container parsing | `parseLeaf`, `parseContainer`, `parsePresenceContainer` | ~250 | `parser.go` (keep) |
| List/multi-leaf parsing | `parseList`, `parseMultiLeaf`, `parseBracketLeafList` | ~250 | `parser_list.go` |
| Freeform/family block | `parseFreeform`, `parseFamilyBlock` | ~200 | `parser_freeform.go` |
| Flex/inline list | `parseFlex`, `parseInlineList` | ~200 | `parser_flex.go` |

#### `internal/plugins/bgp/route/route.go` — 1535 lines, 7 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Route/prefix splitting | `splitPrefix`, `addToAddr` | ~150 | `route.go` (keep) |
| Static route attribute parsing | `parseRouteAttributes`, `parseCommonAttributeBuilder` | ~200 | `route.go` (keep) |
| Community parsing | Community, large-community, extended-community | ~200 | `route_community.go` |
| Labeled unicast attributes | Labeled unicast specific parsing | ~200 | `route_labeled.go` |
| L3VPN attributes | VPN-specific parsing | ~200 | `route_vpn.go` |
| FlowSpec attributes | FlowSpec-specific parsing | ~250 | `route_flowspec.go` |
| MUP attributes | MUP-specific parsing | ~200 | `route_mup.go` |

#### `cmd/ze-chaos/main.go` — 1501 lines, 6 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Main orchestrator loop | `runOrchestrator` | ~300 | `main.go` (keep) |
| Peer loop management | `runPeerLoop` | ~200 | `peer_loop.go` |
| Chaos scheduler | `runScheduler`, `handleManualTrigger` | ~300 | `scheduler.go` |
| Route scheduler | `runRouteScheduler` | ~200 | `route_scheduler.go` |
| Reporting setup | `setupReporting` | ~200 | `reporting.go` |
| Replay/shrink/diff subcommands | Replay/shrink/diff handlers | ~300 | `replay.go` |

#### `internal/test/runner/runner.go` — 1498 lines, 7 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Runner struct + build/run lifecycle | `Runner`, `Build`, `Run` | ~200 | `runner.go` (keep) |
| Per-test execution | `runTest` | ~200 | `runner.go` (keep) |
| Orchestrated test execution | `runOrchestrated` with ze + ze-peer | ~300 | `runner_orchestrated.go` |
| HTTP check execution | HTTP check functions | ~150 | `runner_http.go` |
| JSON validation | JSON validation functions | ~150 | `runner_json.go` |
| Logging validation | Log validation functions | ~150 | `runner_logging.go` |
| Output saving + expect writing | Save/write expect functions | ~200 | `runner_output.go` |

#### `internal/test/peer/peer.go` — 1478 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Peer struct + Config + run loop | `Peer`, `handleConnection` | ~300 | `peer.go` (keep) |
| Open generation + capability overrides | `generateOpen`, `applyCapabilityOverrides` | ~300 | `peer_open.go` |
| Checker struct + message matching | `Checker`, `groupMessages`, `parseExpectRule` | ~400 | `peer_checker.go` |
| Load/parse expect files | `LoadExpectFile`, `parseOptionConfig` | ~250 | `peer_expect.go` |
| Action accessors | `NextNotificationAction`, `NextSendAction`, etc. | ~200 | `peer_actions.go` |

#### `internal/plugin/server.go` — 1447 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Server struct + lifecycle | `Server`, `Start`, `Stop`, `acceptLoop` | ~200 | `server.go` (keep) |
| 5-stage plugin startup | `progressThroughStages`, `handlePluginConflict` | ~400 | `server_startup.go` |
| Plugin RPC dispatch | `dispatchPluginRPC`, `handleUpdateRouteRPC`, etc. | ~300 | `server_dispatch.go` |
| Client management | `handleClient`, `clientLoop`, `removeClient` | ~250 | `server_client.go` |
| Reactor event callbacks | `OnMessageReceived`, `OnPeerStateChange`, etc. | ~250 | `server_events.go` |

#### `internal/config/routeattr.go` — 1437 lines, 6 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Community types + parsers | `Community`, `LargeCommunity`, `ExtendedCommunity` | ~250 | `routeattr_community.go` |
| RouteDistinguisher | `RouteDistinguisher` type + parser | ~100 | `routeattr.go` (keep) |
| ASPath/Aggregator | `ASPath`, `Aggregator` types + parsers | ~200 | `routeattr.go` (keep) |
| RawAttribute + hex parsing | `RawAttribute` | ~100 | `routeattr.go` (keep) |
| PrefixSID + SRv6 parsing | `parsePrefixSID`, `ParsePrefixSIDSRv6` with SRGB | ~300 | `routeattr_prefixsid.go` |
| ParsedRouteAttributes aggregation | `ParsedRouteAttributes` | ~100 | `routeattr.go` (keep) |

#### `internal/plugins/bgp-nlri-flowspec/plugin.go` — 1415 lines, 6 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Plugin entry + NLRI hex decode/encode | Entry point, SDK wiring | ~100 | `plugin.go` (keep) |
| CLI decode runner | CLI handler | ~100 | `plugin.go` (keep) |
| FlowSpec decoding | `decodeFlowSpecNLRI`, `flowSpecToJSON`, `componentToJSON` | ~300 | `decode.go` |
| Numeric/bitmask match formatting | Format helpers | ~150 | `decode.go` (with decoding) |
| Text-to-wire encoding | `EncodeFlowSpecComponents`, `parseComponentText`, per-component parsers | ~400 | `encode.go` |
| Operator/value parsing | Operator/value parse helpers | ~200 | `encode.go` (with encoding) |

#### `internal/plugins/bgp-nlri-flowspec/types.go` — 1381 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| FlowSpec struct + wire methods | `FlowSpec`, `Bytes`, `WriteTo`, `Len` | ~100 | `types.go` (keep) |
| prefixComponent type | Type + wire encoding | ~200 | `types_prefix.go` |
| numericComponent type | Type + wire encoding + string formatting | ~400 | `types_numeric.go` |
| Flow component parsing | `ParseFlowSpec`, `parseFlowComponent`, etc. | ~400 | `types_parse.go` |
| FlowOperator/FlowMatch constants | Types and constants | ~100 | `types.go` (keep) |

#### `internal/exabgp/migrate.go` — 1377 lines, 7 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Top-level orchestrator | `MigrateFromExaBGP` | ~100 | `migrate.go` (keep) |
| Neighbor migration | `migrateNeighbors`, `migrateSingleNeighbor`, `expandInheritance`, `copySimpleFields` | ~300 | `migrate_neighbor.go` |
| Capability migration | `migrateCapability`, `migrateHostnameToCapability` | ~200 | `migrate_capability.go` |
| Route/flow/VPN/VPLS/MVPN/MUP conversion | `convertAnnounceToUpdate`, `convertFlowToUpdate`, etc. | ~400 | `migrate_routes.go` |
| Family/nexthop syntax conversion | Family and nexthop helpers | ~100 | `migrate.go` (keep) |
| Process binding migration | Process binding conversion | ~100 | `migrate.go` (keep) |
| Serializer | `SerializeTree` | ~150 | `migrate_serialize.go` |

#### `internal/plugins/bgp-nlri-ls/types.go` — 1216 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| BGP-LS type/const definitions | Protocol IDs, NLRI types, TLV codes | ~200 | `types.go` (keep) |
| NodeDescriptor/LinkDescriptor/PrefixDescriptor | Descriptor types with `Bytes`/`WriteTo` | ~200 | `types.go` (keep) |
| BGPLSNode | Struct + wire encoding | ~300 | `types_node.go` |
| BGPLSLink | Struct + wire encoding | ~300 | `types_link.go` |
| BGPLSPrefix | Struct + wire encoding | ~200 | `types_prefix.go` |

#### `internal/test/runner/record.go` — 1138 lines, 4 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| Record/State/MessageExpect structs | Type definitions | ~150 | `record.go` (keep) |
| Tests collection | `add`, `enable`, `select`, `summary` | ~200 | `record.go` (keep) |
| EncodingTests discovery + .ci parsing | `parseAndAdd`, `parseLine` | ~400 | `record_parse.go` |
| Option/expect/reject/action/cmd parsers | Per-directive parsers | ~350 | `record_directives.go` |

#### `cmd/ze-chaos/web/state.go` — 1062 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| ControlCommand/ControlState/PeerState structs | Core types | ~150 | `state.go` (keep) |
| RingBuffer generic | Generic ring buffer | ~100 | `ring_buffer.go` |
| ActiveSet | Peer promotion/eviction/pinning/decay | ~300 | `active_set.go` |
| ConvergenceHistogram | Histogram data structure | ~200 | `convergence.go` |
| RouteMatrix | Route tracking matrix | ~250 | `route_matrix.go` |

#### `internal/plugins/bgp-rib/rib.go` — 1055 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| RIBManager struct + entry | `RIBManager`, `RunRIBPlugin` | ~150 | `rib.go` (keep) |
| Route dispatch | `handleReceived`, `handleReceivedPool`, `handleSent`, `handleState`, `handleRefresh` | ~300 | `rib_dispatch.go` |
| Wire format helpers | `prefixToWire`, `wireToPrefix`, `splitNLRIs`, `formatNLRIAsPrefix` | ~200 | `rib_wire.go` |
| Command handling | `handleCommand`, `inboundShowJSON`, `outboundShowJSON`, `outboundResendJSON` | ~250 | `rib_commands.go` |
| Route replay | Replay functions | ~100 | `rib.go` (keep) |

#### `internal/config/loader.go` — 1034 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| parseTreeWithYANG | YANG-based tree parsing | ~100 | `loader.go` (keep) |
| LoadReactor* family | Various loaders (with/without plugins, from string/file) | ~200 | `loader.go` (keep) |
| CreateReactorFromTree | Full config pipeline | ~300 | `loader_reactor.go` |
| Route converters | `convertMVPNRoute`, `convertVPLSRoute`, `convertFlowSpecRoute`, `convertMUPRoute` | ~250 | `loader_routes.go` |
| Prefix expansion helpers | Prefix helpers | ~100 | `loader.go` (keep) |

#### `internal/exabgp/bridge.go` — 1028 lines, 4 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| ZebgpToExabgpJSON | IPC format translation (ze→exabgp) | ~300 | `bridge_format.go` |
| ExabgpToZebgpCommand | Text command translation (exabgp→ze) | ~250 | `bridge_command.go` |
| StartupProtocol | 5-stage negotiation | ~200 | `bridge.go` (keep) |
| Bridge struct | Subprocess management, bidirectional relay | ~250 | `bridge.go` (keep) |

#### `cmd/ze/config/main.go` — 1024 lines, 5 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| cmdEdit | Edit subcommand | ~200 | `edit.go` |
| cmdCheck + validation | `cmdCheck`, `configCheckData`, `findDeprecatedPatterns`, `findUnsupportedFeatures` | ~250 | `check.go` |
| cmdMigrate | `cmdMigrate`, `cmdMigrateDryRun` | ~150 | `migrate.go` |
| cmdFmt | `ConfigFmtBytes`, `cmdFmt`, `printDiff` | ~200 | `fmt.go` |
| cmdDump | `cmdDump`, `printConfig`, `printTreeMap` | ~200 | `dump.go` |

#### `cmd/ze-chaos/peer/simulator.go` — 1014 lines, 4 concerns

| Concern | Functions | Approx Lines | Target File |
|---------|-----------|------:|-------------|
| RunSimulator + action execution | Main loop + chaos/route action dispatch | ~300 | `simulator.go` (keep) |
| Wire protocol helpers | `readLoop`, `parseUpdatePrefixes`, `parseMPReachNLRI`, `parseMPUnreachNLRI` | ~300 | `simulator_wire.go` |
| Reconnect storm + collision | Reconnect storm scenarios | ~200 | `simulator_reconnect.go` |
| Route withdrawal helpers | Route withdrawal utilities | ~150 | `simulator_withdraw.go` |

### Phase 3 — Medium Priority (600–1000 lines, splittable only)

#### `internal/plugins/bgp-nlri-evpn/types.go` — 954 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Shared EVPN infrastructure + ParseEVPN dispatcher | ~200 | `types.go` (keep) |
| EVPNType1 (Ethernet Auto-Discovery) | ~150 | `type1_ead.go` |
| EVPNType2 (MAC/IP Advertisement) | ~200 | `type2_macip.go` |
| EVPNType3 (Inclusive Multicast) | ~100 | `type3_imet.go` |
| EVPNType4 (Ethernet Segment) | ~100 | `type4_es.go` |
| EVPNType5 (IP Prefix) + EVPNGeneric | ~150 | `type5_prefix.go` |

#### `pkg/plugin/sdk/sdk.go` — 938 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Lifecycle + construction + startup protocol | ~300 | `sdk.go` (keep) |
| Engine-call API methods | ~200 | `sdk_engine.go` |
| Callback dispatch + handlers | ~300 | `sdk_callbacks.go` |
| Type aliases re-exporting rpc types | ~100 | `sdk_types.go` |

#### `internal/plugin/process.go` — 928 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Process struct + event delivery | ~250 | `process.go` (keep) |
| Process startup (internal/external) | ~300 | `process_start.go` |
| Process lifecycle (Stop/Wait) | ~150 | `process.go` (keep) |
| ProcessManager | ~240 | `process_manager.go` |

#### `internal/config/editor/editor.go` — 911 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Editor struct + accessors + dirty state | ~200 | `editor.go` (keep) |
| Pending-edit file management | ~100 | `editor.go` (keep) |
| Schema-aware tree traversal | ~200 | `editor_walk.go` |
| Tree mutation (Set/Delete*) | ~150 | `editor_mutate.go` |
| Persistence + backup + rollback | ~150 | `editor_persist.go` |
| Diff utility | ~100 | `editor.go` (keep) |

#### `internal/plugins/bgp-rr/server.go` — 880 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Entry point + setup | ~100 | `server.go` (keep) |
| Dispatch + flow control | ~150 | `server.go` (keep) |
| Forwarding logic | ~250 | `forward.go` |
| State handlers | ~200 | `state.go` |
| JSON event parsing + types | ~200 | `event.go` |

#### `internal/plugins/bgp/format/text.go` — 876 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Top-level dispatcher + UPDATE format engine | ~250 | `text.go` (keep) |
| Per-attribute JSON + text formatters | ~250 | `text_attr.go` |
| NLRI JSON formatting helpers | ~150 | `text_nlri.go` |
| Non-UPDATE message formatters | ~200 | `text_non_update.go` |

#### `internal/plugins/bgp/reactor/config.go` — 826 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Peer + PeersFromTree parsers | ~200 | `config.go` (keep) |
| Family parsing | ~100 | `config.go` (keep) |
| Capability parsing | ~330 | `config_capability.go` |
| Process binding parsing | ~100 | `config.go` (keep) |
| Map navigation helpers | ~100 | `config_helpers.go` |

#### `pkg/plugin/plugin.go` — 783 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Struct + registration + I/O | ~200 | `plugin.go` (keep) |
| Protocol loop (5-stage + command loop) | ~200 | `plugin_protocol.go` |
| Config command handling | ~100 | `plugin.go` (keep) |
| Candidate/running state management | ~200 | `plugin_candidate.go` |
| Handler dispatch | ~100 | `plugin.go` (keep) |

#### `internal/plugins/bgp/handler/bgp.go` — 641 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| RPC registration factories + shared helper | ~100 | `bgp.go` (keep) |
| Peer operation handlers | ~300 | `bgp_peer_ops.go` |
| Introspection/plugin handlers | ~250 | `bgp_introspection.go` |

#### `cmd/ze/cli/main.go` — 633 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Entry point + dispatch | ~100 | `main.go` (keep) |
| cliClient struct + network transport | ~150 | `client.go` |
| BubbleTea interactive model | ~350 | `model.go` |

#### `cmd/ze-chaos/web/dashboard.go` — 626 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Config/Dashboard struct + HTTP lifecycle | ~200 | `dashboard.go` (keep) |
| HTTP handlers | ~200 | `dashboard.go` (keep) |
| SSE event ring buffer + ingestion | ~200 | `dashboard_events.go` |

#### `cmd/ze-chaos/web/control.go` — 699 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| HTTP control handlers | ~400 | `control.go` (keep) |
| HTML rendering helpers | ~300 | `control_html.go` |

#### `internal/plugins/bgp-nlri-ls/plugin.go` — 658 lines

| Concern | Approx Lines | Target File |
|---------|------:|-------------|
| Plugin entry point + SDK wiring | ~100 | `plugin.go` (keep) |
| BGP-LS decode logic | ~300 | `decode.go` |
| BGP-LS encode logic | ~250 | `encode.go` |

### Files Surveyed and NOT Split (single concern or tightly coupled)

These files were analyzed and determined to be single-concern despite their size:

| File | Lines | Why Not Split |
|------|-------|---------------|
| `capability/capability.go` | 783 | Coherent registry of BGP capabilities; parse dispatcher references every type |
| `message/rfc7606.go` | 769 | Single purpose: RFC 7606 validation; validators form dependency chain |
| `pool/pool.go` | 745 | Tightly integrated: Intern/compaction/index share internal state |
| `config/editor/model_load.go` | 736 | Already a deliberate split from spec 221 |
| `config/editor/model.go` | 702 | Already split in spec 221; remaining concerns are tightly coupled Model methods |
| `config/environment.go` | 700 | Single concept: environment config loading; sub-structs + loader + accessors are one unit |
| `config/schema.go` | 649 | Single concept: schema type system; types + validation + navigation are inseparable |
| `cmd/ze-test/bgp.go` | 679 | CLI entrypoint: phases of same pipeline (parse → discover → run → report) |
| `cmd/ze/schema/main.go` | 620 | CLI command file: standard one-function-per-subcommand pattern |
| `config/editor/completer.go` | 606 | Single concern: tab completion |
| `rib/outgoing.go` | 575 | Single concern: per-peer outgoing queue |
| `filter/filter.go` | 574 | Single concern: UPDATE filtering |
| `config/reader.go` | 555 | Single concern: config file discovery + format detection |
| `flowspec/config_builder.go` | 553 | Single concern: FlowSpec config → wire |
| `rib/commit.go` | 534 | Single concern: RIB commit logic |

---

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any split file | All existing tests pass unchanged — `make ze-verify` |
| AC-2 | Files created from splits | Each file has `// Design:` and `// Related:` comments |
| AC-3 | `// Related:` referencing renamed/deleted file | Enforcement hook exits 2 (blocks write) |
| AC-4 | `// Related:` referencing existing sibling file | Enforcement hook exits 0 (allows write) |
| AC-5 | New `.go` file written without `// Related:` in a package with coupled files | Warning (exit 1) — non-blocking since not all files need Related |
| AC-6 | `scripts/check-related-refs.sh` run on clean repo | All `// Related:` comments point to existing files |
| AC-7 | Phase 1 files (>2000 lines) | All split per survey tables above |
| AC-8 | Phase 2 files (1000–2000 lines) | All split per survey tables above |
| AC-9 | Phase 3 files (600–1000 lines, splittable) | All split per survey tables above |

## 🧪 TDD Test Plan

### Unit Tests

No new tests — this is a refactor. All existing tests must continue to pass unchanged.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| All existing tests | `go test ./...` | Behavior unchanged after splits | |

### Functional Tests

No new functional tests. All existing must pass.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing | `make ze-functional-test` | Full system behavior unchanged | |

### Hook Tests

| Test | Validates | Status |
|------|-----------|--------|
| Write file with `// Related: nonexistent.go` | Hook blocks (exit 2) | |
| Write file with `// Related: existing-sibling.go` | Hook allows (exit 0) | |
| `scripts/check-related-refs.sh` on repo | All refs valid, exit 0 | |

## Files to Modify

Every file listed in the survey tables above that has "(keep)" — these get `// Related:` comments added.

## Files to Create

### Source Files (from splits)

**Phase 1 (6 packages, ~30 new files):**
- `reactor/reactor_announce.go`, `reactor_wire.go`, `reactor_config_reconcile.go`, `reactor_peer_ops.go`
- `reactor/peer_static_routes.go`, `peer_rib_routes.go`, `peer_connection.go`, `peer_initial_sync.go`, `peer_send.go`
- `reactor/session_read.go`, `session_handlers.go`, `session_validation.go`, `session_negotiate.go`, `session_flow.go`
- `message/update_build_unicast.go`, `update_build_vpn.go`, `update_build_labeled.go`, `update_build_mvpn.go`, `update_build_vpls.go`, `update_build_flowspec.go`, `update_build_evpn.go`, `update_build_mup.go`, `update_build_grouped.go`
- `handler/update_text_nlri.go`, `update_text_flowspec.go`, `update_text_vpls.go`, `update_text_evpn.go`, `update_text_dispatch.go`
- `cmd/ze/bgp/decode_open.go`, `decode_update.go`, `decode_mp.go`, `decode_extcomm.go`, `decode_bgpls.go`, `decode_human.go`

**Phase 2 (14 packages, ~40 new files):**
- See survey tables above for complete list

**Phase 3 (14 packages, ~30 new files):**
- See survey tables above for complete list

### Infrastructure Files
- `.claude/hooks/check-related-refs.sh` — enforcement hook for `// Related:` staleness
- `scripts/check-related-refs.sh` — standalone script to validate all `// Related:` references

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| `// Design:` comments on new files | Yes | Every new `.go` file |
| `// Related:` comments on all split files | Yes | Every file in a split group |
| Hook registration | Yes | `.claude/settings.json` |

## Implementation Steps

### Ordering

Each phase is independent. Within a phase, each package is independent. Recommended order: largest files first (most context-window benefit).

For each file split:
1. Read the file, confirm concern boundaries match survey
2. Create new files with correct `// Design:` and `// Related:` comments
3. Move functions/types to target files (cut from original, paste to new)
4. Run `go build ./path/to/package/...` — verify compilation
5. Run `go test ./path/to/package/...` — verify all tests pass
6. Add `// Related:` comments to the original file pointing to new siblings
7. Run `make ze-lint` — verify zero lint issues

After all splits in a phase:
8. Run `make ze-verify` — full verification
9. Self-Critical Review

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Check imports — `goimports` should fix. Check file-local types moved with consumers |
| Test fails | Check shared test helpers — must stay in base `_test.go` |
| Lint failure | Fix inline — likely import ordering |

## Risks

- **Import blocks**: Each new file needs imports for only what it uses. Auto-linter hook handles this.
- **File-local types**: Types used only within a concern must move with that concern's functions.
- **Shared test helpers**: Test setup functions (config strings, builder helpers) stay in base `_test.go` file.
- **Init ordering**: Go doesn't guarantee `init()` order across files. Verify no `init()` functions in split targets.
- **Scale**: ~100 new files across ~34 packages. Execute per-package with compilation check after each.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] All split files have `// Design:` comments
- [ ] All split file groups have `// Related:` comments
- [ ] Enforcement hook validates `// Related:` references
- [ ] Architecture docs updated

### Quality Gates
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Critical Review passes (all 6 checks)

### Design
- [ ] No premature abstraction — pure mechanical split
- [ ] No speculative features — reduces existing complexity
- [ ] Single responsibility — each new file has one concern
- [ ] Explicit behavior — no behavior changes
- [ ] Minimal coupling — splitting reduces what must be loaded together

### TDD
- [ ] Tests written (no new tests — refactor validated by existing test suite)
- [ ] Tests FAIL (N/A — refactor, not new feature; existing tests must keep passing)
- [ ] Tests PASS (all existing tests pass after split)
