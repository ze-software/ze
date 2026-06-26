# Spec: OSPF Debug & Introspection Tooling

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-0-umbrella.md (umbrella); spec-ospf-ext-1-opaque-framework.md (IPv4 opaque carrier); spec-ospf-ext-3-router-information.md + spec-ospf-ext-4-extended-link-prefix.md + spec-ospf-ext-5-segment-routing.md (IPv4 SR carriers, optional decoders) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/972-ospf-af-unify.md` -- Ze runs ONE unified `ospf` engine spanning two address families; the FSM, flooding, DR election, SPF, and LSDB sequencing are AF-neutral and shared; the AF-specific wire/LSA/prefix code lives in the `_v6` strategy files and the `internal/plugins/ospf/v3/{types,packet,transport}` leaves; there is NO separate `ospfv3` plugin
4. `plan/spec-ospf-ext-0-umbrella.md` -- the ext-family umbrella; the Child Decomposition row for this feature, the "Out of scope (rested)" decision that this feature REPLACES the standalone `ospfclient` Unix-socket daemon with in-process inject/observe, the `ze_ospf_<ext>_*` / `ze_ospfv3_<ext>_*` metric-naming contract, and the `show ospf <noun>` / `show ospf ipv6 <noun>` command-ownership model
5. `plan/spec-ospf-ext-1-opaque-framework.md` -- the IPv4 opaque carrier this surface decodes/injects through (`RegisterOpaqueConsumer`, the `OnOriginate`/`OriginateOpaque` origination seam, the generic TLV iterator, the LS-ID split `OpaqueType()`/`OpaqueID()`, the AS-opaque store, the consumer-callback recover wrapper). IPv6 has NO opaque carrier: v6 extensions are native scope-aware LSAs
6. `internal/plugins/ospf/cmd_show.go` -- the CENTRAL-namespace `ze-show:ospf-*` builtin-proxy RPC pattern (RPCRegistration + PluginCommand + forwardToOSPF / `dbSubviewForwarder`); this feature adds new proxies here for both AFs
7. `internal/plugins/ospf/register.go` (~206 `runOSPFEngine`, the v6 engine spawn, the `OnExecuteCommand` switch, the `sdk.CommandDecl` rows) -- the engine-side command switch; this feature adds new `show ospf ...` / `show ospf ipv6 ...` / `debug ... ospf inject ...` cases and `sdk.CommandDecl` rows
8. `internal/plugins/ospf/show_database.go` -- the `show ospf database <type>` subview pattern (filters `LSASnapshot` by type); this feature adds opaque/TE/RI/SR/Extended-Link-Prefix subviews + decode (IPv4) and scope-aware native-LSA subviews (IPv6)
9. `internal/plugins/ospf/spf/computer.go` + `spf/route.go` -- `Snapshot`/`BorderRouterSnapshot`/`SPFSnapshot`, `RouteEntry`, `BuildRoutes`/`selectBestRoutes`; the SPF-explain surface reads candidate/tie-break data here (AF-neutral, shared)
10. `internal/plugins/ospf/v3/types/lsa.go` (`LSType` U/S2/S1 + function code, `Scope()`, `Known()`, `LSAKey`) + `internal/plugins/ospf/v3/packet/lsa.go` + the per-type `v3/packet/lsa_*.go` -- the native v6 LSA model + decoders the IPv6 family decodes by LS Type + scope
11. `internal/component/authz/authz.go` -- the profile-based command-path allow/deny gate; the injection command is denied by the built-in read-only profile via a fresh `debug` deny rule
12. `internal/component/web/handler_ospf.go` + `snapshot_views.go` -- the generic read-only web/SSE snapshot-view adapter; this feature adds opaque/TE/SR (IPv4) and per-scope/per-AF native (IPv6) database web views the same way

## Task

Deliver first-class operational debugging and introspection for the unified Ze
OSPF engine across **both address families** (IPv4/OSPFv2, RFC 2328; and
IPv6/OSPFv3, RFC 5340), replacing the two version-split specs with one coherent
feature. Ze implements OSPF as ONE engine named `ospf` (`internal/plugins/ospf/`)
that runs the IPv6 family as a second instance over the v6 codec; the FSM,
flooding, DR election, SPF, and LSDB sequencing are AF-neutral and shared, so the
debug subsystem is ONE subsystem covering both families, with the per-AF
wire/LSA/scope differences isolated to the decode + filter layer.

The feature folds in the genuinely useful capability of FRR's `ospfclient`
(inject and observe LSAs for testing and research) as an **in-process,
authz-gated** surface on the engine's existing origination seams. The ext-0
umbrella records the decision: the standalone `ospfclient` Unix-socket daemon is
rested; this feature replaces it with the same inject/observe value delivered
inside the OSPF engine, with no separate socket or external trust boundary.

The base OSPF already ships a read-only diagnostic surface for both families:
`show ospf` / `show ospf ipv6` and their `... neighbor`, `... interface`,
`... database` (with per-LS-type subviews), `... route` views, the `clear`
resets, two config-sanity doctor checks, the web/SSE neighbor+database views, and
the looking-glass topology graph. The IPv4 family additionally has the opaque
carrier and `show ospf database opaque-link|opaque-area|opaque-as`. What is
missing is the **deep, extension-aware, AF-aware** debugging the operator needs
once the extension LSA bodies actually flow: a way to decode an LSA body into its
typed fields/TLVs, to inspect the TE / SR databases as first-class views, to
explain why an SPF route won (candidate list, tie-breaks), to dump a neighbor or
interface in full state, to view each address family's topology separately, and
-- behind an explicit gate -- to inject a crafted/test LSA into the local LSDB so
flooding, reception, and the consumer/decode paths can be exercised end-to-end
without a second router.

This feature is a pure **introspection consumer** of the existing engine
snapshots plus ONE guarded write (debug injection), which reuses the engine's
existing origination seams exactly as a real origination would. It adds NO new
wire format, NO new LSA type, and NO SPF participation in either family. It is
decode + inspect + explain + (gated) inject, plus the CLI/JSON/web surface that
exposes all of it. Every show command routes through `ApplyPipes` for
pipe-completeness, and discovery is first-class: each command self-documents its
dispatch key and appears in completion, closing the project's known CLI
dispatch-discovery gaps.

The headline per-AF difference is the carrier model: the IPv4 family rides
extensions in Opaque LSAs (RFC 5250) decoded by Opaque Type/ID + TLVs; the IPv6
family carries every extension as a native scope-aware LSA (RFC 5340 §A.4.2.1: the
LS Type embeds the U-bit + S2/S1 flooding scope + function code) decoded by LS
Type + scope. The decoder registry is therefore keyed per-family: by Opaque Type
(IPv4) and by LS Type / function code (IPv6). Each typed decoder is registered
self-containedly by its owning consumer; removing the consumer removes its
decoder and database view, leaving the generic fallback (opaque hex/TLV for IPv4,
scope-aware header + body-hex for IPv6). Generic code never spells a consumer's
body format.

### In scope (this spec)

| Item | Address family | Detail |
|------|----------------|--------|
| Deep LSDB inspection CLI | IPv4 | `show ospf database opaque-*` extended with full per-TLV DECODE: each opaque body rendered via its registered typed decoder (TE / RI / Extended-Link-Prefix / SR / Grace), falling back to the ext-1 generic TLV iterator (type/length/hex); per-area, per-LS-type, per-Opaque-Type filtering |
| Deep LSDB inspection CLI | IPv6 | `show ospf ipv6 database <type> detail` with full per-LSA DECODE: each native v3 LSA body rendered via its registered decoder (the base Router/Network/Inter-Area-Prefix/Inter-Area-Router/AS-External/NSSA/Link/Intra-Area-Prefix decoders, plus RI / extended / SR / Grace), falling back to a generic scope-aware header + body-hex view; per-area, per-LS-type, per-scope (`scope link\|area\|as` on S2/S1), and per-Interface (Link-LSAs) filtering |
| Offline decode helper | IPv4 | the `ze` decoder path takes opaque-LSA hex and renders Opaque Type/ID + typed TLVs (extends `internal/plugins/ospf/cli/decode.go`), no running engine |
| Offline decode helper | IPv6 | a v3 branch of the `ze` decoder takes a v3 LSA/packet hex and renders the scope-aware LS Type (U/S2/S1 + function code) + 20-byte header + typed/generic body (via the v3 codec in `internal/plugins/ospf/v3/packet`), no running engine |
| SPF compute trace / explain | both (shared) | `show ospf spf detail` / `show ospf ipv6 spf detail` (per-area): the candidate vertices considered, the winning path per prefix, the cost composition, and the §16.x tie-break that selected it (read from the AF-neutral `spf` computer's route/candidate data); explains WHY a route won; the IPv6 result is tagged with its AF/Instance-ID |
| TE database view | IPv4 | `show ospf database te`: the TE opaque LSAs (Opaque Type 1) decoded into Router-Address / Link sub-TLVs; empty until ext-2 registers its decoder |
| SR database view | IPv4 | `show ospf database segment-routing`: SR content (SR-Algorithm / SRGB / Prefix-SID / Adjacency-SID carried in RI + Extended-Link/Prefix bodies) summarised; empty until ext-3/ext-4/ext-5 land |
| SR / RI / extended database view | IPv6 | `show ospf ipv6 database segment-routing` / `... router-information` / `... extended`: the v3 Router-Information-LSA + the E-Intra/E-Inter/E-AS extended LSAs + SR TLVs (RFC 8362/8665/8666, native per RFC 5340) decoded into a summary / named TLVs; empty until the v6 consumer decoders land |
| Per-AF database / topology / instance views | IPv6 (ties to multi-AF, ext-15) | `show ospf ipv6 database` and the detail views are AF-aware: each result identifies its address family (from the Instance-ID range, RFC 5838 §2) + Instance ID; `show ospf ipv6 instance` enumerates the active OSPFv3 instances (AF, Instance ID, area count, neighbor count); degrades to a single IPv6-unicast instance when only the base instance is configured |
| Neighbor / interface deep dump | IPv4 | `show ospf neighbor detail` / `... interface detail`: full per-neighbor state (DD seq, options incl. O-bit, retransmission/request/summary list sizes, last-event, timers) and per-interface state (ISM, DR/BDR election detail, timers, opaque-capable neighbour count) |
| Neighbor / interface deep dump | IPv6 | `show ospf ipv6 neighbor detail` / `... interface detail`: full per-neighbor state (the link-local address as identity, the advertised Interface ID, the negotiated Instance ID, DD seq, Options incl. R/V6/E/N/AF bits, list sizes, last-event, timers) and per-interface state (ISM, local Interface ID, Instance ID, DR/BDR election by Router ID, timers, link-local source) |
| Guarded LSA injection / origination | IPv4 | `debug ip ospf inject opaque scope <s> id <opaque-id> [tlv ...\|hex ...]`: registers a debug Opaque Type (Private-Use 128-255) and originates a crafted opaque LSA via the ext-1 `OnOriginate`/`OriginateOpaque` seam; `withdraw` MaxAge-flushes |
| Guarded LSA injection / origination | IPv6 | `debug ipv6 ospf inject lsa scope <s> type <ls-type> id <link-state-id> [tlv ...\|hex ...]`: builds a crafted v3 LSA (scope derived from the LS Type S2/S1 bits) and originates it via the base `OriginateSelf` (area/AS) or `OriginateLinkSelf` (link-local) seam; `withdraw` MaxAge-flushes |
| Injection gating | both (shared) | OFF by default, denied by the read-only authz profile (`deny "debug"`), and gated behind an explicit engine `debug` enablement; both required; LOCAL-only into this router's LSDB then normal flooding; never surfaced on the web |
| Structured JSON output | both | every new show/debug command returns a typed snapshot rendered as JSON and routed through `ApplyPipes` (json/ndjson/table/text/yaml/match/count/resolve/origin/log/no-more) |
| Web / looking-glass surfacing | both | new read-only web/SSE views for the opaque/TE/SR (IPv4) and the per-scope/per-AF native + sr/ri/extended (IPv6) databases via the generic `snapshot_views.go` adapter; the injection path is NEVER surfaced on the web |
| Metrics | IPv4 | `ze_ospf_debug_injected_lsas` (gauge), `ze_ospf_debug_injections_total` (counter), `ze_ospf_debug_decode_errors_total` (counter) |
| Metrics | IPv6 | `ze_ospfv3_debug_injected_lsas` (gauge), `ze_ospfv3_debug_injections_total` (counter), `ze_ospfv3_debug_decode_errors_total` (counter) |
| Discovery / dispatch | both | each command's dispatch key is discoverable (help text names the key; the dispatch-key listing includes the new commands); no hidden RPC-name-only commands |

### Out of scope (noted so it is not silently assumed done)

| Item | Address family | Where |
|------|----------------|-------|
| The TE opaque body + sub-TLV codec | IPv4 | spec-ospf-ext-2 (this feature only DECODES + renders what ext-2 registers) |
| The Router-Information body codec | IPv4 | spec-ospf-ext-3 |
| The Extended-Link / Extended-Prefix body codec | IPv4 | spec-ospf-ext-4 |
| The Segment-Routing TLVs + SID logic | IPv4 | spec-ospf-ext-5 |
| The Grace-LSA body + GR helper | IPv4 | spec-ospf-ext-9 |
| The IPv4 opaque carrier (scope flooding, O-bit, registry, generic TLV iterator) | IPv4 | spec-ospf-ext-1 (consumed, never redefined) |
| The RI-LSA / extended-LSA / SR TLV codecs | IPv6 | the v6 RI/extended/SR consumer spec (this feature only DECODES + renders what it registers) |
| The Grace-LSA body + GR helper | IPv6 | the v6 graceful-restart consumer spec |
| The multi-AF Instance-ID demux + AF-bit + per-AF install | IPv6 | spec-ospf-ext-15 multi-AF (this feature READS the AF/Instance-ID identity it exposes; it does not implement the demux) |
| The OSPFv3 base codec, LSDB, SPF, transport, neighbor FSM, scope-aware LSA model | IPv6 | `plan/learned/972-ospf-af-unify.md` (delivered; consumed, never redefined) |
| Any SPF change | both | none -- this feature reads the SPF result, it does not alter the computation |
| A standalone `ospfclient` Unix-socket daemon / external-injection socket | both | rested by ext-0; in-process inject/observe instead |
| SNMP / OSPF MIB (RFC 4750) / OSPFv3 MIB (RFC 5643) | both | rested by ext-0 ("Defer"); equivalents via CLI/JSON/web only |
| Remote injection (inject into a peer's LSDB over the wire) | both | not done; injection is LOCAL-only into this router's own LSDB, then flooded by the normal machinery |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `plan/learned/972-ospf-af-unify.md` -- the one-engine, AF-strategy design ext-14 introspects across both families
  -> Decision: there is ONE `ospf` engine; the IPv6 family is a second instance over the v6 codec; the FSM/flooding/DR/SPF/LSDB sequencing are AF-neutral; the AF-specific wire/LSA/prefix code lives in the `_v6` strategy files and the `internal/plugins/ospf/v3/{types,packet,transport}` leaves
  -> Constraint: scope-typed v6 LS Types are classified through helpers (`Scope()`, `ASExternal`, `NSSA`, `InterAreaRouter`), NOT OSPFv2 numeric constants; ext-14's v6 decode/filter must key on `LSType.Scope()`, never on a flat v4 type number
- [ ] `docs/research/ospf-implementation-guide.md` ("External LSA API (ospfclient)" + "SNMP and Operational Hooks") -- the FRR capability this feature replaces and the directive to expose the equivalent via Ze's own CLI and web/looking-glass
  -> Decision: deliver the useful `ospfclient` capability (inject + observe LSAs for research/testing) in-process on the existing origination seams; do NOT ship a Unix-domain-socket external-injection daemon (the guide calls it "not needed in production")
  -> Constraint: the guide says "ze should expose the equivalent via its own CLI and via the web/looking-glass components" -- the read-only introspection (decode/inspect/explain) is surfaced on CLI + web; the inject path is CLI + authz only (never web)
- [ ] `plan/spec-ospf-ext-0-umbrella.md` "Child Decomposition" (the debug row), "Out of scope (rested)" (standalone ospfclient), the `ze_ospf_<ext>_*` / `ze_ospfv3_<ext>_*` metric contract, and the `show ospf <noun>` / `show ospf ipv6 <noun>` command-ownership model
  -> Constraint: this feature uses `ze_ospf_debug_*` (IPv4) and `ze_ospfv3_debug_*` (IPv6) metric names and `show ospf ...` / `show ospf ipv6 ...` / `debug ... ospf ...` command nouns; it renames NO existing OSPF series or command
  -> Decision: the IPv4 surface depends on the ext-1 opaque carrier; typed decoders for TE/RI/Extended/SR/Grace (both families) are OPTIONAL and resolved at runtime via the registry, so this feature builds and ships before those consumers do, degrading to generic rendering
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` "In scope", Data Flow, Wiring Test -- the IPv4 opaque carrier (registry, `OnOriginate`/`OriginateOpaque` seams, generic TLV iterator, LS-ID split, AS-opaque store, recover wrapper)
  -> Constraint: IPv4 injection MUST go through `OnOriginate`/`OriginateOpaque` (the carrier owns sequence/age/install/flood); ext-14 builds the opaque header + body and hands it over, it does not write a second origination path
  -> Constraint: IPv4 decode MUST use the ext-1 generic TLV iterator as the fallback and the registered typed decoder as the primary; ext-14 interprets no TLV type itself
- [ ] `ai/rules/cli-grammar.md` -- keyword-before-value; typed selectors
  -> Constraint: every new command places a closed keyword before any value; per-Opaque-Type / per-LS-type filtering uses a typed selector (`type <...>`), per-scope a typed selector (`scope <...>`); the inject commands are `debug ip ospf inject opaque scope <s> id <id> ...` and `debug ipv6 ospf inject lsa scope <s> type <t> id <id> ...` (action/keywords before values, never a free-form positional)
  -> Constraint: injection is a runtime operational debug action (not a config-tree mutation), so it takes an operational verb (`debug ... inject`), not `set`/`delete`
- [ ] `ai/rules/pipe-completeness.md` -- every command that produces output supports all pipe operators
  -> Constraint: each new show/debug command routes its JSON snapshot through `ApplyPipes`; data-transform pipes (`resolve`/`origin`) apply to the IP-bearing fields (advertising router, next-hops, TE link addresses, IPv6 prefixes, link-local addresses)
- [ ] `ai/rules/plugin-self-containment.md` -- removing a plugin removes ALL its features
  -> Constraint: each typed decoder + its database view is registered by its owning consumer through a small ext-14 decoder-registry (keyed by Opaque Type for IPv4, by LS Type / function code for IPv6); removing the consumer removes its decoder and view; generic rendering remains; generic code spells no consumer body format
- [ ] `ai/rules/no-sprintf-alloc.md`, `ai/rules/buffer-first.md` -- rendering uses `textbuf`/`AppendTo`, decode is zero-copy over the LSDB bytes
  -> Constraint: all rendering uses `textbuf.Buffer`; the TLV/body decode returns views over the LSDB raw bytes (the ext-1 iterator for IPv4, the v3 packet decoders for IPv6), no per-field allocation in the hot path; an injected body is built buffer-first

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2328.md` -- the base OSPFv2 LSA/flooding/SPF semantics the IPv4 introspection surfaces (and the shared SPF the explain view reads for both families)
  -> Constraint: §13.1 "which LSA is newer" and §14 aging -- the database decode view shows LS sequence / age / checksum so the operator can reason about freshness; a decoded LSA at MaxAge (§14) is shown as flushing
  -> Constraint: §16.1 two-stage Dijkstra + §16.2 inter-area + §16.4 external preference -- the SPF-explain view surfaces the candidate set and the §16.4 path-preference tie-break (intra > inter > external; Type 1 > Type 2 external); ext-14 reads the result, it does not re-derive it
  -> Constraint: §A.4.1 -- the 20-byte LSA header layout (LS age, Options incl. O-bit, LS type, Link State ID, Advertising Router, LS seq, LS checksum, length) the IPv4 deep database view renders
- [ ] `rfc/short/rfc5250.md` -- the IPv4 Opaque-LSA framework that the injected/decoded IPv4 LSAs conform to
  -> Constraint: §3 / Appendix A.2 -- the injected/decoded Link State ID splits into Opaque Type (high 8 bits) + Opaque ID (low 24 bits); the IPv4 inject command takes a typed scope (9/10/11) and a 24-bit opaque-id, and the decode view renders both subfields
  -> Constraint: §3.1 -- an injected opaque LSA is flooded by ext-1 ONLY to opaque-capable neighbours and ONLY within its scope (never into stub/NSSA for Type 11); ext-14 relies on these gates, it does not bypass them
  -> Constraint: §9 -- Opaque Type 128-255 is Private Use; the IPv4 debug-injection default Opaque Type uses a Private-Use value so a crafted test LSA never collides with a standards-track consumer (TE=1, grace=3, RI=4)
  -> Constraint: §8 -- origination is rate-limited (>= 5 s, MinLSInterval); the IPv4 inject path inherits the ext-1/RFC-2328 pacing
- [ ] `rfc/short/rfc5340.md` -- OSPF for IPv6, the base the IPv6 introspection surfaces
  -> Constraint: §A.4.2.1 -- the LS Type embeds the U-bit + the S2/S1 flooding scope (00 link-local, 01 area, 10 AS, 11 reserved) + a 13-bit function code; the IPv6 decode view renders all four subfields and the per-scope filter keys on S2/S1, NOT a flat type number
  -> Constraint: §A.4 -- the eight base LS types (Router 0x2001, Network 0x2002, Inter-Area-Prefix 0x2003, Inter-Area-Router 0x2004, AS-External 0x4005, NSSA 0x2007, Link 0x0008, Intra-Area-Prefix 0x2009); the database detail view names each and routes Link-LSAs (link-local scope) to the per-interface store
  -> Constraint: §A.4.1 -- IPv6 prefixes are `PrefixLength` + `PrefixOptions` + `((PrefixLength + 31) / 32)` 32-bit words; the IPv6 decode view renders the prefix and validates the byte length / padding (a malformed prefix is shown as raw, never panics)
  -> Constraint: §A.3.1 / §2.1 -- the 16-byte common header carries the 8-bit Instance ID (link-local significance); the IPv6 neighbor/interface detail and AF-aware views surface the Interface ID (32-bit, §A.4.3) and the Instance ID, NOT IPv4 subnet identity; neighbor identity is the link-local address + Router ID
  -> Constraint: §A.4.2 -- the 20-byte LSA header; an injected v6 LSA at MaxAge is shown as flushing
- [ ] `rfc/short/rfc5838.md` -- AF-to-Instance-ID ranges (the IPv6 AF identity the AF-aware views surface)
  -> Constraint: §2 -- each address family is a separate OSPFv3 instance with its own LSDB; the AF-aware views identify each result's AF by its Instance-ID range (IPv6u 0-31, IPv6m 32-63, IPv4u 64-95, IPv4m 96-127), reading the identity multi-AF establishes; ext-14 does NOT implement the demux

**Key insights:** (minimal context to resume after compaction)
- ONE engine, TWO address families: the debug subsystem is shared; only the decode + filter layer is per-AF. IPv4 extensions ride opaque LSAs (Opaque Type/ID + TLVs); IPv6 extensions are native scope-aware LSAs (LS Type U/S2/S1 + function code). This is the single headline per-AF difference.
- This feature is a READ surface plus ONE guarded WRITE (debug inject). IPv4 inject goes through the ext-1 `OnOriginate` seam; IPv6 inject through the base `OriginateSelf`/`OriginateLinkSelf` seam. No new wire format, no SPF change, in either family.
- Typed decoders are optional and runtime-resolved per family: the feature ships and works (generic rendering + base introspection) before the extension consumers land; a typed view fills in when its consumer registers a decoder.
- The SPF-explain and the inject gating (two gates: read-only authz `deny debug` + engine `debug` enablement off by default) are shared across both families; the IPv6 explain result is additionally AF/Instance-ID-tagged.
- Discovery is first-class: the new commands self-document their dispatch keys and appear in completion, deliberately NOT reproducing the project's known CLI dispatch-discovery gap.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/cmd_show.go` -- the CENTRAL-namespace `ze-show:ospf-*` builtin-proxy RPCs: each `RPCRegistration{WireMethod, Handler, PluginCommand}` declares the plugin command it fronts; `forwardToOSPF` rejects extra args and calls `ForwardToPlugin` (the LDP/IS-IS proxy model); `dbSubviewForwarder` is the closure for the `show ospf database <type>` subviews
  -> Constraint: ext-14 adds new `ze-show:ospf-*` proxies (IPv4: decode/detail/te/segment-routing) and distinct `ze-show:ospfv3-*` proxies (IPv6: database detail/scope/sr/ri/extended, instance, neighbor-detail, interface-detail, spf-detail), plus `ze-debug:ospf-inject` (IPv4) and `ze-debug:ospfv3-inject` (IPv6); each MUST declare its PluginCommand and forward via `ForwardToPlugin`, never re-Dispatch (that recurses); the v6 wire methods are DISTINCT from the v4 ones
- [ ] `internal/plugins/ospf/register.go` (~206 `runOSPFEngine`, the v6 engine spawn, the `OnExecuteCommand` switch, the `sdk.CommandDecl` rows) -- the engine-side command switch maps each `show ospf ...` / `show ospf ipv6 ...` string to an engine snapshot method; `sdk.CommandDecl` lists every command the engine claims; the IPv6 family is a second engine instance over the v6 codec
  -> Constraint: ext-14 adds new `case` arms (one per new command, per family) returning a typed snapshot, plus matching `CommandDecl` rows; the inject arms call the relevant origination seam guarded by the debug-enabled flag; the v6 engine is the one whose LSDB/SPF/neighbor state the v6 views read
- [ ] `internal/plugins/ospf/show_database.go` -- `dbSubviewType` maps `show ospf database <type>` to an `LSASnapshot.Type` string; `databaseSnapshotByType` filters the LSDB `Snapshot()` per area + AS-external; `filterLSAsByType` is the filter
  -> Constraint: the IPv4 opaque subviews (ext-1 added opaque-link/area/as) extend this same map; ext-14's IPv4 DECODE view reuses `databaseSnapshotByType` then enriches each opaque LSA with a typed/generic TLV rendering. The IPv6 family adds a v6 subview map (router/network/inter-area-prefix/inter-area-router/external/nssa/link/intra-area-prefix + ri/extended/segment-routing) and a v6 `databaseSnapshotByType` that ALSO filters the per-interface `Links` store (Link-LSAs are link-local scope) and supports per-scope filtering; the existing filter contract and the v4 map are untouched
- [ ] `internal/plugins/ospf/lsdb/` (`Snapshot`/`AreaSnapshot`/`LinkSnapshot`/`LSASnapshot`) -- the snapshot carries `Areas`, `ASExternal`, and (for v3) `Links []LinkSnapshot` (each with `Interface`); `LSASnapshot` carries `Type`, `LinkStateID`, `AdvertisingRouter`, `Sequence`, `Age`, `Checksum`, `Length`, and the v3-only `Interface` + `LinkLocalAddress`
  -> Constraint: the deep database view reads `Snapshot()` and enriches each `LSASnapshot` with a decoded body; the `Links` slice is the link-local (Link-LSA) store the v6 per-scope filter must include; ext-14 reads this snapshot, it does not change its shape (base tests pin it)
- [ ] `internal/plugins/ospf/clear.go` -- `clearResult{Action, Cleared}` JSON payload and the `clearNeighbors`/`clearCounters`/`clearProcess` engine resets
  -> Constraint: the inject path returns a parallel typed result (the injected LS-ID/key + scope + action) in the same small-JSON style; it does NOT reuse `clearResult`
- [ ] `internal/plugins/ospf/spf/computer.go` + `spf/route.go` -- `Snapshot()`/`BorderRouterSnapshot()`/`SPFSnapshot()` rows; `spfState{Area, LastRun, Duration, NodeCount, Pending, CurrentDelay}`; the last computed result (`last`/`lastBorder`); `BuildRoutes` builds candidates from reached vertices, `selectBestRoutes` does the per-prefix best-path compare; `ClearSPFLog` resets per-area state. The SPF computer is AF-neutral (shared by both families)
  -> Constraint: the SPF-explain view reads the last result + the candidate/tie-break data WITHOUT changing the existing `SPFSnapshot` shape (base + ext-13 tests pin it) and WITHOUT re-running SPF; the same explain logic serves both families, tagging the IPv6 result with its AF/Instance-ID
- [ ] `internal/plugins/ospf/v3/types/lsa.go` -- `LSType` (16-bit: U-bit | S2 | S1 | 13-bit function code); `Scope()` returns the S2/S1 flooding scope; `Known()` reports the eight base types; `LSAKey` is `(LS Type, Link State ID, Advertising Router)`; the base type constants `LSTypeRouter` (0x2001) .. `LSTypeIntraAreaPrefix` (0x2009)
  -> Constraint: the v6 scope-aware decode/filter is built on `LSType.Scope()` and the function-code split; ext-14 reads these, it does not redefine the type model; the v6 inject command parses an LS Type and derives its scope from S2/S1, validating S2/S1 != 11 (reserved)
- [ ] `internal/plugins/ospf/v3/packet/lsa.go` + the per-type `v3/packet/lsa_*.go` (router/network/interarea_prefix/interarea_router/external/nssa/link/intraarea_prefix) -- the v3 LSA decoders the base ships; `internal/plugins/ospf/v3/packet/prefix.go` decodes the RFC 5340 §A.4.1 IPv6 prefix
  -> Constraint: the base v3 LSA types decode into typed bodies already; ext-14's v6 database detail view calls these base decoders (registered as defaults for the eight base types) and falls back to header + body-hex for unknown function codes; ext-14 adds NO base codec
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` -- `Neighbor`/`Snapshot` carry `DDSequence`, `Options`, and the v3 `InterfaceID` (zero for OSPFv2); `Address` is the neighbor identity (link-local for v3); the NSM state and per-neighbor list sizes
  -> Constraint: the neighbor-detail snapshots read these (DD seq, Options; for IPv4 the O-bit, for IPv6 the R/V6/E/N/AF bits + advertised Interface ID + link-local address) and the list sizes; ext-14 adds DETAIL snapshots without changing the existing `Snapshot` shape (base tests pin it)
- [ ] `internal/plugins/ospf/dispatcher.go` -- each engine instance carries its OSPFv3 `instanceID uint8`; `dispatch()` drops a packet whose `h.InstanceID != instanceID` (the §4.2.2 demux)
  -> Constraint: the AF-aware views read `instanceID` (and the multi-AF Instance-ID -> AF mapping when present) to identify each result's family; the v6 interface-detail view surfaces the local Instance ID; ext-14 reads this, it does not change the demux
- [ ] `internal/plugins/ospf/iface/` (`ism.go` + `election.go` + `iface.go`) -- the ISM states (Down/Loopback/Waiting/PointToPoint/DROther/Backup/DR), `electDRBDR` (by Router ID for v3), the AF-neutral interface info (`InterfaceID` for v6)
  -> Constraint: the interface-detail snapshots read the ISM state, the DR/BDR election detail, the timers, and (v6) the local Interface ID; ext-14 adds detail snapshots, it does not change the ISM/election
- [ ] `internal/plugins/ospf/cli/decode.go` + `cli/register.go` + `cli/run.go` -- the offline `ze` OSPF decode subcommand (hex -> decoded OSPFv2 LSA/packet JSON)
  -> Constraint: ext-14 extends the offline decode path so an IPv4 opaque-LSA hex renders Opaque Type/ID + typed/generic TLVs, AND adds a v3 branch (`ze ospfv3-decode` or a `--version 3` flag) that decodes a v3 packet/LSA via the v3 codec and renders the scope-aware LS Type + header + typed/generic body, both offline (no running engine), reusing the decoder registry
- [ ] `internal/plugins/ospf/doctor.go` -- the two config-sanity doctor codes (`doctor-ospf-router-id-missing`, `doctor-ospf-interface-area-unbound`); the file explicitly owns ONLY those two and must not re-register the ospf-3 raw-socket check
  -> Constraint: ext-14 adds at most a debug-enabled-sanity doctor note (a Warning when debug-injection is left enabled), registered with its own code; it must NOT touch the existing two codes; one Warning covers the shared `debug` enablement
- [ ] `internal/component/authz/authz.go` -- profile-based command-path allow/deny; `BuiltinReadOnlyProfile` denies `restart`/`kill`/`clear`; `Authorize(username, command, isReadOnly)` walks profiles, fail-closed when users are assigned
  -> Constraint: the read-only profile gains ONE `deny "debug"` entry covering both families' inject commands; the inject commands are ALSO gated by an engine-side `debug` enablement (off by default), so authz + enablement are BOTH required
- [ ] `internal/component/web/handler_ospf.go` + `snapshot_views.go` -- the generic read-only `viewSpec{command, title, streamPath, eventName}` snapshot adapter; OSPF neighbor/database web+SSE views forward `show ospf ...` / `show ospf ipv6 ...` through the `CommandDispatcher` (the `//nolint:dupl` parallel-per-protocol adapter mirroring `handler_isis.go`)
  -> Constraint: ext-14 adds opaque/TE/SR (IPv4) and per-scope/per-AF native + sr/ri/extended (IPv6) database web views by adding `viewSpec` rows + handlers (same pattern as IS-IS); the inject commands are NEVER added as web views

**Behavior to preserve:**
- The existing `show ospf ...` / `show ospf ipv6 ...` / `clear ospf ...` command set, their JSON snapshot shapes, the `ze-show:ospf-*` proxy contract, and the base + ext-13 tests that pin them.
- `SPFSnapshot`/`Snapshot`/`BorderRouterSnapshot`/`LSASnapshot`/`LinkSnapshot` shapes (base + ext-13 + ospf-8/9 tests); the SPF-explain and database-detail views are ADDITIVE.
- The ext-1 IPv4 opaque carrier behaviour (scope flooding, O-bit gate, registry, generic TLV iterator, AS-opaque store, recover wrapper) -- consumed unchanged.
- The base v3 LSA codec, the scope-aware LS-Type model, and the v6 SPF strategy -- read, never re-implemented.
- The two existing doctor codes; the read-only authz profile's existing deny entries.
- The web neighbor/database views and the looking-glass graph.

**Behavior to change:** (all additive; no existing behaviour altered)
- New IPv4 `show ospf` subcommands: `database <type> detail` (decode), `database te`, `database segment-routing`, `neighbor detail`, `interface detail`, `spf detail`.
- New IPv6 `show ospf ipv6` subcommands: `database <type> detail` (scope-aware decode), `database scope <link|area|as>`, `database segment-routing`, `database router-information`, `database extended`, `instance`, `neighbor detail`, `interface detail`, `spf detail`.
- New `debug ip ospf inject opaque ...` (IPv4) and `debug ipv6 ospf inject lsa ...` (IPv6) commands (and engine APIs), OFF by default, authz-denied for read-only, gated by a shared `debug` enablement.
- `BuiltinReadOnlyProfile` gains ONE `deny "debug"` entry.
- New web/SSE views for the opaque/TE/SR (IPv4) and the per-scope/per-AF native + sr/ri/extended (IPv6) databases.
- New `ze_ospf_debug_*` (IPv4) and `ze_ospfv3_debug_*` (IPv6) metrics.
- The offline `ze` OSPF decode path gains IPv4 opaque-TLV rendering and a v3 decode branch.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Read (introspection):** operator runs `show ospf <noun> [detail|type ...]` or `show ospf ipv6 <noun> [detail|type ...|scope ...]` (CLI/SSH, web, or looking-glass) -> the central `ze-show:ospf-*` / `ze-show:ospfv3-*` proxy -> `ForwardToPlugin` -> the engine `OnExecuteCommand` switch (the v6 commands hit the v6 engine instance) -> a typed snapshot method -> JSON -> `ApplyPipes` -> rendered.
- **Decode (offline):** operator runs the `ze` OSPF decode subcommand on opaque-LSA hex (IPv4) or v3 LSA/packet hex (IPv6, v3 branch) -> `cli/decode.go` -> the family codec (ext-1 `OpaqueType()`/`OpaqueID()` for IPv4; the v3 codec + `LSType.Scope()` for IPv6) -> registered typed decoder or generic fallback -> rendered, no running engine.
- **Inject (guarded write):** operator runs `debug ip ospf inject opaque scope <s> id <id> ...` (IPv4) or `debug ipv6 ospf inject lsa scope <s> type <t> id <id> ...` (IPv6) -> authz check (`deny debug` for read-only) -> the `ze-debug:ospf-inject` / `ze-debug:ospfv3-inject` proxy -> engine switch -> debug-enabled gate -> the family origination seam (IPv4: ext-1 `OnOriginate`/`OriginateOpaque`; IPv6: base `OriginateSelf` for area/AS, `OriginateLinkSelf` for link-local) -> `ze_ospf_debug_*` / `ze_ospfv3_debug_*` metrics update.

### Transformation Path
1. **Snapshot (read):** the engine method assembles a typed value snapshot from existing state: `lsdb.Snapshot()` (Areas + ASExternal + Links) for database views, the shared `spf` computer for SPF-explain, the neighbor/interface tables for the detail dumps, and (IPv6) the per-engine `instanceID` (+ multi-AF map) for AF identity. No new state is created.
2. **Body decode enrichment (per AF):**
   - **IPv4:** for an opaque LSA, the view looks up the decoder registry by Opaque Type. A registered typed decoder (TE/RI/Extended/SR/Grace) renders the body into named TLVs; else the ext-1 generic TLV iterator yields `(type, length, value-hex)` rows.
   - **IPv6:** for a native LSA, the view derives the scope from `LSType.Scope()` (S2/S1) and looks up the registry by LS Type / function code. A registered decoder (the base eight, or the RI/extended/SR/Grace ones) renders named fields/TLVs; else the generic view yields the 20-byte header subfields + body length/hex.
   - In both families a malformed body increments `ze_ospf_debug_decode_errors_total` / `ze_ospfv3_debug_decode_errors_total` and renders as raw hex, never panicking.
3. **SPF-explain (shared):** the detail view reads the last SPF result (winning `RouteEntry` per prefix + per-area `spfState`) and the candidate/tie-break data, composing a per-prefix explanation: candidate paths considered, each candidate's cost composition, and the §16.x rule that selected the winner. The IPv6 result is additionally tagged with its AF/Instance-ID.
4. **AF-aware identification (IPv6):** each v6 database / SPF / instance result is tagged with its address family (from the Instance-ID range, RFC 5838 §2; default IPv6-unicast when only the base instance is configured) and Instance ID; `show ospf ipv6 instance` enumerates the active instances.
5. **Render:** the snapshot is marshalled to JSON and routed through `ApplyPipes`; data-transform pipes (`resolve`/`origin`) decorate IP-bearing fields.
6. **Inject (write, per AF):** the engine validates the scope and the body (a TLV/prefix list built buffer-first, or raw hex), and -- only if the `debug` enablement is on -- calls the family origination seam. IPv4 uses a Private-Use Opaque Type; IPv6 derives the scope from the LS Type S2/S1 bits (rejecting 11). The seam owns sequencing/install/flood; ext-14 records the injected key (IPv4 `(scope, opaque-type, opaque-id)`; IPv6 `(scope, LS Type, Link State ID)`) for later withdraw. `withdraw` re-originates at MaxAge via the same seam.
7. **Web/SSE (read only):** the generic snapshot-view adapter re-fetches the database snapshots every refresh interval and pushes them over SSE; the inject path is absent from the web surface in both families.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/web/LG <-> engine | the central `ze-show:ospf-*` / `ze-show:ospfv3-*` / `ze-debug:ospf-inject` / `ze-debug:ospfv3-inject` proxy -> `ForwardToPlugin` -> engine `OnExecuteCommand` (no re-Dispatch) | [ ] |
| Engine <-> IPv4 ext-1 carrier (inject) | `OnOriginate`/`OriginateOpaque` builds + floods the injected opaque LSA; withdraw via MaxAge | [ ] |
| Engine <-> IPv6 base seam (inject) | `OriginateSelf` (area/AS) / `OriginateLinkSelf` (link-local) builds + floods the injected v3 LSA; withdraw via MaxAge | [ ] |
| LSA body <-> typed decoder | the ext-14 per-family decoder registry (Opaque Type for IPv4; LS Type / function code for IPv6); fallback to the generic iterator / scope-aware body-hex view | [ ] |
| Engine <-> SPF computer (shared) | read-only access to the last result + candidate/tie-break data for the explain view | [ ] |
| Engine <-> Instance-ID / AF map (IPv6) | read the per-engine `instanceID` (+ multi-AF map) to tag each result's address family | [ ] |
| Authz <-> inject command | the read-only profile denies `debug`; the engine debug-enablement gate is a second, independent check (both families) | [ ] |
| Engine <-> web/SSE | the generic `snapshot_views.go` adapter forwards read-only database commands; inject is never wired here | [ ] |

### Integration Points
- `internal/plugins/ospf/cmd_show.go` -- new `ze-show:ospf-*` / `ze-show:ospfv3-*` + `ze-debug:ospf-inject` / `ze-debug:ospfv3-inject` proxies (the v6 ones distinct from the v4 ones).
- `internal/plugins/ospf/register.go` -- new `OnExecuteCommand` arms + `CommandDecl` rows for both families; the inject arms call the relevant origination seam.
- `internal/plugins/ospf` (engine) -- the per-family decoder registries, the shared SPF-explain snapshot, the neighbor/interface detail snapshots, the AF-aware instance listing (IPv6), the inject APIs, the shared debug-enablement flag.
- `internal/plugins/ospf/v3/packet` + `v3/types` -- the base v3 LSA decoders + the scope-aware `LSType`/`Scope()` (consumed, not redefined).
- `internal/plugins/ospf/cli/decode.go` -- offline IPv4 opaque-TLV decode + the v3 decode branch.
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- new command-tree nodes binding the new wire methods (both families).
- `internal/component/authz/authz.go` -- the read-only profile `deny debug` entry (shared).
- `internal/component/web/handler_ospf.go` -- the new read-only opaque/TE/SR (IPv4) + per-scope/per-AF (IPv6) web views.
- ext-1 (`RegisterOpaqueConsumer`, `OnOriginate`/`OriginateOpaque`, the generic TLV iterator, `OpaqueType()`/`OpaqueID()`) -- consumed for IPv4.
- The IPv4 ext-2/ext-3/ext-4/ext-5/ext-9 and the IPv6 RI/extended/SR/Grace consumers -- each OPTIONALLY registers a typed decoder + database view through the ext-14 per-family decoder registry (their own specs own that call).
- spec-ospf-ext-15 multi-AF (Instance-ID -> AF map) -- consumed read-only for IPv6 AF identification.

### Architectural Verification
- [ ] No bypassed layers (reads flow CLI/web -> proxy -> engine snapshot; inject flows command -> authz + debug gate -> family origination seam -> normal flooding; no second flooding path in either family)
- [ ] No unintended coupling (ext-14 names no consumer body format in generic code; decoders are registry-resolved per family; SPF access is read-only; the v4 and v6 surfaces do not clobber each other)
- [ ] No duplicated functionality (reuses `databaseSnapshotByType`, the shared `spf` computer snapshots + one `spf/explain.go`, the central proxy pattern, the ext-1 carrier + TLV iterator, the base v3 codec, the generic web snapshot-view adapter)
- [ ] Zero-copy preserved (TLV/body decode returns views over LSDB bytes; rendering is `textbuf`; inject body is buffer-first)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The IPv4 ext-1 carrier exposes `RegisterOpaqueConsumer` + an `OnOriginate`/`OriginateOpaque` seam for injection, and a generic TLV iterator + `OpaqueType()`/`OpaqueID()` for decode | `plan/spec-ospf-ext-1-opaque-framework.md` In-scope + Data Flow | IPv4 injection/decode need new carrier work; scope creep into ext-1 | `TestDebugInjectOpaqueFloods`, `TestOpaqueDecodeGenericTLV` | unvalidated |
| A-2 | The IPv6 base exposes `OriginateSelf` (area/AS) and `OriginateLinkSelf` (link-local) seams for injection, and the v3 codec + `LSType.Scope()` for scope-aware decode | `internal/plugins/ospf/origination_v6.go`, `origination_v6_link.go`, `v3/types/lsa.go` | IPv6 injection/decode need new base work; scope creep into the base | `TestDebugInjectV3LSAFloods`, `TestV3DecodeGenericBody` | unvalidated |
| A-3 | The shared `spf` computer retains (or can cheaply re-expose) the candidate set + per-prefix winning reason for the explain view without re-running SPF, and one `spf/explain.go` is AF-agnostic | `internal/plugins/ospf/spf/route.go` `BuildRoutes`/`selectBestRoutes`; `computer.go` `last`/`SPFSnapshot` | the explain view must re-run SPF or store extra state; larger change | `TestSPFExplainCandidateList`, `TestV3SPFExplainNoRecompute` | unvalidated |
| A-4 | The central proxy + engine `OnExecuteCommand` switch accept new commands additively (new RPCRegistration + new case + new CommandDecl) with no proxy-contract change, and the v6 wire methods/nouns do not collide with the v4 ones | `cmd_show.go`, `register.go` (the switch, the v6 engine) | a new dispatch mechanism is needed, or v4/v6 commands collide | `TestShowOSPFDatabaseDetailWired`, `TestShowOSPFv3DatabaseDetailWired`, `TestV3CommandsDistinctFromV4` | unvalidated |
| A-5 | The read-only authz profile + an engine debug-enablement flag together gate both inject paths; a read-only user is denied and an unconfigured router cannot inject | `authz.go` `BuiltinReadOnlyProfile`/`Authorize`; the inject command paths | injection is reachable by an unprivileged user or a default router; security regression | `TestInjectDeniedReadOnly`, `TestInjectRequiresDebugEnabled`, `TestV3InjectRequiresDebugEnabled` | unvalidated |
| A-6 | ext-14 builds and runs with NONE of the extension consumers present (typed decoders are runtime-optional; generic opaque hex/TLV (IPv4) and the base eight + body-hex (IPv6) are the fallbacks) | ext-0 umbrella; the per-family decoder-registry design | ext-14 hard-depends on consumers and cannot ship before them | `TestDecodeFallbackNoDecoder`, `TestV3DecodeFallbackNoDecoder` | unvalidated |
| A-7 | The generic `snapshot_views.go` web adapter renders the new database snapshots (both families) without a bespoke template (it forwards a `show` command and renders JSON) | `handler_ospf.go`, `snapshot_views.go`, `handler_isis.go` | each new web view needs custom templating; more work | `TestOSPFOpaqueWebView`, `TestOSPFv3DatabaseWebView` | unvalidated |
| A-8 | A Private-Use Opaque Type (128-255, RFC 5250 §9) for IPv4 debug injection cannot collide with a standards-track consumer (TE=1, grace=3, RI=4, Extended, SR) | `rfc/short/rfc5250.md` §9 | an injected IPv4 test LSA is mis-delivered to a real consumer | `TestInjectUsesPrivateOpaqueType` | unvalidated |
| A-9 | The inject paths inherit the engine MinLSInterval pacing (no faster-than-5s re-origination) and the recover wrapper / buffer-first validation (a bad inject cannot crash the engine) | ext-1 origination reuse (IPv4); base origination reuse (IPv6); RFC 5250 §8 / RFC 2328 §B | a debug inject loop can DoS the LSDB or crash the engine | `TestInjectRespectsMinLSInterval`, `TestInjectMalformedBodyRecovered`, `TestV3InjectMalformedBodyRejected` | unvalidated |
| A-10 | The LSDB `Snapshot()` exposes the per-interface Link-LSA store (`Links []LinkSnapshot`) and the v3-only `Interface`/`LinkLocalAddress` on `LSASnapshot`, so the IPv6 per-scope (link-local) filter and the v6 neighbor/interface link-local detail read existing fields | `internal/plugins/ospf/lsdb/` (the snapshot types) | the v6 link-local scope view needs new snapshot plumbing | `TestV3DatabaseScopeFilter`, `TestV3InterfaceDetailSnapshot` | unvalidated |
| A-11 | The multi-AF Instance-ID -> AF mapping is available (when ext-15 is present) and degrades to a single IPv6-unicast instance when absent, so the IPv6 AF-aware views work with or without it | spec-ospf-ext-15; `dispatcher.instanceID` | AF identification breaks when ext-15 is absent, or hard-depends on it | `TestV3InstanceListingSingleAF`, `TestV3InstanceListingMultiAF` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The inject path is reachable by an unprivileged operator or enabled by default -> a crafted LSA is flooded into the live AS (either family) | a read-only user injects; a fresh router floods a test LSA | TWO independent gates (read-only authz `deny debug` + engine `debug` enablement off by default), LOCAL-only, Private-Use Opaque Type (IPv4); `TestInjectDeniedReadOnly`/`TestV3InjectDeniedReadOnly`, `TestInjectRequiresDebugEnabled`/`TestV3InjectRequiresDebugEnabled`; doctor Warning when left enabled |
| R-2 | A malformed body (decode) or a bad inject body crashes the engine | fuzz crash; a panic in a database view (e.g. a bad RFC 5340 §A.4.1 prefix length) | IPv4 decode uses the bound-checked ext-1 TLV iterator; IPv6 decode is bound-checked over LSDB bytes; inject reuses the recover wrapper / validates buffer-first; error metrics; `TestOpaqueDecodeMalformed`, `TestV3DecodeMalformed`, `TestInjectMalformedBodyRecovered`, `TestV3InjectMalformedBodyRejected` |
| R-3 | The SPF-explain view forces an SPF re-run or mutates the installed result | route churn correlated with running `... spf detail`; a benchmark regression | the explain view is strictly read-only over the last result + candidate data; `TestSPFExplainNoRecompute`/`TestV3SPFExplainNoRecompute` assert the route table is untouched and the SPF run-count unchanged |
| R-4 | A new command exists but its dispatch key is undiscoverable (reproduces the known CLI dispatch-discovery gap) | the command works only if you already know the RPC name; help shows the RPC name not the dispatch key | each command's YANG node carries operator help naming the dispatch key; the new commands appear in completion + the dispatch-key listing; `TestNewCommandsDiscoverable`/`TestV3NewCommandsDiscoverable` |
| R-5 | A typed decoder names a consumer body format inside ext-14 generic code -> removing the consumer breaks the build | a grep finds `te`/`sr`/`grace`/`ri`/`extended` body structs referenced in generic ext-14 files | decoders register through the per-family registry from the consumer's own package; generic code only calls the registry interface + the ext-1 iterator / the base v3 codec; `TestDecodeFallbackNoDecoder`/`TestV3DecodeFallbackNoDecoder` + a self-containment grep |
| R-6 | The inject command is surfaced on the web (a remote, possibly unauthenticated, write path) | an `/ospf/inject` or `/ospfv3/inject` route appears; a web test exercises injection | the web adapter wires ONLY read-only `viewSpec` rows; inject is CLI + authz only; `TestNoInjectWebRoute`/`TestNoV3InjectWebRoute` assert no inject web route exists |
| R-7 | A pipe operator (e.g. `resolve`/`origin`) is unsupported on a new command -> inconsistent operator experience | `... database te \| json` works but `\| resolve` errors | every new command routes through `ApplyPipes`; `TestNewCommandsPipeComplete`/`TestV3NewCommandsPipeComplete` exercise each operator |
| R-8 | The injected key is not tracked, so `withdraw` cannot find the instance and a test LSA lingers | a `withdraw` returns "not found" while the LSA is still in the database | ext-14 records each injected key per family; `withdraw` re-originates at MaxAge via the purge path; `TestInjectWithdrawFlushes`/`TestV3InjectWithdrawFlushes` |
| R-9 | The v6 commands/decoders/web views clobber the v4 ones (shared `internal/plugins/ospf` package) | a v4 test fails after the v6 surface lands; a duplicate wire-method registration panics at init | distinct `ze-show:ospfv3-*` wire methods + `show ospf ipv6 ...` nouns; a snapshot test pins the full v4+v6 command/decoder/web inventory; `TestV3CommandsDistinctFromV4` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | Address family | -> | Feature Code | Test |
|-------------|----------------|---|--------------|------|
| `show ospf database opaque-area detail` from the CLI | IPv4 | -> | central proxy -> engine arm -> opaque snapshot enriched via the decoder registry / ext-1 TLV iterator | `TestShowOSPFDatabaseDetailWired` (unit) + `test/ospf/ospf-debug-decode.ci` |
| `show ospf ipv6 database router detail` from the CLI | IPv6 | -> | central proxy -> v6 engine arm -> native v3 LSA snapshot enriched via the decoder registry / generic body view | `TestShowOSPFv3DatabaseDetailWired` (unit) + `test/ospf/ospfv3-debug-decode.ci` |
| `show ospf spf detail` from the CLI | IPv4 | -> | engine arm -> shared SPF-explain snapshot from the `spf` computer's last result + candidates | `TestSPFExplainWired` (unit) + `test/ospf/ospf-debug-spf-explain.ci` |
| `show ospf ipv6 spf detail` from the CLI | IPv6 | -> | v6 engine arm -> shared SPF-explain snapshot, AF/Instance-ID-tagged | `TestV3SPFExplainWired` (unit) + `test/ospf/ospfv3-debug-spf-explain.ci` |
| `show ospf ipv6 instance` from the CLI | IPv6 | -> | v6 engine arm -> AF-aware instance listing (AF, Instance ID, areas, neighbors) | `TestV3InstanceListingWired` (unit) + `test/ospf/ospfv3-debug-instance.ci` |
| `debug ip ospf inject opaque scope area id 1 hex ...` as an authorized debug-enabled operator | IPv4 | -> | authz allow -> `ze-debug:ospf-inject` proxy -> engine arm -> debug gate -> ext-1 `OnOriginate` -> install + flood | `TestDebugInjectWired` (unit) + `test/ospf/ospf-debug-inject.ci` |
| `debug ipv6 ospf inject lsa scope area type 0x2009 id ... hex ...` as an authorized debug-enabled operator | IPv6 | -> | authz allow -> `ze-debug:ospfv3-inject` proxy -> v6 engine arm -> debug gate -> base `OriginateSelf` -> install + flood | `TestDebugInjectV3Wired` (unit) + `test/ospf/ospfv3-debug-inject.ci` |
| `debug ... ospf inject ...` as a read-only operator | both | -> | authz `deny debug` rejects before the engine is reached | `TestInjectDeniedReadOnly`, `TestV3InjectDeniedReadOnly` (unit) |
| GET `/ospf/database/opaque` (web) | IPv4 | -> | generic snapshot-view adapter forwards `show ospf database opaque-area` and renders JSON | `TestOSPFOpaqueWebView` (unit) + web e2e |
| GET `/ospfv3/database` (web) | IPv6 | -> | generic snapshot-view adapter forwards `show ospf ipv6 database` and renders JSON | `TestOSPFv3DatabaseWebView` (unit) + web e2e |
| `ze` OSPF decode of opaque-LSA hex (offline) | IPv4 | -> | `cli/decode.go` -> ext-1 codec -> decoder registry / generic TLV iterator | `test/ospf/ospf-debug-decode-offline.ci` |
| `ze` OSPFv3 decode of v3 LSA/packet hex (offline) | IPv6 | -> | `cli/decode.go` v3 branch -> v3 codec -> decoder registry / generic body view | `test/ospf/ospfv3-debug-decode-offline.ci` |

## Acceptance Criteria

| AC ID | Address family | Input / Condition | Expected Behavior |
|-------|----------------|-------------------|-------------------|
| AC-1 | IPv4 | `show ospf database opaque-area detail` with a registered typed decoder for the Opaque Type | each opaque LSA's body is rendered as named typed TLVs; the LSA header (LS age, Options incl. O-bit, LS type, Opaque Type/ID, Advertising Router, seq, checksum, length) is shown |
| AC-2 | IPv6 | `show ospf ipv6 database router detail` (a base LS type) | each LSA's body is rendered as named typed fields; the 20-byte header incl. the scope-aware LS Type (U/S2/S1 + function code), Link State ID, Advertising Router, seq, checksum, length is shown |
| AC-3 | IPv4 | `show ospf database opaque-area detail` with NO decoder for the Opaque Type | the body renders via the ext-1 generic TLV iterator (type/length/value-hex); a malformed body renders as raw hex, increments `ze_ospf_debug_decode_errors_total`, never panics |
| AC-4 | IPv6 | `show ospf ipv6 database <type> detail` with NO registered decoder | the body renders via the generic scope-aware view (header subfields + body length/hex); a malformed body (bad §A.4.1 prefix) renders as raw hex, increments `ze_ospfv3_debug_decode_errors_total`, never panics |
| AC-5 | IPv4 | `show ospf database te` after ext-2 registers its TE decoder | TE opaque LSAs (Opaque Type 1) are listed and decoded into Router-Address / Link sub-TLVs; before ext-2 lands, the view is empty (no error) |
| AC-6 | IPv4 | `show ospf database segment-routing` after ext-3/ext-4/ext-5 land | SR content (SR-Algorithm / SRGB / Prefix-SID / Adjacency-SID) is summarised; before they land, the view is empty (no error) |
| AC-7 | IPv6 | `show ospf ipv6 database router-information` / `... extended` / `... segment-routing` after the v6 RI/extended/SR consumer lands | the RI-LSA / E-LSAs / SR content are decoded into named TLVs / a summary; before the consumer lands, the views are empty (no error) |
| AC-8 | IPv6 | `show ospf ipv6 database scope <link\|area\|as>` | only LSAs whose LS Type S2/S1 bits match the requested scope are listed (link-local includes the per-interface Link-LSA store); a reserved scope (S2/S1 = 11) is rejected |
| AC-9 | both (shared) | `show ospf spf detail` / `show ospf ipv6 spf detail` for an area | the candidate vertices/paths considered, the winning path per prefix, the cost composition, and the §16.x tie-break that selected it are shown (the IPv6 result also carries AF/Instance-ID); the route table and SPF run-count are UNCHANGED (read-only) |
| AC-10 | IPv4 | `show ospf neighbor detail` / `... interface detail` | full per-neighbor state (DD seq, options incl. O-bit, list sizes, last-event, timers) and per-interface state (ISM, DR/BDR election detail, timers, opaque-capable neighbour count) beyond the summary |
| AC-11 | IPv6 | `show ospf ipv6 neighbor detail` / `... interface detail` | full per-neighbor state (link-local identity, advertised Interface ID, negotiated Instance ID, DD seq, Options incl. R/V6/E/N/AF bits, list sizes, timers) and per-interface state (ISM, local Interface ID, Instance ID, DR/BDR by Router ID, timers, link-local source) |
| AC-12 | IPv6 | `show ospf ipv6 instance` | each active OSPFv3 instance is listed with its AF (from the Instance-ID range), Instance ID, area count, and neighbor count; with only the base IPv6-unicast instance configured, exactly one instance is listed |
| AC-13 | IPv4 | `debug ip ospf inject opaque scope area id <id> hex <body>` as an authorized debug-enabled operator | a crafted opaque LSA (Private-Use Opaque Type, scope area, the given id + body) is originated into the local LSDB via the ext-1 seam, installed, and flooded per scope; `ze_ospf_debug_injections_total` + `ze_ospf_debug_injected_lsas` update |
| AC-14 | IPv6 | `debug ipv6 ospf inject lsa scope area type <ls-type> id <link-state-id> hex <body>` as an authorized debug-enabled operator | a crafted v3 LSA (the given scope/LS Type/Link State ID + body) is originated into the local LSDB via the base `OriginateSelf`/`OriginateLinkSelf` seam, installed, and flooded per scope; `ze_ospfv3_debug_injections_total` + `ze_ospfv3_debug_injected_lsas` update |
| AC-15 | both | `debug ip ospf inject opaque ... withdraw` / `debug ipv6 ospf inject lsa ... withdraw` for a previously injected LSA | the instance is MaxAge-flushed via the purge path so peers withdraw it; the relevant `..._debug_injected_lsas` gauge decrements |
| AC-16 | both | `debug ... ospf inject ...` as a read-only-profile user | the command is DENIED by authz (read-only profile `deny debug`) before the engine is reached |
| AC-17 | both | `debug ... ospf inject ...` while the engine `debug` enablement is off (the default) | the command is rejected with a clear "debug injection not enabled" error; no LSA is originated |
| AC-18 | IPv6 | an inject LS Type whose S2/S1 bits are 11 (reserved), or a Link State ID / body that overflows the LSA Length | rejected with a validation error; no LSA is originated |
| AC-19 | both | any new show/debug command piped (`\| json`, `\| ndjson`, `\| table`, `\| text`, `\| yaml`, `\| match`, `\| count`, `\| resolve`, `\| origin`, `\| log`, `\| no-more`) | every operator is supported; `resolve`/`origin` decorate the IP-bearing fields |
| AC-20 | IPv4 | the offline `ze` OSPF decode subcommand on opaque-LSA hex | renders Opaque Type/ID + typed TLVs (or generic TLV/hex) with no running engine |
| AC-21 | IPv6 | the offline `ze` OSPFv3 decode subcommand on v3 LSA/packet hex | renders the scope-aware LS Type + 20-byte header + typed/generic body with no running engine |
| AC-22 | both | GET the opaque/TE/SR (IPv4) and the per-scope/per-AF native + sr/ri/extended (IPv6) database web views and their SSE streams | read-only snapshots render and stream; NO web route exposes injection in either family |
| AC-23 | both | an injected debug LSA exists and the operator runs the matching `database` view | the injected LSA appears in the database view (the IPv4 one with its Private-Use Opaque Type), marked as locally-originated |
| AC-24 | both | a malformed inject body or a panicking decode | the recover wrapper / validation isolates it; the engine continues; the relevant `..._debug_*` error metric increments |
| AC-25 | both | the engine `debug` enablement is left on | `ze doctor` emits a Warning (debug injection enabled) via a new ext-14 doctor code; the two existing OSPF doctor codes are unaffected |
| AC-26 | both | the full v4 + v6 command/decoder/web inventory after this feature lands | the v4 commands/decoders/web views register and pass; the v6 ones are distinct (no wire-method or command-noun collision) |

## End-to-End User Stories (MANDATORY for new features)

| # | Address family | User does | Path through system | Test proving it works |
|---|----------------|-----------|--------------------|-----------------------|
| 1 | IPv4 | Inspects a received TE opaque LSA decoded into its sub-TLVs | wire -> ext-1 reception -> LSDB; `show ospf database te` -> engine snapshot -> ext-2 TE decoder -> rendered | `test/ospf/ospf-debug-decode.ci` (+ `ospf-debug-te-frr` interop) |
| 2 | IPv6 | Inspects a received Intra-Area-Prefix-LSA decoded into its IPv6 prefixes | wire -> base reception -> LSDB; `show ospf ipv6 database intra-area-prefix detail` -> v6 engine snapshot -> base decoder -> rendered | `test/ospf/ospfv3-debug-decode.ci` (+ `ospfv3-debug-decode-frr` interop) |
| 3 | both | Asks why a route won | `show ospf spf detail` / `show ospf ipv6 spf detail` -> engine -> shared `spf` computer last result + candidates -> per-prefix explanation (IPv6 AF-tagged) | `test/ospf/ospf-debug-spf-explain.ci`, `test/ospf/ospfv3-debug-spf-explain.ci` |
| 4 | IPv6 | Lists the active address-family instances | `show ospf ipv6 instance` -> v6 engine -> AF-aware listing (Instance-ID -> AF) | `test/ospf/ospfv3-debug-instance.ci` |
| 5 | IPv4 | Injects a test opaque LSA to exercise flooding without a second router | `debug ip ospf inject opaque scope area id 1 hex ...` -> authz + debug gate -> ext-1 `OnOriginate` -> install + flood; `show ospf database opaque-area` shows it | `test/ospf/ospf-debug-inject.ci` |
| 6 | IPv6 | Injects a test v3 LSA to exercise flooding without a second router | `debug ipv6 ospf inject lsa scope area type 0x2009 id ... hex ...` -> authz + debug gate -> base `OriginateSelf` -> install + flood; `show ospf ipv6 database intra-area-prefix` shows it | `test/ospf/ospfv3-debug-inject.ci` |
| 7 | both | Withdraws the injected test LSA | `debug ... ospf inject ... withdraw` -> MaxAge flush -> peers purge | `test/ospf/ospf-debug-inject.ci`, `test/ospf/ospfv3-debug-inject.ci` (withdraw steps) + `ospf-debug-inject-frr` / `ospfv3-debug-inject-frr` interop |
| 8 | both | A read-only operator is blocked from injecting | `debug ... ospf inject ...` -> authz `deny debug` -> rejected | `TestInjectDeniedReadOnly`, `TestV3InjectDeniedReadOnly` + `test/ospf/ospf-debug-authz.ci`, `test/ospf/ospfv3-debug-authz.ci` |
| 9 | both | Decodes a captured LSA offline | `ze` OSPF/OSPFv3 decode of hex -> family codec -> decoder/generic -> rendered | `test/ospf/ospf-debug-decode-offline.ci`, `test/ospf/ospfv3-debug-decode-offline.ci` |
| 10 | both | Views the database in the web UI | GET `/ospf/database/opaque` / `/ospfv3/database` -> generic snapshot adapter -> `show ... ospf database ...` -> JSON/SSE | web e2e + `TestOSPFOpaqueWebView`, `TestOSPFv3DatabaseWebView` |
| 11 | IPv6 | Dumps a neighbor in full v3 state | `show ospf ipv6 neighbor detail` -> v6 engine -> neighbor-detail snapshot (link-local, Interface ID, Instance ID, Options) | `test/ospf/ospfv3-debug-detail.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Address family | Validates | Status |
|------|------|----------------|-----------|--------|
| `TestShowOSPFDatabaseDetailWired` | `internal/plugins/ospf/cmd_show_test.go` | IPv4 | AC-1, A-4: the detail proxy + engine arm are registered and reachable | |
| `TestShowOSPFv3DatabaseDetailWired` | `internal/plugins/ospf/cmd_show_test.go` | IPv6 | AC-2, A-4: the v6 detail proxy + engine arm are registered and reachable | |
| `TestV3CommandsDistinctFromV4` | `internal/plugins/ospf/cmd_show_test.go` | both | AC-26, R-9: v6 wire methods + command nouns do not collide with the v4 ones | |
| `TestOpaqueDecodeTypedDecoder` | `internal/plugins/ospf/decode_view_test.go` | IPv4 | AC-1: a registered decoder renders named TLVs | |
| `TestOpaqueDecodeGenericTLV` / `TestDecodeFallbackNoDecoder` | `internal/plugins/ospf/decode_view_test.go` | IPv4 | AC-3, A-6, R-5: no decoder -> generic TLV iterator fallback | |
| `TestOpaqueDecodeMalformed` | `internal/plugins/ospf/decode_view_test.go` | IPv4 | AC-3, AC-24, R-2: malformed body -> raw hex, error metric, no panic | |
| `TestTEDatabaseView` / `TestSRDatabaseView` | `internal/plugins/ospf/decode_view_test.go` | IPv4 | AC-5, AC-6: TE/SR views empty pre-consumer, decoded post (stub decoder) | |
| `TestV3DecodeTypedDecoder` | `internal/plugins/ospf/decode_view_v3_test.go` | IPv6 | AC-2, AC-7: a registered decoder renders named fields/TLVs for a base + extension type | |
| `TestV3DecodeGenericBody` / `TestV3DecodeFallbackNoDecoder` | `internal/plugins/ospf/decode_view_v3_test.go` | IPv6 | AC-4, A-6, R-5: no decoder -> generic scope-aware header + body-hex fallback | |
| `TestV3DecodeMalformed` | `internal/plugins/ospf/decode_view_v3_test.go` | IPv6 | AC-4, R-2: malformed body (bad §A.4.1 prefix) -> raw hex, error metric, no panic | |
| `TestV3DatabaseScopeFilter` | `internal/plugins/ospf/decode_view_v3_test.go` | IPv6 | AC-8, A-10: per-scope filter on S2/S1 (link-local includes the Link-LSA store); reserved scope rejected | |
| `TestV3RIDatabaseView` / `TestV3SRDatabaseView` | `internal/plugins/ospf/decode_view_v3_test.go` | IPv6 | AC-7: RI/SR views empty pre-consumer, decoded post (stub decoder) | |
| `TestSPFExplainCandidateList` / `TestSPFExplainTieBreak` / `TestSPFExplainWired` | `internal/plugins/ospf/spf/explain_test.go` | both (shared) | AC-9, A-3: candidate list + §16.x tie-break rationale; reachable | |
| `TestSPFExplainNoRecompute` / `TestV3SPFExplainNoRecompute` | `internal/plugins/ospf/spf/explain_test.go`, `spf/explain_v3_test.go` | both | AC-9, R-3: route table + SPF run-count unchanged by the explain view | |
| `TestV3SPFExplainCandidateList` / `TestV3SPFExplainWired` | `internal/plugins/ospf/spf/explain_v3_test.go` | IPv6 | AC-9, A-3: candidate list + tie-break, AF-tagged, reachable | |
| `TestV3InstanceListingSingleAF` / `TestV3InstanceListingMultiAF` / `TestV3InstanceListingWired` | `internal/plugins/ospf/instance_view_test.go` | IPv6 | AC-12, A-11: AF-aware listing with/without ext-15; reachable | |
| `TestNeighborDetailSnapshot` / `TestInterfaceDetailSnapshot` | `internal/plugins/ospf/neighbor_detail_test.go`, `interface_detail_test.go` | IPv4 | AC-10: full per-neighbor + per-interface state incl. O-bit / opaque-capable count | |
| `TestV3NeighborDetailSnapshot` / `TestV3InterfaceDetailSnapshot` | `internal/plugins/ospf/neighbor_detail_v3_test.go`, `interface_detail_v3_test.go` | IPv6 | AC-11, A-10: full v3 state (link-local, Interface ID, Instance ID, Options, DR/BDR by Router ID) | |
| `TestDebugInjectWired` / `TestDebugInjectOpaqueFloods` | `internal/plugins/ospf/inject_test.go` | IPv4 | AC-13, A-1: inject -> ext-1 `OnOriginate` -> install + flood | |
| `TestDebugInjectV3Wired` / `TestDebugInjectV3LSAFloods` | `internal/plugins/ospf/inject_v3_test.go` | IPv6 | AC-14, A-2: inject -> base `OriginateSelf`/`OriginateLinkSelf` -> install + flood (per scope) | |
| `TestInjectWithdrawFlushes` / `TestV3InjectWithdrawFlushes` | `internal/plugins/ospf/inject_test.go`, `inject_v3_test.go` | both | AC-15, R-8: tracked key -> MaxAge flush | |
| `TestInjectRequiresDebugEnabled` / `TestV3InjectRequiresDebugEnabled` | `internal/plugins/ospf/inject_test.go`, `inject_v3_test.go` | both | AC-17, A-5, R-1: rejected when debug disabled | |
| `TestInjectUsesPrivateOpaqueType` | `internal/plugins/ospf/inject_test.go` | IPv4 | A-8: injected LSA uses a Private-Use Opaque Type (128-255) | |
| `TestV3InjectReservedScopeRejected` / `TestV3InjectBodyOverflowRejected` | `internal/plugins/ospf/inject_v3_test.go` | IPv6 | AC-18: S2/S1=11 and over-length body rejected | |
| `TestInjectRespectsMinLSInterval` / `TestInjectMalformedBodyRecovered` | `internal/plugins/ospf/inject_test.go` | IPv4 | AC-24, A-9, R-2: MinLSInterval pacing; recover wrapper isolates a bad inject | |
| `TestV3InjectRespectsMinLSInterval` / `TestV3InjectMalformedBodyRejected` | `internal/plugins/ospf/inject_v3_test.go` | IPv6 | AC-24, A-9, R-2: MinLSInterval pacing; a malformed body is rejected before origination | |
| `TestInjectDeniedReadOnly` / `TestV3InjectDeniedReadOnly` | `internal/component/authz/authz_test.go` | both | AC-16, A-5, R-1: read-only profile denies `debug` | |
| `TestNewCommandsDiscoverable` / `TestV3NewCommandsDiscoverable` | `internal/plugins/ospf/yang/cmd_schema_test.go` | both | R-4: new commands self-document dispatch keys + appear in completion | |
| `TestNewCommandsPipeComplete` / `TestV3NewCommandsPipeComplete` | `internal/plugins/ospf/pipe_test.go`, `pipe_v3_test.go` | both | AC-19, R-7: every pipe operator on each new command | |
| `TestOSPFOpaqueWebView` / `TestNoInjectWebRoute` | `internal/component/web/handler_ospf_test.go` | IPv4 | AC-22, A-7, R-6: read-only web views exist; no inject web route | |
| `TestOSPFv3DatabaseWebView` / `TestNoV3InjectWebRoute` | `internal/component/web/handler_ospf_test.go` | IPv6 | AC-22, A-7, R-6: read-only web views exist; no inject web route | |
| `TestDebugEnabledDoctorWarning` | `internal/plugins/ospf/doctor_test.go` | both | AC-25: doctor Warning when debug left enabled; existing codes untouched | |
| `TestOSPFOfflineDecode` / `TestV3OfflineDecode` | `internal/plugins/ospf/cli/decode_test.go`, `cli/decode_v3_test.go` | both | AC-20, AC-21: offline decode renders type/id/header + body | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Address family | Range | Last Valid | Invalid Below | Invalid Above |
|-------|----------------|-------|------------|---------------|---------------|
| Inject scope (LS type) | IPv4 | {9,10,11} | 11 | a non-opaque type rejected | a non-opaque type rejected |
| Inject Opaque ID (24-bit) | IPv4 | 0-16777215 | 16777215 | N/A | 16777216 rejected (exceeds 24 bits) |
| Inject Opaque Type (Private-Use) | IPv4 | 128-255 | 255 | 127 rejected (standards-track) | N/A (1 byte) |
| Inject body / TLV value length | IPv4 | 0-65515 | within LSA max length | N/A | a length past LSA max length rejected |
| Inject scope (LS Type S2/S1 bits) | IPv6 | {00 link-local, 01 area, 10 AS} | 10 (AS) | N/A | 11 (reserved) rejected |
| Inject LS Type (16-bit) | IPv6 | 0x0000-0xFFFF | 0xFFFF | N/A | N/A (2 bytes); S2/S1=11 rejected |
| Inject Link State ID (32-bit) | IPv6 | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (4 bytes) |
| Inject body / LSA length | IPv6 | 20-65535 (header + body) | within LSA max length | below the 20-byte header rejected | past 65535 rejected |
| IPv6 prefix length (decode, §A.4.1) | IPv6 | 0-128 | 128 | N/A | >128 -> malformed, shown as raw |
| SPF-explain / database area selector | both | valid area IDs | any configured area | an undeclared area -> empty result | N/A |

### Functional Tests
| Test | Location | Address family | End-User Scenario | Status |
|------|----------|----------------|-------------------|--------|
| `ospf-debug-decode` | `test/ospf/ospf-debug-decode.ci` | IPv4 | `show ospf database opaque-* detail` decodes TLVs (typed + generic) | |
| `ospf-debug-decode-offline` | `test/ospf/ospf-debug-decode-offline.ci` | IPv4 | offline `ze` decode of opaque hex renders Opaque Type/ID + TLVs | |
| `ospf-debug-spf-explain` | `test/ospf/ospf-debug-spf-explain.ci` | IPv4 | `show ospf spf detail` explains the winning route + tie-break | |
| `ospf-debug-detail` | `test/ospf/ospf-debug-detail.ci` | IPv4 | `show ospf neighbor detail` / `interface detail` full state | |
| `ospf-debug-inject` | `test/ospf/ospf-debug-inject.ci` | IPv4 | inject + observe + withdraw a test opaque LSA (debug enabled, authorized) | |
| `ospf-debug-authz` | `test/ospf/ospf-debug-authz.ci` | IPv4 | read-only user denied inject; debug-disabled router rejects inject | |
| `ospf-debug-pipes` | `test/ospf/ospf-debug-pipes.ci` | IPv4 | each new IPv4 command honours all pipe operators | |
| `ospfv3-debug-decode` | `test/ospf/ospfv3-debug-decode.ci` | IPv6 | `show ospf ipv6 database <type> detail` decodes bodies (typed base + generic; RI/SR steps gated on the v6 consumer) | |
| `ospfv3-debug-decode-offline` | `test/ospf/ospfv3-debug-decode-offline.ci` | IPv6 | offline `ze` OSPFv3 decode renders scope-aware type + header + body | |
| `ospfv3-debug-spf-explain` | `test/ospf/ospfv3-debug-spf-explain.ci` | IPv6 | `show ospf ipv6 spf detail` explains the winning route + tie-break (AF-tagged) | |
| `ospfv3-debug-instance` | `test/ospf/ospfv3-debug-instance.ci` | IPv6 | `show ospf ipv6 instance` lists the active AF instances | |
| `ospfv3-debug-detail` | `test/ospf/ospfv3-debug-detail.ci` | IPv6 | `show ospf ipv6 neighbor detail` / `interface detail` full v3 state | |
| `ospfv3-debug-inject` | `test/ospf/ospfv3-debug-inject.ci` | IPv6 | inject + observe + withdraw a test v3 LSA (debug enabled, authorized) | |
| `ospfv3-debug-authz` | `test/ospf/ospfv3-debug-authz.ci` | IPv6 | read-only user denied inject; debug-disabled router rejects inject | |
| `ospfv3-debug-pipes` | `test/ospf/ospfv3-debug-pipes.ci` | IPv6 | each new IPv6 command honours all pipe operators | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Address family | Peer Daemon | What It Proves | Status |
|----------|-----------|----------------|-------------|----------------|--------|
| `ospf-debug-inject-frr` | `test/interop/scenarios/ospf-debug-inject-frr/` | IPv4 | FRR `ospfd` (opaque on) | a Ze-injected Private-Use opaque LSA floods to FRR, appears in FRR's `show ip ospf database opaque-area`, and is purged on withdraw; FRR's adjacency is unaffected | |
| `ospf-debug-te-frr` | `test/interop/scenarios/ospf-debug-te-frr/` | IPv4 | FRR `ospfd` (TE on) | a TE opaque LSA originated by FRR is decoded by Ze's `show ospf database te` into the same sub-TLVs FRR shows (cross-decode parity); gated on ext-2's TE decoder | |
| `ospfv3-debug-inject-frr` | `test/interop/scenarios/ospfv3-debug-inject-frr/` | IPv6 | FRR `ospf6d` | a Ze-injected test v3 LSA floods to FRR, appears in FRR's `show ipv6 ospf6 database`, and is purged on withdraw; FRR's adjacency is unaffected | |
| `ospfv3-debug-decode-frr` | `test/interop/scenarios/ospfv3-debug-decode-frr/` | IPv6 | FRR `ospf6d` | the base v3 LSAs FRR originates (Router / Network / Intra-Area-Prefix / Link / AS-External) are decoded by Ze's `show ospf ipv6 database <type> detail` into the same fields FRR shows (cross-decode parity) | |

> Interop is required: injection changes wire behaviour (a new LSA is flooded)
> and the decode must match FRR's interpretation in each family. The raw-IP /
> multicast paths (IPv4 `224.0.0.5`/`224.0.0.6`; IPv6 `ff02::5`/`ff02::6`) are
> Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPF interop set. The IPv4 `ospf-debug-te-frr`
> step is gated on ext-2's TE decoder; the IPv6 RI/SR decode-parity step is
> gated on the v6 RI/SR consumer; until then each is skipped with a justification.

### Future (if deferring any tests)
- `ospf-debug-te-frr` (IPv4) runs only once ext-2 registers the TE decoder; the RI / SR decode-parity steps (both families) run once their owning consumer registers a decoder. Recorded here, not silently dropped. All other ACs are covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/cmd_show.go` -- new `ze-show:ospf-*` (IPv4: database detail/te/segment-routing, neighbor-detail, interface-detail, spf-detail) + distinct `ze-show:ospfv3-*` (IPv6: database detail/scope/sr/ri/extended, instance, neighbor-detail, interface-detail, spf-detail) proxies + `ze-debug:ospf-inject` (IPv4) + `ze-debug:ospfv3-inject` (IPv6) proxies; each inject proxy forwards only when authz allows
- `internal/plugins/ospf/register.go` -- new `OnExecuteCommand` arms returning the new typed snapshots for both families; the inject arms (debug-gate + the relevant origination seam); new `sdk.CommandDecl` rows
- `internal/plugins/ospf/show_database.go` -- extend `dbSubviewType`/`databaseSnapshotByType` with the IPv4 opaque detail/te/segment-routing filters + decode enrichment, AND add the v6 subview map + a v6 `databaseSnapshotByType` filtering Areas + ASExternal + the per-interface `Links` store with per-scope filtering + decode enrichment
- `internal/plugins/ospf/cli/decode.go` + `cli/register.go` + `cli/run.go` -- offline IPv4 opaque-TLV rendering AND a v3 decode branch (scope-aware type + header + typed/generic body)
- `internal/plugins/ospf/spf/route.go` -- retain/re-expose the candidate set + winning reason for the (shared, AF-agnostic) explain snapshot, without altering the installed result
- `internal/plugins/ospf/spf/computer.go` -- a read-only SPF-explain snapshot method built from the last result, tagged with AF/Instance-ID for the IPv6 case
- `internal/plugins/ospf/doctor.go` -- a NEW debug-enabled-sanity doctor code (Warning when injection left enabled, covering the shared `debug` enablement); the two existing codes untouched
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- command-tree nodes for the new IPv4 + IPv6 show subcommands and the `debug ip ospf inject opaque ...` + `debug ipv6 ospf inject lsa ...` trees, each with operator help naming the dispatch key
- `internal/component/authz/authz.go` -- `BuiltinReadOnlyProfile` gains ONE `deny "debug"` entry (covers both families)
- `internal/component/web/handler_ospf.go` -- new read-only `viewSpec` rows + handlers for the opaque/TE/SR (IPv4) + per-scope/per-AF native + sr/ri/extended (IPv6) database web/SSE views (no inject route)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new commands) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- new `show ospf`/`show ospf ipv6`/`debug ... ospf` nodes; read `ai/rules/cli-grammar.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | inject leaves use native `enumeration`/`range`/`pattern`: IPv4 scope enum {link,area,as}, opaque-id 24-bit range, opaque-type Private-Use range, body hex pattern; IPv6 scope enum {link-local,area,as}, LS Type 16-bit hex pattern, Link State ID 32-bit, body hex pattern |
| YANG custom validators | [ ] yes | a `CompleteFn` for the registered decoder types (dynamic completion of known IPv4 Opaque Types and IPv6 LS types) |
| CLI commands/flags | [ ] yes | the offline `ze` OSPF/OSPFv3 decode flags for opaque hex + v3 hex in `cli/decode.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf database type <opaque-type>`, `show ospf ipv6 database type <ls-type>` / `scope <scope>`, `debug ip ospf inject opaque scope <s> id <id> ...`, `debug ipv6 ospf inject lsa scope <s> type <t> id <id> ...` |
| Editor autocomplete | [ ] yes | automatic for the new YANG enums + `CompleteFn` for dynamic decoder types |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-debug-*.ci`, `test/ospf/ospfv3-debug-*.ci` |
| Pipe completeness | [ ] yes | each new command routes through `ApplyPipes`; `resolve`/`origin` on IP fields (`ai/rules/pipe-completeness.md`) |
| Env var registration | [ ] no | the `debug` enablement is operational runtime state, not an `environment/` leaf (a runtime `debug ... ospf` toggle, not config) |
| Doctor check for runtime dependencies | [ ] yes | a debug-enabled-sanity Warning code (no new socket/port/binary/cert; the inject path adds no runtime dependency) per `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Address family | Type | Labels |
|--------|----------------|------|--------|
| `ze_ospf_debug_injected_lsas` | IPv4 | gauge | `scope` (link/area/as) |
| `ze_ospf_debug_injections_total` | IPv4 | counter | `scope`, `action` (originate/withdraw) |
| `ze_ospf_debug_decode_errors_total` | IPv4 | counter | `opaque_type` |
| `ze_ospfv3_debug_injected_lsas` | IPv6 | gauge | `scope` (link-local/area/as) |
| `ze_ospfv3_debug_injections_total` | IPv6 | counter | `scope`, `action` (originate/withdraw) |
| `ze_ospfv3_debug_decode_errors_total` | IPv6 | counter | `ls_type` |

> These follow the ext-0 `ze_ospf_<ext>_*` / `ze_ospfv3_<ext>_*` contract (here
> `ze_ospf_debug_*` and `ze_ospfv3_debug_*`), registered by this feature's owner
> code. They are added to the ext-0 metrics mapping when this spec lands; no
> existing OSPF series is renamed.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF debug & introspection tooling (both families) |
| 2 | Config syntax changed? | [ ] no | inject is a runtime debug command, not config; no YANG config leaf added |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- the new `show ospf ...` / `show ospf ipv6 ...` detail/te/scope/segment-routing/instance + `debug ... ospf inject ...` |
| 4 | API/RPC added/changed? | [ ] yes | document the `ze-show:ospf-*` / `ze-show:ospfv3-*` / `ze-debug:ospf-inject` / `ze-debug:ospfv3-inject` RPCs under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains the debug/introspection surface + the per-family decoder registry |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a debug/introspection section (decode, explain, gated inject) for both families |
| 7 | Wire format changed? | [ ] no | no new wire format; injected LSAs are RFC 5250 opaque (IPv4) / RFC 5340 native (IPv6) LSAs |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document the per-family decoder-registry interface for the extension consumers |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5250.md`, `rfc/short/rfc2328.md`, `rfc/short/rfc5340.md`, `rfc/short/rfc5838.md` -- note the inject/observe debug surface, the §16 preference explain, and the scope-aware/AF-aware views |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- Ze's in-process inject/observe vs FRR's ospfclient / ospf6d tooling |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc + `docs/architecture/wire/ospfv3.md` -- the per-family decoder registry + the gated inject path |
| 13 | Route metadata keys added/changed? | [ ] no | introspection installs no routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the `ze_ospf_debug_*` + `ze_ospfv3_debug_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` -- the new commands + the read-only profile `deny debug` |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `cmd_show.go`, `show_database.go`, `authz.go`, `handler_ospf.go`, `cli/decode.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF / OSPFv3 CLI examples against the new subcommands |

## Files to Create
- `internal/plugins/ospf/decode_view.go` -- the IPv4 opaque/extension decode view: the decoder registry (keyed by Opaque Type), the typed-vs-generic rendering, the te/segment-routing aggregations
- `internal/plugins/ospf/decode_view_v3.go` -- the IPv6 native LSA decode view: the decoder registry (keyed by LS Type / function code), the typed-vs-generic scope-aware rendering, the sr/ri/extended aggregations, the per-scope filter
- `internal/plugins/ospf/inject.go` -- the guarded IPv4 inject API: the (shared) debug-enablement flag, scope/opaque-id/body validation, the Private-Use Opaque Type, the injected-key tracking, the ext-1 `OnOriginate` call + withdraw
- `internal/plugins/ospf/inject_v3.go` -- the guarded IPv6 inject API: scope/LS-Type/Link-State-ID/body validation, the injected-key tracking, the base `OriginateSelf`/`OriginateLinkSelf` call + withdraw
- `internal/plugins/ospf/instance_view.go` -- the IPv6 AF-aware instance listing (AF from the Instance-ID range, Instance ID, areas, neighbors)
- `internal/plugins/ospf/neighbor_detail.go` + `interface_detail.go` -- the full IPv4 per-neighbor / per-interface state snapshots
- `internal/plugins/ospf/neighbor_detail_v3.go` + `interface_detail_v3.go` -- the full IPv6 per-neighbor / per-interface state snapshots (link-local, Interface ID, Instance ID)
- `internal/plugins/ospf/spf/explain.go` -- the read-only, AF-agnostic SPF-explain snapshot (candidates + tie-break) built from the last result, shared by both families
- `internal/plugins/ospf/decode_view_test.go`, `decode_view_v3_test.go`, `inject_test.go`, `inject_v3_test.go`, `instance_view_test.go`, `neighbor_detail_test.go`, `neighbor_detail_v3_test.go`, `interface_detail_test.go`, `interface_detail_v3_test.go`, `pipe_test.go`, `pipe_v3_test.go`, `doctor_test.go` (new cases)
- `internal/plugins/ospf/spf/explain_test.go`, `spf/explain_v3_test.go`
- `internal/plugins/ospf/cli/decode_test.go`, `cli/decode_v3_test.go`
- `test/ospf/ospf-debug-decode.ci`, `ospf-debug-decode-offline.ci`, `ospf-debug-spf-explain.ci`, `ospf-debug-detail.ci`, `ospf-debug-inject.ci`, `ospf-debug-authz.ci`, `ospf-debug-pipes.ci`
- `test/ospf/ospfv3-debug-decode.ci`, `ospfv3-debug-decode-offline.ci`, `ospfv3-debug-spf-explain.ci`, `ospfv3-debug-instance.ci`, `ospfv3-debug-detail.ci`, `ospfv3-debug-inject.ci`, `ospfv3-debug-authz.ci`, `ospfv3-debug-pipes.ci`
- `test/interop/scenarios/ospf-debug-inject-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-debug-te-frr/` -- `ze.conf`, `frr.conf`, `check.py` (gated on ext-2)
- `test/interop/scenarios/ospfv3-debug-inject-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospfv3-debug-decode-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the ext-1 seams (IPv4), the base `OriginateSelf`/`OriginateLinkSelf` seams + v3 decoders (IPv6), the shared `spf` last result, and the LSDB `Snapshot()` (incl. `Links`) exist |
| 3. Wiring phase | Wiring Test table -- the per-family proxies + engine arms + failing wiring tests |
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

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- the per-family proxies + engine arms as stubs + failing wiring tests
   - Tests: `TestShowOSPFDatabaseDetailWired`, `TestShowOSPFv3DatabaseDetailWired`, `TestV3CommandsDistinctFromV4`, `TestSPFExplainWired`, `TestV3SPFExplainWired`, `TestDebugInjectWired`, `TestDebugInjectV3Wired`, `TestInjectDeniedReadOnly`, `TestV3InjectDeniedReadOnly`
   - Files: `cmd_show.go` (new RPCRegistrations, v4 + distinct v6), `register.go` (new arms + CommandDecls), `yang/ze-ospf-cmd.yang` (new nodes), `authz.go` (`deny debug`), stub snapshot/inject functions
   - Verify: each command is reachable, the v6 commands distinct from v4; read-only is denied inject; the deeper tests still fail because the snapshots/inject are stubs
2. **Phase: Decode views + per-family decoder registries** -- generic + typed rendering
   - Tests: `TestOpaqueDecodeGenericTLV`, `TestDecodeFallbackNoDecoder`, `TestOpaqueDecodeMalformed`, `TestOpaqueDecodeTypedDecoder`, `TestTEDatabaseView`, `TestSRDatabaseView`, `TestV3DecodeGenericBody`, `TestV3DecodeFallbackNoDecoder`, `TestV3DecodeMalformed`, `TestV3DecodeTypedDecoder`, `TestV3DatabaseScopeFilter`, `TestV3RIDatabaseView`, `TestV3SRDatabaseView`, `TestOSPFOfflineDecode`, `TestV3OfflineDecode`
   - Files: `decode_view.go`, `decode_view_v3.go`, `show_database.go` (both enrichment paths + the v6 subview map / `Links` filter), `cli/decode.go` (offline IPv4 + v3 branch); register the base eight v6 LSA types as default decoders
   - Verify: generic fallback works with no decoder (both families); a stub typed decoder renders named TLVs/fields; the v6 per-scope filter keys on S2/S1; malformed bodies never panic; offline decode works
3. **Phase: SPF-explain (shared) + IPv6 instance + neighbor/interface detail** -- the read-only deep views
   - Tests: `TestSPFExplainCandidateList`, `TestSPFExplainTieBreak`, `TestSPFExplainNoRecompute`, `TestV3SPFExplainCandidateList`, `TestV3SPFExplainNoRecompute`, `TestV3InstanceListingSingleAF`, `TestV3InstanceListingMultiAF`, `TestNeighborDetailSnapshot`, `TestInterfaceDetailSnapshot`, `TestV3NeighborDetailSnapshot`, `TestV3InterfaceDetailSnapshot`
   - Files: `spf/explain.go` (+ `route.go` retain candidates, `computer.go` snapshot, AF-tag), `instance_view.go`, `neighbor_detail.go`, `interface_detail.go`, `neighbor_detail_v3.go`, `interface_detail_v3.go`
   - Verify: the explanation matches the installed winner without recompute (both families); the IPv6 instance listing degrades to a single AF without ext-15; the detail dumps surface O-bit (IPv4) and link-local / Interface ID / Instance ID (IPv6)
4. **Phase: Guarded inject (both families) + authz + doctor** -- the ospfclient-equivalent
   - Tests: `TestDebugInjectOpaqueFloods`, `TestInjectWithdrawFlushes`, `TestInjectRequiresDebugEnabled`, `TestInjectUsesPrivateOpaqueType`, `TestInjectRespectsMinLSInterval`, `TestInjectMalformedBodyRecovered`, `TestDebugInjectV3LSAFloods`, `TestV3InjectWithdrawFlushes`, `TestV3InjectRequiresDebugEnabled`, `TestV3InjectReservedScopeRejected`, `TestV3InjectBodyOverflowRejected`, `TestV3InjectRespectsMinLSInterval`, `TestV3InjectMalformedBodyRejected`, `TestDebugEnabledDoctorWarning`, `ospf-debug-inject.ci`, `ospf-debug-authz.ci`, `ospfv3-debug-inject.ci`, `ospfv3-debug-authz.ci`
   - Files: `inject.go`, `inject_v3.go`, `register.go` (inject arms), `authz.go` (`deny debug`), `doctor.go` (debug-enabled Warning)
   - Verify: inject floods through the family seam, withdraw flushes, both gates enforced, IPv4 Private-Use type, IPv6 reserved-scope / over-length rejection, paced, recovered/validated; the doctor Warning fires
5. **Phase: Pipes + web + discovery** -- the surface
   - Tests: `TestNewCommandsPipeComplete`, `TestV3NewCommandsPipeComplete`, `TestOSPFOpaqueWebView`, `TestNoInjectWebRoute`, `TestOSPFv3DatabaseWebView`, `TestNoV3InjectWebRoute`, `TestNewCommandsDiscoverable`, `TestV3NewCommandsDiscoverable`, `ospf-debug-pipes.ci`, `ospfv3-debug-pipes.ci`, `ospf-debug-detail.ci`, `ospfv3-debug-detail.ci`, `ospf-debug-spf-explain.ci`, `ospfv3-debug-spf-explain.ci`, `ospfv3-debug-instance.ci`
   - Files: `pipe` routing in each command, `handler_ospf.go` (v4 + v6 web views), `yang/ze-ospf-cmd.yang` (help text/dispatch keys), metric registration
   - Verify: all pipes (both families); read-only web views; no inject web route; commands discoverable; metrics + doctor
6. **Functional tests** -> the fifteen `.ci` cover the user-visible behaviour across both families
7. **RFC refs** -> add `// RFC 5250 Section X` / `// RFC 2328 Section 16.x` (IPv4) and `// RFC 5340 Section A.4.2.1` (scope), `// RFC 5340 Section A.4` (types), `// RFC 5838 Section 2` (AF identity) (IPv6) comments on the enforcing code
8. **Interop** -> `ospf-debug-inject-frr` (+ `ospf-debug-te-frr` once ext-2 lands), `ospfv3-debug-inject-frr`, `ospfv3-debug-decode-frr` QEMU scenarios
9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation, in the correct address family |
| Feature completeness | each user story has a working path; the inject/observe value matches FRR's ospfclient (inject, observe, withdraw) without a separate daemon in both families; decode parity with FRR for IPv4 TE and IPv6 base LSAs |
| Correctness | IPv4: scope/opaque-id/opaque-type validation, Private-Use type, MinLSInterval pacing; IPv6: scope derived from S2/S1 (not a flat type), base v3 decoders, per-scope filter includes the Link-LSA store, AF identity from the Instance-ID range, reserved-scope / over-length rejection; shared: explain tie-break matches §16.x, decode fallback exact; inject reuses the family seam (no second flooding path) |
| Naming | `ze_ospf_debug_*` / `ze_ospfv3_debug_*` metrics; distinct `ze-show:ospfv3-*` wire methods; `show ospf ...` / `show ospf ipv6 ...` / `debug ... ospf ...` nouns; no v4/v6 collision; JSON + YANG kebab-case |
| Data flow | reads are read-only over existing snapshots; inject goes through the family seam only; no consumer body format in generic code; SPF read-only |
| CLI grammar | keyword-before-value on every command; typed selectors (`type`/`scope`/`id`); inject is an operational verb, not config mutation |
| Doctor checks | the single debug-enabled Warning code registered; the two existing codes untouched |
| YANG validation | inject leaves use native enum/range/pattern; `CompleteFn` for dynamic decoder types |
| Prometheus counters | the six `ze_ospf_debug_*` / `ze_ospfv3_debug_*` series defined, registered, listed; ext-0 mapping updated |
| Rule: plugin-self-containment | decoders register from their owning consumer (per family); removing a consumer removes its decoder + view; generic fallback remains |
| Rule: pipe-completeness | every new command routes through `ApplyPipes`; `resolve`/`origin` on IP fields |
| Rule: authz | inject denied by read-only profile AND off by default (both families); never on the web |
| Rule: no v4/v6 regression | the IPv4 surface still registers + passes after the IPv6 surface lands, and vice versa |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| New show subcommands wired (both families) | `grep -rn 'ze-show:ospf-\|ze-show:ospfv3-' internal/plugins/ospf/cmd_show.go` |
| Inject proxies wired (both families) | `grep -rn 'ze-debug:ospf-inject\|ze-debug:ospfv3-inject' internal/plugins/ospf` |
| Per-family decoder registry + generic fallback | `go test ./internal/plugins/ospf -run 'Decode'` |
| SPF-explain read-only (shared) | `go test ./internal/plugins/ospf/spf -run 'Explain'` |
| AF-aware instance listing (IPv6) | `go test ./internal/plugins/ospf -run 'V3InstanceListing'` |
| Inject gated (authz + debug, both families) | `go test ./internal/component/authz -run 'InjectDeniedReadOnly' && go test ./internal/plugins/ospf -run 'InjectRequiresDebugEnabled'` |
| Six metric series registered | `grep -rn 'ze_ospf_debug_\|ze_ospfv3_debug_' internal/plugins/ospf` |
| No inject web route (both families) | `go test ./internal/component/web -run 'NoInjectWebRoute\|NoV3InjectWebRoute'` |
| Functional + interop tests present | `ls test/ospf/ospf-debug-*.ci test/ospf/ospfv3-debug-*.ci test/interop/scenarios/ospf-debug-*-frr/ test/interop/scenarios/ospfv3-debug-*-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | IPv4: scope in {9,10,11}, opaque-id <= 24 bits, opaque-type in Private-Use, body within LSA max length, hex bound-checked. IPv6: S2/S1 != 11, length within 65535, §A.4.1 prefixes well-formed. Decode bound-checked over LSDB bytes, never panics on a malformed body |
| Privilege / gate | inject denied by the read-only authz profile AND off by default (engine `debug` enablement); both required; fail-closed; both families |
| Surface minimization | inject is CLI + authz only, never web/SSE; LOCAL-only (no remote injection); a doctor Warning flags an accidentally-left-on debug enablement |
| Resource exhaustion | inject inherits MinLSInterval pacing + the existing LSDB/area caps; an inject loop cannot grow memory unbounded or out-pace flooding |
| Decoder isolation | typed-decoder + inject callbacks run under the recover wrapper (IPv4) / buffer-first validation (IPv6); a bad decoder/inject cannot crash OSPF or wedge the LSDB lock; errors counted, not surfaced to peers |
| Error leakage | decode/inject errors increment metrics and return operator-facing messages; they are not flooded to peers |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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
<!-- LIVE -->

## Core Insight
Because Ze runs ONE OSPF engine spanning two address families (the FSM,
flooding, DR election, SPF, and LSDB sequencing are AF-neutral), the debug and
introspection surface is ONE subsystem, not two. The genuinely useful part of
FRR's `ospfclient` is not its Unix socket -- it is the ability to inject and
observe LSAs for testing and research. Once the engine's origination seams exist
(the ext-1 `OnOriginate` carrier for IPv4 opaque LSAs; the base
`OriginateSelf`/`OriginateLinkSelf` for IPv6 native LSAs), that capability is just
a registered consumer driving the existing seam, plus a decode/inspect/explain
surface over snapshots Ze already produces. The feature therefore needs no new
wire format and no SPF change in either family: it is a read surface plus one
guarded write, with the external trust boundary (a socket) replaced by two
in-process gates (authz + debug enablement). The ONLY per-AF divergence is the
carrier/decode model: opaque Type/ID + TLVs (IPv4) versus native scope-aware LS
Type + body (IPv6); everything else -- the proxy pattern, the SPF-explain, the
neighbor/interface detail, the inject gating, the web adapter, the metric
contract -- is shared.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One debug subsystem for both address families | two version-split specs (ospf-ext-14 + ospfv3-ext-8) | Ze runs one `ospf` engine; the FSM/flooding/SPF/LSDB are AF-neutral and shared, so the introspection surface is shared; only the decode + filter layer is per-AF |
| Per-family decoder registry: keyed by Opaque Type (IPv4) and by LS Type / function code (IPv6) | one registry, or bake extension awareness into generic code | OSPFv2 carries extensions in opaque LSAs (RFC 5250); OSPFv3 carries them as native scope-aware LSAs (RFC 5340 §A.4.2.1); the key model differs by family; plugin-self-containment keeps the consumer owning its decoder |
| Inject through the family origination seam (ext-1 `OnOriginate` for IPv4; base `OriginateSelf`/`OriginateLinkSelf` for IPv6) | a dedicated debug flooding path | the seam owns sequence/age/install/flood; reusing it keeps debug LSAs on the same validated path as real ones, no second flooding path |
| Two independent gates (read-only authz `deny debug` + engine `debug` enablement off by default), shared across families | a single authz rule | defence in depth: an unprivileged user is denied AND a default router cannot inject even for an admin until debug is explicitly enabled |
| Private-Use Opaque Type (128-255) for injected IPv4 test LSAs; scope from S2/S1 for IPv6 | reuse a standards-track Opaque Type / a flat v6 type number | RFC 5250 §9 reserves 128-255 for Private Use (no mis-delivery); RFC 5340 §A.4.2.1 puts the v6 scope in the LS Type bits, so the v6 inject must derive scope from S2/S1 (rejecting 11) |
| Shared, AF-agnostic `spf/explain.go` reading the last result read-only, the IPv6 result AF-tagged | re-run SPF / two explain implementations | the `spf` computer is AF-neutral; one read-only explain serves both families; re-running could change install timing and waste CPU |
| Distinct `ze-show:ospfv3-*` wire methods + `show ospf ipv6` nouns alongside the IPv4 ones | reuse the v4 wire methods for v6 | v4 and v6 share `internal/plugins/ospf`; distinct names prevent a registration collision and keep each family's surface intact |
| AF-aware views read the multi-AF Instance-ID -> AF map, degrade to single AF | hard-depend on ext-15 | the feature depends only on the base engine; AF identity is additive and degrades cleanly to one IPv6-unicast instance when ext-15 is absent |
| Inject never on the web (both families) | a web inject form | a remote write path into the LSDB is unjustified surface; the web carries only read-only views, matching the guide's read-only operational-hooks intent |

## Known Limitations
- Injection is LOCAL-only (this router's LSDB, then normal flooding) in both families; there is no remote injection into a peer's database.
- Typed extension decoding is empty until the owning consumer registers its decoder: IPv4 TE/RI/SR/Grace via ext-2/3/4/5/9; IPv6 RI/extended/SR/Grace via the v6 consumer specs. Before then the generic view (opaque hex/TLV for IPv4, scope-aware header + body-hex for IPv6) is the only rendering.
- No SNMP / OSPF MIB / OSPFv3 MIB; the equivalents are exposed via CLI/JSON/web only (ext-0 rested the MIBs).
- The SPF-explain view reflects the LAST computed result; it does not replay historical SPF runs.
- Full IPv6 AF-aware identification requires spec-ospf-ext-15 multi-AF; without it every result is the base IPv6-unicast instance.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above the enforcing code:
- RFC 5250 §3 / App A.2 -- the IPv4 inject command's LS-ID split (Opaque Type / Opaque ID) and the decode view's rendering of both subfields.
- RFC 5250 §9 -- the Private-Use Opaque Type range gate on IPv4 injection.
- RFC 5250 §3.1 -- injected IPv4 opaque LSAs flood only within scope and only to opaque-capable neighbours (enforced by ext-1, relied on here).
- RFC 5250 §8 / RFC 2328 §B -- the MinLSInterval pacing the inject paths inherit.
- RFC 2328 §13.1 / §14 -- the freshness/age fields the database decode views render.
- RFC 2328 §16.1/§16.2/§16.4 -- the path-preference tie-break the shared SPF-explain view surfaces (both families).
- RFC 5340 §A.4.2.1 -- the IPv6 scope-aware LS Type decode (U/S2/S1 + function code); the per-scope filter and the reserved-scope (11) rejection on injection.
- RFC 5340 §A.4 -- the eight base IPv6 LSA-type decoders the default registry registers.
- RFC 5340 §A.4.1 -- the IPv6 prefix decode (`((PrefixLength + 31) / 32)` words) + padding validation.
- RFC 5340 §A.3.1 / §2.1 -- the Instance ID + Interface ID surfaced in the IPv6 neighbor/interface detail and the AF-aware views.
- RFC 5838 §2 -- the AF-per-Instance-ID identity the IPv6 AF-aware views read.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]
- [If docs were changed: `make ze-doc-test` result]

### Deviations from Plan
- [Differences from original plan and why]

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
| Deep LSDB inspection with full DECODE (IPv4 opaque + IPv6 scope-aware native) | functional + interop | `test/ospf/ospf-debug-decode.ci`, `test/ospf/ospfv3-debug-decode.ci`, `ospfv3-debug-decode-frr` |
| SPF computation trace/explain (shared, IPv6 AF-tagged) | functional test | `test/ospf/ospf-debug-spf-explain.ci`, `test/ospf/ospfv3-debug-spf-explain.ci` |
| TE / SR / RI database views (per family) | unit + interop | `TestTEDatabaseView`/`TestSRDatabaseView`/`TestV3RIDatabaseView`/`TestV3SRDatabaseView`, `ospf-debug-te-frr` |
| Per-AF views + instance listing (IPv6) | unit + functional | `TestV3InstanceListingMultiAF`, `test/ospf/ospfv3-debug-instance.ci` |
| Neighbor/interface deep-state dump (both families) | functional test | `test/ospf/ospf-debug-detail.ci`, `test/ospf/ospfv3-debug-detail.ci` |
| Guarded LSA injection (ospfclient-equivalent) behind authz (both families) | functional + interop | `test/ospf/ospf-debug-inject.ci`, `test/ospf/ospfv3-debug-inject.ci`, `ospf-debug-authz.ci`, `ospfv3-debug-authz.ci`, `ospf-debug-inject-frr`, `ospfv3-debug-inject-frr` |
| Structured JSON + web/LG surfacing (both families) | unit + functional | `TestOSPFOpaqueWebView`, `TestOSPFv3DatabaseWebView`, `test/ospf/ospf-debug-pipes.ci`, `test/ospf/ospfv3-debug-pipes.ci` |
| Discovery/dispatch excellent (both families) | unit | `TestNewCommandsDiscoverable`, `TestV3NewCommandsDiscoverable` |

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
- [ ] AC-1..AC-26 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`, `internal/component/{authz,web}/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (RFC 2328 / 5250 / 5340 / 5838)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the per-family decoder registry has multiple consumers: IPv4 ext-2/3/4/5/9 + IPv6 base eight + v6 consumers)
- [ ] No speculative features (decode + explain + detail + instance + gated inject only)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no consumer body in generic code; v4/v6 surfaces do not clobber each other)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-debug-inject-frr`, `ospfv3-debug-inject-frr`, `ospfv3-debug-decode-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
