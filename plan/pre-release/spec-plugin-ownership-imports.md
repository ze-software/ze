# Spec: plugin-ownership-imports

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-12 |

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

| # | Question | Recommendation |
|---|----------|----------------|
| Q-1 | One spec in phases, or split route D (the FlowSpec and BGP-LS core move, 11 edges, the largest phase) into its own spec? | one spec, D1 and D2 as separate phases and commits |
| Q-2 | `update text` for SR-Policy refuses three inputs it accepts today: (a) an endpoint whose address family does not match the NLRI's AFI (today encoded); (b) `path-information N` that applies to an SR-Policy section (top level or inside it; Q-A keeps one inside another family's section accepted), including a mixed command such as `path-information 5 nlri ipv4/unicast add ... nlri ipv4/sr-policy add ...` (today the SR-Policy section drops the identifier silently): the whole command is refused, so one command never half-applies; (c) a second `add`/`del` after the first field of a section (today the last one wins); `add`/`del` must come first for every registry family. Accept all three? | accept: (a) the plugin's encoder and config path already refuse it; (b) SR-Policy NLRIs do not support ADD-PATH and a silent drop is a wrong value; (c) one grammar for every plugin family. Every other changed input in AC-11 was already refused and only gets a clearer message |
| Q-A | A `path-information` written inside ANOTHER family's section, in a command that also has an SR-Policy section (`nlri ipv4/unicast path-information 5 add ... nlri ipv4/sr-policy add ...`): refuse the whole command, or accept it? The identifier lives in the accumulator copy of the unicast section only (`ParseUpdateText` hands each section `attrs.snapshot()`), so it never reaches the SR-Policy NLRI | **Decided 2026-10-12: accept.** Refuse only a `path-information` that applies to the SR-Policy section: written at top level before that section's `nlri`, or inside the SR-Policy section. That is the silent drop Q-2 (b) is about; per-section identifiers of other families stay as documented. The other option, refusing the whole command whenever any `path-information` sits beside an SR-Policy section, was not taken. Phase F step 8 and AC-11d state this one rule |
| Q-B | Route I makes `crashes` a name the `plugin ... use` completion offers (`InternalPluginNameValidator` completes from `registry.Names()`) and adds `ze plugin crashes`, a no-op that exits 0 (`registry.Register` requires a `CLIHandler`): the exposure `loop` and `bgp-nlri-srpolicy` carry today. Accept and journal the class, or widen this spec to hide no-op registrations from completion and the CLI? | **Decided 2026-10-12: accept.** Both surfaces are listed under "Behavior to change"; the class has one row in `plan/journal/inert-registration-offered-as-a-choice.md`, written with this spec edit. Hiding no-op registrations is a separate change to the completion contract, not done here |

### Route D symbol inventory (verified by declaration scan, 2026-10-12)

Go methods live with their type, so every method of a moving type moves with
it, including its `AppendJSON` (this resolves `flowspec/json.go` reading the
unexported `.family`, `.flowSpec`, `.rd`, and `ls/json.go` reading `cachedBytes`).
Nothing that registers moves: every `family.MustRegister` call and every fill of
a registry stays in the plugin.

**FlowSpec: `internal/core/bgp/nlri/flowspec`**

| Plugin file | Moves to core | Stays in the plugin | Newly exported by core (called from the plugin) |
|-------------|---------------|---------------------|-----------------------------------------------|
| `types.go` | all types, constants, `NewFlowSpec`, `ParseFlowSpec`, methods, `parseFlowComponent`, `componentOfType` | the four `family.MustRegister` vars (lines 47-50), moved to `register.go`; core uses `family.Family` values built from `internal/core/family` constants | - (`parseFlowComponent` is called only inside `types.go`) |
| `types_numeric.go` | whole file | - | `NewFlowNumericComponent` (`config_builder.go`, `plugin_encode_text.go`), `NewFlowTCPFlagsMatchComponent`, `NewFlowFragmentMatchComponent` (`plugin_encode_text.go`), `NewFlowICMPCodeComponent` (`config_builder.go`) |
| `types_prefix.go` | whole file | - | `NewFlowDestPrefixComponentWithOffset`, `NewFlowSourcePrefixComponentWithOffset` (`config_builder.go`) |
| `types_vpn.go` | whole file | - | - |
| `compare.go` | whole file (`Compare` uses `componentOfType`) | - | - |
| `json.go` | `AppendJSON` methods and their helpers (`appendFlowSpecJSON`, `flowSpecKey`, `appendNestedStringArray`) | - | - |
| `config.go`, `config_builder.go`, `encode.go`, `plugin.go`, `plugin_decode.go`, `plugin_encode_text.go`, `plugin_protocol.go`, `register.go` | - | whole files | - |

**BGP-LS: `internal/core/bgp/nlri/bgpls`**

| Plugin file | Moves to core | Stays in the plugin | Newly exported by core (called from the plugin) |
|-------------|---------------|---------------------|-----------------------------------------------|
| `types.go` | all NLRI types, the NLRI interface `bGPLSNLRI` (line 237), `BGPLSNLRIType`, `BGPLSProtocolID`, `bgplsBase` and its methods, `parseBGPLS`, descriptor parsers, `parseBGPLSWithRest`, `tlv`, `writeTLV`, `uint32ToBytes`, `WriteTLVBytes`, the `ErrBGPLS*` error vars, the AFI/SAFI aliases | `BGPLSFamily`, `BGPLSVPNFamily` (`family.MustRegister`, lines 40-41), moved to `register.go`; `ls_export` names its families from `internal/core/family` constants | `BGPLSNLRI` (the interface; staying `plugin.go` `bgplsToJSON(n bGPLSNLRI, ...)` line 376), `ParseBGPLSWithRest` (`plugin.go`), `TLV` and `WriteTLV` (used by `attr*.go`, `plugin.go`); the already-exported `ErrBGPLS*` vars are named through the core package by the staying `attr*.go` |
| `types_nlri.go`, `types_descriptor.go`, `types_srv6.go` | whole files | - | - |
| `json.go` | `AppendJSON` methods and helpers | - | - |
| `attr.go` | the two JSON key constants the moving `LsSRCapabilities.ToJSON` reads, `jsonKeyFlags` ("flags", line 20) and `jsonKeyReserved` ("RSV", line 27), declared once in core | the TLV decoder registry: `lsAttrTLV`, `lsAttrTLVDecoder`, `lsAttrTLVRegistry`, `registerLsAttrTLV`, `lookupLsAttrTLVDecoder`, `registeredLsAttrTLVCount`, `iterateAttrTLVs`, `decodeAttrTLV`, `decodeAllAttrTLVs`, `AttrTLVsToJSON`, every other JSON helper and key | `JSONKeyFlags`, `JSONKeyReserved` (read by the staying attribute types' `ToJSON`) |
| `attr_node.go` | every `TLV*` code constant; `LsSrLabelRange`, `LsSRCapabilities` with their methods; the label-range helpers `srLabelRangesLen`, `writeSrLabelRanges`, `decodeSrLabelRanges`, `srLabelRangesToJSON` (lines 236-320); `decodeSRCapabilities`, returning `(*LsSRCapabilities, error)` in core | every other `ls*` attribute type and decoder, including `lsSRLocalBlock` (TLV 1036) and `decodeSRLocalBlock` (lines 406-440) | `DecodeSRCapabilities` (adapted by `register_attr.go`, A-4); `SRLabelRangesLen`, `WriteSRLabelRanges`, `DecodeSRLabelRanges`, `SRLabelRangesToJSON` (called by the staying `lsSRLocalBlock.Len`, `WriteTo`, `ToJSON` and `decodeSRLocalBlock`) |
| `attr_link.go`, `attr_prefix.go`, `attr_srv6.go` | the `TLV*` code constants (including `TLVPeerNodeSID`, `TLVPeerAdjSID`, `TLVPeerSetSID`, `TLVSIDLabel`) | attribute types, decoders, the TLV name maps | - |
| `plugin.go`, `register.go`, `register_attr.go` | - | whole files | - |

The moving files import only `internal/core/{bgp/nlri,bgp/wire,family,textbuf}`
today, so core gains no dependency on component or plugin code.

The BGP-LS rows were re-scanned in both directions at design review round 3
(2026-10-12): every unexported symbol a moving declaration holds that a staying
file names is in the "Newly exported" column (`bGPLSNLRI`, the four label-range
helpers, `tlv`, `writeTLV`, `parseBGPLSWithRest`), and the only staying symbols
a moving declaration names are the two JSON keys that move with it and
`lsAttrTLV`, which the core `DecodeSRCapabilities` no longer returns (A-4).

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
- 15 of 35 edges are blank imports the generated composition already carries (`all.go` lines 97, 101, 114; `all_ze_bgp.go` lines 20-35, 85, 98). Deleting them changes nothing a shipped binary links.
- The `config/yang/cli` comment "not reachable through plugin/all (import cycles, see plugin_imports.go rpcDirs)" is stale: since `5398df0a23` (2026-06-04) the generator auto-discovers RPC packages and `tree.go` already imports `plugin/all`.
- Nothing on the per-UPDATE path changes its call shape: the RIB FlowSpec validator keeps a direct call into the same code at a new import path; the loop filter keeps its atomic-pointer metrics load.
- `pool-stats` names its pools from a hand list of 13 while `pool.AllPools()` returns 14 (`RibOut`), so the 14th row prints `unknown` today. Route G removes the second declaration.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/tier/ownership.go` - `pluginOwner`, `compositionRegistration`, `schemaDependency`, `pluginNonProduction`, `pluginOwnershipGate`
- [ ] `internal/component/bgp/reactor/reactor.go` - lines 42-48 six blank RPC imports plus `filter`; line 1218 `filter.SetMetricsRegistry(r.metricsRegistry)` in the metrics-enable block beside `filterapi.SetMetricsRegistry`
- [ ] `internal/component/bgp/reactor/filter/register.go`, `loop_metrics.go` - registers `loop` through `filterapi.Register` and `registry.Register`; metrics held in an `atomic.Pointer`, nil until set
- [ ] `internal/component/bgp/filterapi/metrics.go` - `SetMetricsRegistry` builds the attr-mod counters
- [ ] `internal/component/config/yang/cli/tree.go`, `tree_bgp.go`, `tree_bfd.go` - `tree.go` imports `plugin/all`; `tree_bgp.go` (`//go:build ze_bgp`) only blank-imports five RPC packages
- [ ] `internal/component/plugin/all/all.go`, `all_ze_bgp.go` - generated; already blank-import every package in edges 1-6, 8-16 and the three `*-cmd/yang` packages
- [ ] `internal/component/{firewall,iface,traffic}/cli/main.go`, `internal/plugins/init/main.go` - blank import a backend so `LoadBackend("netlink"|"nft")` resolves
- [ ] `internal/component/{mtu,ping,traceroute}/cmd/doc.go` and `internal/plugins/{mtu,ping,traceroute}-cmd/yang/` - the `-cmd` directories hold only a `yang/` package (module, `embed.go`, `register.go`, `cmd_schema_test.go`); `mtu` already has `internal/component/mtu/yang` for its config module
- [ ] `internal/component/bgp/plugins/nlri/ls/` - `types*.go`, `attr*.go` import only `internal/core/{bgp/nlri,bgp/wire,family,textbuf}`; `types.go` lines 40-41 call `family.MustRegister`; `register_attr.go` fills the TLV-decoder registry `registerLsAttrTLV`
- [ ] `internal/component/bgp/plugins/nlri/flowspec/` - `types*.go`, `compare.go` import only core; `types.go` lines 47-50 call `family.MustRegister`
- [ ] `internal/component/bgp/plugins/rib/rib_flowspec_validation.go` - parses each received FlowSpec NLRI with `flowspec.ParseFlowSpec`/`ParseFlowSpecVPN` (RFC 8955 Section 6 validation)
- [ ] `internal/component/bgp/cli/decode_update.go` - attribute switch, case 29 calls `ls.AttrTLVsToJSON`; `internal/component/bgp/message/rfc7606_bgpls.go` documents that this is its only non-test caller
- [ ] `internal/component/bgp/plugins/cmd/update/update_text_srpolicy.go`, `update_text_nlri.go` - SAFI 73 branch runs a second SR-Policy grammar and calls `srpolicy.New`; `parseRegistryNLRISection`/`encodeViaRegistry` already serve other plugin families through `registry.EncodeNLRIByFamily`
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/register.go`, `encode.go` - registers `InProcessNLRIEncoder: EncodeNLRIHex`, which parses the same three fields and validates the endpoint against the AFI
- [ ] `internal/component/bgp/plugins/cmd/rib/pool_stats.go`, `cmd/rib/yang/ze-rib-poolstats-cmd.yang`, `rib/pool/attributes.go` - in-process reader of `pool.AllPools()`, names from a parallel 13-entry array
- [ ] `internal/component/bfd/bfd.go` lines 841, 936 (the two `diagCmd.SetBFDRawCaptureProvider` calls, re-read 2026-10-12); `internal/plugins/diag/cmd/capture_raw.go` lines 15-47 - BFD pushes itself into a package variable owned by the diag plugin
- [ ] `internal/component/bfd/api/registry.go`, `profile_check.go` - the existing nil-able seams (`SetService`/`GetService`, `SetProfileChecker`)
- [ ] `cmd/ze/hub/main.go` line 1023, `main_system.go` lines 170-175; `internal/plugins/crashes/readiness.go` - `HarvestAtBoot` runs once at daemon start, logs, publishes two gauges; crashes registers no engine, only an offline fallback, doctor checks and RPCs

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
- The plugin listing gains one `crashes` row: `plugins.snapshot`, `docs/features/plugins.md`, and the `docs/DESIGN.md` "Shipped Plugins" table with its registered-plugin count (AC-16).

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
| A-4 | The Route D inventory moves to core without moving any `family.MustRegister` call or any registry: the BGP-LS attribute TLV registry (`lsAttrTLVRegistry`, `registerLsAttrTLV`, `lookupLsAttrTLVDecoder`, `decodeAllAttrTLVs`) stays whole in the plugin. Core holds no registry and no package-level map written from `init()`; what the plugin calls into core is plain functions, types and constants, exactly the "Newly exported by core" column of the inventory (for BGP-LS: `BGPLSNLRI`, `ParseBGPLSWithRest`, `TLV`, `WriteTLV`, `JSONKeyFlags`, `JSONKeyReserved`, `DecodeSRCapabilities` and the four label-range helpers). Core `DecodeSRCapabilities` returns `(*LsSRCapabilities, error)`; `register_attr.go` registers a closure adapting it to `lsAttrTLVDecoder`, and on error that closure returns a nil `lsAttrTLV` interface with the error, never the typed nil pointer wrapped in a non-nil interface | declaration scan 2026-10-12; moving files import only core | the move drags a registry into core | `./le arch tier check` core-direction and placement gates; grep core packages for `MustRegister` and package-level maps written from `init()` | unvalidated |
| A-5 | `ls_export` can name its families from `internal/core/family` constants (`AFIBGPLS`, `SAFIBGPLinkState`, `SAFIBGPLinkStateVPN`) instead of the plugin's registered vars | `family.go` declares the AFI name; plugin registers the name pair | export path needs a registered `Family` value | `bgpls-export-lifecycle.ci` green | unvalidated |
| A-6 | The BFD raw capture interface returns `plugin.BGPRawCaptureEntry`, a component/plugin type `bfd/api` may import without a cycle | `capture_raw.go` line 18 | import cycle | independent design review 2026-10-12 | validated |
| A-8 | `add`/`del` must come first in a REGISTRY family's section, as `parseRegistryNLRISection` already requires; a later `add`/`del` is refused by that shared parser with an error saying to write one `add` or `del` right after the family. The registry families are the ones `nativeTextFamily` answers false for: today mup and mvpn, and SR-Policy once F removes its two cases (lines 448-449). For mup and mvpn no accepted input is lost (a later `add`/`del` already reaches their encoders, which refuse it); only the message changes. The native families, unicast, multicast, vpn, labeled, flowspec, flowspec-vpn, vpls and evpn, keep their own grammar, in which a later `add`/`del` switches mode (`add P1 del P2`, `parseNLRISection` lines 178-190); vpn and labeled register an `InProcessNLRIEncoder` but are parsed natively and call it per prefix | `update_text_nlri.go` `nativeTextFamily` (425-452), `parseRegistryNLRISection` (466-491), mode-switch loop (178-190); SR-Policy copy lines 51-66; mup, mvpn encoders | a registry family accepts a later `add`/`del` as a value today, or a native family is routed through the registry parser | grep of the mup and mvpn `EncodeNLRIHex` argument parsers; `TestRegistrySectionRefusesLaterAddDel`; native-family unit tests green unchanged | unvalidated |
| A-9 | `crashes` holds no `registry.Registration` today (only `RegisterRPCs`, an offline fallback and doctor checks); route I adds one, and `registry.Register` requires a non-nil `RunEngine`, so it carries the no-op engine `loop` and `bgp-nlri-srpolicy` already carry. `registry.Register` also requires a non-nil `CLIHandler` (registry.go 512), so it carries the no-op handler `loop` carries (`reactor/filter/register.go` line 28). The server never auto-loads it: auto-load starts a plugin only for a configured family, an owned config root, a custom event or send type, a dependency, or as the data-plane writer a `NeedsDataPlane` plugin requires (`server/startup_autoload.go`, `appendDataPlaneWriter` 150-183), and the registration declares none of these (no `ProgramsFIB`, no `DataPlane`). The boot effect runs from hub, never from the engine. It is visible in the plugin listing (`plugins.snapshot`, `docs/features/plugins.md`, the `docs/DESIGN.md` "Shipped Plugins" table) exactly as `loop` is, with a description saying it is crash capture and needs no configuration. The exposure IS widened by one name: `InternalPluginNameValidator` completes from `registry.Names()` (`config/validators.go` 706-729), so `plugin ... use` completion offers `crashes`, naming it in a `plugin` block starts an engine that exits 0 at once, and `ze plugin crashes` is a no-op that exits 0. `loop` and `bgp-nlri-srpolicy` carry the same exposure today. Owner decision Q-B (2026-10-12): accept, list it under "Behavior to change", one row in `plan/journal/inert-registration-offered-as-a-choice.md` | grep of `internal/plugins/crashes`, `registry.go`, `startup_autoload.go`, `config/validators.go` 2026-10-12 | an engine starts that nobody configured | AC-16; `TestAutoloadStartsNoCrashesEngine` | unvalidated |
| A-7 | Moving the three `-cmd` YANG modules changes no module name, namespace or prefix, so `ze-mtu-cmd`, `ze-ping-cmd`, `ze-traceroute-cmd` stay byte-identical and their imports still resolve | `yang/glue` keys by module, not directory | YANG load failure | `TestYANGImportsFollowGoImports`, `cmd_schema_test.go` moved with the module | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deleting a blank import drops a registration in some build-tag combination | `./le build` matrix or feature-gate check red; `all_test.go` registration counts change | the generated composition is the only place a registration is pinned; any missing row is a generator defect fixed there |
| R-2 | Core move of `nlri/ls` and `nlri/flowspec` leaves a re-export alias in the plugin (layering) | grep for type aliases or wrapper funcs in the plugins | `ai/rules/no-layering.md`: move, update every caller, no alias |
| R-3 | RFC-tagged tests and discrimination records name the old package paths | `./le rfc check` red | move tests with their code; re-record discrimination where a tagged unit's file path changes |
| R-4 | Doc source anchors (`<!-- source: -->`, `// Design:`) point at moved files | `./le spec citation anchors spec plan/pre-release/spec-plugin-ownership-imports.md` | update every anchor in the phase that moves the file |
| R-5 | Filter metrics hook runs before the filter registered (init order) | loop counter absent from `/metrics` | hooks run at reactor start, long after every `init()`; a unit test asserts the counter appears |
| R-6 | Concurrent sessions edit `reactor.go`, `ls_export`, `rib` | foreign hunks in the same files | `.claude/rules/foreign-files.md`; edit only the import block and the one call site |
| R-7 | `spec-bgp-ls-origination-and-the-scheduled-marker` (pre-release) touches `ls_export` | merge conflict | land route D's `ls` move as one phase and tell that spec's owner the new import path |
| R-8 | The boot-effect field (route I) becomes a general hook other code piles onto | review finds a second, unrelated use or a request for ordering | one run point only: hub calls the registered boot effects once, right after `applyHostTuning` and before `startSmartManager` (`cmd/ze/hub/main.go` lines 1022-1024); no ordering field, no reload-time call, documented as "runs once at daemon start" |
| R-9 | `bfdRawCapture` is written by BFD start/stop and read by the `capture-raw` handler with no synchronisation today | race detector on the diag/BFD tests | the seam in `bfd/api` holds the provider in an `atomic.Pointer` (or `atomic.Value`), matching `loopMetricsPtr`; the race is fixed by the move, not carried |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a command, backend, filter counter or YANG module missing from the binary; FlowSpec/BGP-LS bytes mis-encoded if the move changes code |
| How is it reverted? | per-phase commits, each a single revert |
| Who else touches this path? | reactor, RIB, ls_export (BGP-LS origination spec), flowspec-firewall, diag, crashes |

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
| daemon start, config with no `plugin` block | → | `server/startup_autoload.go` with the crashes registration present | `TestAutoloadStartsNoCrashesEngine` (`internal/component/plugin/server/startup_autoload_test.go`) |
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
| AC-11g | the code | `update_text_srpolicy.go` deleted; no `SAFISRPolicy` in `update_text_nlri.go`: the SAFI 73 branch, the `parseNLRI` case list (line 262) and the native-family switch (lines 448-449) are gone, SR-Policy is reached because the registry says a plugin encodes it |
| AC-12 | `pool-stats` | answered by `internal/component/bgp/plugins/rib/poolcmd` with its module `ze-rib-poolstats-cmd.yang` in `internal/component/bgp/plugins/rib/poolcmd/yang/` (WireMethod `ze-bgp:pool-stats` unchanged); one row per pool in `AllPools()`, every row named from names declared beside the pools in `rib/pool/attributes.go`, no `unknown` |
| AC-13 | `capture-raw protocol bfd` with BFD running / not running | works / returns today's not-available error; the provider is held in an atomic in `bfd/api`; `bfd.go` imports no `plugins/diag`; race-enabled test green |
| AC-14 | daemon start | the crashes registration's boot effect runs once, after `applyHostTuning` and before `startSmartManager`, with the same log lines and gauges, and the "no crash directory is writable" warning (now `main_system.go` line 173) emitted by the crashes package; `cmd/ze/hub` imports no `plugins/crashes`; a config reload does not run it |
| AC-15 | `BenchmarkForwardDirect` and `BenchmarkForwardDirect_Batch` (`internal/component/bgp/reactor/forward_update_bench_test.go`), and a new `BenchmarkRIBFlowSpecValidation` in `internal/component/bgp/plugins/rib/` run before the D1 move and after it | allocs/op identical, ns/op within noise; numbers pasted in the spec |
| AC-16 | the plugin listing after route I, and daemon start with a config holding no `plugin` block | three surfaces each gain exactly one `crashes` row: `internal/component/plugin/all/testdata/plugins.snapshot`, `docs/features/plugins.md`, and the `docs/DESIGN.md` "Shipped Plugins" table (the yangcontract drift gate, `checkDesignMD` in `internal/le/doc/yangcontract/drift.go`, reports a registered name missing from that table); the "N registered plugins" count in `docs/DESIGN.md` line 45 states the registry's count after route I (the drift gate does not check that count, so it is checked by hand against `registry.All()`); no crashes engine is started, proven by `TestAutoloadStartsNoCrashesEngine` (A-9) |

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
| `TestRegistrySectionRefusesLaterAddDel` | `internal/component/bgp/plugins/cmd/update/update_text_test.go` | AC-11b for every registry family: one row each for mup, mvpn and sr-policy, each refused by the shared parser with the same message; one native row (vpn `add P1 del P2`) still accepted | |
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
- `internal/component/bgp/cli/decode_update.go`
- `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` (SAFI 73 branch, `parseNLRI` case list line 262, native-family switch lines 448-449, later `add`/`del` in `parseRegistryNLRISection`, ADD-PATH refusal from the registration)
- `internal/component/bgp/plugins/nlri/srpolicy/register.go`, `encode.go` (declare no ADD-PATH; refuse `rd`/`label` with a clear error)
- `internal/component/plugin/registry/registry.go` (attribute JSON decoder field and lookup, no-ADD-PATH fact, boot-effect field)
- `internal/component/bgp/plugins/rib/pool/attributes.go` (names declared with pools)
- `internal/component/bfd/bfd.go`, `internal/component/bfd/api/` (seam), `internal/plugins/diag/cmd/capture_raw.go`
- `cmd/ze/hub/main.go`, `main_system.go`, `internal/plugins/crashes/readiness.go`, `register.go`
- `internal/component/plugin/all/testdata/plugins.snapshot`, `docs/features/plugins.md`, `docs/DESIGN.md` "Shipped Plugins" table and its line-45 plugin count (one `crashes` row, AC-16)
- `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` line 202 (comment names `parseFlowComponent in types.go`, which D1 moves to core)
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
| 7 | Wire format changed? | No; source paths change | `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/wire/nlri-flowspec.md` (also the `// Design:` page of `plugin_encode_text.go`, whose line-202 comment D1 corrects): source anchors to the core packages |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/plugin/plugin-system.md` (filter metrics hook, attribute JSON decoder registration, boot effect) |
| 9 | RFC behavior changed? | No | anchors in `rfc/short/rfc8955.md`, `rfc9552.md`, `rfc9830.md` updated where they cite moved files |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/module-tiers.md` (gate green, examples), `docs/architecture/command-ownership.md` (mtu/ping/traceroute schema home), `docs/architecture/diagnostics/crash-capture.md` (boot harvest seam) |
| 13 | Route metadata keys changed? | No | - |
| 14 | Prometheus counters changed? | No | names unchanged |
| 15 | Registered plugin/command/inventory changed? | Yes | `docs/features/plugins.md` and the `docs/DESIGN.md` "Shipped Plugins" table (the `crashes` row each) plus the `docs/DESIGN.md` line-45 registered-plugin count, AC-16; `docs/plugin-overview.md` if it lists `*-cmd` |
| 16 | Changed files with doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/pre-release/spec-plugin-ownership-imports.md` at each phase. Declared by `// Design:` headers of files this spec changes: `docs/architecture/api/architecture.md` (affected only if it names the `update` SR-Policy parser; check in phase F), `docs/architecture/config/yang-config-design.md` (affected: the analysis-tree blank-import partition described for `tree.go`/`tree_bgp.go`, phase A), `docs/architecture/hub-architecture.md` (affected: daemon-start crash harvest call, phase I), `docs/architecture/plugin/rib-storage-design.md` (affected: FlowSpec validation imports the core package, phase D1), `docs/architecture/pool-architecture.md` (affected: pool names declared beside `AllPools`, pool-stats owner, phase G), `docs/architecture/wire/nlri.md` (affected: SR-Policy registration declares no ADD-PATH and the `update text` path goes through the registered encoder, phase F), `rfc/short/rfc5880.md` (unaffected: `bfd.go` changes one seam call, no BFD protocol behavior; anchor checked in phase H) |
| 17 | Existing docs show examples for this area? | Yes | SR-Policy `update text` examples checked against the plugin grammar |

## Implementation Steps

1. **Phase: Wiring** -- write the failing tests in the Wiring Test table that do not exist yet (filter metrics hook, attribute JSON decoder, pool-stats `.ci`, diag BFD seam, boot effect). Record the `go list -deps` baseline for AC-3.
2. **Phase A: redundant registrations** (edges 1-6, 8-16) -- delete the blank imports and `tree_bgp.go`; add `_test.go` imports where a package test needs them (A-2); correct the stale comments. Verify AC-3, AC-4.
3. **Phase C: filter metrics** (edge 7) -- optional metrics hook on `filterapi.Filter`, called by `filterapi.SetMetricsRegistry`; loop filter declares it; reactor drops the import. AC-5.
4. **Phase B: schema homes** (17-19) -- move each `-cmd/yang` package into `internal/component/<x>/yang/`, regenerate, delete the `-cmd` directories. AC-6.
5. **Phase D1: FlowSpec core** (25-30) -- move the wire vocabulary to `internal/core/bgp/nlri/flowspec`, plugin keeps `family.MustRegister`, config, JSON/text, hex codecs; update RIB and flowspec-firewall; correct the `plugin_encode_text.go` line-202 comment that places `parseFlowComponent` in the plugin's `types.go`. AC-7, AC-8, AC-15.
6. **Phase D2: BGP-LS core** (20-24) -- same for `internal/core/bgp/nlri/bgpls`, exporting exactly the inventory's "Newly exported by core" column (including `BGPLSNLRI`, the four label-range helpers and the two JSON keys); `register_attr.go` adapts core `DecodeSRCapabilities` returning a nil interface on error (A-4); `ls_export` uses core family constants (A-5). AC-7, AC-8.
7. **Phase E: attribute JSON decoder** (31) -- registry field keyed by attribute code, `bgp-nlri-ls` registers `AttrTLVsToJSON` for 29, decoder looks it up. AC-9, AC-10.
8. **Phase F: SR-Policy text** (32) -- write `TestSRPolicyTextMatchesPluginGrammar` over AC-11a to AC-11f first and record today's outcome for each row; validate A-8; then declare no ADD-PATH in the SR-Policy registration, and make `parseRegistryNLRISection`, when the registry says the section's family declared no ADD-PATH, refuse with the AC-11d error in exactly the three positions AC-11d lists and in no other: (1) the accumulator it was handed already carries `HasPathID` (a top-level `path-information`), tested on entry; (2) the token after the family is `path-information`, tested before the `add`/`del` check that would answer `ErrMissingAddDel`; (3) a `path-information` token inside the run `endsRegistrySection` keeps, tested before the run is handed to the encoder. A `path-information` inside another family's section never reaches this parser for the SR-Policy section, because each section gets its own accumulator copy, so it is accepted (owner decision Q-A, AC-11d control row). Make the encoder refuse `rd`/`label`, make the shared registry section parser refuse a later `add`/`del` for every registry family with the message AC-11b names (`TestRegistrySectionRefusesLaterAddDel`), and delete the SAFI 73 branch, the two SAFI 73 case lists and `update_text_srpolicy.go`. AC-11a to AC-11g.
9. **Phase G: pool-stats** (33) -- move handler and module into the rib owner; names declared beside the pools. AC-12.
10. **Phase H: BFD capture seam** (34) -- interface and setter into `bfd/api`; diag reads it. AC-13.
11. **Phase I: crash boot effect** (35) -- boot-effect field on `registry.Registration`; crashes registers it (A-9) with `HarvestAtBoot` and the not-writable warning; hub runs registered effects once between `applyHostTuning` and `startSmartManager` and drops `harvestKernelCrashes`; update `plugins.snapshot`, `docs/features/plugins.md` and the `docs/DESIGN.md` "Shipped Plugins" table with the one `crashes` row and correct the `docs/DESIGN.md` line-45 plugin count; write `TestAutoloadStartsNoCrashesEngine`. AC-14, AC-16.
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
| 4 | - | owed before implementation: the round-3 fixes above plus their sibling sites | - | - |

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
