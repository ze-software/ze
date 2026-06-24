# Spec: ospfv3-ext-8 -- OSPFv3 Debug & Introspection Tooling

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospfv3-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5340.md` -- OSPFv3 base: §A.3.1 16-byte common header (Instance ID), §A.4.2.1 scope-aware LS Type (U/S2/S1 + function code), §A.4 the eight base LSA types (Router 0x2001 .. Intra-Area-Prefix 0x2009), §A.4.1 IPv6 prefix encoding `((PrefixLength + 31) / 32)` words, §2.1 per-link/Interface-ID model
4. `plan/spec-ospfv3-0-umbrella.md` -- the delivered OSPFv3 base: the scope-aware LSA model, the per-AF engine (`eng6`), the reserved+validated Instance ID plumbing, the `show ipv6 ospf ...` diagnostic surface, the FIB-install-via-Loc-RIB path
5. `plan/spec-ospfv3-ext-0-umbrella.md` ext-8 row -- "Extension-wide debug/introspection for OSPFv3: decode + inspect v3 LSAs (including the RI / extended / SR / Grace LSAs added by ext-4/ext-6), inject test LSAs for functional testing, and the show/diagnostic surface for every v3 extension above"; the `ze_ospfv3_<ext>_*` metric contract and the `show ipv6 ospf <noun>` command-ownership model; the build-order rule (ext-8 last so it decodes the LSAs ext-4/ext-6 add)
6. `plan/spec-ospf-ext-14-debug-introspection.md` -- the OSPFv2 parallel of this spec (decode + explain + neighbor/interface detail + guarded inject + web/JSON/metrics); ext-8 mirrors its shape but for OSPFv3 native scope-aware LSAs (no opaque framework) and adds per-AF views + link-local/Interface-ID/Instance-ID state
7. `internal/plugins/ospf/cmd_show.go` -- the CENTRAL-namespace `ze-show:ospf-*` builtin-proxy RPCs (`RPCRegistration{WireMethod, Handler, PluginCommand}`, `ForwardToPlugin`, `dbSubviewForwarder`); the v6 equivalents register the same way against the v6 engine commands
8. `internal/plugins/ospf/show_database.go` -- `dbSubviewType` map + `databaseSnapshotByType`/`filterLSAsByType` over `lsdb.Snapshot()` (Areas + ASExternal + Links)
9. `internal/plugins/ospf/lsdb/lsdb.go` (~523) -- `Snapshot`/`AreaSnapshot`/`LinkSnapshot`/`LSASnapshot` (the v3 `LinkSnapshot` already carries `Interface` + `LinkLocalAddress`; `LSASnapshot` carries `Type`, `LinkStateID`, `AdvertisingRouter`, `Sequence`, `Age`, `Checksum`, `Length`, `Interface`, `LinkLocalAddress`)
10. `internal/plugins/ospf/neighbor/neighbor.go` (~138/~188/~220) -- `Neighbor`/`Snapshot` carry `DDSequence`, `Options`, `InterfaceID`; `internal/plugins/ospf/dispatcher.go` (~29) -- `instanceID uint8` + the §4.2.2 Instance-ID demux; `internal/plugins/ospf/v3/types/lsa.go` -- `LSType` (U/S2/S1 + function code), `Scope()`, `Known()`, `LSAKey`

## Task

Deliver first-class operational debugging and introspection for the OSPFv3
extension stack, mirroring the OSPFv2 `ospf-ext-14` surface but adapted to the
OSPFv3 wire model. The user explicitly wants the debug tooling kept and made as
good as possible: ext-8 is the operator's lens onto the OSPFv3 link-state
database, SPF, neighbor/interface state, and the address-family topologies, plus
a guarded test-LSA injection path for exercising flooding, reception, and
extension decode without a second router.

Unlike OSPFv2 (where extensions ride in Opaque LSAs, RFC 5250, decoded by
`ospf-ext-14`), **OSPFv3 carries every extension as a native scope-aware LSA**
(RFC 5340 §A.4.2.1: the LS Type embeds the U-bit and the S2/S1 flooding scope;
RFC 8362 E-LSAs, RI-LSAs, and the SR LSAs added by `ospfv3-ext-6`, plus the
Grace-LSA added by `ospfv3-ext-4`, are all native LS types, not opaque
carriers). ext-8 therefore decodes and inspects **v3 LSAs by LS Type and scope**,
not opaque type/ID. It is a deep, scope-aware, AF-aware introspection consumer
of the existing OSPFv3 LSDB / codec / SPF snapshots, plus one guarded write (test
LSA injection) that reuses the base origination seam (`OriginateSelf` /
`OriginateLinkSelf`) exactly as a real LSA origination would.

The OSPFv3 base (`plan/spec-ospfv3-0-umbrella.md`) already ships a read-only
diagnostic surface: `show ipv6 ospf`, `... neighbor`, `... interface`,
`... database` (with per-LS-type subviews), `... route`, the LSDB `Snapshot()`
(Areas + AS-external + per-interface Link-LSAs), the per-AF engine (`eng6`), the
neighbor/interface state (incl. the OSPFv3 Interface ID + Instance ID +
link-local address), and the SPF computer's last result. What is missing is the
**deep, extension-aware, AF-aware** debugging the operator needs once the v3 RI /
extended / SR / Grace LSAs flow: a way to decode a v3 LSA body into its typed
fields/TLVs (scope-aware), to inspect the SR / extended databases as first-class
views, to explain why an SPF route won (candidate list, tie-breaks), to dump a
neighbor or interface in full OSPFv3 state (link-local address, Interface ID,
Instance ID, DD seq, Options), to view each address family's topology
separately (ties to `ospfv3-ext-1`), and -- behind an explicit gate -- to inject
a crafted/test v3 LSA into the local LSDB so the flooding, reception, and
extension-decode paths can be exercised end-to-end without a second router.

ext-8 originates no protocol behaviour of its own except the guarded
debug-injection path, which reuses the base origination seam exactly as a real
LSA would. It adds NO new wire format, NO new LSA type, and NO SPF participation.
It is decode + inspect + explain + (gated) inject, plus the CLI/JSON/web surface
that exposes all of it. Every show command routes through `ApplyPipes` for
pipe-completeness, and discovery is first-class: each command self-documents its
dispatch key and appears in completion, closing the project's known CLI
dispatch-discovery gaps.

Each typed v3 LSA decoder (RI / extended-prefix-link / SR / Grace) is keyed by
LS Type and registered self-containedly: removing the owning consumer
(`ospfv3-ext-4` Grace, `ospfv3-ext-6` RI / extended / SR) removes its decoder
registration and its database view together, leaving the generic scope-aware
LSA-header + body-hex view as the fallback. ext-8 never spells a consumer's body
format inside generic code; it asks the registered decoder for a typed rendering
and falls back to a generic byte/TLV view when none is registered.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Deep, scope-aware LSDB inspection CLI | `show ipv6 ospf database <type>` extended with full per-LSA DECODE: each v3 LSA body rendered via its registered typed decoder (the base Router/Network/Inter-Area-Prefix/Inter-Area-Router/AS-External/NSSA/Link/Intra-Area-Prefix decoders, plus the RI / extended / SR / Grace decoders registered by ext-4/ext-6), falling back to a generic scope-aware header + body-hex view when no decoder is registered; per-area, per-LS-type, per-scope (link-local / area / AS), and per-Interface (for Link-LSAs) filtering |
| v3 LSA decode helper (offline) | An offline `ze` decode path that takes a v3 LSA (or full v3 packet) hex and renders the scope-aware LS Type (U/S2/S1 + function code), the 20-byte LSA header, and the typed/generic body, so a captured v3 LSA can be decoded without a running engine (extends the existing `internal/plugins/ospf/cli/decode.go` path with a v3 codec branch) |
| SPF compute trace / explain (per AF) | `show ipv6 ospf spf detail` (per-area, per-AF): the candidate vertices considered, the winning path per prefix, the cost composition, and the tie-break that selected it (read from the `spf` computer's route/candidate data); explains WHY a route won, not just THAT it did; AF/Instance-ID identified per result |
| Per-AF database / topology views | `show ipv6 ospf database` and the detail views are AF-aware: each result identifies its address family + Instance ID (the `ospfv3-ext-1` model maps Instance ID -> AF). A `show ipv6 ospf instance` listing enumerates the active OSPFv3 instances (AF, Instance ID, area count, neighbor count). Ties to `ospfv3-ext-1`: when only the base IPv6-unicast instance is configured the view degrades to a single instance |
| SR database view | `show ipv6 ospf database segment-routing`: SR-related v3 content (the SR-Algorithm / SRGB Router-Information-LSA TLVs and the Prefix-SID / Adjacency-SID in the extended LSAs, RFC 8665/8666 carried natively per RFC 8362) decoded into a Segment-Routing summary; empty (no error) until `ospfv3-ext-6` lands its decoder |
| RI / extended database view | `show ipv6 ospf database router-information` and `... extended`: the Router-Information-LSA and the E-Intra/E-Inter/E-AS extended LSAs decoded into named TLVs; empty until ext-6 registers its decoders |
| Neighbor / interface deep dump | `show ipv6 ospf neighbor detail` and `show ipv6 ospf interface detail`: the full per-neighbor state (the OSPFv3 link-local address as the neighbor identity, the advertised Interface ID, the negotiated Instance ID, DD seq, Options incl. v3 R/V6/E/N/AF bits, retransmission/request/summary list sizes, last-event, timers) and per-interface state (ISM, the local Interface ID, the Instance ID, DR/BDR election detail by Router ID, timers, link-local source) beyond the summary rows the base ships |
| Guarded v3 LSA injection / origination | A debug-only `debug ipv6 ospf inject lsa ...` command (and the equivalent in-process API) that builds a crafted v3 LSA (scope from the LS Type S2/S1 bits, LS Type + Link State ID + body TLVs or raw hex) and originates it into the local LSDB via the base `OriginateSelf` (area / AS scope) or `OriginateLinkSelf` (link-local scope) seam; withdraw via `debug ipv6 ospf inject lsa ... withdraw` (MaxAge flush through the existing purge path); OFF by default, denied by the read-only authz profile, and gated behind an explicit `debug` enablement |
| Structured JSON output | Every new show/debug command returns a typed snapshot rendered as JSON and routed through `ApplyPipes` (json/ndjson/table/text/yaml/match/count/resolve/origin/log/no-more) |
| Web / looking-glass surfacing | New read-only web/SSE views for the v3 database (per scope / per AF) and the SR/RI/extended databases via the generic `snapshot_views.go` adapter (mirroring `handler_ospf.go`/`handler_isis.go`); the injection path is NEVER surfaced on the web (CLI + authz only) |
| Metrics | `ze_ospfv3_debug_injected_lsas` (gauge), `ze_ospfv3_debug_injections_total` (counter), `ze_ospfv3_debug_decode_errors_total` (counter) |
| Discovery / dispatch | Each command's dispatch key is discoverable (help text names the key; the dispatch-key listing includes the new commands); no hidden RPC-name-only commands |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| The OSPFv3 base codec, LSDB, SPF, transport, neighbor FSM | `plan/spec-ospfv3-0-umbrella.md` (consumed, never redefined) |
| The multi-AF Instance-ID demux + AF-bit + per-AF install | `spec-ospfv3-ext-1-multi-af` (ext-8 READS the AF/Instance-ID identity it exposes; it does not implement the demux) |
| The RI-LSA / extended-LSA / SR TLV codecs themselves | `spec-ospfv3-ext-6` (ext-8 only DECODES + renders what ext-6 registers) |
| The Grace-LSA body + GR helper | `spec-ospfv3-ext-4` (ext-8 only decodes the Grace-LSA ext-4 registers) |
| Any SPF change | none -- ext-8 reads the SPF result, it does not alter the computation |
| A standalone `ospfclient`-style Unix-socket external-injection daemon | not done; ext-8 delivers inject/observe in-process instead (the FRR ospfclient capability the guide calls "not needed in production") |
| SNMP / OSPFv3 MIB (RFC 5643) | out of scope; ext-8 exposes equivalents via CLI/JSON/web only |
| Remote injection (inject into a peer's LSDB over the wire) | not done; injection is LOCAL-only into this router's own LSDB, then flooded by the normal base machinery |
| OSPFv2 opaque decode / inject | `spec-ospf-ext-14` (the OSPFv2 parallel; OSPFv3 has no opaque framework) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1558-1564 ("External LSA API (ospfclient)" + "SNMP and Operational Hooks") -- the FRR capability ext-8 replaces and the directive to expose the equivalent via Ze's own CLI and web/looking-glass
  → Decision: deliver the useful `ospfclient` capability (inject + observe LSAs for research/testing) in-process on the base origination seam; do NOT ship a Unix-domain-socket external-injection daemon (the guide calls it "not needed in production")
  → Constraint: the guide says "ze should expose the equivalent via its own CLI and via the web/looking-glass components" -- the read-only introspection (decode/inspect/explain) is surfaced on CLI + web; the inject path is CLI + authz only (never web)
- [ ] `plan/spec-ospfv3-ext-0-umbrella.md` ext-8 row, the `ze_ospfv3_<ext>_*` metric contract, the `show ipv6 ospf <noun>` command-ownership model, and the build-order rule
  → Constraint: ext-8 uses `ze_ospfv3_debug_*` metric names and `show ipv6 ospf ...` / `debug ipv6 ospf ...` command nouns; it renames NO existing OSPFv3 series or command
  → Decision: ext-8 depends ONLY on the base (`spec-ospfv3-0-umbrella`); the typed decoders for RI / extended / SR (ext-6) and Grace (ext-4) are OPTIONAL and resolved at runtime via the decoder registry, so ext-8 builds and ships before ext-4/ext-6 do, degrading to the generic scope-aware header + body-hex rendering
- [ ] `plan/spec-ospf-ext-14-debug-introspection.md` (the OSPFv2 parallel) "In scope", Data Flow, Wiring Test -- the decode/explain/detail/inject/web surface shape ext-8 mirrors
  → Decision: ext-8 reuses ext-14's architecture (decoder registry, SPF-explain snapshot, neighbor/interface detail, guarded inject via the origination seam, generic web snapshot-view adapter, the three `*_debug_*` metrics) but keyed by v3 LS Type + scope (not opaque type/ID) and AF-aware
  → Constraint: the OSPFv2/OSPFv3 engines share `internal/plugins/ospf`; ext-8 MUST register its v6 commands / decoders / web views without clobbering the v4 ones ext-14 owns (distinct `ze-show:ospfv3-*` wire methods and `show ipv6 ospf ...` command nouns)
- [ ] `ai/rules/cli-grammar.md` -- keyword-before-value; typed selectors
  → Constraint: every new command places a closed keyword before any value; per-LS-type filtering uses a typed selector (`type <ls-type>`), per-scope a typed selector (`scope <link|area|as>`); the inject command is `debug ipv6 ospf inject lsa scope <scope> type <ls-type> id <link-state-id> ...` (action/keywords before values, never a free-form positional)
  → Constraint: injection is a runtime operational debug action (not a config-tree mutation), so it correctly takes an operational verb (`debug ... inject`), not `set`/`delete`
- [ ] `ai/rules/pipe-completeness.md` -- every command that produces output supports all pipe operators
  → Constraint: each new show/debug command routes its JSON snapshot through `ApplyPipes`; data-transform pipes (`resolve`/`origin`) apply to the IP-bearing fields (advertising router, IPv6 prefixes, next-hops, link-local addresses)
- [ ] `ai/rules/plugin-self-containment.md` -- removing a plugin removes ALL its features
  → Constraint: each typed decoder + its database view is registered by its owning consumer (ext-4 Grace, ext-6 RI / extended / SR) through a small ext-8 decoder-registry; removing the consumer removes its decoder and view; generic scope-aware header + body-hex rendering remains; ext-8 generic code spells no consumer body format
- [ ] `ai/rules/no-sprintf-alloc.md`, `ai/rules/buffer-first.md` -- rendering uses `textbuf`/`AppendTo`, decode is zero-copy over the LSDB bytes
  → Constraint: all rendering uses `textbuf.Buffer`; the body decode returns views over the LSDB raw bytes, no per-field allocation in the hot path; an injected LSA body is built buffer-first

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5340.md` -- OSPF for IPv6, the base protocol the introspection surfaces
  → Constraint: §A.4.2.1 -- the LS Type embeds the U-bit (unknown handling) and the S2/S1 flooding scope (00 link-local, 01 area, 10 AS, 11 reserved) plus a 13-bit function code; the decode view renders all four subfields and the per-scope filter keys on S2/S1, NOT on a flat type number
  → Constraint: §A.4 -- the eight base LS types (Router 0x2001, Network 0x2002, Inter-Area-Prefix 0x2003, Inter-Area-Router 0x2004, AS-External 0x4005, NSSA 0x2007, Link 0x0008, Intra-Area-Prefix 0x2009); the database detail view names each and routes Link-LSAs (0x0008, link-local scope) to the per-interface store
  → Constraint: §A.4.1 -- IPv6 prefixes are `PrefixLength` + `PrefixOptions` + `((PrefixLength + 31) / 32)` 32-bit words; the decode view renders the prefix and validates the byte length / non-zero padding (a malformed prefix is shown as raw, never panics)
  → Constraint: §A.3.1 / §2.1 -- the 16-byte common header carries the 8-bit Instance ID (link-local significance); the neighbor/interface detail and the AF-aware views surface the Interface ID (32-bit, §A.4.3) and the Instance ID, NOT IPv4 subnet identity; neighbor identity is the link-local address + Router ID
  → Constraint: §A.4.2 -- the 20-byte LSA header (LS Age, LS Type, Link State ID, Advertising Router, LS Sequence Number, LS Checksum, Length); the deep database view renders the header so the operator can reason about freshness/aging exactly as the base §13.1 compare does; an injected LSA at MaxAge is shown as flushing
- [ ] `rfc/short/rfc5838.md` -- AF-to-Instance-ID ranges (the AF identity ext-8 surfaces per result)
  → Constraint: §2 -- each address family is a separate OSPFv3 instance with its own LSDB; ext-8's AF-aware views identify each result's AF by its Instance-ID range (IPv6u 0-31, IPv6m 32-63, IPv4u 64-95, IPv4m 96-127), reading the identity `ospfv3-ext-1` establishes; ext-8 does NOT implement the demux

**Key insights:** (minimal context to resume after compaction)
- OSPFv3 has NO opaque framework: every extension is a native scope-aware LSA (RFC 5340 §A.4.2.1), so ext-8 keys decode/inspect on LS Type + S2/S1 scope, not opaque type/ID. This is the headline difference from the OSPFv2 ext-14 parallel.
- ext-8 is a READ surface plus ONE guarded WRITE (debug inject); both go through the base's existing origination seams (`OriginateSelf` for area/AS, `OriginateLinkSelf` for link-local). No new wire format, no SPF change.
- Typed decoders are optional and runtime-resolved: ext-8 ships and works (generic scope-aware header + body-hex + base LSA decode + base introspection) before ext-4/ext-6 land; a typed RI/extended/SR/Grace view fills in when its consumer registers a decoder.
- Views are AF-aware: each result identifies its address family + Instance ID (the `ospfv3-ext-1` Instance-ID -> AF mapping); with only the base IPv6-unicast instance configured, the view is a single instance.
- Discovery is a first-class requirement: the new commands self-document their dispatch keys and appear in completion, deliberately NOT reproducing the project's known CLI dispatch-discovery gaps.
- The inject path is OFF by default, denied by the read-only authz profile, gated by an explicit `debug` enablement, LOCAL-only (this router's LSDB, then normal flooding), and never surfaced on the web.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/cmd_show.go` -- the CENTRAL-namespace `ze-show:ospf-*` builtin-proxy RPCs: each `RPCRegistration{WireMethod, Handler, PluginCommand}` declares the plugin command it fronts; `forwardToOSPF` rejects extra args and calls `ForwardToPlugin` (the LDP/IS-IS proxy model); `dbSubviewForwarder` is the closure used for the `show ip ospf database <type>` subviews
  → Constraint: ext-8 adds new `ze-show:ospfv3-*` proxies here (database detail/sr/ri/extended subviews, instance, spf-detail, neighbor-detail, interface-detail) and a new `ze-debug:ospfv3-inject` proxy; each MUST declare its PluginCommand (a `show ipv6 ospf ...` / `debug ipv6 ospf ...` string) and forward via `ForwardToPlugin`, never re-Dispatch (that recurses); the v6 wire methods are DISTINCT from the v4 ones ext-14 owns
- [ ] `internal/plugins/ospf/register.go` (~222 the `eng6` v6 engine spawn, the `OnExecuteCommand` switch, the `sdk.Registration.Commands`) -- the engine-side command switch maps each `show ipv6 ospf ...` string to an engine snapshot method; `sdk.CommandDecl` lists every command the engine claims; `eng6 := newEngineWithCodec(ospfv3transport.New(...), v6Codec{})` is the IPv6 (OSPFv3) family
  → Constraint: ext-8 adds new `case` arms (one per new v6 command) returning a typed snapshot, plus matching `CommandDecl` rows; the inject command's arm calls the base origination seam guarded by the debug-enabled flag; the v6 engine is the one whose LSDB/SPF/neighbor state these views read
- [ ] `internal/plugins/ospf/show_database.go` -- `dbSubviewType` maps `show ip ospf database <type>` to an `LSASnapshot.Type` string; `databaseSnapshotByType` filters the LSDB `Snapshot()` per area + AS-external; `filterLSAsByType` is the filter
  → Constraint: ext-8 adds a v6 subview map (router/network/inter-area-prefix/inter-area-router/external/nssa/link/intra-area-prefix + ri/extended/segment-routing) and a v6 `databaseSnapshotByType` that ALSO filters the per-interface `Links` store (Link-LSAs are link-local scope) and enriches each LSA with a typed/generic body rendering; the existing filter contract and the v4 map are untouched
- [ ] `internal/plugins/ospf/lsdb/lsdb.go` (~472 `Snapshot()`, ~523 `Snapshot`/`AreaSnapshot`/`LinkSnapshot`/`LSASnapshot`) -- the snapshot already carries `Areas`, `ASExternal`, and (for v3) `Links []LinkSnapshot` (each with `Interface`); `LSASnapshot` carries `Type`, `LinkStateID`, `AdvertisingRouter`, `Sequence`, `Age`, `Checksum`, `Length`, and the v3-only `Interface` + `LinkLocalAddress`
  → Constraint: the deep database view reads `Snapshot()` and enriches each `LSASnapshot` with a decoded body; the `Links` slice is the link-local (Link-LSA) store ext-8's per-scope filter must include; ext-8 reads this snapshot, it does not change its shape (base tests pin it)
- [ ] `internal/plugins/ospf/v3/types/lsa.go` -- `LSType` (16-bit: U-bit | S2 | S1 | 13-bit function code); `Scope()` returns the S2/S1 flooding scope; `Known()` reports the eight base types; `LSAKey` is `(LS Type, Link State ID, Advertising Router)`; the base type constants `LSTypeRouter` (0x2001) .. `LSTypeIntraAreaPrefix` (0x2009)
  → Constraint: the scope-aware decode/filter is built on `LSType.Scope()` and the function-code split; ext-8 reads these, it does not redefine the type model; the inject command parses an LS Type and derives its scope from S2/S1, validating S2/S1 != 11 (reserved)
- [ ] `internal/plugins/ospf/v3/packet/lsa.go` + the per-type `lsa_*.go` (router/network/interarea_prefix/interarea_router/external/nssa/link/intraarea_prefix) -- the v3 LSA decoders the base ships; each renders its body; `internal/plugins/ospf/v3/packet/prefix.go` decodes the RFC 5340 §A.4.1 IPv6 prefix
  → Constraint: the base v3 LSA types decode into typed bodies already; ext-8's database detail view calls these base decoders (registered as the default decoders for the eight base types) and falls back to header + body-hex for unknown function codes; ext-8 adds NO base codec
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` (~138 `NeighborInfo`-style fields, ~188 `Neighbor`, ~220 `Snapshot`) -- `Neighbor`/`Snapshot` carry `DDSequence`, `Options`, and the v3 `InterfaceID` (zero for OSPFv2); `Address` is the neighbor identity (link-local for v3); the NSM state
  → Constraint: the neighbor-detail snapshot reads these (DD seq, Options incl. the v3 R/V6/E/N/AF bits, the advertised Interface ID, the link-local address) and the per-neighbor list sizes; ext-8 adds a DETAIL snapshot without changing the existing `Snapshot` shape (base tests pin it)
- [ ] `internal/plugins/ospf/dispatcher.go` (~29 `instanceID uint8`, ~64 the §4.2.2 demux) -- each engine instance carries its OSPFv3 Instance ID; `dispatch()` drops a packet whose `h.InstanceID != instanceID`
  → Constraint: the AF-aware views read `instanceID` (and the `ospfv3-ext-1` Instance-ID -> AF mapping when present) to identify each result's family; the interface-detail view surfaces the local Instance ID; ext-8 reads this, it does not change the demux
- [ ] `internal/plugins/ospf/iface/ism.go` + `iface/election.go` + `iface/iface.go` -- the ISM states (Down/Loopback/Waiting/PointToPoint/DROther/Backup/DR), `electDRBDR` (by Router ID for v3, RFC 5340 §4.2), the AF-neutral interface info (`InterfaceID` for v6)
  → Constraint: the interface-detail snapshot reads the ISM state, the DR/BDR election detail (by Router ID), the local Interface ID, and the timers; ext-8 adds a detail snapshot, it does not change the ISM/election
- [ ] `internal/plugins/ospf/spf/computer.go` + `spf/route.go` -- `Snapshot()`/`SPFSnapshot()`/route rows; the last computed result (`last`); `BuildRoutes`/`selectBestRoutes` build candidates and do the per-prefix best-path compare; the v6 strategy (`v6Strategy`) builds the graph from address-free v3 Router/Network LSAs and attaches Intra-Area-Prefix prefixes
  → Constraint: the SPF-explain view reads the last result + the candidate/tie-break data WITHOUT changing the existing `SPFSnapshot` shape (base tests pin it) and WITHOUT re-running SPF; it parallels the ext-14 `spf/explain.go` (shared package) but identifies the AF/Instance-ID per result
- [ ] `internal/plugins/ospf/cli/decode.go` + `cli/register.go` + `cli/run.go` -- the offline `ze ospf-decode` subcommand (hex -> decoded OSPFv2 packet JSON via `packet.DecodePacket`)
  → Constraint: ext-8 extends the offline decode path with a v3 branch (`ze ospfv3-decode` or a `--version 3` flag) that decodes a v3 packet/LSA via the v3 codec and renders the scope-aware LS Type + header + typed/generic body offline (no running engine), reusing the decoder registry
- [ ] `internal/plugins/ospf/doctor.go` -- the two config-sanity doctor codes (`doctor-ospf-router-id-missing`, `doctor-ospf-interface-area-unbound`); the file explicitly owns ONLY those two and must not re-register the ospf-3 raw-socket check
  → Constraint: ext-8 adds at most a debug-enabled-sanity doctor note (a Warning when debug-injection is left enabled) registered with its own code; it must NOT touch the existing two codes
- [ ] `internal/component/authz/authz.go` (~217 `Authorize(command, isReadOnly)`, ~252 `BuiltinReadOnlyProfile`) -- profile-based command-path allow/deny; the read-only profile denies edit-section verbs; `Authorize` walks ordered allow/deny entries, fail-closed
  → Constraint: the read-only profile gains a `deny "debug"` entry (shared with ext-14; if ext-14 already added it, ext-8 reuses it and asserts it) so the inject command is denied for read-only users; the inject command is ALSO gated by an engine-side `debug` enablement (off by default), so authz + enablement are BOTH required
- [ ] `internal/component/web/handler_ospf.go` + `snapshot_views.go` -- the generic read-only `viewSpec{command, title, streamPath, eventName}` snapshot adapter; OSPF neighbor/database web+SSE views forward `show ip ospf ...` through the `CommandDispatcher` (the `//nolint:dupl` parallel-per-protocol adapter mirroring `handler_isis.go`)
  → Constraint: ext-8 adds v6 database web views (per scope / per AF, plus sr/ri/extended) by adding `viewSpec` rows + handlers in `handler_ospf.go` (or a sibling `handler_ospfv3.go`) forwarding `show ipv6 ospf ...`; the inject command is NEVER added as a web view

**Behavior to preserve:**
- The existing `show ipv6 ospf ...` command set, their JSON snapshot shapes, the `ze-show:ospf-*`/`ze-show:ospfv3-*` proxy contract, and the base tests that pin them.
- `SPFSnapshot`/`Snapshot`/`LSASnapshot`/`LinkSnapshot` shapes (base + ospfv3-0 tests); the SPF-explain and database-detail views are ADDITIVE.
- The OSPFv2 ext-14 surface (decode/explain/inject/web) -- ext-8 mirrors it for v6 without clobbering the v4 wire methods, commands, decoders, or web views.
- The two existing doctor codes; the read-only authz profile's existing deny entries (ext-8 reuses any `deny debug` ext-14 added, it does not duplicate it).
- The web neighbor/database views and the looking-glass graph.
- The base v3 LSA codec and the v6 SPF strategy (ext-8 reads, never re-implements).

**Behavior to change:** (all additive; no existing behaviour altered)
- New `show ipv6 ospf` subcommands: `database <type> detail` (scope-aware decode), `database scope <link|area|as>`, `database segment-routing`, `database router-information`, `database extended`, `instance`, `neighbor detail`, `interface detail`, `spf detail`.
- New `debug ipv6 ospf inject lsa ...` command (and engine API), OFF by default, authz-denied for read-only, gated by a `debug` enablement.
- `BuiltinReadOnlyProfile` gains (or reuses) a `deny "debug"` entry.
- New web/SSE views for the v6 database (per scope / per AF) and the sr/ri/extended databases.
- New `ze_ospfv3_debug_*` metrics.
- The offline `ze` OSPF decode path gains a v3 branch.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Read (introspection):** operator runs `show ipv6 ospf <noun> [detail|type ...|scope ...]` (CLI/SSH, web, or looking-glass) → the central `ze-show:ospfv3-*` proxy → `ForwardToPlugin` → the v6 engine `OnExecuteCommand` switch → a typed snapshot method → JSON → `ApplyPipes` → rendered.
- **Decode (offline):** operator runs the `ze` OSPFv3 decode subcommand on v3 LSA/packet hex → `cli/decode.go` (v3 branch) → v3 codec (`v6Codec`/`ospfv3packet`) → scope-aware LS Type + registered typed decoder or generic body-hex → rendered, with no running engine.
- **Inject (guarded write):** operator runs `debug ipv6 ospf inject lsa scope <s> type <ls-type> id <link-state-id> [tlv ...|hex ...]` → authz check (`deny debug` for read-only) → the `ze-debug:ospfv3-inject` proxy → v6 engine switch → debug-enabled gate → the base `OriginateSelf` (area/AS) or `OriginateLinkSelf` (link-local) seam (the base assigns sequence, installs, floods per scope) → `ze_ospfv3_debug_*` metrics update.

### Transformation Path
1. **Snapshot (read):** the v6 engine method assembles a typed value snapshot from existing state: `lsdb.Snapshot()` (Areas + ASExternal + Links) for database views, the `spf` computer for SPF-explain, the neighbor/interface tables for the detail dumps, the per-engine `instanceID` (+ ext-1 AF map) for AF identity. No new state is created.
2. **Scope-aware body decode enrichment:** for a v3 LSA, the view derives the scope from `LSType.Scope()` (S2/S1) and looks up the decoder registry by LS Type / function code. A registered decoder (the base eight, or the RI/extended/SR/Grace decoders ext-4/ext-6 register) renders the body into named fields/TLVs; else a generic view yields the 20-byte header subfields + the body as length/hex. A malformed body (e.g. a bad RFC 5340 §A.4.1 prefix length) increments `ze_ospfv3_debug_decode_errors_total` and renders as raw hex, never panicking.
3. **SPF-explain (per AF):** the detail view reads the last SPF result (winning route per prefix + per-area state) and the candidate/tie-break data, composing a per-prefix explanation tagged with the AF/Instance-ID: candidate paths considered, each candidate's cost composition, and the rule that selected the winner.
4. **AF-aware identification:** each database / SPF / instance result is tagged with its address family (from the Instance-ID range, via the `ospfv3-ext-1` map; default IPv6-unicast when only the base instance is configured) and Instance ID; `show ipv6 ospf instance` enumerates the active instances.
5. **Render:** the snapshot is marshalled to JSON and routed through `ApplyPipes`; data-transform pipes (`resolve`/`origin`) decorate IP-bearing fields (advertising router, IPv6 prefixes, link-local next-hops).
6. **Inject (write):** the v6 engine validates the LS Type (S2/S1 != 11 reserved), the scope it implies, the Link State ID, and the body (a TLV/prefix list built buffer-first, or raw hex), and -- only if the `debug` enablement is on -- calls the base origination seam: `OriginateSelf` for area/AS scope, `OriginateLinkSelf` for link-local scope (per the LS Type S2/S1 bits). The base owns sequencing/install/flood; ext-8 records the injected `(scope, LS Type, Link State ID)` for later withdraw. `withdraw` re-originates at MaxAge via the same seam.
7. **Web/SSE (read only):** the generic snapshot-view adapter re-fetches the v6 database / sr / ri / extended snapshot every refresh interval and pushes it over SSE; the inject path is absent from the web surface.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/web/LG ↔ v6 engine | the central `ze-show:ospfv3-*` / `ze-debug:ospfv3-*` proxy → `ForwardToPlugin` → v6 engine `OnExecuteCommand` (no re-Dispatch) | [ ] |
| v6 engine ↔ base origination seam (inject) | `OriginateSelf` (area/AS) / `OriginateLinkSelf` (link-local) builds + floods the injected v3 LSA; withdraw via MaxAge | [ ] |
| v3 LSA body ↔ typed decoder | the ext-8 decoder registry (keyed by LS Type / function code); fallback to a generic scope-aware header + body-hex view | [ ] |
| v6 engine ↔ SPF computer | read-only access to the last result + candidate/tie-break data for the explain view | [ ] |
| v6 engine ↔ Instance-ID / AF map | read the per-engine `instanceID` (+ ext-1 AF map) to tag each result's address family | [ ] |
| Authz ↔ inject command | the read-only profile denies `debug`; the engine debug-enablement gate is a second, independent check | [ ] |
| v6 engine ↔ web/SSE | the generic `snapshot_views.go` adapter forwards read-only database commands; inject is never wired here | [ ] |

### Integration Points
- `internal/plugins/ospf/cmd_show.go` -- new `ze-show:ospfv3-*` + `ze-debug:ospfv3-inject` proxies (distinct from the v4 ones).
- `internal/plugins/ospf/register.go` -- new v6-engine `OnExecuteCommand` arms + `CommandDecl` rows; the inject arm calls the base seam.
- `internal/plugins/ospf` (v6 engine) -- the decoder registry, the SPF-explain snapshot, the neighbor/interface detail snapshots, the AF-aware instance listing, the inject API, the debug-enablement flag.
- `internal/plugins/ospf/v3/packet` + `v3/types` -- the base v3 LSA decoders + the scope-aware `LSType`/`Scope()` (consumed, not redefined).
- `internal/plugins/ospf/cli/decode.go` -- offline v3 decode branch.
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- new command-tree nodes binding the new wire methods.
- `internal/component/authz/authz.go` -- the read-only profile `deny debug` entry (shared with ext-14).
- `internal/component/web/handler_ospf.go` -- the new read-only v6 database web views.
- `spec-ospfv3-ext-1` (Instance-ID -> AF map) -- consumed read-only for AF identification.
- `spec-ospfv3-ext-4` (Grace decoder) / `spec-ospfv3-ext-6` (RI / extended / SR decoders) -- each OPTIONALLY registers a typed decoder + database view through the ext-8 decoder registry (their own specs own that call).

### Architectural Verification
- [ ] No bypassed layers (reads flow CLI/web → proxy → v6 engine snapshot; inject flows command → authz + debug gate → base origination seam → normal flooding; no second flooding path)
- [ ] No unintended coupling (ext-8 names no consumer body format in generic code; decoders are registry-resolved; SPF access is read-only; the v4 ext-14 surface is untouched)
- [ ] No duplicated functionality (reuses `databaseSnapshotByType` shape, the `spf` computer snapshots + the shared `spf/explain.go`, the central proxy pattern, the base v3 codec, the generic web snapshot-view adapter, the ext-14 inject/decode architecture)
- [ ] Zero-copy preserved (body decode returns views over LSDB bytes; rendering is `textbuf`; inject body is buffer-first)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The base exposes an `OriginateSelf` (area/AS) and `OriginateLinkSelf` (link-local) origination seam ext-8 can drive to inject a v3 LSA of any scope, and the v3 codec + `LSType.Scope()` for scope-aware decode | `internal/plugins/ospf/origination_v6.go` (`v6OriginateSelf`/`OriginateSelf`), `origination_v6_link.go` (`OriginateLinkSelf`), `v3/types/lsa.go` (`Scope()`) | injection/decode need new base work; scope creep into the base | `TestDebugInjectV3LSAFloods`, `TestV3DecodeGenericBody` | unvalidated |
| A-2 | The `spf` computer retains (or can cheaply re-expose) the candidate set + per-prefix winning reason for the v6 explain view without re-running SPF, and the shared `spf/explain.go` (ext-14) is AF-agnostic | `internal/plugins/ospf/spf/computer.go`/`route.go`; the ext-14 `spf/explain.go` shared package | the explain view must re-run SPF or store extra state; larger change | `TestV3SPFExplainCandidateList`, `TestV3SPFExplainNoRecompute` | unvalidated |
| A-3 | The central `ze-show:ospf-*` proxy + the v6 engine `OnExecuteCommand` switch accept new commands additively (new `RPCRegistration` + new case + new `CommandDecl`) with no change to the proxy contract, and v6 wire methods do not collide with the v4 ones | `cmd_show.go`, `register.go` (`eng6`, the switch) | a new dispatch mechanism is needed, or v4/v6 commands collide | `TestShowOSPFv3DatabaseDetailWired`, `TestDebugInjectV3Wired` | unvalidated |
| A-4 | The read-only authz profile + an engine debug-enablement flag together gate the inject path; a read-only user is denied and an unconfigured router cannot inject | `authz.go` `BuiltinReadOnlyProfile`/`Authorize`; the inject command path | injection is reachable by an unprivileged user or a default router; security regression | `TestV3InjectDeniedReadOnly`, `TestV3InjectRequiresDebugEnabled` | unvalidated |
| A-5 | ext-8 builds and runs with NEITHER ext-4 NOR ext-6 present (typed RI/extended/SR/Grace decoders are runtime-optional; the base eight LSA decoders + generic body-hex are the fallback) | `plan/spec-ospfv3-ext-0-umbrella.md` (ext-8 depends only on base); the decoder-registry design | ext-8 hard-depends on ext-4/ext-6 and cannot ship before them | `TestV3DecodeFallbackNoDecoder`, build with only the base present | unvalidated |
| A-6 | The generic `snapshot_views.go` web adapter renders the new v6 database snapshots without a bespoke template (it forwards a `show ipv6 ospf ...` command and renders JSON) | `handler_ospf.go`, `snapshot_views.go`, `handler_isis.go` (the dupl-marked parallel adapter) | each new web view needs custom templating; more work | `TestOSPFv3DatabaseWebView`, web view e2e | unvalidated |
| A-7 | The LSDB `Snapshot()` already exposes the per-interface Link-LSA store (`Links []LinkSnapshot`) and the v3-only `Interface`/`LinkLocalAddress` on `LSASnapshot`, so the per-scope (link-local) filter and the neighbor/interface link-local detail read existing fields | `internal/plugins/ospf/lsdb/lsdb.go` (~523 the snapshot types) | the link-local scope view needs new snapshot plumbing | `TestV3DatabaseScopeFilter`, `TestV3InterfaceDetailSnapshot` | unvalidated |
| A-8 | The `ospfv3-ext-1` Instance-ID -> AF mapping is available (when ext-1 is present) and degrades to a single IPv6-unicast instance when absent, so AF-aware views work with or without ext-1 | `spec-ospfv3-ext-1-multi-af` (the AF map); `dispatcher.instanceID` | AF identification breaks when ext-1 is absent, or hard-depends on ext-1 | `TestV3InstanceListingSingleAF`, `TestV3InstanceListingMultiAF` | unvalidated |
| A-9 | An injected v3 LSA originated through the base seam inherits the base MinLSInterval pacing and the base never panics on a malformed self-LSA body (built buffer-first by ext-8) | base origination reuse; RFC 5340 base aging/pacing; `origination_v6.go` | a debug inject loop can DoS the LSDB or a bad body crashes the engine | `TestV3InjectRespectsMinLSInterval`, `TestV3InjectMalformedBodyRejected` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The inject path is reachable by an unprivileged operator or enabled by default → a crafted LSA is flooded into the live AS | a read-only user injects; a fresh router floods a test LSA | TWO independent gates (read-only authz `deny debug` + engine `debug` enablement off by default), LOCAL-only; `TestV3InjectDeniedReadOnly`, `TestV3InjectRequiresDebugEnabled`; doctor Warning when left enabled |
| R-2 | A malformed v3 LSA body (decode) or a bad inject body crashes the engine | fuzz crash; a panic in a database view (e.g. a bad RFC 5340 §A.4.1 prefix length) | decode is bound-checked over the LSDB bytes (never panics); inject validates before origination; `ze_ospfv3_debug_decode_errors_total`; `TestV3DecodeMalformed`, `TestV3InjectMalformedBodyRejected` |
| R-3 | The SPF-explain view forces an SPF re-run or mutates the installed result | route churn correlated with running `show ipv6 ospf spf detail`; a benchmark regression | the explain view is strictly read-only over the last result + candidate data; `TestV3SPFExplainNoRecompute` asserts the route table is untouched and the SPF run-count unchanged |
| R-4 | A new command exists but its dispatch key is undiscoverable (reproduces the known CLI dispatch-discovery gap) | the command works only if you already know the RPC name; help shows the RPC name not the dispatch key | each command's YANG node carries operator help naming the dispatch key; the new commands appear in completion + the dispatch-key listing; `TestV3NewCommandsDiscoverable` |
| R-5 | A typed decoder names a consumer body format inside ext-8 generic code → removing the consumer breaks the build | a grep finds `ri`/`sr`/`grace`/`extended` body structs referenced in generic ext-8 files | decoders register through the ext-8 registry from the consumer's own package; generic code only calls the registry interface + the base codec; `TestV3DecodeFallbackNoDecoder` + a self-containment grep |
| R-6 | The inject command is surfaced on the web (a remote, possibly unauthenticated, write path) | an `/ospfv3/inject` route appears; a web test exercises injection | the web adapter wires ONLY read-only `viewSpec` rows; inject is CLI + authz only; a test asserts no inject web route exists |
| R-7 | A pipe operator (e.g. `resolve`/`origin`) is unsupported on a new command → inconsistent operator experience | `show ipv6 ospf database segment-routing \| json` works but `\| resolve` errors | every new command routes through `ApplyPipes`; `TestV3NewCommandsPipeComplete` exercises each operator |
| R-8 | The injected LS key is not tracked, so `withdraw` cannot find the instance and a test LSA lingers | a `withdraw` returns "not found" while the LSA is still in the database | ext-8 records each injected `(scope, LS Type, Link State ID)`; `withdraw` re-originates at MaxAge via the base purge path; `TestV3InjectWithdrawFlushes` |
| R-9 | The v6 commands/decoders/web views clobber the v4 ext-14 ones (shared `internal/plugins/ospf` package) | a v4 ext-14 test fails after ext-8 lands; a duplicate wire-method registration panics at init | distinct `ze-show:ospfv3-*` wire methods + `show ipv6 ospf ...` nouns; a snapshot test pins the full v4+v6 command/decoder/web inventory; `TestV3CommandsDistinctFromV4` |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show ipv6 ospf database router detail` from the CLI | → | central proxy → v6 engine arm → v3 LSA snapshot enriched via the decoder registry / generic body view | `TestShowOSPFv3DatabaseDetailWired` (unit) + `test/ospfv3/ospfv3-debug-decode.ci` |
| `show ipv6 ospf spf detail` from the CLI | → | v6 engine arm → SPF-explain snapshot from the `spf` computer's last result + candidates, AF-tagged | `TestV3SPFExplainWired` (unit) + `test/ospfv3/ospfv3-debug-spf-explain.ci` |
| `show ipv6 ospf instance` from the CLI | → | v6 engine arm → AF-aware instance listing (AF, Instance ID, areas, neighbors) | `TestV3InstanceListingWired` (unit) + `test/ospfv3/ospfv3-debug-instance.ci` |
| `debug ipv6 ospf inject lsa scope area type 0x2009 id ... hex ...` as an authorized debug-enabled operator | → | authz allow → `ze-debug:ospfv3-inject` proxy → v6 engine arm → debug gate → base `OriginateSelf` → install + flood | `TestDebugInjectV3Wired` (unit) + `test/ospfv3/ospfv3-debug-inject.ci` |
| `debug ipv6 ospf inject ...` as a read-only operator | → | authz `deny debug` rejects before the engine is reached | `TestV3InjectDeniedReadOnly` (unit) |
| GET `/ospfv3/database` (web) | → | generic snapshot-view adapter forwards `show ipv6 ospf database` and renders JSON | `TestOSPFv3DatabaseWebView` (unit) + web e2e |
| `ze` OSPFv3 decode of a v3 LSA/packet hex (offline) | → | `cli/decode.go` v3 branch → v3 codec → decoder registry / generic body view | `test/ospfv3/ospfv3-debug-decode-offline.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show ipv6 ospf database router detail` (a base LS type) | each LSA's body is rendered as named typed fields; the 20-byte LSA header (LS age, scope-aware LS Type incl. U/S2/S1 + function code, Link State ID, Advertising Router, seq, checksum, length) is shown |
| AC-2 | `show ipv6 ospf database <type> detail` for an LS Type with NO registered decoder | the body is rendered via the generic scope-aware view (header subfields + body length/hex); a malformed body renders as raw hex, increments `ze_ospfv3_debug_decode_errors_total`, and never panics |
| AC-3 | `show ipv6 ospf database router-information` after ext-6 registers its RI decoder | RI-LSAs are listed and decoded into named TLVs; before ext-6 lands, the view is empty (no error) |
| AC-4 | `show ipv6 ospf database segment-routing` after ext-6 lands | SR-related content (SR-Algorithm / SRGB / Prefix-SID / Adjacency-SID, carried in the v3 RI + extended LSAs) is summarised; before ext-6 lands, the view is empty (no error) |
| AC-5 | `show ipv6 ospf database scope <link\|area\|as>` | only LSAs whose LS Type S2/S1 bits match the requested scope are listed (link-local includes the per-interface Link-LSA store); a reserved scope (S2/S1 = 11) is rejected |
| AC-6 | `show ipv6 ospf spf detail` for an area | the candidate vertices/paths considered, the winning path per prefix, the cost composition, the tie-break that selected it, and the AF/Instance-ID are shown; the route table and SPF run-count are UNCHANGED (read-only) |
| AC-7 | `show ipv6 ospf neighbor detail` | per-neighbor full OSPFv3 state (link-local address as identity, advertised Interface ID, negotiated Instance ID, DD seq, Options incl. R/V6/E/N/AF bits, retransmission/request/summary list sizes, last event, timers) beyond the summary view |
| AC-8 | `show ipv6 ospf interface detail` | per-interface full state (ISM, local Interface ID, Instance ID, DR/BDR election detail by Router ID, timers, link-local source) beyond the summary view |
| AC-9 | `show ipv6 ospf instance` | each active OSPFv3 instance is listed with its address family (from the Instance-ID range), Instance ID, area count, and neighbor count; with only the base IPv6-unicast instance configured, exactly one instance is listed |
| AC-10 | `debug ipv6 ospf inject lsa scope area type <ls-type> id <link-state-id> hex <body>` as an authorized operator with debug enabled | a crafted v3 LSA (the given scope/LS Type/Link State ID + body) is originated into the local LSDB via the base `OriginateSelf`/`OriginateLinkSelf` seam, installed, and flooded per scope; `ze_ospfv3_debug_injections_total` and `ze_ospfv3_debug_injected_lsas` update |
| AC-11 | `debug ipv6 ospf inject lsa scope area type <ls-type> id <link-state-id> withdraw` for a previously injected LSA | the instance is MaxAge-flushed via the base purge path so peers withdraw it; `ze_ospfv3_debug_injected_lsas` decrements |
| AC-12 | `debug ipv6 ospf inject ...` as a read-only-profile user | the command is DENIED by authz (read-only profile `deny debug`) before the engine is reached |
| AC-13 | `debug ipv6 ospf inject ...` while the engine `debug` enablement is off (the default) | the command is rejected with a clear "debug injection not enabled" error; no LSA is originated |
| AC-14 | An inject LS Type whose S2/S1 bits are 11 (reserved), or a Link State ID / body that overflows the LSA Length | the command is rejected with a validation error; no LSA is originated |
| AC-15 | Any new show/debug command piped (`\| json`, `\| ndjson`, `\| table`, `\| text`, `\| yaml`, `\| match`, `\| count`, `\| resolve`, `\| origin`, `\| log`, `\| no-more`) | every operator is supported; `resolve`/`origin` decorate the IP-bearing fields (advertising router, IPv6 prefixes, link-local addresses) |
| AC-16 | The offline `ze` OSPFv3 decode subcommand on a v3 LSA/packet hex | renders the scope-aware LS Type + 20-byte header + typed/generic body with no running engine |
| AC-17 | GET the v6 database web views (per scope / per AF) and the sr/ri/extended views (and their SSE streams) | read-only snapshots render and stream; NO web route exposes injection |
| AC-18 | An injected debug LSA exists and the operator runs `show ipv6 ospf database <its type>` | the injected LSA appears in the database view, marked as locally-originated |
| AC-19 | The engine `debug` enablement is left on | `ze doctor` emits a Warning (debug injection enabled) via a new ext-8 doctor code; the two existing OSPF doctor codes are unaffected |
| AC-20 | The full v4 ext-14 + v6 ext-8 command/decoder/web inventory after ext-8 lands | the v4 ext-14 commands/decoders/web views still register and pass; the v6 ones are distinct (no wire-method or command-noun collision) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Inspects a received Intra-Area-Prefix-LSA decoded into its IPv6 prefixes | wire → base reception → LSDB; `show ipv6 ospf database intra-area-prefix detail` → v6 engine snapshot → base decoder → rendered (prefix/options) | `test/ospfv3/ospfv3-debug-decode.ci` (+ `ospfv3-debug-decode-frr` interop) |
| 2 | Inspects a received RI-LSA decoded into its TLVs after ext-6 lands | wire → base reception → LSDB; `show ipv6 ospf database router-information` → v6 engine → ext-6 RI decoder → rendered | `test/ospfv3/ospfv3-debug-decode.ci` (RI step, gated on ext-6) |
| 3 | Asks why an IPv6 route won | `show ipv6 ospf spf detail` → v6 engine → `spf` computer last result + candidates → per-prefix explanation (AF-tagged) | `test/ospfv3/ospfv3-debug-spf-explain.ci` |
| 4 | Lists the active address-family instances | `show ipv6 ospf instance` → v6 engine → AF-aware listing (Instance-ID → AF) | `test/ospfv3/ospfv3-debug-instance.ci` |
| 5 | Injects a test v3 LSA to exercise flooding without a second router | `debug ipv6 ospf inject lsa scope area type 0x2009 id ... hex ...` → authz + debug gate → base `OriginateSelf` → install + flood; `show ipv6 ospf database intra-area-prefix` shows it | `test/ospfv3/ospfv3-debug-inject.ci` |
| 6 | Withdraws the injected test LSA | `debug ipv6 ospf inject lsa scope area type 0x2009 id ... withdraw` → base MaxAge flush → peers purge | `test/ospfv3/ospfv3-debug-inject.ci` (withdraw step) + `ospfv3-debug-inject-frr` interop |
| 7 | A read-only operator is blocked from injecting | `debug ipv6 ospf inject ...` → authz `deny debug` → rejected | `TestV3InjectDeniedReadOnly` + `test/ospfv3/ospfv3-debug-authz.ci` |
| 8 | Decodes a captured v3 LSA offline | `ze` OSPFv3 decode of v3 hex → v3 codec → decoder/generic body → rendered | `test/ospfv3/ospfv3-debug-decode-offline.ci` |
| 9 | Views the v6 database (per scope) in the web UI | GET `/ospfv3/database` → generic snapshot adapter → `show ipv6 ospf database` → JSON/SSE | web e2e + `TestOSPFv3DatabaseWebView` |
| 10 | Dumps a neighbor in full v3 state | `show ipv6 ospf neighbor detail` → v6 engine → neighbor-detail snapshot (link-local, Interface ID, Instance ID, Options) | `test/ospfv3/ospfv3-debug-detail.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowOSPFv3DatabaseDetailWired` | `internal/plugins/ospf/cmd_show_test.go` | AC-1, A-3: the v6 detail proxy + engine arm are registered and reachable | |
| `TestV3DecodeTypedDecoder` | `internal/plugins/ospf/decode_view_v3_test.go` | AC-1: a registered decoder renders named fields/TLVs for a base + extension type | |
| `TestV3DecodeGenericBody` / `TestV3DecodeFallbackNoDecoder` | `internal/plugins/ospf/decode_view_v3_test.go` | AC-2, A-5, R-5: no decoder → generic scope-aware header + body-hex fallback | |
| `TestV3DecodeMalformed` | `internal/plugins/ospf/decode_view_v3_test.go` | AC-2, R-2: malformed body (bad §A.4.1 prefix) → raw hex, error metric, no panic | |
| `TestV3DatabaseScopeFilter` | `internal/plugins/ospf/decode_view_v3_test.go` | AC-5, A-7: per-scope filter on S2/S1 (link-local includes the Link-LSA store); reserved scope rejected | |
| `TestV3RIDatabaseView` | `internal/plugins/ospf/decode_view_v3_test.go` | AC-3: RI view empty pre-ext-6, decoded post (stub decoder) | |
| `TestV3SRDatabaseView` | `internal/plugins/ospf/decode_view_v3_test.go` | AC-4: SR view empty pre-ext-6, summarised post (stub decoder) | |
| `TestV3SPFExplainCandidateList` / `TestV3SPFExplainWired` | `internal/plugins/ospf/spf/explain_v3_test.go` | AC-6, A-2: candidate list + tie-break rationale, AF-tagged, reachable | |
| `TestV3SPFExplainNoRecompute` | `internal/plugins/ospf/spf/explain_v3_test.go` | AC-6, R-3: route table + SPF run-count unchanged by the explain view | |
| `TestV3InstanceListingSingleAF` / `TestV3InstanceListingMultiAF` / `TestV3InstanceListingWired` | `internal/plugins/ospf/instance_view_test.go` | AC-9, A-8: AF-aware listing with/without ext-1; reachable | |
| `TestV3NeighborDetailSnapshot` | `internal/plugins/ospf/neighbor_detail_v3_test.go` | AC-7: full per-neighbor v3 state (link-local, Interface ID, Instance ID, Options, DD seq) | |
| `TestV3InterfaceDetailSnapshot` | `internal/plugins/ospf/interface_detail_v3_test.go` | AC-8, A-7: full per-interface v3 state (ISM, Interface ID, Instance ID, DR/BDR by Router ID) | |
| `TestDebugInjectV3Wired` / `TestDebugInjectV3LSAFloods` | `internal/plugins/ospf/inject_v3_test.go` | AC-10, A-1: inject → base `OriginateSelf`/`OriginateLinkSelf` → install + flood (per scope) | |
| `TestV3InjectWithdrawFlushes` | `internal/plugins/ospf/inject_v3_test.go` | AC-11, R-8: tracked `(scope, type, id)` → MaxAge flush | |
| `TestV3InjectRequiresDebugEnabled` | `internal/plugins/ospf/inject_v3_test.go` | AC-13, A-4, R-1: rejected when debug disabled | |
| `TestV3InjectReservedScopeRejected` / `TestV3InjectBodyOverflowRejected` | `internal/plugins/ospf/inject_v3_test.go` | AC-14: S2/S1=11 and over-length body rejected | |
| `TestV3InjectRespectsMinLSInterval` / `TestV3InjectMalformedBodyRejected` | `internal/plugins/ospf/inject_v3_test.go` | A-9, R-2: MinLSInterval pacing; a malformed body is rejected before origination | |
| `TestV3InjectDeniedReadOnly` | `internal/component/authz/authz_test.go` | AC-12, A-4, R-1: read-only profile denies `debug` | |
| `TestV3NewCommandsDiscoverable` | `internal/plugins/ospf/yang/cmd_schema_test.go` | R-4: new commands self-document dispatch keys + appear in completion | |
| `TestV3NewCommandsPipeComplete` | `internal/plugins/ospf/pipe_v3_test.go` | AC-15, R-7: every pipe operator on each new command | |
| `TestOSPFv3DatabaseWebView` / `TestNoV3InjectWebRoute` | `internal/component/web/handler_ospf_test.go` | AC-17, A-6, R-6: read-only web views exist; no inject web route | |
| `TestV3DebugEnabledDoctorWarning` | `internal/plugins/ospf/doctor_test.go` | AC-19: doctor Warning when debug left enabled; existing codes untouched | |
| `TestV3CommandsDistinctFromV4` | `internal/plugins/ospf/cmd_show_test.go` | AC-20, R-9: v6 wire methods + command nouns do not collide with the v4 ext-14 ones | |
| `TestV3OfflineDecode` | `internal/plugins/ospf/cli/decode_v3_test.go` | AC-16: offline v3 decode renders scope-aware type + header + body | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Inject scope (LS Type S2/S1 bits) | {00 link-local, 01 area, 10 AS} | 10 (AS) | N/A | 11 (reserved) rejected |
| Inject LS Type (16-bit) | 0x0000-0xFFFF | 0xFFFF | N/A | N/A (2 bytes); S2/S1=11 rejected |
| Inject Link State ID (32-bit) | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (4 bytes) |
| Inject body / LSA length | 20-65535 (header + body) | within LSA max length | a length below the 20-byte header rejected | a length pushing past 65535 rejected |
| IPv6 prefix length (decode, §A.4.1) | 0-128 | 128 | N/A | >128 → malformed, shown as raw |
| SPF-explain / database area selector | valid area IDs | any configured area | an undeclared area → empty result | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-debug-decode` | `test/ospfv3/ospfv3-debug-decode.ci` | `show ipv6 ospf database <type> detail` decodes bodies (typed base + generic; RI/SR steps gated on ext-6) | |
| `ospfv3-debug-decode-offline` | `test/ospfv3/ospfv3-debug-decode-offline.ci` | offline `ze` OSPFv3 decode renders scope-aware type + header + body | |
| `ospfv3-debug-spf-explain` | `test/ospfv3/ospfv3-debug-spf-explain.ci` | `show ipv6 ospf spf detail` explains the winning route + tie-break (AF-tagged) | |
| `ospfv3-debug-instance` | `test/ospfv3/ospfv3-debug-instance.ci` | `show ipv6 ospf instance` lists the active AF instances | |
| `ospfv3-debug-detail` | `test/ospfv3/ospfv3-debug-detail.ci` | `show ipv6 ospf neighbor detail` / `interface detail` full v3 state | |
| `ospfv3-debug-inject` | `test/ospfv3/ospfv3-debug-inject.ci` | inject + observe + withdraw a test v3 LSA (debug enabled, authorized) | |
| `ospfv3-debug-authz` | `test/ospfv3/ospfv3-debug-authz.ci` | read-only user denied inject; debug-disabled router rejects inject | |
| `ospfv3-debug-pipes` | `test/ospfv3/ospfv3-debug-pipes.ci` | each new command honours all pipe operators | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-debug-inject-frr` | `test/interop/scenarios/ospfv3-debug-inject-frr/` | FRR `ospf6d` | a Ze-injected test v3 LSA floods to FRR, appears in FRR's `show ipv6 ospf6 database`, and is purged on withdraw; FRR's adjacency is unaffected | |
| `ospfv3-debug-decode-frr` | `test/interop/scenarios/ospfv3-debug-decode-frr/` | FRR `ospf6d` | the base v3 LSAs FRR originates (Router / Network / Intra-Area-Prefix / Link / AS-External) are decoded by Ze's `show ipv6 ospf database <type> detail` into the same fields FRR shows (cross-decode parity) | |

> Interop is required: injection changes wire behaviour (a new v3 LSA is flooded)
> and the decode must match FRR `ospf6d`'s interpretation. The raw-IPv6 /
> multicast (`ff02::5`/`ff02::6`) paths are Linux-only and run as QEMU
> integration tests (`ai/rules/qemu-testing.md`), consistent with the rest of the
> OSPFv3 interop set. The RI / SR decode-parity step is gated on `ospfv3-ext-6`
> landing its decoder; until then that step is skipped with a justification
> referencing ext-6.

### Future (if deferring any tests)
- The RI / SR cross-decode-parity steps in `ospfv3-debug-decode` / `ospfv3-debug-decode-frr` run only once `ospfv3-ext-6` registers the RI / SR decoders; recorded here, not silently dropped. All other ACs are covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/cmd_show.go` -- new `ze-show:ospfv3-*` proxies (database detail/scope/sr/ri/extended, instance, neighbor-detail, interface-detail, spf-detail) + a new `ze-debug:ospfv3-inject` proxy; distinct from the v4 ext-14 ones; the inject proxy forwards only when authz allows
- `internal/plugins/ospf/register.go` -- new v6-engine (`eng6`) `OnExecuteCommand` arms returning the new typed snapshots; the inject arm (debug-gate + base `OriginateSelf`/`OriginateLinkSelf`); new `sdk.CommandDecl` rows
- `internal/plugins/ospf/show_database.go` -- a v6 subview map + a v6 `databaseSnapshotByType` that filters Areas + ASExternal + the per-interface `Links` store, supports per-scope filtering, and adds the decode-enrichment hook
- `internal/plugins/ospf/cli/decode.go` + `cli/register.go` + `cli/run.go` -- a v3 decode branch (`ze ospfv3-decode` or `--version 3`) rendering scope-aware type + header + typed/generic body offline
- `internal/plugins/ospf/spf/route.go` -- retain/re-expose the candidate set + winning reason for the explain snapshot (shared with ext-14; AF-agnostic), if not already done by ext-14
- `internal/plugins/ospf/spf/computer.go` -- a read-only SPF-explain snapshot method built from the last result (shared with ext-14), tagged with AF/Instance-ID for v6
- `internal/plugins/ospf/doctor.go` -- a NEW debug-enabled-sanity doctor code (Warning when injection left enabled); the two existing codes untouched
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- command-tree nodes for the new `show ipv6 ospf ...` subcommands + the `debug ipv6 ospf inject lsa ...` tree, each with operator help naming the dispatch key
- `internal/component/authz/authz.go` -- `BuiltinReadOnlyProfile` gains (or reuses) a `deny "debug"` entry
- `internal/component/web/handler_ospf.go` -- new read-only `viewSpec` rows + handlers for the v6 database web/SSE views (per scope / per AF; no inject route)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new commands) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- new `show ipv6 ospf`/`debug ipv6 ospf` nodes; read `ai/rules/cli-grammar.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | the inject leaves (scope enum {link-local,area,as}; LS Type 16-bit hex pattern; Link State ID 32-bit; body hex pattern) use native `enumeration`/`range`/`pattern` |
| YANG custom validators | [ ] yes | a `CompleteFn` for the LS Type / registered decoder types (dynamic completion of known v3 LS types) |
| CLI commands/flags | [ ] yes | the offline `ze` OSPFv3 decode subcommand/flag in `cli/decode.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ipv6 ospf database type <ls-type>`, `show ipv6 ospf database scope <scope>`, `debug ipv6 ospf inject lsa scope <s> type <t> id <id> ...` |
| Editor autocomplete | [ ] yes | automatic for the new YANG enums + `CompleteFn` for dynamic v3 LS types |
| Functional test for new RPC/API | [ ] yes | `test/ospfv3/ospfv3-debug-*.ci` |
| Pipe completeness | [ ] yes | each new command routes through `ApplyPipes`; `resolve`/`origin` on IP fields (`ai/rules/pipe-completeness.md`) |
| Env var registration | [ ] no | the `debug` enablement is operational runtime state, not an `environment/` leaf (a runtime `debug ipv6 ospf` toggle, not config) |
| Doctor check for runtime dependencies | [ ] yes | a debug-enabled-sanity Warning code (no new socket/port/binary/cert; the inject path adds no runtime dependency) per `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospfv3_debug_injected_lsas` | gauge | `scope` (link-local/area/as) |
| `ze_ospfv3_debug_injections_total` | counter | `scope`, `action` (originate/withdraw) |
| `ze_ospfv3_debug_decode_errors_total` | counter | `ls_type` |

> These follow the ext-0 `ze_ospfv3_<ext>_*` contract (here `ze_ospfv3_debug_*`),
> registered by ext-8's owner code. They are added to the ext-0 metrics mapping
> when this spec lands; no existing OSPFv3 series is renamed.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv3 debug & introspection tooling |
| 2 | Config syntax changed? | [ ] no | inject is a runtime debug command, not config; no YANG config leaf added |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- the new `show ipv6 ospf ... detail/scope/sr/ri/extended/instance` + `debug ipv6 ospf inject lsa ...` |
| 4 | API/RPC added/changed? | [ ] yes | document the `ze-show:ospfv3-*` / `ze-debug:ospfv3-inject` RPCs under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPFv3 gains the debug/introspection surface + decoder registry |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospfv3.md` (or the OSPF guide's v3 section) -- a debug/introspection section (decode, explain, instance, gated inject) |
| 7 | Wire format changed? | [ ] no | no new wire format; injected LSAs are RFC 5340 native v3 LSAs |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document the ext-8 decoder-registry interface for ext-4/ext-6 authors |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5340.md` -- note the inject/observe debug surface + the scope-aware decode; `rfc/short/rfc5838.md` -- the AF-aware views |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- Ze's in-process inject/observe vs FRR ospf6d's tooling |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the v3 decoder registry + the gated inject path |
| 13 | Route metadata keys added/changed? | [ ] no | introspection installs no routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the three `ze_ospfv3_debug_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` -- the new v6 commands + the read-only profile `deny debug` |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `cmd_show.go`, `show_database.go`, `authz.go`, `handler_ospf.go`, `cli/decode.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPFv3 CLI examples against the new subcommands |

## Files to Create
- `internal/plugins/ospf/decode_view_v3.go` -- the v3 LSA decode view: the decoder registry (keyed by LS Type / function code), the typed-vs-generic scope-aware rendering, the sr/ri/extended aggregations, the per-scope filter
- `internal/plugins/ospf/inject_v3.go` -- the guarded v3 inject API: the debug-enablement flag, scope/LS-Type/Link-State-ID/body validation, the injected-key tracking, the base `OriginateSelf`/`OriginateLinkSelf` call + withdraw
- `internal/plugins/ospf/instance_view.go` -- the AF-aware instance listing (AF from the Instance-ID range, Instance ID, areas, neighbors)
- `internal/plugins/ospf/neighbor_detail_v3.go` -- the full per-neighbor v3 state snapshot (link-local, Interface ID, Instance ID, Options, DD seq, list sizes)
- `internal/plugins/ospf/interface_detail_v3.go` -- the full per-interface v3 state snapshot (ISM, Interface ID, Instance ID, DR/BDR by Router ID, timers)
- `internal/plugins/ospf/spf/explain_v3_test.go` (the explain logic itself is shared with ext-14's `spf/explain.go`; this spec adds the AF-tagging + v6 tests)
- `internal/plugins/ospf/decode_view_v3_test.go`, `inject_v3_test.go`, `instance_view_test.go`, `neighbor_detail_v3_test.go`, `interface_detail_v3_test.go`, `pipe_v3_test.go`, `doctor_test.go` (new cases)
- `internal/plugins/ospf/cli/decode_v3_test.go`
- `test/ospfv3/ospfv3-debug-decode.ci`, `ospfv3-debug-decode-offline.ci`, `ospfv3-debug-spf-explain.ci`, `ospfv3-debug-instance.ci`, `ospfv3-debug-detail.ci`, `ospfv3-debug-inject.ci`, `ospfv3-debug-authz.ci`, `ospfv3-debug-pipes.ci`
- `test/interop/scenarios/ospfv3-debug-inject-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospfv3-debug-decode-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the v6 engine, the LSDB `Snapshot()` (incl. `Links`), the base v3 decoders, the `spf` computer last result, and the base origination seam exist |
| 3. Wiring phase | Wiring Test table -- the v6 proxies + engine arms + failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | from critical review |
| 9. Re-verify | re-run stage 6 |
| 10. Repeat 7-9 | until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | re-run stage 6 |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

<!-- Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test. -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- the v6 command proxies + engine arms + failing wiring tests
   - Tests: `TestShowOSPFv3DatabaseDetailWired`, `TestDebugInjectV3Wired`, `TestV3InstanceListingWired`, `TestV3CommandsDistinctFromV4`, `test/ospfv3/ospfv3-debug-instance.ci`
   - Files: `cmd_show.go` (the `ze-show:ospfv3-*` + `ze-debug:ospfv3-inject` proxies), `register.go` (the v6 engine arms + `CommandDecl` rows, stubbed snapshots), `yang/ze-ospf-cmd.yang` (the new command nodes)
   - Verify: the v6 commands are registered, reachable, and distinct from v4; the deeper snapshot tests still fail because the snapshots are stubs
2. **Phase: scope-aware decode view + decoder registry** -- the v3 LSA decode primitives
   - Tests: `TestV3DecodeTypedDecoder`, `TestV3DecodeGenericBody`, `TestV3DecodeFallbackNoDecoder`, `TestV3DecodeMalformed`, `TestV3DatabaseScopeFilter`, `TestV3OfflineDecode`
   - Files: `decode_view_v3.go` (the registry + typed/generic rendering + per-scope filter), `show_database.go` (the v6 subview map + filter incl. `Links`), `cli/decode.go` (the offline v3 branch); register the base eight LSA types as default decoders
   - Verify: typed render for the base types; generic fallback for unknown; per-scope filter on S2/S1; malformed bodies are raw-hex + error metric; offline decode works
3. **Phase: SPF-explain + instance + neighbor/interface detail** -- the read-only deep views
   - Tests: `TestV3SPFExplainCandidateList`, `TestV3SPFExplainNoRecompute`, `TestV3InstanceListingSingleAF`, `TestV3InstanceListingMultiAF`, `TestV3NeighborDetailSnapshot`, `TestV3InterfaceDetailSnapshot`, `ospfv3-debug-spf-explain.ci`, `ospfv3-debug-detail.ci`
   - Files: `spf/explain_v3_test.go` (+ the shared `spf/explain.go`/`route.go`/`computer.go` AF-tagging), `instance_view.go`, `neighbor_detail_v3.go`, `interface_detail_v3.go`
   - Verify: the explain view reads the last result without recompute and AF-tags it; the instance listing degrades to a single AF without ext-1; the detail dumps surface link-local / Interface ID / Instance ID / Options
4. **Phase: guarded inject + authz + doctor** -- the one write path
   - Tests: `TestDebugInjectV3LSAFloods`, `TestV3InjectWithdrawFlushes`, `TestV3InjectRequiresDebugEnabled`, `TestV3InjectReservedScopeRejected`, `TestV3InjectBodyOverflowRejected`, `TestV3InjectRespectsMinLSInterval`, `TestV3InjectMalformedBodyRejected`, `TestV3InjectDeniedReadOnly`, `TestV3DebugEnabledDoctorWarning`, `ospfv3-debug-inject.ci`, `ospfv3-debug-authz.ci`
   - Files: `inject_v3.go` (validation + the base seam call + key tracking + withdraw + the debug-enablement flag + metrics), `authz.go` (the `deny debug` entry), `doctor.go` (the debug-enabled Warning), `register.go` (the inject arm)
   - Verify: inject floods through the base seam, withdraw flushes, both gates hold, reserved-scope / over-length / malformed bodies are rejected, the doctor Warning fires
5. **Phase: web/SSE + pipes + discovery** -- the remaining surface
   - Tests: `TestOSPFv3DatabaseWebView`, `TestNoV3InjectWebRoute`, `TestV3NewCommandsPipeComplete`, `TestV3NewCommandsDiscoverable`, `ospfv3-debug-pipes.ci`, `ospfv3-debug-decode.ci`
   - Files: `handler_ospf.go` (the v6 read-only `viewSpec` rows + handlers), the `ApplyPipes` routing on each new command, the YANG operator-help dispatch-key text
   - Verify: read-only web views render/stream (no inject route); every pipe operator works on each command; the dispatch keys are discoverable
6. **Functional tests** → the eight `.ci` cover the user-visible behaviour
7. **RFC refs** → add `// RFC 5340 Section A.4.2.1` (scope-aware type), `// RFC 5340 Section A.4` (LSA types), `// RFC 5338 Section 2` (AF identity) comments on the enforcing code
8. **Interop** → `ospfv3-debug-inject-frr` + `ospfv3-debug-decode-frr` QEMU scenarios
9. **Full verification** → `make ze-verify`
10. **Complete spec** → audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; v3 introspection parity with the OSPFv2 ext-14 surface (decode/explain/detail/inject/web), adapted to native scope-aware LSAs + AF-awareness |
| Correctness | scope derived from S2/S1 (not a flat type); base LSA decoders correct; per-scope filter includes the Link-LSA store; AF identity from the Instance-ID range; inject reuses the base seam (no second flooding path); reserved-scope / over-length rejection |
| Naming | `ze_ospfv3_debug_*` metrics; `ze-show:ospfv3-*` wire methods; `show ipv6 ospf ...` / `debug ipv6 ospf ...` nouns; no v4 collision |
| Data flow | v3 introspection touches the v6 engine snapshots + base codec only; SPF read-only; inject through the base origination seam; no consumer body format in generic code |
| CLI grammar | `show ipv6 ospf database type <t>` / `scope <s>`, `debug ipv6 ospf inject lsa scope <s> type <t> id <id> ...` action-before-identifier, typed selectors |
| Doctor checks | the debug-enabled Warning code added; the two existing OSPF codes untouched |
| YANG validation | the inject leaves have native enum/range/pattern; the `CompleteFn` lists known v3 LS types |
| Prometheus counters | the three `ze_ospfv3_debug_*` series defined, registered, listed; ext-0 mapping updated |
| Rule: plugin-self-containment | ext-8 names no consumer body format; removing ext-4/ext-6 removes their decoders cleanly; generic fallback remains |
| Rule: buffer-first | rendering is `textbuf`; body decode is zero-copy over LSDB bytes; the inject body is buffer-first |
| Rule: no v4 regression | the OSPFv2 ext-14 surface still registers + passes after ext-8 lands |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| The v6 show/debug commands are registered + distinct from v4 | `grep -rn 'ze-show:ospfv3-\|ze-debug:ospfv3-' internal/plugins/ospf` |
| Scope-aware decode + per-scope filter | `go test ./internal/plugins/ospf -run TestV3Decode` |
| Guarded inject (both gates) | `go test ./internal/plugins/ospf -run TestV3Inject` + `go test ./internal/component/authz -run TestV3InjectDeniedReadOnly` |
| SPF-explain (no recompute) | `go test ./internal/plugins/ospf/spf -run TestV3SPFExplain` |
| AF-aware instance listing | `go test ./internal/plugins/ospf -run TestV3InstanceListing` |
| Three metric series registered | `grep -rn 'ze_ospfv3_debug_' internal/plugins/ospf` |
| No inject web route | `go test ./internal/component/web -run TestNoV3InjectWebRoute` |
| Interop scenarios present | `ls test/interop/scenarios/ospfv3-debug-inject-frr/ test/interop/scenarios/ospfv3-debug-decode-frr/` |
| Functional tests present | `ls test/ospfv3/ospfv3-debug-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the inject LS Type / scope / Link State ID / body are validated (S2/S1 != 11; length within 65535; §A.4.1 prefixes well-formed) before origination; decode is bound-checked over LSDB bytes and never panics on a malformed body |
| Authorization | the inject path is denied by the read-only profile (`deny debug`) AND gated by the engine `debug` enablement (off by default); both are independently tested; no web route exposes injection |
| Resource exhaustion | the inject path inherits the base MinLSInterval pacing (no faster-than-base re-origination); injected LSAs share the existing LSDB caps; a flood of debug injects cannot grow memory unbounded |
| Error leakage | decode/inject validation errors are counted (`ze_ospfv3_debug_decode_errors_total`) and surfaced to the operator, not to peers |
| Trust boundary | injection is LOCAL-only into this router's own LSDB then flooded by the normal base machinery; received LSAs rely on the existing OSPFv3 auth (no new auth surface) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
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
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Core Insight
OSPFv3 debug tooling is a *scope-aware native-LSA* problem, not an opaque-carrier
problem: where OSPFv2 ext-14 decodes opaque type/ID + TLVs, OSPFv3 ext-8 decodes
the native LS Type (U/S2/S1 + function code) and its typed body, keying the
decoder registry on LS Type and the per-scope filter on S2/S1. The work is a
read surface over the existing v6 engine snapshots (LSDB incl. the per-interface
Link-LSA store, SPF last result, neighbor/interface state with link-local /
Interface ID / Instance ID) plus one guarded write that reuses the base
`OriginateSelf`/`OriginateLinkSelf` seam, AF-tagged via the `ospfv3-ext-1`
Instance-ID map, sharing the ext-14 inject/explain/web architecture without
clobbering the v4 surface.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decoder registry keyed by LS Type / function code (not opaque type) | reuse ext-14's opaque-type registry verbatim | OSPFv3 has no opaque framework; extensions are native scope-aware LSAs (RFC 5340 §A.4.2.1); the registry must key on the v3 LS Type |
| Per-scope filter on S2/S1 (incl. the Link-LSA store) | a flat type filter like the v2 subview map | OSPFv3 scope lives in the LS Type bits and link-local LSAs live in a separate per-interface store; the filter must be scope-aware |
| Inject via the base `OriginateSelf`/`OriginateLinkSelf` seam | a dedicated debug flooding path | the base owns sequence/age/install/flood; reusing it keeps debug LSAs on the same validated path as real ones and avoids a second flooding path |
| AF-aware views read the ext-1 Instance-ID → AF map, degrade to single AF | hard-depend on ext-1 | ext-8 depends only on the base; AF identity is additive and degrades cleanly to one IPv6-unicast instance when ext-1 is absent |
| Distinct `ze-show:ospfv3-*` wire methods + `show ipv6 ospf` nouns | reuse the v4 ext-14 wire methods | v4 and v6 share `internal/plugins/ospf`; distinct names prevent a registration collision and keep the v4 ext-14 surface intact |
| Typed RI/extended/SR/Grace decoders are runtime-optional | hard-depend on ext-4/ext-6 | ext-8 ships before them; a typed view fills in when its consumer registers a decoder; generic body-hex is the fallback |

## Known Limitations
- ext-8 ships with NO RI/extended/SR/Grace typed decoder of its own; without ext-4/ext-6 those database views are empty and bodies render generically (by design -- the decoders are ext-4/ext-6's).
- Injection is LOCAL-only (this router's own LSDB, then normal flooding); it does not inject into a peer's LSDB over the wire.
- AF-aware identification is full only when `ospfv3-ext-1` is present; otherwise every result is the base IPv6-unicast instance.
- v3 LSAs never participate differently in SPF because of ext-8; ext-8 reads the SPF result, it does not change the computation.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 5340 §A.4.2.1 -- the scope-aware LS Type decode (U/S2/S1 + function code); the per-scope filter and the reserved-scope (11) rejection
- RFC 5340 §A.4 -- the eight base LSA-type decoders the default registry registers
- RFC 5340 §A.4.1 -- the IPv6 prefix decode (`((PrefixLength + 31) / 32)` words) + padding validation in the database detail view
- RFC 5340 §A.3.1 / §2.1 -- the Instance ID + Interface ID surfaced in the neighbor/interface detail and the AF-aware views
- RFC 5338 §2 -- the AF-per-Instance-ID identity the AF-aware views read

## Implementation Summary

### What Was Implemented
- [filled at implementation time]

### Bugs Found/Fixed
- [filled at implementation time]

### Documentation Updates
- [filled at implementation time]

### Deviations from Plan
- [filled at implementation time]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Scope-aware deep LSDB decode (per-area / per-LS-type / per-scope) | functional + interop | `ospfv3-debug-decode.ci`, `ospfv3-debug-decode-frr` |
| SPF trace / explain (AF-tagged, no recompute) | unit + functional | `TestV3SPFExplainNoRecompute`, `ospfv3-debug-spf-explain.ci` |
| Per-AF views + instance listing | unit + functional | `TestV3InstanceListingMultiAF`, `ospfv3-debug-instance.ci` |
| Neighbor / interface deep state (link-local / Interface ID / Instance ID) | unit + functional | `TestV3NeighborDetailSnapshot`, `ospfv3-debug-detail.ci` |
| Guarded LSA injection behind authz (LOCAL-only, base seam) | unit + functional + interop | `TestV3InjectDeniedReadOnly`, `ospfv3-debug-inject.ci`, `ospfv3-debug-inject-frr` |
| Structured JSON + web/looking-glass | unit + web e2e | `TestV3NewCommandsPipeComplete`, `TestOSPFv3DatabaseWebView` |

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

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
- [ ] AC-1..AC-20 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`, `internal/component/{authz,web}/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 5340 / RFC 5838 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the decoder registry has multiple consumers: base eight + ext-4/ext-6)
- [ ] No speculative features (decode + explain + detail + gated inject only)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (ext-8 names no consumer body; v4 surface untouched)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospfv3-debug-inject-frr`, `ospfv3-debug-decode-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-8-debug-introspection.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospfv3-ext-8-debug-introspection.md`
