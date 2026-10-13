# Spec: plugin-ownership-imports

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le arch tier check` has been red since `9f316482c1` (2026-10-03) added the
plugin-ownership gate (`pluginOwnershipGate`, `internal/le/arch/tier/ownership.go`)
with no baseline. The gate refuses a production import into another plugin's
implementation (`ai/rules/plugins.md`: removing a plugin MUST make its features
disappear and leave every other plugin and the core working). The owner decided
on 2026-10-12 that the crossing edges get their own spec.

Measured on 2026-10-12, the gate reports **35** edges (the brief said 34; the
journal row of 2026-10-05 also says 35). Every edge is listed below with the
route that removes it. The goal: the gate reports `plugin ownership OK` with no
baseline, no new exemption in `ownership.go`, no behavior an operator can see
changed except where a row below names it, and no allocation or indirection
added per UPDATE.

The red also holds `test/ui/le-placement-answers.ci` red
(`plan/journal/gate-red-where-nothing-blocks-on-it.md`, row 2026-10-05).

### The 35 edges and their routes

| # | Importer | Imports | Edge kind (verified in source) | Route |
|---|----------|---------|-------------------------------|-------|
| 1-6 | `internal/component/bgp/reactor/reactor.go` | `bgp/plugins/cmd/{cache,commit,peer,raw,update}`, `bgp/plugins/route_refresh/handler` | blank import, RPC registration only; no symbol called | A: delete, composition already carries them |
| 7 | `internal/component/bgp/reactor/reactor.go` | `bgp/reactor/filter` | one call, `filter.SetMetricsRegistry`, at reactor start | C: filter metrics through `filterapi` |
| 8-12 | `internal/component/config/yang/cli/tree_bgp.go` | `bgp/plugins/cmd/{peer,raw,rib,update}`, `route_refresh/handler` | blank import, RPC registration only | A: delete the file |
| 13 | `internal/component/firewall/cli/main.go` | `plugins/firewall/nft` | blank import, backend registration | A |
| 14 | `internal/component/iface/cli/main.go` | `plugins/iface/netlink` | blank import, backend registration | A |
| 15 | `internal/component/traffic/cli/main.go` | `plugins/traffic/netlink` | blank import, backend registration | A |
| 16 | `internal/plugins/init/main.go` | `plugins/iface/netlink` | blank import, backend registration | A |
| 17 | `internal/component/mtu/cmd/doc.go` | `plugins/mtu-cmd/yang` | blank import of the command schema | B: schema moves into its owner |
| 18 | `internal/component/ping/cmd/doc.go` | `plugins/ping-cmd/yang` | same | B |
| 19 | `internal/component/traceroute/cmd/doc.go` | `plugins/traceroute-cmd/yang` | same | B |
| 20 | `internal/component/bgp/plugins/epe/epe_source.go` | `bgp/plugins/nlri/ls` | `TLVSRCapabilities`, `LsSRCapabilities`, `LsSrLabelRange` | D: BGP-LS wire vocabulary to core |
| 21-24 | `internal/component/bgp/plugins/ls_export/export_{encode,epe,plugin,state}.go` | `bgp/plugins/nlri/ls` | NLRI constructors, descriptors, TLV codes, `WriteTLVBytes`, `BGPLSProtocolID`, `BGPLSFamily` | D |
| 25 | `internal/component/bgp/plugins/rib/rib_flowspec_validation.go` | `bgp/plugins/nlri/flowspec` | `ParseFlowSpec`, `ParseFlowSpecVPN`, `FlowComponent`, `FlowDestPrefix` | D: FlowSpec wire vocabulary to core |
| 26-30 | `internal/plugins/flowspec-firewall/{numeric_ranges,selected,state,translate,transport}.go` | `bgp/plugins/nlri/flowspec` | `FlowSpec`, `FlowComponent`, component and operator constants, `ParseFlowSpec`, `Compare` | D |
| 31 | `internal/component/bgp/cli/decode_update.go` | `bgp/plugins/nlri/ls` | `AttrTLVsToJSON` for attribute 29 in the offline decoder | E: attribute JSON decoder registered by the owner |
| 32 | `internal/component/bgp/plugins/cmd/update/update_text_srpolicy.go` | `bgp/plugins/nlri/srpolicy` | `srpolicy.New`, inside a second copy of the SR-Policy text grammar | F: use the encoder the plugin already registers |
| 33 | `internal/component/bgp/plugins/cmd/rib/pool_stats.go` | `bgp/plugins/rib/pool` | `pool.AllPools()` for `pool-stats` | G: the handler moves into the rib owner |
| 34 | `internal/component/bfd/bfd.go` | `plugins/diag/cmd` | `diagCmd.SetBFDRawCaptureProvider` | H: provider seam moves to `bfd/api` |
| 35 | `cmd/ze/hub/main_system.go` | `plugins/crashes` | `crashes.HarvestAtBoot` at daemon start | I: boot effect registered by the owner |

The earlier grouping this spec started from said reactor.go and tree_bgp.go
"look handlers up through the command registry". That is wrong: neither file
calls a handler. Every one of their eleven plugin edges is a blank import that
duplicates the generated composition, and the twelfth is a startup-time metrics
setter.

### Owner questions (scope and product only)

Answered by Thomas, 2026-10-12: Q-1 "One spec (Recommended)"; Q-2 "Accept all
three (Recommended)". Q-A and Q-B came from design review round 3; Thomas took
the recommended option on both, 2026-10-12 (relayed by the coordinating session).
Q-C came from design review round 4 and is open.

| # | Question | Recommendation |
|---|----------|----------------|
| Q-1 | One spec in phases, or split route D (the FlowSpec and BGP-LS core move, 11 edges, the largest phase) into its own spec? | one spec, D1 and D2 as separate phases and commits |
| Q-2 | `update text` for SR-Policy refuses three inputs it accepts today: (a) an endpoint whose address family does not match the NLRI's AFI (today encoded); (b) `path-information N` that applies to an SR-Policy section (top level or inside it; Q-A keeps one inside another family's section accepted), including a mixed command such as `path-information 5 nlri ipv4/unicast add ... nlri ipv4/sr-policy add ...` (today the SR-Policy section drops the identifier silently): the whole command is refused, so one command never half-applies; (c) a second `add`/`del` after the first field of a section (today the last one wins); `add`/`del` must come first for every registry family. Accept all three? | accept: (a) the plugin's encoder and config path already refuse it; (b) SR-Policy NLRIs do not support ADD-PATH and a silent drop is a wrong value; (c) one grammar for every plugin family. Every other changed input in AC-11 was already refused and only gets a clearer message |
| Q-A | A `path-information` written inside ANOTHER family's section, in a command that also has an SR-Policy section (`nlri ipv4/unicast path-information 5 add ... nlri ipv4/sr-policy add ...`): refuse the whole command, or accept it? The identifier lives in the accumulator copy of the unicast section only (`ParseUpdateText` hands each section `attrs.snapshot()`), so it never reaches the SR-Policy NLRI | **Decided 2026-10-12: accept.** Refuse only a `path-information` that applies to the SR-Policy section: written at top level before that section's `nlri`, or inside the SR-Policy section. That is the silent drop Q-2 (b) is about; per-section identifiers of other families stay as documented. The other option, refusing the whole command whenever any `path-information` sits beside an SR-Policy section, was not taken. Phase F step 8 and AC-11d state this one rule |
| Q-C | **Open, from design review round 4 (owner question).** Where do the `l2tp-health` and `bgp-health` RPCs go? `internal/component/cmd/show/yang/ze-cli-show-api.yang` declares them in an always-linked module, while their handlers are gated (`internal/component/l2tp/show_health.go` under `ze_l2tp`; `bgp/plugins/healthcheck/` under `ze_bgp`). Options: (a) this spec; (b) `plan/spec-yang-feature-opt-ins-owned-by-feature.md` (its Q-8 raised it); (c) a spec of their own. Neither RPC is an import edge, so `./le arch tier check` names neither and AC-1 cannot prove the move; this spec moves a command schema only where its location is itself one of the 35 edges (routes B and G) | (b), the opt-ins spec: its rule is the defect, a node that survives in a binary without its owner, and this spec's gate cannot see it. Recorded identically in that spec |
| Q-B | Route I makes `crashes` a name the `plugin ... use` completion offers (`InternalPluginNameValidator` completes from `registry.Names()`) and adds `ze plugin crashes`, a no-op that exits 0 (`registry.Register` requires a `CLIHandler`): the exposure `loop` and `bgp-nlri-srpolicy` carry today. Accept and journal the class, or widen this spec to hide no-op registrations from completion and the CLI? | **Decided 2026-10-12: accept.** Both surfaces are listed under "Behavior to change"; the class has one row in `plan/journal/inert-registration-offered-as-a-choice.md`, written with this spec edit. Hiding no-op registrations is a separate change to the completion contract, not done here |

### Route D symbol inventory (verified by type-checked cross-reference scan, 2026-10-12, design reviews rounds 4 and 5)

Go methods live with their type, so every method of a moving type moves with
it, including its `AppendJSON` (this resolves `flowspec/json.go` reading the
unexported `.family`, `.flowSpec`, `.rd`, and `ls/json.go` reading `cachedBytes`).
Nothing that registers moves: every `family.MustRegister` call and every fill of
a registry stays in the plugin. A package-level table that moves (the keyword
constants, the flag and protocol name maps) is a fixed vocabulary initialised
from literals, never a registry written from `init()` (A-4).

**FlowSpec: `internal/core/bgp/nlri/flowspec`**

| Plugin file | Moves to core | Stays in the plugin | Newly exported by core (called from the plugin) |
|-------------|---------------|---------------------|-----------------------------------------------|
| `types.go` | all types, constants, `NewFlowSpec`, `ParseFlowSpec`, methods (including `FlowSpec.componentBytes`), `parseFlowComponent`, `componentOfType` | the four `family.MustRegister` vars (`IPv4FlowSpec`, `IPv6FlowSpec`, `IPv4FlowSpecVPN`, `IPv6FlowSpecVPN`), moved to `register.go`; core uses `family.Family` values built from `internal/core/family` constants | `FlowSpec.ComponentBytes` (staying `config.go` `parseConfigRoute`); `parseFlowComponent` is called only inside `types.go` |
| `types_numeric.go` | whole file | - | `NewFlowNumericComponent` (`config_builder.go`, `plugin_encode_text.go`), `NewFlowTCPFlagsMatchComponent`, `NewFlowFragmentMatchComponent` (`plugin_encode_text.go`), `NewFlowICMPCodeComponent` (`config_builder.go`) |
| `types_prefix.go` | whole file | - | `NewFlowDestPrefixComponentWithOffset`, `NewFlowSourcePrefixComponentWithOffset` (`config_builder.go`) |
| `types_vpn.go` | whole file | - | - |
| `compare.go` | whole file (`Compare` uses `componentOfType`) | - | - |
| `json.go` | whole file: the `AppendJSON` methods and their helpers (`appendFlowSpecJSON`, `flowSpecKey`, `appendNestedStringArray`) | - | - |
| `plugin_decode.go` | the rendering helpers the moving `json.go` and `types_numeric.go` call, with their closure: `formatPrefixWithOffset`, `formatNumericMatches`, `formatNumericValue`, `formatWithOperator`, `formatBitmaskMatches`, `formatBitmaskValue`, `protocolNumberToName`, `reverseProtocolNames`, `tcpFlagValueToNames`, `fragmentFlagValueToNames` | `componentToJSON`, `flowSpecToJSON`, `formatFlowSpecText`, `jsonToTextComponents`, `isPrefixComponentKey`, `normalizeJSONValue` and every other declaration | `FormatPrefixWithOffset`, `FormatNumericMatches`, `FormatBitmaskMatches`, `TCPFlagValueToNames`, `FragmentFlagValueToNames` (staying `componentToJSON`) |
| `plugin_encode_text.go` | the component keyword const block (all 18 `kw*` names: `kwTrafficClass` stays beside the word it aliases, `kwRD` beside the rest); `protocolNameToNumber` (read by the moving `reverseProtocolNames`); `tcpFlagNameToValue` and `fragmentFlagNameToValue`, so each name table sits beside its reverse and `TestFlagNamesRoundTrip` compares two tables of one package | every parser, every error var, `isComponentKeyword`, `parseComponentText` | the 18 keywords (named by the staying `config_builder.go`, `plugin_decode.go`, `plugin_encode_text.go`), `ProtocolNameToNumber`, `TCPFlagNameToValue`, `FragmentFlagNameToValue` |
| `config.go`, `config_builder.go`, `encode.go`, `plugin.go`, `plugin_protocol.go`, `register.go` | - | whole files | - |

**BGP-LS: `internal/core/bgp/nlri/bgpls`**

| Plugin file | Moves to core | Stays in the plugin | Newly exported by core (called from the plugin) |
|-------------|---------------|---------------------|-----------------------------------------------|
| `types.go` | all NLRI types, the NLRI interface `bGPLSNLRI`, `BGPLSNLRIType`, `BGPLSProtocolID`, `bgplsBase` and its methods, `parseBGPLS`, descriptor parsers, `parseBGPLSWithRest`, `tlv`, `writeTLV`, `uint32ToBytes`, `WriteTLVBytes`, the `ErrBGPLS*` error vars, the AFI/SAFI aliases | `BGPLSFamily`, `BGPLSVPNFamily` (`family.MustRegister`), moved to `register.go`; `ls_export` names its families from `internal/core/family` constants | `BGPLSNLRI` (the interface; staying `plugin.go` `bgplsToJSON(n bGPLSNLRI, ...)`), `ParseBGPLSWithRest` (`plugin.go` `decodeBGPLSNLRI`), `WriteTLV` (every staying `attr*.go` `WriteTo`); `tlv` is named only by the moving descriptor `Bytes` methods and stays unexported; the already-exported `ErrBGPLS*` vars are named through the core package by the staying `attr*.go` |
| `types_nlri.go`, `types_descriptor.go`, `types_srv6.go` | whole files | - | - |
| `json.go` | whole file: `AppendJSON` methods and helpers | - | - |
| `plugin.go` | the NLRI JSON helpers the moving `json.go` calls, with their closure: `bgplsNLRITypeString`, `prefixDescriptorInfo`, `parseBGPLSPrefixTLVs`, `parseBGPLSNodeTLVs`, `linkDescriptorInfo`, `parseBGPLSLinkTLVs`, `parseNodeDescriptorSubTLVs`, `formatRouterID`, `formatIPReachability`, `formatIPv6Compressed`, `jsonKeyRouterID` | `bgplsToJSON`, `decodeBGPLSNLRI`, `formatBGPLSText`, `formatBGPLSTextSingle`, the CLI and decode-mode runners, every other declaration | `BGPLSNLRITypeString`, `ParseBGPLSNodeTLVs`, `ParseBGPLSLinkTLVs`, `ParseBGPLSPrefixTLVs`, `FormatIPv6Compressed`, and the two result structs with the six fields the staying `bgplsToJSON` reads (`LinkDescriptorInfo` `.IfAddrs`, `.LinkIDs`, `.MTIDs`, `.NeighAddrs`; `PrefixDescriptorInfo` `.MTIDs`, `.Prefix`: each struct declares its own `mtIDs`, and `bgplsToJSON` reads the link one); `formatRouterID`, `formatIPReachability`, `parseNodeDescriptorSubTLVs` and `jsonKeyRouterID` are named only by moving code and stay unexported |
| `attr.go` | the two JSON key constants the moving `LsSRCapabilities.ToJSON` reads, `jsonKeyFlags` ("flags") and `jsonKeyReserved` ("RSV"), declared once in core | the TLV decoder registry: `lsAttrTLV`, `lsAttrTLVDecoder`, `lsAttrTLVRegistry`, `registerLsAttrTLV`, `lookupLsAttrTLVDecoder`, `registeredLsAttrTLVCount`, `iterateAttrTLVs`, `decodeAttrTLV`, `decodeAllAttrTLVs`, `AttrTLVsToJSON`, every other JSON helper and key | `JSONKeyFlags`, `JSONKeyReserved` (read by the staying attribute types' `ToJSON`) |
| `attr_node.go` | every `TLV*` code constant; `LsSrLabelRange`, `LsSRCapabilities` with their methods; the label-range helpers `srLabelRangesLen`, `writeSrLabelRanges`, `decodeSrLabelRanges`, `srLabelRangesToJSON`; `decodeSRCapabilities`, returning `(*LsSRCapabilities, error)` in core | every other `ls*` attribute type and decoder, including `lsSRLocalBlock` (TLV 1036) and `decodeSRLocalBlock` | `DecodeSRCapabilities` (adapted by `register_attr.go`, A-4); `SRLabelRangesLen`, `WriteSRLabelRanges`, `DecodeSRLabelRanges`, `SRLabelRangesToJSON` (called by the staying `lsSRLocalBlock.Len`, `WriteTo`, `ToJSON` and `decodeSRLocalBlock`) |
| `attr_link.go`, `attr_prefix.go`, `attr_srv6.go` | the `TLV*` code constants (including `TLVPeerNodeSID`, `TLVPeerAdjSID`, `TLVPeerSetSID`, `TLVSIDLabel`) | attribute types, decoders, the TLV name maps | - |
| `register.go`, `register_attr.go` | - | whole files | - |

The moving declarations import only the standard library and
`internal/core/{bgp/nlri,bgp/wire,family,textbuf}` (the moved `plugin.go` and
`plugin_decode.go` helpers use `encoding/binary`, `net/netip`, `strings` and
`textbuf` only), so core gains no dependency on component or plugin code.

**How the inventory was closed (round 4, both directions, every moving
declaration of both packages).** The round-3 paragraph claimed a BGP-LS
re-scan in both directions; it missed the `plugin.go` helpers, and FlowSpec had
not been re-scanned at all. The method, so a later round can re-run it
(round 5 re-ran it with an independent scanner, agreed on the production
counts, found one more field, and rewrote step 6):
1. Type-check each plugin package, its production files and its in-package
   `_test.go` files, with `go/types` over the compiler's export data
   (`go list -export -deps -test`).
2. For every identifier use whose object is declared in the same package at
   package level, or is a method or a field, record the top-level declaration
   holding the use and the top-level declaration holding the object (a method
   is its own declaration, `Type.Method`; one `const` or `var` spec is one
   declaration).
3. Mark every declaration moving or staying by the two tables above.
4. Forward: list every moving declaration that names a staying one. Add each
   named declaration to the moving set and repeat until the list is empty,
   except `decodeSRCapabilities` naming `lsAttrTLV`, which A-4 removes by
   changing the core return type.
5. Reverse: list every staying production declaration that names a moving
   one. Each unexported name in that list is in the "Newly exported" column;
   an exported one is reached through the core package by its name. This list
   is never empty after the move, because staying code names core exports by
   design; the check after the move is that it holds no edge to an UNEXPORTED
   core name.
6. Tests (rewritten in round 5, which found the round-4 step blind to test
   helpers and to staying names). For every test function and every test
   helper, follow its test-to-test edges (a helper, a fixture constant, a
   builder in any `_test.go` file of the package) transitively, and list over
   that closure (a) every moving declaration it names that stays UNEXPORTED in
   core, so is not in the "Newly exported" column, and (b) every STAYING
   declaration it names, exported or not. The registered family vars
   (FlowSpec `IPv4FlowSpec`, `IPv6FlowSpec`, `IPv4FlowSpecVPN`,
   `IPv6FlowSpecVPN`; BGP-LS `BGPLSFamily`, `BGPLSVPNFamily`) are counted
   apart: a core test names a family by a `family.Family` value built from
   `internal/core/family` constants, as core itself does (A-4). Routing:
   - a test file every test function of which has an empty (b) list moves to
     core whole, beside the code it tests;
   - in any other file a test function moves when it must, (a) non-empty and
     (b) empty, and every other function stays, naming core exports;
   - a function with both lists non-empty cannot be placed and is rewritten
     (one: `TestRFC7752UnknownTLVPreservedAndPropagated`);
   - a test helper goes where its users go; a helper with users on both sides
     is declared in each package, because a `_test.go` declaration of one
     package cannot be imported by another, and the core copy names the
     plugin copy as its source.
   A core in-package test cannot import the plugin (import cycle), so after
   the move no core test file may name a staying declaration. Routes are in
   the test table below.
7. Outside the package: list every file in the module that imports the plugin
   package, with every moving AND every staying symbol it names (round 6: the
   round-5 step checked only moving names). Routing:
   - a file naming only moving symbols is updated to the core path (Files to
     Modify), with no alias left behind (R-2);
   - a test file naming only staying symbols keeps the plugin import unchanged
     (`rib/rib_parsefamily_test.go`, `ls_export/rfc7752_export_family_test.go`);
     a production file naming only staying symbols cannot, because its import
     is the edge this spec removes: the two are `ls_export/export_plugin.go`
     and `export_state.go` (`ls.BGPLSFamily`), which A-5 rewrites to core
     constants;
   - a file naming both must reach both packages. For BGP-LS the two package
     names differ (`ls`, `bgpls`), so the file imports both: the one such file
     is `bgp/cli/rfc8955_decode_test.go`, which names the staying
     `ls.AttrTLVsToJSON` and the moving `ls.TLVAdjacencySID`. For FlowSpec both
     packages are named `flowspec`, so the file names its family by a
     `family.Family` built from `internal/core/family` constants and drops the
     plugin import. This is the repo idiom: `flowspec-firewall` tests already
     build FlowSpec families that way (`rfc8956_zero_prefix_test.go`,
     `selected_test.go`, `value_completeness_test.go`), and the moved core tests
     do the same (A-4). The repo aliases an import only where two package names
     collide and both packages are needed (`mrtplugin`, the `ospf*` aliases);
     here the plugin would be imported only for a value core can build, and an
     alias rewrites every reference to the staying name anyway. Four files:
     `flowspec-firewall/transport_test.go` (`IPv4FlowSpec`, `IPv6FlowSpec`
     beside `ParseFlowSpec`), `numeric_ranges_test.go` (`IPv4FlowSpec`),
     `rfc8956_zero_prefix_test.go` (`IPv6FlowSpec`) and
     `bgp/message/flowspec_wire_contract_test.go` (`IPv4FlowSpecVPN` beside
     `ParseFlowSpecVPN`). After this step no file in the `flowspec-firewall` or
     `bgp/message` test binaries imports the FlowSpec plugin, so the four
     FlowSpec families are unregistered there and `family.Family.String`
     falls back to the AFI and SAFI names. No test in either package asserts
     an `ipv4/flow` or `ipv6/flow` string (grep, 2026-10-13).

| Measure | FlowSpec | BGP-LS |
|---------|----------|--------|
| Forward edges (moving names staying) under the round-3 inventory | 40 edges, 21 symbols: 3 `plugin_decode.go` helpers, its 2 value-to-name maps, 16 `kw*` keywords | 12 edges, 12 symbols: 5 `plugin.go` functions, 6 fields of its two result structs (round 4 merged the two `mtIDs` fields by name and wrote 5), `lsAttrTLV` |
| Declarations the forward closure added | 10 in `plugin_decode.go`, `protocolNameToNumber`, the keyword block (18 names); by choice, not closure, `tcpFlagNameToValue` and `fragmentFlagNameToValue` | 11 in `plugin.go` |
| Forward edges after closure | 0 | 1, `lsAttrTLV`, removed by A-4 |
| Reverse: distinct moving symbols named by a staying production declaration | 91 | 76 |
| of which unexported today, so newly exported | 33: 18 keywords, 3 name-to-value tables, 2 value-to-name tables, 3 rendering helpers, 6 component constructors, `componentBytes` | 21: `bGPLSNLRI`, `parseBGPLSWithRest`, `writeTLV`, `decodeSRCapabilities`, 4 label-range helpers, 2 JSON keys, `bgplsNLRITypeString`, 3 `parseBGPLS*TLVs`, `formatIPv6Compressed`, 6 struct fields (the 2 result structs are exported with them; round 4 wrote 5 and missed `linkDescriptorInfo.mtIDs`) |
| Round-3 exports found unneeded | - | `TLV`: `tlv` is named only by moving code |
| Plugin test files naming a moving symbol unexported today (round-4 step 6) | 8 | 12 |
| Test functions that also name an unexported staying symbol (round-4 step 6) | 1, `TestFlagNamesRoundTrip`, resolved by moving the two name-to-value tables | 1, `TestRFC7752UnknownTLVPreservedAndPropagated`; plus the two TLV 1034 tests whose result type changes (A-4) |
| Round-4 test routes naming a staying declaration the round-4 step 6 did not check (round-5 scan) | 14 references in 8 functions of 4 files routed to core: 13 to the registered family vars, 1 to `EncodeRoute` (`TestFlowSpecPreferredNumericEmission`) | 3 test-to-test edges from the 2 routed `rfc9552_test.go` functions into the staying `rfc7752_test.go`: `buildAttrTLV` (direct and through `nodeNLRIOddButLegal`) and `unknownAttrTLVType` |
| Round-5 step 6: test functions that must move, (a) non-empty and (b) empty | 5 | 18 |
| Round-5 step 6: test functions with both lists non-empty | 0 | 1, `TestRFC7752UnknownTLVPreservedAndPropagated`, rewritten (test table) |
| Round-5 step 6: test files moving whole (every function's (b) list empty) | 5 | 6 |
| Round-5 step 6: test functions moving to core, and in how many plugin files | 15, in 8 files | 55, in 9 files |
| of which name a registered family var, renamed to a core-built `family.Family` | 13 | 0 |
| of which name any other staying declaration, after routing | 0 | 0 |
| Test helpers with users on both sides, declared in each package | 0 | 2: `buildAttrTLV`, `unknownAttrTLVType` (plugin `rfc7752_test.go`; used by the staying `rfc7752_test.go`, `rfc9514_reserved_test.go`, `rfc9552_attr_flags_test.go` and `rfc9552_test.go` functions) |
| Files outside the package naming a moving symbol, beyond the 35 edges | production `internal/chaos/peer/sender.go`; 12 test files: 9 in `internal/plugins/flowspec-firewall/`, `bgp/plugins/cmd/update/update_text_test.go`, `bgp/message/flowspec_wire_contract_test.go`, `bgp/format/json_test.go` | production `internal/le/interoplab/bgp/check_ospf_bgpls.go` (`AFIBGPLS`, `ProtoOSPFv2`, `SAFIBGPLinkState`); test `bgp/cli/rfc8955_decode_test.go` (`TLVAdjacencySID`) |
| Outside files naming a staying family var beside a moving symbol (round-6 step 7) | 4 test files, listed in step 7; each builds its family from core constants | 0 (the production `ls_export/export_plugin.go` and `export_state.go` name only `ls.BGPLSFamily`, rewritten by A-5) |

Plugin test files, by route:

| Route | FlowSpec | BGP-LS |
|-------|----------|--------|
| Moves to the core package whole: no test function names a staying declaration (step 6) | `compare_rfc8955_test.go`, `rfc5575_arrival_order_test.go`, `rfc8955_vpn_test.go`, `rfc8956_fragment_test.go`, `types_fuzz_test.go`; every use of a registered family var (13 functions) becomes a `family.Family` built from core constants | `rfc9552_node_descriptor_unique_test.go`, `rfc9552_ordering_test.go`, `rfc9552_types_descriptor_key_test.go`, `srv6_sid_descriptor_walk_test.go`, `types_fuzz_test.go`, `types_test.go` |
| Split by test function: a function that must move moves, the rest stay | `rfc8955_width_test.go`: `TestFlowSpecPreferredNumericWideReception` (names `numericComponent`) moves, `TestFlowSpecPreferredNumericEmission` stays (calls the staying `encode.go` `EncodeRoute`, and reaches the component constructors through core exports); `rfc8956_prefix_test.go`: `TestIPv6FlowPrefixPatternAndPadding` (names `prefixComponent`) moves, `TestIPv6FlowPrefixRejectsMalformedBounds` stays (calls the staying `buildFlowSpecComponents`); `vocabulary_test.go`: `TestFlagNamesRoundTrip` (names `tcpFlagsToString`) and `TestProtocolNameReverseAgrees` (names `protocolNumberToName`) move, the three `TestConfig*` functions and `TestTrafficClassIsOneComponentInBothPaths` stay (they name config parsers) | `attr_test.go`: `TestDescriptorBGPRouterID`, `TestDescriptorConfedMember` (name `parseNodeDescriptorTLVs`), `TestDescriptorSRv6SID` (names `parseBGPLS`) move; `plugin_test.go`: `TestFormatRouterID` moves; `rfc9552_test.go`: `TestRFC9552NLRIContentsNeverMakeItMalformed` with its helper `nodeNLRIOddButLegal`, and `TestRFC9552NLRIFramingErrorsRejected` move (both name `parseBGPLS`), while `TestRFC9552ISISMTIDReservedBitsIgnoredOnReceipt` stays (it names the newly exported `ParseBGPLS*TLVs`); the two moving functions use the `rfc7752_test.go` helpers `buildAttrTLV` and `unknownAttrTLVType`, which staying tests also use, so the moved core test file declares its own copy of each, naming the plugin copy as its source, and the plugin keeps its own |
| Stays, naming core exports | `asn_notation_test.go`, `component_parity_test.go`, `config_test.go`, `config_validation_test.go`, `decode_scanner_error_test.go`, `plugin_test.go`, `rfc8955_encode_test.go`, `types_test.go` | `attr_fuzz_test.go`, `decode_scanner_error_test.go`, `json_test.go`, `rfc8571_attr_reserved_test.go`, `rfc9086_rsvd_receive_test.go`, `rfc9514_reserved_test.go`, `rfc9552_attr_flags_test.go`, `unknown_type_skip_test.go`; in `rfc7752_test.go`, `TestRFC7752UnknownTLVPreservedAndPropagated`, which names both the moving `parseBGPLS` and the staying `decodeAllAttrTLVs`, `AttrTLVsToJSON`, `genericTLVKey`, is rewritten in place to parse through the exported `ParseBGPLSWithRest` and assert the rest is empty |
| Stays, reaching TLV 1034 through the plugin's registered decoder (A-4) | - | `attr_test.go` `TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags` and `rfc9085_capabilities_receipt_test.go` `TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`: each calls the decoder `lookupLsAttrTLVDecoder(TLVSRCapabilities)` returns, so the assertion on `lsAttrTLV` stays valid, the RFC-tagged file stays whole beside its TLV 1036 rows, and the tests also prove the `register_attr.go` adapter |

`rfc/discrimination/` records whose producer moves are re-recorded with
`./le rfc discriminate-record` in the phase that moves it (R-3). D1, 14 records:
`rfc5575.json` 2 on `compare.go::Compare`, `rfc8955.json` 8 on
`types_numeric.go::writeToAFI` and 4 on `types_vpn.go::ParseFlowSpecVPN`. D2, 9
records: `rfc9085.json` 4 on `attr_node.go::decodeSRCapabilities` (RFC9085-2.1.2-1
and -2, each with the two units above), `rfc7752.json` 1 and `rfc9552.json` 1 on
`types_descriptor.go::addressTLVs`, `rfc9552.json` 3 on `types.go`
(`parseNodeDescriptorTLVsAt`, `writeTLV`, `WriteTLVBytes`). A record whose unit
moves to a core test file is re-recorded in the same step; of the 23, the units
of `rfc5575.json` 2, `rfc8955.json` 4 (`rfc8955_vpn_test.go`) and `rfc9552.json`
and `rfc7752.json` 5 (`rfc9552_node_descriptor_unique_test.go`,
`rfc9552_ordering_test.go` `TestNoDescriptorEmitsADescendingTLVSequence`,
`rfc9552_types_descriptor_key_test.go` `TestSameNodeHasOneKeyWhateverTheStorageOrder`)
move with their files.

The 23 are not the whole obligation (design review round 5). A tagged unit
whose file moves to core, or that moves alone out of a split file, is a new
path in the commit, and no move here is byte-pure (the package clause changes,
and 13 FlowSpec functions rename a family var), so `./le rfc check`
(`checkDiscriminationRatchet`) reads each cover as ADDED by the D1 or D2 commit
and refuses one without a record. 27 moving tagged covers carry no record today,
all on gated MUST-level requirements, and one staying unit changes behavior
(`ai/rules/rfc-compliance.md`: a tagged unit whose behavior changes owes a
record). Each gets a fresh record with `./le rfc discriminate-record`, in the
same phase commit as its move:

| Phase | Test file (plugin today) | Tagged unit | Covers (requirement, polarity) | Count |
|-------|--------------------------|-------------|--------------------------------|-------|
| D1 | `compare_rfc8955_test.go` (moves whole) | `TestFlowSpecPrecedenceFromWire` | RFC8955-5.1-1 +/-, RFC8955-5.1-2 +/-, RFC5575-5.1-1 +/- | 6 |
| D1 | `compare_rfc8955_test.go` | `TestFlowSpecNumericReservedAndEightOctetOperand` | RFC8955-4.2.1.1-3 +/- | 2 |
| D1 | `rfc8956_fragment_test.go` (moves whole) | `TestRFC8956FragmentSingleOctet` | RFC8956-3.6-1 +/- | 2 |
| D1 | `rfc8956_fragment_test.go` | `TestRFC8956FragmentEncodingRespectsEnclosingFamily` | RFC8956-3.6-2 +/- | 2 |
| D1 | `rfc8956_fragment_test.go` | `TestRFC8956FragmentDecodeIgnoresReservedBits` | RFC8956-3.6-3 +/- | 2 |
| D1 | `rfc8956_prefix_test.go` (split) | `TestIPv6FlowPrefixPatternAndPadding` | RFC8956-3.1-1 +/-, RFC8956-3.1-2 +/- | 4 |
| D2 | `rfc9552_ordering_test.go` (moves whole) | `TestLinkDescriptorOrdersMixedFamilyAddressesAscending` | RFC9552-5.1-1 +, RFC7752-3.1-2 + | 2 |
| D2 | `rfc9552_types_descriptor_key_test.go` (moves whole) | `TestNodeDescriptorEncodesLegalZeroKeyFields` | RFC9552-5.2.1.1-2 + | 1 |
| D2 | `rfc9552_types_descriptor_key_test.go` | `TestNodeDescriptorKeepsBackboneDistinctFromAreaLess` | RFC9552-5.2.1.1-2 - | 1 |
| D2 | `rfc9552_types_descriptor_key_test.go` | `TestNodeDescriptorWriteToMatchesBytes` | RFC9552-5.2.1.1-1 - | 1 |
| D2 | `rfc9552_test.go` (split) | `TestRFC9552NLRIContentsNeverMakeItMalformed` | RFC9552-8.2.2-1 +, RFC9552-8.2.2-3 + | 2 |
| D2 | `rfc9552_test.go` | `TestRFC9552NLRIFramingErrorsRejected` | RFC9552-8.2.2-1 -, RFC9552-8.2.2-3 - | 2 |
| D2 | `rfc7752_test.go` (stays, rewritten to `ParseBGPLSWithRest`) | `TestRFC7752UnknownTLVPreservedAndPropagated` | RFC7752-3.1-1 +, RFC9552-5.1-3 + | 2 |

A third set goes stale with neither a move nor a behavior change (design review
round 6). The record fingerprints the text of its unit and its producer through
`behaviorBytes` (`internal/le/rfc/tags.go`), which strips only comments and
whitespace. So the gate's rule is: any record whose unit or producer text
changes is re-recorded, and a staying function that reaches a moved symbol
through a new package qualifier (`ErrBGPLSTruncated` becoming
`bgpls.ErrBGPLSTruncated`, `ls.NewBGPLSLink` becoming `bgpls.NewBGPLSLink`), a
renamed export, or a rewritten family var has changed text. The core FlowSpec
package keeps the name `flowspec`, so an outside FlowSpec caller that names only
moving symbols keeps its text; the core BGP-LS package is `bgpls`, so every
outside `ls.` reference to a moved symbol changes. Found by reading the unit and
producer body of every record in the eight touched record files against the
moving set; 88 records, re-recorded in the same phase commit:

| Phase | Record file | Count | Unit or producer whose text changes |
|-------|-------------|-------|-------------------------------------|
| D1 | `rfc8956.json` | 1 | unit `flowspec-firewall/rfc8956_zero_prefix_test.go` `TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress` (RFC8956-3.1-3): `flowspec.IPv6FlowSpec` becomes a core-built family (step 7) |
| D2 | `rfc9514.json` | 14 (all its plugin records) | producers `decodeSRv6Capabilities`, `decodeSRv6EndXSID`, `decodeSRv6Locator`, `decodeSRv6EndpointBehavior`, `decodeSRv6BGPPeerNodeSID` name `ErrBGPLSTruncated`; their units name `TLV*` constants |
| D2 | `rfc9552.json` | 7 | producers `decodeNodeFlagBits`, `decodeMPLSProtocolMask`, `decodeIGPFlags` (2 each) and `iterateAttrTLVs` (RFC9552-8.2.2-10) name `ErrBGPLSTruncated`; units name `TLV*` and `jsonKeyReserved` |
| D2 | `rfc9085.json` | 4 | `decodeSRLocalBlock` (RFC9085-2.1.4-1, -2; names `decodeSrLabelRanges`), `decodeAdjacencySID` (2.2.1-1), `decodePrefixSID` (2.3.1-1) |
| D2 | `rfc9086.json` | 2 | units `attr_test.go` `TestRFC9086PeerSIDIgnoresReservedFields` and `rfc9086_rsvd_receive_test.go` `TestRFC9086PeerSIDRsvdBitsIgnoredWhenReceived` (RFC9086-5-3) name `TLVPeer*` |
| D2 | `rfc9552.json` 30, `rfc9086.json` 13, `rfc9514.json` 7, `rfc7752.json` 6, `rfc9085.json` 4 | 60 | outside producers naming `ls.`: `ls_export/export_encode.go` `encodeNativeLink` (20), `encodeTopology` (23), `originateAttributes` (2), `exportNodeID` (1); `export_epe.go` `validateNativeEPE` (3); `epe/epe_source.go` `publishLocked` (9); `export_state.go` `send` (2; it names only `ls.BGPLSFamily`, which A-5 replaces with core constants) |

Totals per phase. D1: 15 re-records (14 producer moves, 1 changed unit) and 18
new records. D2: 96 re-records (9 producer moves, 27 changed plugin units or
producers, 60 changed outside producers) and 11 new records (9 moved, 2
rewritten). A record whose unit and producer text are both unchanged owes
no re-record. Every other tagged unit in the two plugins stays at its path, but
"stays" does not mean "unchanged": many staying units change text (the approval
table below), and an outside FlowSpec caller (`rib`, `flowspec-firewall`
production) keeps its text only because it reaches the core package under the
same name.

A tagged unit with no record whose text changes owes no record:
`discriminationChangedUnits` (`internal/le/rfc/check_ratchets.go`) only counts
it, and its comment records the owner's 2026-08-31 decision to publish the count
and "enforce nothing yet". These units do raise that published "changed" count:
up to 27 covers in 16 units for D1, and up to 20 covers in 10 units for D2
(round-7 count; the unit counts are re-derived from the approval table less the
units that carry a record). The counts are upper bounds because
`discriminationChangedUnits` also skips a cover whose RID is not gated, and
this count did not check gating.

**Owner approval of changed tagged units (design review round 7).** Two gates
refuse a change to an RFC-tagged unit whatever records exist, both through
`rfc.ChangedTags`: `rfcChangeProblems` (`internal/le/commit/rfcchange.go`,
called from `prepare.go` `checkSourceGates`) refuses `./le commit create` unless
each changed unit has a row written by `./le rfc approve` or the commit sets
`rfc-change-ok`, and `proposedRFCChanges` (`internal/le/test/weakened/proposed.go`)
backs the Write/Edit hook. A qualifier-only text change counts, and so does a
function removed from a split file. The `<package>` prefix of each unit is
the directory name of the path the gate judges (`changedRFCUnits`,
`internal/le/commit/rfcchange.go`): the new path for a paired rename, else the
old path. Units that need approval:

| Phase | In place (text changes) | Moved |
|-------|-------------------------|-------|
| D1, 27 | 18: FlowSpec `types_test.go` 14; `rfc8955_width_test.go` `TestFlowSpecPreferredNumericEmission`; `rfc8956_prefix_test.go` `TestIPv6FlowPrefixRejectsMalformedBounds`; `flowspec-firewall/rfc8956_zero_prefix_test.go` `TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress`; `bgp/message/flowspec_wire_contract_test.go` `TestFlowSpecVPNUpdateCountsRouteDistinguisher`; prefixes `flowspec.`, `flowspec-firewall.`, `message.` | 9: the 8 tagged units of the four whole-file moves (`compare_rfc8955_test.go` 2, `rfc5575_arrival_order_test.go` 1, `rfc8955_vpn_test.go` 2, `rfc8956_fragment_test.go` 3) and `TestIPv6FlowPrefixPatternAndPadding`; prefix `flowspec.` (the core directory is also `flowspec`); the 8 count whether or not git pairs the files as renames, because each names a FlowSpec family var and the moved test builds that family from core constants |
| D2, 27 (35 if the whole-file moves are not paired as renames) | 25: `rfc7752_test.go` 7, `rfc9514_reserved_test.go` 5, `rfc9552_attr_flags_test.go` 5, `rfc9552_test.go` 4, `attr_test.go` 2, `rfc9085_capabilities_receipt_test.go` 1, `rfc9086_rsvd_receive_test.go` 1; prefix `ls.` | 2 split from `rfc9552_test.go`; plus up to 8, the tagged units of the three whole-file moves (`rfc9552_node_descriptor_unique_test.go` 2, `rfc9552_ordering_test.go` 2, `rfc9552_types_descriptor_key_test.go` 4), unless `ProspectiveCommit` (`--find-renames=1%`) pairs each file as a rename; the 8 are conditional because none of them names `BGPLSFamily` or `BGPLSVPNFamily`, so a paired move can keep their text; prefix `ls.`, or `bgpls.` only for a paired rename |

Before the D1 commit and again before the D2 commit, the owner rules on these
units, and each ruling is recorded with one `./le rfc approve unit
<package>.<TestName> reason "<owner's words>"` per unit. The reason quotes the
owner; it is never the author's own argument that the change is fine, and no
agent runs the command before the owner has ruled.

The command writes `tmp/commit-rfc-approved-<session>.md` (`rfc.ApprovalPath`,
`internal/le/rfc/approve.go`). `<session>` is the commit namespace that
`CommitSession` (`internal/le/le/path/commitsession.go`) derives from
`CLAUDE_CODE_SESSION_ID` (`ResolveSession`, `internal/le/le/path/session.go`),
so the file belongs to the session that runs the command:

- The implementing agent runs the command after the owner rules.
- If the permission classifier refuses the agent, the owner runs it with
  `! <command>` at the implementing session's own prompt, so it inherits that
  session id. A command from a separate terminal writes another session's file,
  and the gate keeps refusing.
- Before the first D1 or D2 edit, the agent confirms the rows are in the file
  that the command's `path` field reports.
- The commit script drops the rows a commit used once that commit lands
  (`RFCApprovals.Dropped`), so this is done once per phase.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/module-tiers.md` - "Plugin import ownership"
  → Constraint: owner = first non-schema `register.go` on the ancestry under a `plugins` root; `bgp/reactor/filter` is a nested standalone root and owns its subtree; top-level component roots (bfd, iface, mtu, ping, traceroute) are host infrastructure whose shared API stays a valid dependency, while a registered descendant owns its implementation.
  → Constraint: the only blank-import exemptions are `cmd/ze`'s main package, the import-only `internal/component/plugin/all` files, and schema-to-schema (`yang` to `yang`). `cmd/ze/hub` is NOT `cmd/ze`'s main package, so its edge is not a registration edge.
  → Decision: no baseline, no new exemption. Every edge is removed at its source.
- [ ] `ai/rules/plugins.md`
  → Constraint: a plugin owns its entire feature surface; every RPC carries YANG in its owner; no switch/case dispatch.
  → Constraint: a cross-boundary payload is a self-contained value type.
- [ ] `ai/rules/architecture.md`
  → Constraint: a package with no config lifecycle and no registry side effect is a core library (`internal/core/`). A family registration (`family.MustRegister`) is a registry side effect, so it MUST stay in the plugin, never move to core.
  → Constraint: a compile-out-able feature reaches always-on code only through a registry or a seam.
- [ ] `docs/architecture/command-ownership.md` - "YANG as Data, Not Code", owner schema placement, "Dedicated feature modules"
  → Constraint: a command schema lives at `<owner>/yang/ze-<x>-cmd.yang` and is container-merged; the codegen (`./le repo generate`) produces `embed.go`, `register.go` and the `all.go` row.
  → Decision: `mtu`, `ping`, `traceroute` are dedicated feature modules under `internal/component/<x>`, so their command schema belongs under that module, not in a separate `internal/plugins/<x>-cmd`.
- [ ] `docs/architecture/core-design.md` - registration pattern, small core
  → Constraint: core discovers components and plugins through registries, never imports them.
- [ ] `docs/architecture/plugin/plugin-system.md` - "Cross-boundary value types"
  → Constraint: a type that moves to core to be shared MUST hold no pointer into data a plugin owns.
- [ ] `feature-gates.txt` rows `ze_bgp` and `ze_bfd`
  → Constraint: `internal/component/bfd/api` and `bfd/packet` are the always-on client contract of the gated BFD engine; consumers nil-check. A new BFD seam goes there.
  → Constraint: the whole BGP subtree and `flowspec-firewall` are dropped transitively under `ze_bgp`; a new `internal/core/bgp/...` package is linked only when imported, so it does not widen the always-on binary.

### RFC Summaries (Scope: protocol)
- N-A: no protocol behavior changes. Code that moves keeps its RFC comments verbatim (`rfc/short/rfc8955.md`, `rfc9552.md`, `rfc9085.md`, `rfc9086.md`, `rfc9514.md`, `rfc9830.md` stay the cited summaries).

**Key insights:**
- 15 of 35 edges are blank imports the generated composition already carries (`all.go` and `all_ze_bgp.go` blank-import each of their packages). Deleting them changes nothing a shipped binary links.
- The `config/yang/cli` comment "not reachable through plugin/all (import cycles, see plugin_imports.go rpcDirs)" is stale: since `5398df0a23` (2026-06-04) the generator auto-discovers RPC packages and `tree.go` already imports `plugin/all`.
- Nothing on the per-UPDATE path changes its call shape: the RIB FlowSpec validator keeps a direct call into the same code at a new import path; the loop filter keeps its atomic-pointer metrics load.
- `pool-stats` names its pools from a hand list of 13 while `pool.AllPools()` returns 14 (`RibOut`), so the 14th row prints `unknown` today. Route G removes the second declaration.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/tier/ownership.go` - `pluginOwner`, `compositionRegistration`, `schemaDependency`, `pluginNonProduction`, `pluginOwnershipGate`
- [ ] `internal/component/bgp/reactor/reactor.go` - the import block's six blank RPC imports plus `filter`; `filter.SetMetricsRegistry(r.metricsRegistry)` in the metrics-enable block beside `filterapi.SetMetricsRegistry`
- [ ] `internal/component/bgp/reactor/filter/register.go`, `loop_metrics.go` - registers `loop` through `filterapi.Register` and `registry.Register`; metrics held in an `atomic.Pointer`, nil until set
- [ ] `internal/component/bgp/filterapi/metrics.go` - `SetMetricsRegistry` builds the attr-mod counters
- [ ] `internal/component/config/yang/cli/tree.go`, `tree_bgp.go`, `tree_bfd.go` - `tree.go` imports `plugin/all`; `tree_bgp.go` (`//go:build ze_bgp`) only blank-imports five RPC packages
- [ ] `internal/component/plugin/all/all.go`, `all_ze_bgp.go` - generated; already blank-import every package in edges 1-6, 8-16 and the three `*-cmd/yang` packages
- [ ] `internal/component/{firewall,iface,traffic}/cli/main.go`, `internal/plugins/init/main.go` - blank import a backend so `LoadBackend("netlink"|"nft")` resolves
- [ ] `internal/component/{mtu,ping,traceroute}/cmd/doc.go` and `internal/plugins/{mtu,ping,traceroute}-cmd/yang/` - the `-cmd` directories hold only a `yang/` package (module, `embed.go`, `register.go`, `cmd_schema_test.go`); `mtu` already has `internal/component/mtu/yang` for its config module
- [ ] `internal/component/bgp/plugins/nlri/ls/` - `types*.go`, `attr*.go` import only `internal/core/{bgp/nlri,bgp/wire,family,textbuf}`; `types.go` `BGPLSFamily` and `BGPLSVPNFamily` call `family.MustRegister`; `register_attr.go` fills the TLV-decoder registry `registerLsAttrTLV`
- [ ] `internal/component/bgp/plugins/nlri/flowspec/` - `types*.go`, `compare.go` import only core; `types.go` the four FlowSpec family vars call `family.MustRegister`
- [ ] `internal/component/bgp/plugins/rib/rib_flowspec_validation.go` - parses each received FlowSpec NLRI with `flowspec.ParseFlowSpec`/`ParseFlowSpecVPN` (RFC 8955 Section 6 validation)
- [ ] `internal/component/bgp/cli/decode_update.go` - attribute switch, case 29 calls `ls.AttrTLVsToJSON`; `internal/component/bgp/message/rfc7606_bgpls.go` documents that this is its only non-test caller
- [ ] `internal/component/bgp/plugins/cmd/update/update_text_srpolicy.go`, `update_text_nlri.go` - SAFI 73 branch runs a second SR-Policy grammar and calls `srpolicy.New`; `parseRegistryNLRISection`/`encodeViaRegistry` already serve other plugin families through `registry.EncodeNLRIByFamily`
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/register.go`, `encode.go` - registers `InProcessNLRIEncoder: EncodeNLRIHex`, which parses the same three fields and validates the endpoint against the AFI
- [ ] `internal/component/bgp/plugins/cmd/rib/pool_stats.go`, `cmd/rib/yang/ze-rib-poolstats-cmd.yang`, `rib/pool/attributes.go` - in-process reader of `pool.AllPools()`, names from a parallel 13-entry array
- [ ] `internal/component/bfd/bfd.go` (the two `diagCmd.SetBFDRawCaptureProvider` calls: `nil` on stop, `svc` on start; re-read 2026-10-13); `internal/plugins/diag/cmd/capture_raw.go` `BFDRawCaptureProvider` and `SetBFDRawCaptureProvider` - BFD pushes itself into a package variable owned by the diag plugin
- [ ] `internal/component/bfd/api/registry.go`, `profile_check.go` - the existing nil-able seams (`SetService`/`GetService`, `SetProfileChecker`)
- [ ] `cmd/ze/hub/main.go` `runYANGConfig` (its `harvestKernelCrashes` call), `main_system.go` `harvestKernelCrashes`; `internal/plugins/crashes/readiness.go` - `HarvestAtBoot` runs once at daemon start, logs, publishes two gauges; crashes registers no engine, only an offline fallback, doctor checks and RPCs

**Behavior to preserve:**
- Every RPC, YANG module, CLI command, backend and filter stays registered in the shipped `ze` binary under every feature-gate combination it is registered in today.
- Wire bytes of BGP-LS, FlowSpec and SR-Policy NLRI and the BGP-LS export are byte-identical; existing `.ci` encode/decode tests unchanged.
- `ze bgp decode` prints the same `bgp-ls` attribute JSON.
- `update text ... nlri ipv4/sr-policy ...` produces the same NLRI bytes as `test/encode/bgp-srpolicy-{1,2}-encode.ci` expect.
- `ze_bgp_as_path_loop_detected_total` and the attr-mod counters appear when metrics are enabled.
- `capture-raw` for BFD works when BFD runs and answers its existing error when it does not.
- Crash harvest runs exactly once at daemon start, with the same log lines and gauges.

**Behavior to change:**
- `pool-stats` names its 14th pool instead of printing `unknown` (route G).
- The SR-Policy text grammar in `update text` becomes the one the plugin registers. Every changed input and its outcome is in AC-11b to AC-11f; the inputs accepted today that become refused for a reason other than already being invalid are the three Q-2 lists: endpoint/AFI mismatch, a `path-information` that applies to an SR-Policy section (top level or inside it; Q-A), and a later `add`/`del` in an SR-Policy section. A later `add`/`del` in a mup or mvpn section is refused today by their encoders and after F by the shared parser: same outcome, new message (A-8).
- `crashes` becomes a value the `plugin ... use` completion offers (`InternalPluginNameValidator` completes from `registry.Names()`), and `ze plugin crashes` exists as a no-op that exits 0, because `registry.Register` refuses a registration without a `CLIHandler` (route I, Q-B). Same exposure as `loop` and `bgp-nlri-srpolicy` today; the class is journaled, not fixed here.
- The plugin listing gains one `crashes` row: `plugins.snapshot`, `docs/features/plugins.md`, and the `docs/DESIGN.md` "Shipped Plugins" table with its registered-plugin count, which the yangcontract drift gate now checks (AC-16).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build time: Go import graph scanned by `./le arch tier check`.
- Run time, per route: daemon start (A, C, H, I), RIB receive of a FlowSpec UPDATE (D), BGP-LS export on IGP change (D), FlowSpec to firewall translation (D), `ze bgp decode` (E), `update text` command (F), `pool-stats` RPC (G), `capture-raw` RPC (H).

### Transformation Path
1. A: `init()` in each RPC/backend package runs because `plugin/all` blank-imports it, not because the host does.
2. C: reactor start calls `filterapi.SetMetricsRegistry(reg)`, which builds its own counters and then calls the metrics hook each registered `filterapi.Filter` declared; the loop filter's hook stores its counter.
3. D: callers import `internal/core/bgp/nlri/bgpls` or `internal/core/bgp/nlri/flowspec` directly; the plugins import the same packages and keep family registration, config, JSON/text and hex codecs.
4. E: decode looks up a JSON decoder registered for attribute code 29 by the `bgp-nlri-ls` plugin; no registration means the attribute prints as today's unknown-attribute fallback.
5. F: SAFI 73 falls through to `parseRegistryNLRISection`, which calls the SR-Policy plugin's registered `EncodeNLRIHex`. Before it reads `add`/`del` and before it hands the run to the encoder, that parser asks the registry whether the section's family declared no ADD-PATH, and refuses on the three positions AC-11d lists.
6. G: the rib owner registers `ze-bgp:pool-stats` and reads its own pools with names declared beside them.
7. H: BFD calls `bfdapi.SetRawCaptureProvider`; diag reads `bfdapi.RawCaptureProvider()` and nil-checks.
8. I: crashes registers a boot effect; hub runs the registered effects where it calls `harvestKernelCrashes` now.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ loop filter | `filterapi.Filter` metrics hook, startup only | No |
| RIB / flowspec-firewall ↔ FlowSpec plugin | shared core value types, direct calls | No |
| epe / ls_export ↔ BGP-LS plugin | shared core value types, direct calls | No |
| Offline decoder ↔ BGP-LS plugin | registry lookup by attribute code | No |
| `update` command ↔ SR-Policy plugin | `registry.EncodeNLRIByFamily` | No |
| BFD component ↔ diag plugin | `bfd/api` seam | No |
| hub ↔ crashes plugin | registered boot effect | No |

### Integration Points
- `filterapi.Filter` (new optional field), `filterapi.SetMetricsRegistry`.
- Plugin `registry.Registration` (attribute JSON decoder field, route E).
- `bfd/api` (new seam beside `SetProfileChecker`).
- A boot-effect field on `registry.Registration` (route I), run once by hub between `applyHostTuning` and `startSmartManager`.
- `./le repo generate` for the moved schema and handler packages.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound: the SAFI 73 branch in `update_text_nlri.go` and the `case 29` arm in `decode_update.go` are removed, and the pool name list is derived | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every production binary that links the reactor, `config/yang/cli`, the three CLIs or `plugins/init` also links `plugin/all` | `cmd/ze/ze_core_dispatch.go` imports both `plugin/all` and the CLI packages; `ze-installer` and `ze-serial-shell` import none of them (grep 2026-10-12) | a binary loses RPCs or a backend silently | `go list -deps` of each `cmd/*` main, recorded in the spec | unvalidated |
| A-2 | Unit tests in `bgp/reactor`, `config/yang/cli`, `firewall/cli`, `iface/cli`, `traffic/cli`, `plugins/init`, `mtu/ping/traceroute/cmd` that need the deleted registrations can import them from `_test.go` files (non-production, gate-exempt) | `pluginNonProduction` exempts `_test.go` | tests go red after deletion | package tests of each touched package under `./le job run` | unvalidated |
| A-3 | Byte identity does NOT hold for every input: the two grammars differ on `path-information`, on `rd`/`label`, on a later `add`/`del`, and on endpoint/AFI mismatch. AC-11 lists each changed input and its outcome; every other input the `update` copy accepts builds the same bytes through the plugin encoder | independent review 2026-10-12; read of `update_text_srpolicy.go` and `srpolicy/encode.go` | an input accepted today changes bytes silently | `TestSRPolicyTextMatchesPluginGrammar` over AC-11's table; `bgp-srpolicy-{1,2}-encode.ci` green unchanged | unvalidated |
| A-4 | The Route D inventory moves to core without moving any `family.MustRegister` call or any registry: the BGP-LS attribute TLV registry (`lsAttrTLVRegistry`, `registerLsAttrTLV`, `lookupLsAttrTLVDecoder`, `decodeAllAttrTLVs`) stays whole in the plugin. Core holds no registry and no package-level map written from `init()`; what the plugin calls into core is plain functions, types and constants, exactly the "Newly exported by core" column of the inventory (FlowSpec: 33 names; BGP-LS: 21 names plus the two `plugin.go` result structs; both closed by the round-4 scan and corrected by the round-5 scan the inventory records). Core `DecodeSRCapabilities` returns `(*LsSRCapabilities, error)`; `register_attr.go` registers a closure adapting it to `lsAttrTLVDecoder`, and on error that closure returns a nil `lsAttrTLV` interface with the error, never the typed nil pointer wrapped in a non-nil interface. The two plugin tests that call `decodeSRCapabilities` and assert the `lsAttrTLV` result (`attr_test.go` `TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`, the RFC-tagged `rfc9085_capabilities_receipt_test.go` `TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`) stay in the plugin and call the decoder `lookupLsAttrTLVDecoder(TLVSRCapabilities)` returns, so the tagged file is not split and the adapter is under test | declaration scan 2026-10-12; moving files import only core | the move drags a registry into core | `./le arch tier check` core-direction and placement gates; grep core packages for `MustRegister` and package-level maps written from `init()` | unvalidated |
| A-5 | `ls_export` can name its families from `internal/core/family` constants (`AFIBGPLS`, `SAFIBGPLinkState`, `SAFIBGPLinkStateVPN`) instead of the plugin's registered vars | `family.go` declares the AFI name; plugin registers the name pair | export path needs a registered `Family` value | `bgpls-export-lifecycle.ci` green | unvalidated |
| A-6 | The BFD raw capture interface returns `plugin.BGPRawCaptureEntry`, a component/plugin type `bfd/api` may import without a cycle | `capture_raw.go` `BFDRawCaptureProvider.BFDRawCaptureSnapshot` | import cycle | independent design review 2026-10-12 | validated |
| A-8 | `add`/`del` must come first in a REGISTRY family's section, as `parseRegistryNLRISection` already requires; a later `add`/`del` is refused by that shared parser with an error saying to write one `add` or `del` right after the family. The registry families are the ones `nativeTextFamily` answers false for: today mup and mvpn, and SR-Policy once F removes its `SAFISRPolicy` case from `nativeTextFamily`. For mup and mvpn no accepted input is lost (a later `add`/`del` already reaches their encoders, which refuse it); only the message changes. The native families, unicast, multicast, vpn, labeled, flowspec, flowspec-vpn, vpls and evpn, keep their own grammar, in which a later `add`/`del` switches mode (`add P1 del P2`, the `add`/`del` mode switch in `parseNLRISection`); vpn and labeled register an `InProcessNLRIEncoder` but are parsed natively and call it per prefix | `update_text_nlri.go` `nativeTextFamily`, `parseRegistryNLRISection`, the mode switch in `parseNLRISection`; its copy in `update_text_srpolicy.go` `parseSRPolicySection`; mup, mvpn encoders | a registry family accepts a later `add`/`del` as a value today, or a native family is routed through the registry parser | grep of the mup and mvpn `EncodeNLRIHex` argument parsers; `TestRegistrySectionRefusesLaterAddDel`; native-family unit tests green unchanged | unvalidated |
| A-9 | `crashes` holds no `registry.Registration` today (only `RegisterRPCs`, an offline fallback and doctor checks); route I adds one, and `registry.Register` requires a non-nil `RunEngine`, so it carries the no-op engine `loop` and `bgp-nlri-srpolicy` already carry. `registry.Register` also requires a non-nil `CLIHandler` (`registry.go` `Register`), so it carries the no-op handler `loop` carries (`reactor/filter/register.go` `init`). The server never auto-loads it: auto-load starts a plugin only for a configured family, an owned config root, a custom event or send type, a dependency, or as the data-plane writer a `NeedsDataPlane` plugin requires (`server/startup_autoload.go` `appendDataPlaneWriter`), and the registration declares none of these (no `ProgramsFIB`, no `DataPlane`). The boot effect runs from hub, never from the engine. It is visible in the plugin listing (`plugins.snapshot`, `docs/features/plugins.md`, the `docs/DESIGN.md` "Shipped Plugins" table) exactly as `loop` is, with a description saying it is crash capture and needs no configuration. The exposure IS widened by one name: `InternalPluginNameValidator` completes from `registry.Names()` (`config/validators.go` `sortedInternalPluginNames`), so `plugin ... use` completion offers `crashes`, naming it in a `plugin` block starts an engine that exits 0 at once, and `ze plugin crashes` is a no-op that exits 0. `loop` and `bgp-nlri-srpolicy` carry the same exposure today. Owner decision Q-B (2026-10-12): accept, list it under "Behavior to change", one row in `plan/journal/inert-registration-offered-as-a-choice.md` | grep of `internal/plugins/crashes`, `registry.go`, `startup_autoload.go`, `config/validators.go` 2026-10-12 | an engine starts that nobody configured | AC-16; `TestAutoloadStartsNoCrashesEngine` | unvalidated |
| A-7 | Moving the three `-cmd` YANG modules changes no module name, namespace or prefix, so `ze-mtu-cmd`, `ze-ping-cmd`, `ze-traceroute-cmd` stay byte-identical and their imports still resolve | `yang/glue` keys by module, not directory | YANG load failure | `TestYANGImportsFollowGoImports`, `cmd_schema_test.go` moved with the module | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deleting a blank import drops a registration in some build-tag combination | `./le build` matrix or feature-gate check red; `all_test.go` registration counts change | the generated composition is the only place a registration is pinned; any missing row is a generator defect fixed there |
| R-2 | Core move of `nlri/ls` and `nlri/flowspec` leaves a re-export alias in the plugin (layering) | grep for type aliases or wrapper funcs in the plugins | `ai/rules/no-layering.md`: move, update every caller, no alias |
| R-3 | RFC-tagged tests and discrimination records name the old package paths | `./le rfc check` red | route each test as the inventory's test table says. The rule is that the split follows the code: a tagged unit goes where step 6 of the inventory method routes it, so a tagged file may be split across the two packages (`compare_rfc8955_test.go`, `rfc5575_arrival_order_test.go`, `rfc8955_vpn_test.go`, `rfc8956_fragment_test.go`, `rfc9552_node_descriptor_unique_test.go`, `rfc9552_ordering_test.go`, `rfc9552_types_descriptor_key_test.go` move whole; `rfc8956_prefix_test.go`, `rfc8955_width_test.go`, `rfc9552_test.go`, `attr_test.go` are split), and every moved tagged unit is re-recorded. With `./le rfc discriminate-record`, in the phase commit that moves it: every record whose producer moves (D1 14, D2 9), every record whose unit or producer text changes without a move, because the gate fingerprints that text with only comments and whitespace stripped (D1 1, D2 87), and a fresh record for every moved tagged cover that has none and for the one rewritten unit (D1 18, D2 9 + 2), both listed under the inventory. `rfc9085_capabilities_receipt_test.go` stays whole, reaching TLV 1034 through the registered decoder (A-4) |
| R-4 | Doc source anchors (`<!-- source: -->`, `// Design:`) point at moved files | `./le spec citation anchors spec plan/pre-release/spec-plugin-ownership-imports.md` | update every anchor in the phase that moves the file |
| R-5 | Filter metrics hook runs before the filter registered (init order) | loop counter absent from `/metrics` | hooks run at reactor start, long after every `init()`; a unit test asserts the counter appears |
| R-6 | Concurrent sessions edit `reactor.go`, `ls_export`, `rib` | foreign hunks in the same files | `.claude/rules/foreign-files.md`; edit only the import block and the one call site |
| R-7 | `spec-bgp-ls-origination-and-the-scheduled-marker` (pre-release) touches `ls_export` | merge conflict | land route D's `ls` move as one phase and tell that spec's owner the new import path |
| R-8 | The boot-effect field (route I) becomes a general hook other code piles onto | review finds a second, unrelated use or a request for ordering | one run point only: hub calls the registered boot effects once, right after `applyHostTuning` and before `startSmartManager` (`cmd/ze/hub/main.go` `runYANGConfig`, where `harvestKernelCrashes` runs today); no ordering field, no reload-time call, documented as "runs once at daemon start" |
| R-9 | `bfdRawCapture` is written by BFD start/stop and read by the `capture-raw` handler with no synchronisation today | race detector on the diag/BFD tests | the seam in `bfd/api` holds the provider in an `atomic.Pointer` (or `atomic.Value`), matching `loopMetricsPtr`; the race is fixed by the move, not carried |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a command, backend, filter counter or YANG module missing from the binary; FlowSpec/BGP-LS bytes mis-encoded if the move changes code |
| How is it reverted? | per-phase commits, each a single revert |
| Who else touches this path? | reactor, RIB, ls_export (BGP-LS origination spec), flowspec-firewall, diag, crashes; `plan/spec-yang-feature-opt-ins-owned-by-feature.md` regenerates the same `plugin/all/all*.go` and `feature-gates.txt` (no node or import in common); `plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md` edits `internal/component/bfd/bfd.go` (`applyPinned`, its Files to Modify) beside route H's two `SetBFDRawCaptureProvider` calls; `bfd/api` also holds `opt_in_check.go` (committed in `b3cfb912d0`), so route H rereads both files before its edit (R-6) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch tier check` | → | `pluginOwnershipGate` | `test/ui/le-placement-answers.ci` (red today, green after) |
| daemon start with metrics | → | `filterapi.SetMetricsRegistry` → loop filter hook | `TestSetMetricsRegistryWiresRegisteredFilterMetrics` (`internal/component/bgp/filterapi/metrics_test.go`) |
| receive FlowSpec UPDATE | → | `rib` validation via core `flowspec.ParseFlowSpec` | existing `rib_flowspec_validation` tests plus `test/plugin/flowspec-fw-withdraw.ci` |
| BGP-LS export | → | `ls_export` via core `bgpls` | `test/plugin/bgpls-export-lifecycle.ci` |
| `ze bgp decode` with attribute 29 | → | registered attribute JSON decoder | `test/decode/bgp-ls-10.ci` (existing; asserts the `bgp-ls` attribute) |
| `update text ... nlri ipv4/sr-policy` | → | `parseRegistryNLRISection` → `srpolicy.EncodeNLRIHex` | `test/encode/bgp-srpolicy-1-encode.ci`, `bgp-srpolicy-2-encode.ci` |
| `pool-stats` RPC | → | rib-owned handler | `test/plugin/bgp-pool-stats.ci` (new) |
| `capture-raw protocol bfd` | → | `bfdapi.RawCaptureProvider()` | `TestCaptureRawBFDReadsProviderFromBFDAPI` (`internal/plugins/diag/cmd/capture_raw_test.go`) |
| daemon start | → | registered crash boot effect | `test/ui/cli-crashes-readiness.ci` plus `TestBootEffectsRunOnce` (home per route I) |
| daemon start, config with no `plugin` block | → | `server/startup_autoload.go` with the crashes registration present | `TestAutoloadStartsNoCrashesEngine` (`internal/component/plugin/server/startup_autoload_test.go`); a regression guard, green on today's tree, its red shown in phase I against a registration declaring a trigger (Implementation step 1) |
| `update text ... nlri ipv4/mup add ... del ...` (and the mvpn and sr-policy forms) | → | `parseRegistryNLRISection` later-`add`/`del` refusal | `TestRegistrySectionRefusesLaterAddDel` (`internal/component/bgp/plugins/cmd/update/update_text_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch tier check` on the finished tree | exit 0; `plugin ownership OK`; `ownership.go` and `module-tiers.md` gain no exemption or baseline |
| AC-2 | `test/ui/le-placement-answers.ci` | passes |
| AC-3 | `go list -deps` of `cmd/ze` under each shipped tag set, `ze_core`, `ze_core,ze_bgp`, `ze_setup`, `ze_core,ze_appliance`, `ze_distro,ze_core,ze_bgp`, plus the `./le` build set, and of `cmd/ze-installer` and `cmd/ze-serial-shell`, before and after | for each set, the `internal/plugins/...` and `bgp/plugins/...` packages linked are identical except the three deleted `-cmd/yang` paths, replaced by their new homes, the moved pool-stats package, and the two new `internal/core/bgp/nlri/...` packages; the diff is pasted in the spec |
| AC-4 | `internal/component/config/yang/cli/tree_bgp.go` | deleted; stale "import cycles / rpcDirs" comments in `tree.go` and `tree_bfd.go` corrected |
| AC-5 | metrics enabled, a route with a local-AS loop received | `ze_bgp_as_path_loop_detected_total{peer=...}` increments, as today; reactor imports no `reactor/filter` |
| AC-6 | `internal/plugins/{mtu,ping,traceroute}-cmd` | gone; each module lives at `internal/component/<x>/yang/` with unchanged module name, namespace and prefix; `show mtu`, `ping`, `traceroute` completion and help unchanged |
| AC-7 | `epe`, `ls_export`, `rib`, `flowspec-firewall` | import `internal/core/bgp/nlri/bgpls` or `.../flowspec`, never `bgp/plugins/nlri/...`; the plugins hold no alias or wrapper of a moved symbol |
| AC-8 | encode/decode `.ci` suites for BGP-LS, FlowSpec, SR-Policy | byte-identical outputs, no expectation edited |
| AC-9 | `ze bgp decode` of an UPDATE carrying attribute 29 | same `bgp-ls` JSON; `decode_update.go` has no `case 29` and no `nlri/ls` import |
| AC-10 | `ze bgp decode` in a build without `bgp-nlri-ls` registered (unit test with an empty registry) | attribute 29 falls back to the generic unknown-attribute output, no panic |
| AC-11a | `update text ... nlri ipv4/sr-policy add distinguisher D color C endpoint E` with E of the NLRI's AFI (every input `bgp-srpolicy-{1,2}-encode.ci` uses) | same NLRI bytes as before |
| AC-11b | a later `add`/`del` inside the run (`add distinguisher 1 del color 2 endpoint 10.0.0.1`) | refused by the shared registry section parser with an error saying to write one `add` or `del` right after the family (Q-2 (c)); the same refusal, same message, for every registry family (mup, mvpn, sr-policy: A-8), pinned by `TestRegistrySectionRefusesLaterAddDel`; a native family's later `add`/`del` still switches mode |
| AC-11c | a field before any `add`/`del` | refused as today (`route.ErrMissingAddDel`) |
| AC-11d | a `path-information N` that applies to an SR-Policy section, in each of three positions: (1) at top level before that section's `nlri`, including a mixed command (`path-information 5 nlri ipv4/unicast add ... nlri ipv4/sr-policy add ...`); (2) inside the section before `add`/`del` (`nlri ipv4/sr-policy path-information 5 add ...`); (3) inside the section's run (`nlri ipv4/sr-policy add distinguisher 1 path-information 5 color 2 endpoint 10.0.0.1`). Control row, Q-A: a `path-information` inside ANOTHER family's section (`nlri ipv4/unicast path-information 5 add ... nlri ipv4/sr-policy add ...`) | (1)-(3): the whole command is refused and nothing is applied (Q-2 (b)), with one error naming the problem (SR-Policy NLRI does not support ADD-PATH, RFC 9830) and the fix (drop `path-information`, or put SR-Policy in its own command); never stripped silently, and never `ErrMissingAddDel` (position 2) or the encoder's "unknown sr-policy keyword" (position 3). Control row: accepted; the unicast NLRI carries path identifier 5 and the SR-Policy NLRI carries none, same bytes as today. The refusal comes from a fact the SR-Policy plugin declares in its registration, which `parseRegistryNLRISection` tests for the section's family before it reads `add`/`del` and before it calls the encoder; never from a SAFI test in `update` |
| AC-11e | `rd` or `label` inside an SR-Policy section | refused by the SR-Policy encoder with an error naming the token and saying SR-Policy takes only `distinguisher`, `color`, `endpoint` (today these end the section and the top-level parser refuses them; refused before, refused after, clearer message) |
| AC-11f | endpoint of the other address family (`ipv4/sr-policy ... endpoint 2001:db8::1`) | refused by `validateSRPolicyEndpoint`: a deliberate behavior change, owner question Q-2 (a) |
| AC-11g | the code | `update_text_srpolicy.go` deleted; no `SAFISRPolicy` in `update_text_nlri.go`: the SAFI 73 branch, the `SAFISRPolicy` entry in the `parseNLRI` case list and the SR-Policy case of `nativeTextFamily` are gone, SR-Policy is reached because the registry says a plugin encodes it |
| AC-12 | `pool-stats` | answered by `internal/component/bgp/plugins/rib/poolcmd` with its module `ze-rib-poolstats-cmd.yang` in `internal/component/bgp/plugins/rib/poolcmd/yang/` (WireMethod `ze-bgp:pool-stats` unchanged); one row per pool in `AllPools()`, every row named from names declared beside the pools in `rib/pool/attributes.go`, no `unknown` |
| AC-13 | `capture-raw protocol bfd` with BFD running / not running | works / returns today's not-available error; the provider is held in an atomic in `bfd/api`; `bfd.go` imports no `plugins/diag`; race-enabled test green |
| AC-14 | daemon start | the crashes registration's boot effect runs once, after `applyHostTuning` and before `startSmartManager`, with the same log lines and gauges, and the "no crash directory is writable" warning (now in `main_system.go` `harvestKernelCrashes`) emitted by the crashes package; `cmd/ze/hub` imports no `plugins/crashes`; a config reload does not run it |
| AC-15 | `BenchmarkForwardDirect` and `BenchmarkForwardDirect_Batch` (`internal/component/bgp/reactor/forward_update_bench_test.go`), and a new `BenchmarkRIBFlowSpecValidation` in `internal/component/bgp/plugins/rib/` run before the D1 move and after it | allocs/op identical, ns/op within noise; numbers pasted in the spec |
| AC-16 | the plugin listing after route I, and daemon start with a config holding no `plugin` block | three surfaces each gain exactly one `crashes` row: `internal/component/plugin/all/testdata/plugins.snapshot`, `docs/features/plugins.md`, and the `docs/DESIGN.md` "Shipped Plugins" table (the yangcontract drift gate, `checkDesignMD` in `internal/le/doc/yangcontract/drift.go`, reports a registered name missing from that table); the "N registered plugins" count in the `docs/DESIGN.md` "Goals" section is checked by the same gate: `checkDesignMD` gains a registered-plugin count check against `registryPluginNames()`, beside the address-family count it already checks, so the source is the build the drift gate runs in (the `./le` binary, whose `yangcontract/contract.go` imports `plugin/all`). That count depends on the build and on the OS of the host running `./le`: `iface-dhcp`, `iface-ra` and `memlock` register on Linux only (`//go:build linux` on each `register.go`; the `platformOnly` set of `TestRegisteredPluginNames`, `plugin/all/all_test.go`), the owner runs gates on darwin, and an exact number is red on one platform whichever is written (the class of `plan/journal/gate-verdict-depends-on-the-machine.md`). So the check reads the floor form the interop count already uses (`extractFloorCount`, "100+ interop scenarios"): a claim "N+ registered plugins" is reported only when the registry holds fewer than N, an exact claim "N registered plugins" is still reported when unequal, and that count is written as a floor set to the cross-platform count, the `plugins.snapshot` set the darwin registry holds (90 names today, 91 after route I; today's text says 97), pinned by `TestCheckDesignMDPluginCount`; no crashes engine is started, proven by `TestAutoloadStartsNoCrashesEngine` (A-9) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `./le verify worktree` | tier check passes | `test/ui/le-placement-answers.ci` |
| 2 | peer sends FlowSpec, firewall applies it | wire → RIB validation → flowspec-firewall → nft | `test/plugin/flowspec-fw-withdraw.ci` |
| 3 | exports IGP topology as BGP-LS | ls_export → core bgpls → UPDATE | `test/plugin/bgpls-export-lifecycle.ci` |
| 4 | asks `pool-stats` | RPC → rib-owned handler | `test/plugin/bgp-pool-stats.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSetMetricsRegistryWiresRegisteredFilterMetrics` | `internal/component/bgp/filterapi/metrics_test.go` | a registered filter's metrics hook receives the registry; nil registry stays a no-op | |
| `TestDecodeAttributeUsesRegisteredJSONDecoder` | `internal/component/bgp/cli/decode_update_test.go` | code 29 resolved through the registry; absent registration falls back | |
| `TestPoolStatsNamesEveryPool` | rib-owned handler package | row count equals `len(AllPools())`, no `unknown` | |
| `TestCaptureRawBFDReadsProviderFromBFDAPI` | `internal/plugins/diag/cmd/capture_raw_test.go` | provider set through `bfdapi` is used; nil gives today's error | |
| `TestBootEffectsRunOnce` | the boot-effect home (route I) | each registered effect runs once with the config path | |
| `TestSRPolicyTextMatchesPluginGrammar` | `internal/component/bgp/plugins/cmd/update/update_text_test.go` | A-3's input table through `update text`, including AC-11d's three refused positions and its Q-A control row | |
| `TestRegistrySectionRefusesLaterAddDel` | `internal/component/bgp/plugins/cmd/update/update_text_test.go` | AC-11b for every registry family: one row each for mup, mvpn and sr-policy, each refused by the shared parser with the same message; one native row (`ipv4/unicast add P1 del P2`) still accepted and switching mode; a vpn row would be refused as written (`ErrMissingRD`, then `ErrMissingLabel`, both in `parseVPNNLRI`), so a vpn control row carries `rd` and `label` | |
| `TestCheckDesignMDPluginCount` | `internal/le/doc/yangcontract/drift_test.go` | a `docs/DESIGN.md` line claiming the floor `N+ registered plugins` is reported when `len(registryPluginNames())` is below N and not when it is at or above N; an exact `N registered plugins` is reported when N differs and not when it is equal (AC-16) | |
| `TestAutoloadStartsNoCrashesEngine` | `internal/component/plugin/server/startup_autoload_test.go` | with the crashes registration present and a config holding no `plugin` block, the auto-load set does not contain `crashes` (A-9, AC-16) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SR-Policy distinguisher | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| SR-Policy color | 0-4294967295 | 4294967295 | N/A | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-placement-answers` | `test/ui/le-placement-answers.ci` | tier check passes | |
| `bgp-pool-stats` (new) | `test/plugin/bgp-pool-stats.ci` | operator reads named pool occupancy | |
| existing BGP-LS, FlowSpec, SR-Policy encode/decode, flowspec-fw, bgpls-export, crashes, diag-capture suites | `test/{encode,decode,plugin,ui}/` | unchanged behavior | |

### Interop Tests (Scope: protocol)
N-A, under the "no wire-visible change" exemption of `ai/rules/interop-and-goal-validation.md`, stated precisely: for every input Ze still accepts, the bytes Ze puts on the wire are unchanged (AC-8, AC-11a); the inputs AC-11b to AC-11f are refused at the `update text` command before any UPDATE is built, so they send nothing and no peer can observe them. That refusal is proven by `TestSRPolicyTextMatchesPluginGrammar`, not by a peer.

## Files to Modify
- `internal/component/bgp/reactor/reactor.go` (imports, metrics comment and call)
- `internal/component/bgp/filterapi/filterapi.go`, `metrics.go` (metrics hook on `Filter`)
- `internal/component/bgp/reactor/filter/register.go` (declare the hook)
- `internal/component/config/yang/cli/tree.go`, `tree_bfd.go` (stale comments)
- `internal/component/{firewall,iface,traffic}/cli/main.go`, `internal/plugins/init/main.go` (imports, comments)
- `internal/component/{mtu,ping,traceroute}/cmd/doc.go`
- `internal/component/bgp/plugins/epe/epe_source.go`, `ls_export/export_{encode,epe,plugin,state}.go`
- `internal/component/bgp/plugins/nlri/ls/*`, `nlri/flowspec/*` (moved symbols removed, imports updated, attribute JSON decoder registered)
- `internal/component/bgp/plugins/rib/rib_flowspec_validation.go`, `internal/plugins/flowspec-firewall/*.go`
- callers of moved symbols outside the 35 edges (inventory scan, step 7): `internal/chaos/peer/sender.go`, `internal/le/interoplab/bgp/check_ospf_bgpls.go`; tests `internal/plugins/flowspec-firewall/*_test.go` (9 files), `internal/component/bgp/plugins/cmd/update/update_text_test.go`, `internal/component/bgp/message/flowspec_wire_contract_test.go`, `internal/component/bgp/format/json_test.go`, `internal/component/bgp/cli/rfc8955_decode_test.go`
- plugin test files of `nlri/flowspec` and `nlri/ls`, routed by the inventory's test table (moved whole, split by function, or kept); `rfc/discrimination/rfc5575.json`, `rfc8955.json`, `rfc8956.json` (D1, 15) and `rfc7752.json`, `rfc9085.json`, `rfc9086.json`, `rfc9514.json`, `rfc9552.json` (D2, 96) re-recorded, and the new records of moved or rewritten tagged units added to `rfc5575.json`, `rfc8955.json`, `rfc8956.json` (D1, 18) and `rfc7752.json`, `rfc9552.json` (D2, 11)
- `internal/le/doc/yangcontract/drift.go`, `drift_test.go` (`checkDesignMD` checks the registered-plugin count, AC-16)
- `internal/component/bgp/cli/decode_update.go`
- `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` (SAFI 73 branch, the `SAFISRPolicy` entry in the `parseNLRI` case list, the SR-Policy case of `nativeTextFamily`, later `add`/`del` in `parseRegistryNLRISection`, ADD-PATH refusal from the registration)
- `internal/component/bgp/plugins/nlri/srpolicy/register.go`, `encode.go` (declare no ADD-PATH; refuse `rd`/`label` with a clear error)
- `internal/component/plugin/registry/registry.go` (attribute JSON decoder field and lookup, no-ADD-PATH fact, boot-effect field)
- `internal/component/bgp/plugins/rib/pool/attributes.go` (names declared with pools)
- `internal/component/bfd/bfd.go`, `internal/component/bfd/api/` (seam), `internal/plugins/diag/cmd/capture_raw.go`
- `cmd/ze/hub/main.go`, `main_system.go`, `internal/plugins/crashes/readiness.go`, `register.go`
- `internal/component/plugin/all/testdata/plugins.snapshot`, `docs/features/plugins.md`, `docs/DESIGN.md` "Shipped Plugins" table and its "Goals" registered-plugin count (one `crashes` row, AC-16)
- `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` comments D1 makes stale: the `parseComponentText` doc comment (names `parseFlowComponent in types.go`, which D1 moves to core), the `familyModeBoth` doc comment (places `family.MustRegister` in `types.go`, moved to `register.go`), the `protocolNameToNumber` doc comment (calls `protocolNameToNumber` the ONLY protocol vocabulary in this package, moved to core)
- `internal/component/plugin/server/startup_autoload_test.go` (`TestAutoloadStartsNoCrashesEngine`)
- generated: `internal/component/plugin/all/*.go`, `AGENTS.md` plugin lists (via `./le repo generate`)
- docs listed in the Documentation checklist

## Files to Create
- `internal/core/bgp/nlri/bgpls/` (BGP-LS wire vocabulary moved from `nlri/ls`)
- `internal/core/bgp/nlri/flowspec/` (FlowSpec wire vocabulary moved from `nlri/flowspec`)
- `internal/component/{mtu,ping,traceroute}/yang/` (ping and traceroute new; mtu existing) holding the moved `-cmd` modules
- `internal/component/bgp/plugins/rib/poolcmd/` (handler moved from `cmd/rib/pool_stats.go`) and `rib/poolcmd/yang/ze-rib-poolstats-cmd.yang` (moved from `cmd/rib/yang/`)
- crashes `registry.Registration` in `internal/plugins/crashes/register.go` (boot effect, no-op engine)
- `BenchmarkRIBFlowSpecValidation` in `internal/component/bgp/plugins/rib/`
- `test/plugin/bgp-pool-stats.ci`

Deleted: `internal/component/config/yang/cli/tree_bgp.go`, `internal/component/bgp/plugins/cmd/update/update_text_srpolicy.go`, `internal/component/bgp/plugins/cmd/rib/pool_stats.go` and its YANG module, `internal/plugins/{mtu,ping,traceroute}-cmd/`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | three `-cmd` modules and `ze-rib-poolstats-cmd` move to their owners, content unchanged |
| YANG validation constraints | N-A | no leaf changes |
| YANG custom validators | N-A | none added |
| CLI commands/flags | N-A | no command added or renamed |
| CLI grammar (keyword before value) | Yes | SR-Policy `update text` grammar converges on the plugin's (A-3) |
| Editor autocomplete | N-A | schema content unchanged |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-pool-stats.ci` (existing RPC, first test) |
| Pipe completeness | N-A | outputs unchanged |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | Yes | `ze_bgp_as_path_loop_detected_total` wiring path changes, name and labels unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family; existing families' code moves |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope/evidence/level changed? | No | refactor |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if SR-Policy `update text` wording changes (A-3) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (pool-stats owner; SR-Policy text through the registry) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it lists the `-cmd` directories |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No; source paths change | `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/wire/nlri-flowspec.md` (also the `// Design:` page of `plugin_encode_text.go`, whose `familyModeBoth`, `protocolNameToNumber` and `parseComponentText` doc comments D1 corrects): source anchors to the core packages |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/plugin/plugin-system.md` (filter metrics hook, attribute JSON decoder registration, boot effect) |
| 9 | RFC behavior changed? | No | anchors in `rfc/short/rfc8955.md`, `rfc9552.md`, `rfc9830.md` updated where they cite moved files |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/module-tiers.md` (gate green, examples), `docs/architecture/command-ownership.md` (mtu/ping/traceroute schema home), `docs/architecture/diagnostics/crash-capture.md` (boot harvest seam) |
| 13 | Route metadata keys changed? | No | - |
| 14 | Prometheus counters changed? | No | names unchanged |
| 15 | Registered plugin/command/inventory changed? | Yes | `docs/features/plugins.md` and the `docs/DESIGN.md` "Shipped Plugins" table (the `crashes` row each) plus the `docs/DESIGN.md` "Goals" registered-plugin count, written as the floor `N+` set to the cross-platform `plugins.snapshot` count, which the extended `checkDesignMD` checks, AC-16; `docs/plugin-overview.md` if it lists `*-cmd` |
| 16 | Changed files with doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/pre-release/spec-plugin-ownership-imports.md` at each phase. Declared by `// Design:` headers of files this spec changes: `docs/architecture/api/architecture.md` (affected only if it names the `update` SR-Policy parser; check in phase F), `docs/architecture/config/yang-config-design.md` (affected: the analysis-tree blank-import partition described for `tree.go`/`tree_bgp.go`, phase A), `docs/architecture/hub-architecture.md` (affected: daemon-start crash harvest call, phase I), `docs/architecture/plugin/rib-storage-design.md` (affected: FlowSpec validation imports the core package, phase D1), `docs/architecture/pool-architecture.md` (affected: pool names declared beside `AllPools`, pool-stats owner, phase G), `docs/architecture/wire/nlri.md` (affected: SR-Policy registration declares no ADD-PATH and the `update text` path goes through the registered encoder, phase F), `rfc/short/rfc5880.md` (unaffected: `bfd.go` changes one seam call, no BFD protocol behavior; anchor checked in phase H) |
| 17 | Existing docs show examples for this area? | Yes | SR-Policy `update text` examples checked against the plugin grammar |

## Implementation Steps

1. **Phase: Wiring** -- write the failing tests in the Wiring Test table that do not exist yet (filter metrics hook, attribute JSON decoder, pool-stats `.ci`, diag BFD seam, boot effect). Record the `go list -deps` baseline for AC-3. `TestAutoloadStartsNoCrashesEngine` is the exception: it is a regression guard and passes on today's tree, which holds no crashes registration, so it cannot be written red here. Its red is shown in phase I instead, against a crashes registration temporarily declaring an auto-load trigger, and the output is pasted. The trigger is a family the test config configures (`server/startup_autoload.go` `getUnclaimedFamilyPlugins`), or a `DataPlane` declaration with a configured `NeedsDataPlane` producer; `NeedsDataPlane` on crashes itself starts nothing, because `appendDataPlaneWriter` adds the data-plane WRITER a needed plugin requires, never the plugin declaring the need.
2. **Phase A: redundant registrations** (edges 1-6, 8-16) -- delete the blank imports and `tree_bgp.go`; add `_test.go` imports where a package test needs them (A-2); correct the stale comments. Verify AC-3, AC-4.
3. **Phase C: filter metrics** (edge 7) -- optional metrics hook on `filterapi.Filter`, called by `filterapi.SetMetricsRegistry`; loop filter declares it; reactor drops the import. AC-5.
4. **Phase B: schema homes** (17-19) -- move each `-cmd/yang` package into `internal/component/<x>/yang/`, regenerate, delete the `-cmd` directories. AC-6.
5. **Phase D1: FlowSpec core** (25-30) -- owner step first: the owner rules on the 27 D1 tagged units in the approval table under the inventory, and each ruling is recorded with `./le rfc approve unit <package>.<TestName> reason "<owner's words>"`; then move the wire vocabulary to `internal/core/bgp/nlri/flowspec`, plugin keeps `family.MustRegister`, config, the JSON/text entry points and hex codecs; the `plugin_decode.go` rendering helpers and the `plugin_encode_text.go` keyword block and name tables move as the inventory lists, exporting exactly its "Newly exported by core" column (33 names); update RIB, flowspec-firewall, `internal/chaos/peer/sender.go` and the 12 outside test files; route the plugin tests by the inventory's test table, naming a family in the moved tests by a `family.Family` built from core constants; the four outside test files that name a staying family var build it from core constants (inventory step 7); re-record the 14 D1 discrimination records whose producer moves and the 1 `rfc8956.json` record whose unit text changes, and record the 18 new D1 covers listed under the inventory, in this phase's commit; correct the three `plugin_encode_text.go` comments D1 makes stale: the `parseComponentText` doc comment (places `parseFlowComponent` in the plugin's `types.go`), the `familyModeBoth` doc comment (places `family.MustRegister` in `types.go`; it moves to `register.go`) and the `protocolNameToNumber` doc comment (calls `protocolNameToNumber` the ONLY protocol vocabulary in this package; it moves to core). Re-run the inventory scan on the result: no forward edge, no reverse edge to an unexported core name (the reverse list itself is never empty: staying code names core exports by design), and no core test naming a staying declaration. AC-7, AC-8, AC-15.
6. **Phase D2: BGP-LS core** (20-24) -- owner step first: the owner rules on the 27 D2 tagged units in the approval table (35 if the three whole-file moves are not paired as renames), recorded the same way with `./le rfc approve`; then same for `internal/core/bgp/nlri/bgpls`, including the `plugin.go` JSON helpers, exporting exactly the inventory's "Newly exported by core" column (21 names plus the two result structs); `register_attr.go` adapts core `DecodeSRCapabilities` returning a nil interface on error (A-4), and the two TLV 1034 tests reach it through `lookupLsAttrTLVDecoder` and stay in the plugin; `ls_export` uses core family constants (A-5); update `internal/le/interoplab/bgp/check_ospf_bgpls.go` and `bgp/cli/rfc8955_decode_test.go`; route the other plugin tests by the test table, declaring the core copies of `buildAttrTLV` and `unknownAttrTLVType` it names; re-record the 9 D2 discrimination records, the 4 in `rfc/discrimination/rfc9085.json` naming the moved producer as core `DecodeSRCapabilities`, re-record the 87 D2 records whose unit or producer text changes without a move (27 in the plugin, 60 in `ls_export` and `epe`, listed under the inventory), and record the 9 new moved D2 covers and the 2 of the rewritten `TestRFC7752UnknownTLVPreservedAndPropagated` listed under the inventory, in this phase's commit. Re-run the inventory scan: before the change only the `lsAttrTLV` forward edge A-4 removes may appear; after it, no forward edge, no reverse edge to an unexported core name, and no core test naming a staying declaration. AC-7, AC-8.
7. **Phase E: attribute JSON decoder** (31) -- registry field keyed by attribute code, `bgp-nlri-ls` registers `AttrTLVsToJSON` for 29, decoder looks it up. AC-9, AC-10.
8. **Phase F: SR-Policy text** (32) -- write `TestSRPolicyTextMatchesPluginGrammar` over AC-11a to AC-11f first and record today's outcome for each row; validate A-8; then declare no ADD-PATH in the SR-Policy registration, and make `parseRegistryNLRISection`, when the registry says the section's family declared no ADD-PATH, refuse with the AC-11d error in exactly the three positions AC-11d lists and in no other: (1) the accumulator it was handed already carries `HasPathID` (a top-level `path-information`), tested on entry; (2) the token after the family is `path-information`, tested before the `add`/`del` check that would answer `ErrMissingAddDel`; (3) a `path-information` token inside the run `endsRegistrySection` keeps, tested before the run is handed to the encoder. A `path-information` inside another family's section never reaches this parser for the SR-Policy section, because each section gets its own accumulator copy, so it is accepted (owner decision Q-A, AC-11d control row). Make the encoder refuse `rd`/`label`, make the shared registry section parser refuse a later `add`/`del` for every registry family with the message AC-11b names (`TestRegistrySectionRefusesLaterAddDel`), and delete the SAFI 73 branch, the two SAFI 73 case lists and `update_text_srpolicy.go`. AC-11a to AC-11g.
9. **Phase G: pool-stats** (33) -- move handler and module into the rib owner; names declared beside the pools. AC-12.
10. **Phase H: BFD capture seam** (34) -- interface and setter into `bfd/api`; diag reads it. AC-13.
11. **Phase I: crash boot effect** (35) -- boot-effect field on `registry.Registration`; crashes registers it (A-9) with `HarvestAtBoot` and the not-writable warning; hub runs registered effects once between `applyHostTuning` and `startSmartManager` and drops `harvestKernelCrashes`; update `plugins.snapshot`, `docs/features/plugins.md` and the `docs/DESIGN.md` "Shipped Plugins" table with the one `crashes` row; teach `checkDesignMD` the registered-plugin count in the floor and exact forms, checked against `registryPluginNames()` (`TestCheckDesignMDPluginCount`), then write the `docs/DESIGN.md` "Goals" registered-plugin count as the floor `N+`, N the `plugins.snapshot` line count after the `crashes` row (the cross-platform set, green on darwin and Linux); write `TestAutoloadStartsNoCrashesEngine` and show its red against a trigger-declaring registration (step 1). AC-14, AC-16.
12. Docs per checklist, in each phase. AC-1, AC-2 at the end.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every one of the 35 edges gone from the gate output, each AC at file:line |
| Correctness | no registration lost (AC-3), byte identity (AC-8) |
| Naming | core package names `bgpls`, `flowspec` match their family; YANG module names unchanged |
| Data flow | no plugin imports another plugin's implementation; core imports no plugin |
| Rule: no-layering | no alias, wrapper or re-export left in a plugin for a moved symbol |
| Rule: performance | rib FlowSpec path and loop filter path unchanged in shape, AC-15 numbers pasted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| gate green | `./le arch tier check` |
| no `-cmd` YANG-only plugin dirs | `ls internal/plugins | grep -- -cmd$` excludes mtu, ping, traceroute |
| moved packages | `ls internal/core/bgp/nlri/{bgpls,flowspec}` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | SR-Policy text: endpoint AFI validation now applied in `update text` (plugin's `validateSRPolicyEndpoint`); FlowSpec/BGP-LS parsers moved unchanged, fuzz tests move with them |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Most of the gate's red is not coupling but redundancy: hosts pinning registrations the generator already pins. The generator learned to find RPC packages in June; the hand-written pins outlived the reason they were written.
- The edges that are real coupling share one cause: a wire vocabulary (FlowSpec components, BGP-LS TLVs) that several features need was placed inside the one plugin that registers the family.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D (FlowSpec, BGP-LS): move the wire vocabulary to `internal/core/bgp/nlri/<family>`, keep registration in the plugin | (a) decode through the family registry (`InProcessNLRIDecoder`, hex string in, `any` out); (b) baseline the edges; (c) fold `ls_export`/`epe` into the `nlri/ls` owner | (a) adds a hex round trip, an interface call and allocations per FlowSpec NLRI on the RIB receive path and erases the types the firewall translator needs; (b) is banned by the gate's design; (c) breaks the delete-the-folder rule in the other direction. The core move keeps every call direct: zero added allocation or indirection per UPDATE |
| A: delete redundant blank imports | keep them behind a build tag | the composition already pins them; a second pin is a second declaration |
| C: metrics hook on `filterapi.Filter` | `registry.Registration.ConfigureMetrics` | the loop filter is not a run plugin, so `ConfigureMetrics` never fires (the reactor comment says so); `filterapi` already owns the sibling attr-mod counters and is the seam the filter registers through. Startup only; `LoopIngress` keeps its atomic load |
| B: schema into `internal/component/<x>/yang/` | delete only the blank import | the import is a symptom: a handler owner whose schema lives in another directory fails the delete-the-folder test either way. `command-ownership.md` puts it at `<owner>/yang` |
| E: attribute JSON decoder registered by attribute code | move `AttrTLVsToJSON` and all TLV decoders to core | the decoders are the `bgp-nlri-ls` plugin's feature; removing the plugin should remove BGP-LS attribute decoding from `ze bgp decode`. Offline tool, no hot path |
| F: SR-Policy text through `registry.EncodeNLRIByFamily` | move `srpolicy.New` to core | the plugin already registers an encoder for exactly this grammar; the `update` copy is a duplicate. Command path only: one hex round trip per operator command, as every other plugin family already pays. ADD-PATH refusal comes from a fact the plugin declares in its registration (the encoder's own comment: "SR-Policy NLRIs do not support ADD-PATH"), so `update` keeps no SAFI 73 test |
| F: `add`/`del` first for every registry family, a later one refused by the shared registry section parser (A-8, Q-2 (c)) | widen the shared parser to honour a later `add`/`del` for every plugin family | widening changes the accepted grammar of every plugin family to keep one SR-Policy quirk; one grammar for every family, decided at design review round 2 (2026-10-12) |
| G: pool-stats into `internal/component/bgp/plugins/rib/poolcmd` with its schema in `poolcmd/yang` | a registry for "pool reporters" | one reader, one owner; moving the handler is simpler than a seam. The pool names move beside `AllPools()`, removing the 13-vs-14 drift |
| H: seam in `bfd/api`, provider held in an atomic | diag registry of capture providers | `bfd/api` is the always-on BFD client contract and already holds two nil-able seams of this shape; the atomic fixes today's unsynchronised `bfdRawCapture` (R-9) |
| I: a boot-effect field on `registry.Registration`, set by a new crashes registration (A-9), run once by hub after `applyHostTuning` and before `startSmartManager`; the not-writable warning moves into the crashes effect | (a) give crashes an engine and run the harvest in `RunEngine`; (b) move `HarvestAtBoot` into `internal/core/crashlog`; (c) a separate boot-hook registry package | (a) changes the plugin's process model for one call; (b) puts config reading and gauge publication into core; (c) is a new registry for one user, where `registry.Registration` is already the place a plugin declares what the host calls. R-8 holds it to one run point (decided at design review, 2026-10-12) |

## Review Gate

Design review rounds (`ai/rules/planning.md`: round 1 reviews the whole spec,
round N+1 only the fixes round N made plus their sibling sites; each round's
scope is written here). Every round was run by a read-only agent whose context
did not author the spec.

| Round | Date | Scope | Findings | Answered by |
|-------|------|-------|----------|-------------|
| 1 | 2026-10-12 | the whole spec as of `295fd32542` | 0 BLOCKER, 5 ISSUE, 4 NOTE | `2d4dc7842f` (symbol inventory, AC-11 input list, boot effect on `registry.Registration`, SAFI 73 enumerations, AC-3 tag sets, pool-stats home, atomic BFD seam, benchmarks) |
| 2 | 2026-10-12 | the round-1 fixes in `2d4dc7842f` and their sibling sites | counts not recorded in the spec; fixes listed in the commit | `6abb510587`, `af16392b19` (Q-2 three refusals, add/del first, AC-16, inventory and A-4); owner answers `cebf9620b1` |
| 3 | 2026-10-12 | the round-2 fixes in `6abb510587`, `af16392b19`, `cebf9620b1` (Q-2, A-4, A-8, A-9, AC-11b, AC-11d, AC-16, Phase F and I steps, F decision row, inventory `types.go` row, doc row 15) plus their sibling sites; out-of-scope claims spot-checked | 0 BLOCKER, 7 ISSUE, 5 NOTE (`tmp/review/spec-plugin-ownership-imports-design-round3.md`); every finding re-verified against the code, none disproved | this edit: ISSUE 1 and 7 (AC-11d three positions with a mechanism in Phase F, Q-A), 2 (A-8 registry vs native families), 3 (`docs/DESIGN.md` table and count in AC-16), 4 (A-9 exposure stated, Behavior to change, Q-B, journal row), 5 (BGP-LS inventory and A-4; the re-scan also found the two JSON keys), 6 (two named tests), NOTE 8-12 (A-9 data-plane trigger, A-4 nil interface, D1 comment, `bfd.go` lines, this section) |
| 4 | 2026-10-12 | the round-3 fixes in `ab33d353ec` (Q-A, Q-B, Q-2, Route D BGP-LS inventory and its both-directions paragraph, A-4, A-8, A-9, AC-11b, AC-11d, AC-16, Data Flow step 5, Phase D1/D2/F/I steps, Behavior to change, doc rows 7 and 15, two new tests, this section, the journal row) plus their sibling sites, and the overlap with `plan/spec-yang-feature-opt-ins-owned-by-feature.md` | 0 BLOCKER, 3 ISSUE, 5 NOTE (`tmp/review/spec-plugin-ownership-imports-design-round4.md`); every finding re-verified against the code, none disproved; two corrected in detail: ISSUE 3 names 4 `rfc9085.json` records on `decodeSRCapabilities`, not 2, and NOTE 8's "this spec does not move command schemas" holds only outside routes B and G | this edit: ISSUE 1 and 2 (both inventories closed by a type-checked scan of every moving declaration in both directions; method and counts under the inventory; it also found `FlowSpec.componentBytes`, `protocolNameToNumber`, the two outside production callers, and that `TLV` needs no export), 3 (test table, A-4, R-3, D2 re-records the 4 `rfc9085.json` records and D1/D2 the other 19), NOTE 4 (`checkDesignMD` checks the plugin count, `TestCheckDesignMDPluginCount`), 5 (native control row), 6 (autoload guard's red in phase I), 7 (Blast Radius row), 8 (Q-C open, in both specs) |
| 5 | 2026-10-12 | the round-4 fixes in `5e4c7c6cbe` and their sibling sites: the Route D inventory, its re-scan method and counts, the test routing table, the 23 discrimination re-records, the two outside production callers and 13 outside test files, A-4, R-3, AC-16 and `TestCheckDesignMDPluginCount`, the native control row, the autoload guard's red, the Blast Radius row, Q-C in both specs; an independent `go/types` scanner re-ran the inventory | 0 BLOCKER, 5 ISSUE, 3 NOTE (`tmp/review/spec-plugin-ownership-imports-design-round5.md`); every finding re-verified against the code (the reviewer's scanner re-run on today's tree gives byte-identical output), none disproved; one sibling corrected beyond the finding: the round-3 forward-edge row also merged the two `mtIDs` fields (12 edges, not 11) | this edit: ISSUE 1 (`LinkDescriptorInfo.MTIDs`, 21 names, A-4, D2), 2 and 3 (method step 6 rewritten with test-to-test edges and staying-name checks, the test scan re-run under it and its counts recorded, test table rerouted: `rfc8955_width_test.go` split, `TestProtocolNameReverseAgrees` and the two `rfc9552_test.go` functions move, `rfc5575_arrival_order_test.go`, `rfc8955_vpn_test.go` and FlowSpec `types_fuzz_test.go` move whole, core copies of the two shared helpers), 4 (main-thread decision: tagged units move with their code; the 18 D1 and 9 + 2 D2 new records listed per file, R-3 rewritten), 5 (floor `N+` form, set to the cross-platform count), NOTE 6 (auto-load trigger), 7 (two more stale comments), 8 (reverse-list check) |
| 6 | 2026-10-13 | the round-5 fixes above plus their sibling sites | round 6 design review, 2026-10-13, owner-authorised ('ok to both'): 1 ISSUE, 4 NOTE; every finding re-verified against the code and `rfc/discrimination/`; ISSUE 1 corrected beyond the finding: it named 28 records staled by a qualifier change, and the re-count over every unit and producer body found 60 more, the `ls_export` and `epe` producers whose `ls.` references become `bgpls.` | this edit: ISSUE 1 (the gate's text rule, the 88 qualifier re-records per phase, D1 15 and D2 96 re-records, Files to Modify, R-3, Phase D1/D2), NOTE 2 (step 7 checks staying names; FlowSpec files build the family from core constants), 3 (5, not 6), 4 (R-3 whole-move list), 5 (`bfd.go` start-time `SetBFDRawCaptureProvider` call, `opt_in_check.go` committed) |
| 7 | 2026-10-13 | the round-6 fixes above plus their sibling sites | round 7 design review, 2026-10-13, owner-authorised ('ok to both'): 0 BLOCKER, 1 ISSUE, 5 NOTE; every count re-verified against the code, tagged units re-derived per test function | this edit: ISSUE 1 (approval gates `rfcChangeProblems` and `proposedRFCChanges`, the approval table, the owner step before D1 and D2, unrecorded changed units owe no record, "stays" is not "unchanged"), NOTE 2 (FlowSpec families unregistered in two test binaries), 3 (`bgp/cli/rfc8955_decode_test.go` imports both), 4 ("also" wording on `export_state.go` `send`), 5 (line numbers replaced by symbols), 6 (Updated, these rows) |
| 8 | 2026-10-13 | the round-7 fixes above plus their sibling sites | round 8 design review, 2026-10-13, owner-authorised: 0 BLOCKER, 1 ISSUE, 4 NOTE; every finding re-verified against the code | this edit: ISSUE 1 (who runs `./le rfc approve`, the session file it writes, the `!` route at the implementing session's prompt, the `path` check, rows dropped once their commit lands), NOTE 2 (package prefix per approval-table row), 3 (D1 whole-file units count unconditionally, D2 conditionally), 4 ("up to" for the changed-cover counts), 5 (the remaining hand-typed line citations replaced by symbols; while replacing, three of the handoff's symbols for the `plugin_encode_text.go` comments proved wrong and were corrected to the `parseComponentText`, `familyModeBoth` and `protocolNameToNumber` doc comments) |
| 9 | - | owed before implementation: the round-8 fixes above plus their sibling sites | - | - |

The implementation's own `/ze-review` gate is recorded here at closure, by
`./le spec review record`.

## Known Limitations
- `update_text_nlri.go` still dispatches FlowSpec, VPLS and EVPN by SAFI in its own parsers. Those are central enumerations under `ai/rules/principles.md` but not ownership edges (no plugin import); not in this spec.
- The gate checks import edges only. Copied plugin policy without an import (`module-tiers.md`, "does not prove full feature ownership") stays out of scope.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A, internal refactor with no wire-visible change

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/pre-release/spec-plugin-ownership-imports.md` only, in the same `./le commit create` script
