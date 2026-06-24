# Spec: ospf-ext-14 -- OSPFv2 Debug & Introspection Tooling

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-1-opaque-framework.md |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-ospf-ext-1-opaque-framework.md` -- the opaque carrier this surface decodes/injects through: `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)`, the `OnOriginate(opaqueID, scope, body, withdraw)` origination seam, the generic TLV iterator, the LS-ID split (`OpaqueType()`/`OpaqueID()`), the AS-opaque store, and the consumer-callback recover wrapper
4. `plan/spec-ospf-ext-0-umbrella.md` -- the ext-family umbrella; ext-14 row (Child Decomposition), the "Out of scope (rested)" decision that ext-14 REPLACES the standalone `ospfclient` Unix-socket daemon with in-process inject/observe on the ext-1 registry, the `ze_ospf_<ext>_*` metric-naming contract, and the `show ip ospf <noun>` command-ownership model
5. `internal/plugins/ospf/cmd_show.go` -- the CENTRAL-namespace `ze-show:ospf-*` builtin-proxy RPC pattern (RPCRegistration + PluginCommand + forwardToOSPF); ext-14 adds new proxies here
6. `internal/plugins/ospf/register.go` (~330) -- `OnExecuteCommand` engine-side command switch; ext-14 adds new `show ip ospf ...` / `debug ip ospf ...` cases and `sdk.CommandDecl` rows
7. `internal/plugins/ospf/show_database.go` -- the `show ip ospf database <type>` subview pattern (filters `LSASnapshot` by `types.LSType.String()`); ext-14 adds opaque/TE/RI/SR/Extended-Link-Prefix subviews + decode
8. `internal/plugins/ospf/spf/computer.go` (~400-490) -- `Snapshot`/`BorderRouterSnapshot`/`SPFSnapshot`, `RouteEntry`, `spfState`; the SPF-trace surface reads candidate/tie-break data from here
9. `internal/plugins/ospf/spf/route.go` (~56-160) -- `RouteEntry`, `BuildRoutes`, `selectBestRoutes` (the candidate -> winner compare ext-14's SPF-explain surfaces)
10. `internal/component/authz/authz.go` -- the profile-based command-path allow/deny gate; the injection command is denied by the built-in read-only profile and a fresh `debug ip ospf` deny rule
11. `internal/component/web/handler_ospf.go` + `snapshot_views.go` -- the generic read-only web/SSE snapshot-view adapter; ext-14 adds opaque/TE/SR database web views the same way

## Task

Deliver first-class operational debugging and introspection for the OSPFv2
extension stack, and fold in the genuinely useful capability of FRR's
`ospfclient` (inject and observe Opaque LSAs for testing and research) as an
**in-process, authz-gated** surface on the ext-1 carrier. The ext-0 umbrella
records the decision: the standalone `ospfclient` Unix-socket daemon is rested;
ext-14 replaces it with the same inject/observe value delivered inside the OSPF
engine over `RegisterOpaqueConsumer`, with no separate socket or external trust
boundary.

The base OSPFv2 (`plan/spec-ospf-13`) already ships a read-only diagnostic
surface: `show ip ospf`, `... neighbor`, `... interface`, `... database` (with
per-LS-type subviews), `... route`, `... border-routers`, `... spf`, the
`clear ip ospf ...` resets, two config-sanity doctor checks, the web/SSE
neighbor+database views, and the looking-glass topology graph. ext-1 added the
opaque carrier and `show ip ospf database opaque-link|opaque-area|opaque-as`.
What is missing is the **deep extension-aware** debugging the operator needs once
TE / RI / Extended-Link-Prefix / SR / Grace bodies actually flow: a way to
decode an opaque body into its typed TLVs, to inspect the TE and SR databases as
first-class views, to explain why an SPF route won (candidate list, tie-breaks),
to dump a neighbor or interface in full state, and -- behind an explicit gate --
to inject a crafted opaque/test LSA into the local LSDB so the flooding,
reception, and consumer paths can be exercised end-to-end without a second
router.

ext-14 is a pure **introspection consumer** of the ext-1 carrier and the
existing snapshots. It originates no protocol behaviour of its own except the
guarded debug-injection path, which reuses the ext-1 origination seam
(`OnOriginate` / `OriginateOpaque`) exactly as a real consumer would. It adds NO
new wire format, NO new LSA type, and NO SPF participation. It is decode +
inspect + explain + (gated) inject, plus the CLI/JSON/web surface that exposes
all of it. Every show command routes through `ApplyPipes` for pipe-completeness,
and discovery is first-class: each command self-documents its dispatch key and
appears in completion, closing the project's known CLI dispatch-discovery gaps.

Each typed decoder (TE / RI / Extended-Link-Prefix / SR / Grace) is keyed by
Opaque Type and registered self-containedly: removing the owning consumer
(ext-2/3/4/5/9) removes its decoder registration and its database view together,
leaving the generic opaque hex/TLV view as the fallback. ext-14 never spells a
consumer's body format inside generic code; it asks the registered decoder for a
typed rendering and falls back to the ext-1 generic TLV iterator when none is
registered.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Deep LSDB inspection CLI | `show ip ospf database opaque-*` extended with full per-TLV DECODE: each opaque body rendered via its registered typed decoder (TE/RI/Extended-Link-Prefix/SR/Grace), falling back to the ext-1 generic TLV iterator (type/length/hex) when no decoder is registered; per-area, per-LS-type, per-Opaque-Type filtering |
| Opaque/extension decode helper | An offline `ze` decoder path that takes opaque-LSA hex and renders Opaque Type/ID + typed TLVs (extends the existing `internal/plugins/ospf/cli/decode.go` path), so a captured LSA can be decoded without a running engine |
| SPF compute trace / explain | `show ip ospf spf detail` (per-area): the candidate vertices considered, the winning path per prefix, the cost composition, and the tie-break that selected it (read from the `spf` computer's route/candidate data); explains WHY a route won, not just THAT it did |
| TE database view | `show ip ospf database te` (and a typed render): the Traffic-Engineering opaque LSAs (ext-2, Opaque Type 1) decoded into Router-Address / Link sub-TLVs; a no-op-empty view until ext-2 lands its decoder |
| SR database view | `show ip ospf database segment-routing`: SR-related opaque content (ext-5: SR-Algorithm / SRGB / Prefix-SID / Adjacency-SID carried in RI + Extended-Link/Prefix bodies) decoded into a Segment-Routing summary; empty until ext-3/ext-4/ext-5 land |
| Neighbor / interface deep dump | `show ip ospf neighbor detail` and `show ip ospf interface detail`: the full per-neighbor state (DD seq, options incl. O-bit, retransmission/request/summary list sizes, last-event, timers) and per-interface state (ISM, DR/BDR election detail, timers, opaque-capable neighbour count) beyond the summary rows ext-13 ships |
| Guarded LSA injection / origination | A debug-only `debug ip ospf inject opaque ...` command (and the equivalent in-process API) that registers a debug Opaque Type on the ext-1 registry and originates a crafted opaque LSA (scope + opaque-id + TLV body or raw hex) into the local LSDB via the ext-1 `OnOriginate` seam; withdraw via `debug ip ospf inject opaque ... withdraw`; OFF by default, denied by the read-only authz profile, and gated behind an explicit `debug` enablement |
| Structured JSON output | Every new show/debug command returns a typed snapshot rendered as JSON and routed through `ApplyPipes` (json/ndjson/table/text/yaml/match/count/resolve/origin/log/no-more) |
| Web / looking-glass surfacing | New read-only web/SSE views for the opaque/TE/SR databases via the generic `snapshot_views.go` adapter (mirroring `handler_ospf.go`); the injection path is NEVER surfaced on the web (CLI + authz only) |
| Metrics | `ze_ospf_debug_injected_lsas` (gauge), `ze_ospf_debug_injections_total` (counter), `ze_ospf_debug_decode_errors_total` (counter) |
| Discovery / dispatch | Each command's dispatch key is discoverable (help text names the key; the dispatch-key listing includes the new commands); no hidden RPC-name-only commands |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| The TE opaque body + sub-TLV codec itself | spec-ospf-ext-2 (ext-14 only DECODES + renders what ext-2 registers) |
| The Router-Information body codec | spec-ospf-ext-3 |
| The Extended-Link / Extended-Prefix body codec | spec-ospf-ext-4 |
| The Segment-Routing TLVs + SID logic | spec-ospf-ext-5 |
| The Grace-LSA body + GR helper | spec-ospf-ext-9 |
| The opaque carrier (scope flooding, O-bit, registry, generic TLV iterator) | spec-ospf-ext-1 (consumed, never redefined) |
| Any SPF change | none -- ext-14 reads the SPF result, it does not alter the computation |
| A standalone `ospfclient` Unix-socket daemon / external-injection socket | rested by ext-0; ext-14 delivers inject/observe in-process instead |
| SNMP / OSPF MIB (RFC 4750) | rested by ext-0 ("Defer"); ext-14 exposes equivalents via CLI/JSON/web only |
| Remote injection (inject into a peer's LSDB over the wire) | not done; injection is LOCAL-only into this router's own LSDB, then flooded by the normal ext-1 machinery |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1558-1564 ("External LSA API (ospfclient)" + "SNMP and Operational Hooks") -- the FRR capability ext-14 replaces and the directive to expose the equivalent via Ze's own CLI and web/looking-glass
  -> Decision: deliver the useful `ospfclient` capability (inject + observe Opaque LSAs for research/testing) in-process on the ext-1 registry; do NOT ship a Unix-domain-socket external-injection daemon (the guide calls it "not needed in production")
  -> Constraint: the guide says "ze should expose the equivalent via its own CLI and via the web/looking-glass components" -- the read-only introspection (decode/inspect/explain) is surfaced on CLI + web; the inject path is CLI + authz only (never web)
- [ ] `plan/spec-ospf-ext-0-umbrella.md` "Child Decomposition" (ext-14 row), "Out of scope (rested)" (standalone ospfclient), the `ze_ospf_<ext>_*` metric contract, and the `show ip ospf <noun>` command-ownership model
  -> Constraint: ext-14 uses `ze_ospf_debug_*` metric names and `show ip ospf ...` / `debug ip ospf ...` command nouns; it renames NO existing OSPF series or command
  -> Decision: ext-14 depends ONLY on ext-1 (the carrier); the typed decoders for TE/RI/Extended/SR/Grace are OPTIONAL and resolved at runtime via the registry, so ext-14 builds and ships before ext-2..ext-9 do, degrading to generic opaque hex/TLV rendering
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` "In scope", Data Flow steps 5-7, Wiring Test -- the registry, the `OnOriginate`/`OnReceive` seams, the generic TLV iterator, the LS-ID split, the AS-opaque store, the consumer-callback recover wrapper
  -> Constraint: injection MUST go through `OnOriginate` / `OriginateOpaque` (the carrier owns sequence/age/install/flood); ext-14 builds the opaque header + body and hands it over, it does not write a second origination path
  -> Constraint: decode MUST use the ext-1 generic TLV iterator as the fallback and the registered typed decoder as the primary; ext-14 interprets no TLV type itself
- [ ] `ai/rules/cli-grammar.md` -- keyword-before-value; typed selectors
  -> Constraint: every new command places a closed keyword before any value; per-Opaque-Type filtering uses a typed selector (`type <opaque-type>`); the inject command is `debug ip ospf inject opaque scope <scope> id <opaque-id> ...` (action/keywords before values, never a free-form positional)
  -> Constraint: injection is a runtime operational debug action (not a config-tree mutation), so it correctly takes an operational verb (`debug ... inject`), not `set`/`delete`
- [ ] `ai/rules/pipe-completeness.md` -- every command that produces output supports all pipe operators
  -> Constraint: each new show/debug command routes its JSON snapshot through `ApplyPipes`; data-transform pipes (`resolve`/`origin`) apply to the IP-bearing fields (advertising router, next-hops, TE link addresses)
- [ ] `ai/rules/plugin-self-containment.md` -- removing a plugin removes ALL its features
  -> Constraint: each typed decoder + its database view is registered by its owning consumer (ext-2 TE, ext-3 RI, ext-4 Extended, ext-5 SR, ext-9 Grace) through a small ext-14 decoder-registry; removing the consumer removes its decoder and view; generic opaque hex/TLV rendering remains
- [ ] `ai/rules/no-sprintf-alloc.md`, `ai/rules/buffer-first.md` -- rendering uses `textbuf`/`AppendTo`, decode is zero-copy over the LSDB bytes
  -> Constraint: all rendering uses `textbuf.Buffer`; the TLV decode returns views over the LSDB raw bytes (the ext-1 iterator), no per-TLV allocation in the hot path

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5250.md` -- the Opaque-LSA framework that the injected/decoded LSAs conform to
  -> Constraint: §3 / Appendix A.2 -- the injected/decoded Link State ID splits into Opaque Type (high 8 bits) + Opaque ID (low 24 bits); ext-14's inject command takes a typed scope (9/10/11) and an opaque-id within the 24-bit namespace, and the decode view renders both subfields
  -> Constraint: §3.1 -- an injected opaque LSA is flooded by the ext-1 machinery ONLY to opaque-capable neighbours and ONLY within its scope (link/area/AS, never into stub/NSSA for Type 11); ext-14 does not bypass these gates, it relies on them
  -> Constraint: §9 -- Opaque Type 128-255 is Private Use; the debug-injection default Opaque Type uses a Private-Use value so a crafted test LSA never collides with a standards-track consumer (TE=1, grace=3, RI=4)
  -> Constraint: §8 -- origination is rate-limited (>= 5 s, MinLSInterval); the inject path inherits the ext-1/RFC-2328 MinLSInterval pacing, it does not flood faster
- [ ] `rfc/short/rfc2328.md` -- the base LSA/flooding/SPF semantics the introspection surfaces
  -> Constraint: §13.1 "which LSA is newer" and §14 aging -- the database decode view shows LS sequence / age / checksum so the operator can reason about freshness exactly as §13.1 compares; a decoded LSA at MaxAge (§14) is shown as flushing
  -> Constraint: §16.1 two-stage Dijkstra + §16.2 inter-area + §16.4 external preference -- the SPF-explain view surfaces the candidate set and the §16.4 path-preference tie-break (intra > inter > external; Type 1 > Type 2 external) that selected the winning route; ext-14 reads the result, it does not re-derive it
  -> Constraint: §A.4.1 -- the 20-byte LSA header layout the deep database view renders (LS age, Options incl. O-bit, LS type, Link State ID, Advertising Router, LS seq, LS checksum, length)

**Key insights:** (minimal context to resume after compaction)
- ext-14 is a READ surface plus ONE guarded WRITE (debug inject); both go through ext-1's existing seams. No new wire format, no SPF change.
- Typed decoders are optional and runtime-resolved: ext-14 ships and works (generic opaque hex/TLV + base introspection) before any of ext-2..ext-9 land; a typed view fills in when its consumer registers a decoder.
- Discovery is a first-class requirement: the new commands self-document their dispatch keys and appear in completion, deliberately NOT reproducing the project's known CLI dispatch-discovery gaps.
- The inject path is OFF by default, denied by the read-only authz profile, gated by an explicit `debug` enablement, LOCAL-only (this router's LSDB, then normal flooding), uses a Private-Use Opaque Type, and is never surfaced on the web.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/cmd_show.go` -- the CENTRAL-namespace `ze-show:ospf-*` builtin-proxy RPCs: each `RPCRegistration{WireMethod, Handler, PluginCommand}` declares the plugin command it fronts; `forwardToOSPF` rejects extra args and calls `ForwardToPlugin` (the LDP/IS-IS proxy model); `dbSubviewForwarder` is the closure used for the six `show ip ospf database <type>` subviews
  -> Constraint: ext-14 adds new `ze-show:ospf-*` proxies here (decode/detail/te/segment-routing subviews) and a new `ze-debug:ospf-*` proxy for inject; each MUST declare its PluginCommand and forward via `ForwardToPlugin`, never re-Dispatch (that recurses)
- [ ] `internal/plugins/ospf/register.go` (~330 `OnExecuteCommand`, ~368 `sdk.Registration.Commands`) -- the engine-side command switch maps each `show ip ospf ...`/`clear ip ospf ...` string to an engine snapshot method; `sdk.CommandDecl` lists every command the engine claims
  -> Constraint: ext-14 adds new `case` arms (one per new command) returning a typed snapshot, plus matching `CommandDecl` rows; the inject command's arm calls the ext-1 origination seam guarded by the debug-enabled flag
- [ ] `internal/plugins/ospf/show_database.go` -- `dbSubviewType` maps `show ip ospf database <type>` to an `LSASnapshot.Type` string; `databaseSnapshotByType` filters the LSDB `Snapshot()` per area + AS-external; `filterLSAsByType` is the filter
  -> Constraint: the opaque subviews (ext-1 added opaque-link/area/as) extend this same map; ext-14's DECODE view reuses `databaseSnapshotByType` then enriches each opaque LSA with a typed/generic TLV rendering, keeping the existing filter contract
- [ ] `internal/plugins/ospf/clear.go` -- `clearResult{Action, Cleared}` JSON payload and the `clearNeighbors`/`clearCounters`/`clearProcess` engine resets
  -> Constraint: the inject path returns a parallel typed result (the injected LS-ID + scope + action) in the same small-JSON style; it does NOT reuse `clearResult`
- [ ] `internal/plugins/ospf/spf/computer.go` (~400-490) -- `Snapshot()`/`BorderRouterSnapshot()`/`SPFSnapshot()` return value rows; `spfState{Area, LastRun, Duration, NodeCount, Pending, CurrentDelay}`; `RouteEntry`/`BorderRouterEntry`/`last`/`lastBorder` hold the last computed result; `ClearSPFLog` resets the per-area state
  -> Constraint: the SPF-explain view reads the last result (`c.last` routes + the area `spfState`) and the candidate/winner data; it must add a detail snapshot WITHOUT changing the existing `SPFSnapshot` shape (ext-13 tests pin it)
- [ ] `internal/plugins/ospf/spf/route.go` (~56-160) -- `RouteEntry{...path type, cost, next-hops...}`, `BuildRoutes` builds candidates from reached vertices, `selectBestRoutes` does the per-prefix best-path compare (lower cost, ECMP merge, one winner per prefix)
  -> Constraint: the SPF-explain candidate list + tie-break rationale is derived here; ext-14 may need `BuildRoutes`/`selectBestRoutes` to retain (or re-expose) the rejected candidates and the winning reason for the detail snapshot, without altering the installed result
- [ ] `internal/plugins/ospf/cli/decode.go` + `cli/register.go` + `cli/run.go` -- the offline `ze` OSPF decode subcommand (hex -> decoded LSA)
  -> Constraint: ext-14 extends the decode path so an opaque-LSA hex input renders Opaque Type/ID + typed/generic TLVs offline (no running engine), reusing the ext-1 codec + the decoder registry
- [ ] `internal/plugins/ospf/doctor.go` -- the two config-sanity doctor codes (`doctor-ospf-router-id-missing`, `doctor-ospf-interface-area-unbound`); the file explicitly owns ONLY those two and must not re-register the ospf-3 raw-socket check
  -> Constraint: ext-14 adds at most a debug-enabled-sanity doctor note (a Warning when debug-injection is left enabled), registered with its own code; it must NOT touch the existing two codes
- [ ] `internal/component/authz/authz.go` -- profile-based command-path allow/deny; `BuiltinReadOnlyProfile` denies `restart`/`kill`/`clear`; `Authorize(username, command, isReadOnly)` walks profiles, fail-closed when users are assigned
  -> Constraint: the read-only profile gains a `deny "debug"` entry so the inject command is denied for read-only users; the inject command is also gated by an engine-side `debug` enablement (off by default), so authz + enablement are BOTH required
- [ ] `internal/component/web/handler_ospf.go` + `snapshot_views.go` -- the generic read-only `viewSpec{command, title, streamPath, eventName}` snapshot adapter; OSPF neighbor/database web+SSE views forward `show ip ospf ...` through the `CommandDispatcher`
  -> Constraint: ext-14 adds opaque/TE/SR database web views by adding `viewSpec` rows + handlers in `handler_ospf.go` (same pattern as IS-IS); the inject command is NEVER added as a web view

**Behavior to preserve:**
- The existing `show ip ospf ...` / `clear ip ospf ...` command set, their JSON snapshot shapes, the `ze-show:ospf-*` proxy contract, and the ext-13 tests that pin them.
- `SPFSnapshot`/`Snapshot`/`BorderRouterSnapshot` shapes (ext-13 + ospf-8/9 tests); the SPF-explain detail view is ADDITIVE.
- The ext-1 opaque carrier behaviour (scope flooding, O-bit gate, registry, generic TLV iterator, AS-opaque store, recover wrapper) -- ext-14 consumes it unchanged.
- The two existing doctor codes; the read-only authz profile's existing deny entries.
- The web neighbor/database views and the looking-glass graph.

**Behavior to change:** (all additive; no existing behaviour altered)
- New `show ip ospf` subcommands: `database <type> detail` (decode), `database te`, `database segment-routing`, `neighbor detail`, `interface detail`, `spf detail`.
- New `debug ip ospf inject opaque ...` command (and engine API), OFF by default, authz-denied for read-only, gated by a `debug` enablement.
- `BuiltinReadOnlyProfile` gains a `deny "debug"` entry.
- New web/SSE views for the opaque/TE/SR databases.
- New `ze_ospf_debug_*` metrics.
- The offline `ze` OSPF decode path gains opaque-TLV rendering.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Read (introspection):** operator runs `show ip ospf <noun> [detail|type ...]` (CLI/SSH, web, or looking-glass) -> the central `ze-show:ospf-*` proxy -> `ForwardToPlugin` -> the engine `OnExecuteCommand` switch -> a typed snapshot method -> JSON -> `ApplyPipes` -> rendered.
- **Decode (offline):** operator runs the `ze` OSPF decode subcommand on opaque-LSA hex -> `cli/decode.go` -> ext-1 codec (`OpaqueType()`/`OpaqueID()`) -> registered typed decoder or generic TLV iterator -> rendered, with no running engine.
- **Inject (guarded write):** operator runs `debug ip ospf inject opaque scope <s> id <id> [tlv ...|hex ...]` -> authz check (`deny debug` for read-only) -> the `ze-debug:ospf-inject` proxy -> engine switch -> debug-enabled gate -> the ext-1 `OnOriginate`/`OriginateOpaque` seam (the carrier assigns sequence, installs, floods per scope) -> `ze_ospf_debug_*` metrics update.

### Transformation Path
1. **Snapshot (read):** the engine method assembles a typed value snapshot from existing state: `lsdb.Snapshot()` for database views, the `spf` computer for SPF-explain, the neighbor/interface tables for the detail dumps. No new state is created.
2. **TLV decode enrichment:** for an opaque LSA, the view looks up the decoder registry by Opaque Type. If a consumer (ext-2/3/4/5/9) registered a typed decoder, it renders the body into named TLVs; else the ext-1 generic TLV iterator yields `(type, length, value-hex)` rows. A malformed body increments `ze_ospf_debug_decode_errors_total` and renders as raw hex, never panicking.
3. **SPF-explain:** the detail view reads the last SPF result (winning `RouteEntry` per prefix + per-area `spfState`) and the candidate/tie-break data from `route.go`, composing a per-prefix explanation: candidate paths considered, each candidate's cost composition, and the §16.x rule that selected the winner.
4. **Render:** the snapshot is marshalled to JSON and routed through `ApplyPipes`; data-transform pipes (`resolve`/`origin`) decorate IP-bearing fields.
5. **Inject (write):** the engine validates the scope (9/10/11), the opaque-id (24-bit), and the body (a TLV list built buffer-first, or raw hex), and -- only if the `debug` enablement is on -- calls the ext-1 origination seam with a Private-Use Opaque Type. The carrier owns sequencing/install/flood; ext-14 records the injected LS-ID for later withdraw. `withdraw` re-originates at MaxAge via the same seam.
6. **Web/SSE (read only):** the generic snapshot-view adapter re-fetches the opaque/TE/SR database snapshot every refresh interval and pushes it over SSE; the inject path is absent from the web surface.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/web/LG <-> engine | the central `ze-show:ospf-*` / `ze-debug:ospf-*` proxy -> `ForwardToPlugin` -> engine `OnExecuteCommand` (no re-Dispatch) | [ ] |
| Engine <-> ext-1 carrier (inject) | `OnOriginate`/`OriginateOpaque` builds + floods the injected opaque LSA; withdraw via MaxAge | [ ] |
| Opaque body <-> typed decoder | the ext-14 decoder registry (keyed by Opaque Type); fallback to the ext-1 generic TLV iterator | [ ] |
| Engine <-> SPF computer | read-only access to the last result + candidate/tie-break data for the explain view | [ ] |
| Authz <-> inject command | the read-only profile denies `debug`; the engine debug-enablement gate is a second, independent check | [ ] |
| Engine <-> web/SSE | the generic `snapshot_views.go` adapter forwards read-only database commands; inject is never wired here | [ ] |

### Integration Points
- `internal/plugins/ospf/cmd_show.go` -- new `ze-show:ospf-*` + `ze-debug:ospf-*` proxies.
- `internal/plugins/ospf/register.go` -- new `OnExecuteCommand` arms + `CommandDecl` rows; the inject arm calls the ext-1 seam.
- `internal/plugins/ospf` (engine) -- the decoder registry, the SPF-explain snapshot, the neighbor/interface detail snapshots, the inject API, the debug-enablement flag.
- `internal/plugins/ospf/cli/decode.go` -- offline opaque-TLV decode.
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- new command-tree nodes binding the new wire methods.
- `internal/component/authz/authz.go` -- the read-only profile `deny debug` entry.
- `internal/component/web/handler_ospf.go` -- the new read-only opaque/TE/SR web views.
- ext-1 (`RegisterOpaqueConsumer`, `OnOriginate`/`OriginateOpaque`, the generic TLV iterator, `OpaqueType()`/`OpaqueID()`) -- consumed.
- ext-2/ext-3/ext-4/ext-5/ext-9 -- each OPTIONALLY registers a typed decoder + database view through the ext-14 decoder registry (their own specs own that call).

### Architectural Verification
- [ ] No bypassed layers (reads flow CLI/web -> proxy -> engine snapshot; inject flows command -> authz + debug gate -> ext-1 seam -> normal flooding; no second flooding path)
- [ ] No unintended coupling (ext-14 names no consumer body format in generic code; decoders are registry-resolved; SPF access is read-only)
- [ ] No duplicated functionality (reuses `databaseSnapshotByType`, the `spf` computer snapshots, the central proxy pattern, the ext-1 carrier + TLV iterator, the generic web snapshot-view adapter)
- [ ] Zero-copy preserved (TLV decode returns views over LSDB bytes; rendering is `textbuf`; inject body is buffer-first)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ext-1 exposes `RegisterOpaqueConsumer` + an `OnOriginate`/`OriginateOpaque` origination seam ext-14 can drive for injection, and a generic TLV iterator + `OpaqueType()`/`OpaqueID()` for decode | `plan/spec-ospf-ext-1-opaque-framework.md` In-scope (consumer registry, generic TLV carriage, LS-ID split) + Data Flow steps 6-7 | injection/decode need new carrier work; scope creep into ext-1 | `TestDebugInjectOpaqueFloods`, `TestOpaqueDecodeGenericTLV` | unvalidated |
| A-2 | The `spf` computer retains (or can cheaply re-expose) the candidate set + per-prefix winning reason needed for the explain view without re-running SPF | `internal/plugins/ospf/spf/route.go` `BuildRoutes`/`selectBestRoutes`; `computer.go` `last`/`SPFSnapshot` | the explain view must re-run SPF or store extra state; larger change | `TestSPFExplainCandidateList`, `TestSPFExplainTieBreak` | unvalidated |
| A-3 | The central `ze-show:ospf-*` proxy + engine `OnExecuteCommand` switch accept new commands additively (new RPCRegistration + new case + new CommandDecl) with no change to the proxy contract | `cmd_show.go`, `register.go` (~330/~368) | a new dispatch mechanism is needed | `TestShowOSPFDatabaseDetailWired`, `TestDebugInjectWired` | unvalidated |
| A-4 | The read-only authz profile + an engine debug-enablement flag together gate the inject path; a read-only user is denied and an unconfigured router cannot inject | `authz.go` `BuiltinReadOnlyProfile`/`Authorize`; the inject command path | injection is reachable by an unprivileged user or a default router; security regression | `TestInjectDeniedReadOnly`, `TestInjectRequiresDebugEnabled` | unvalidated |
| A-5 | ext-14 builds and runs with NONE of ext-2..ext-9 present (typed decoders are runtime-optional; generic opaque hex/TLV is the fallback) | ext-0 umbrella (ext-14 depends only on ext-1); the decoder-registry design | ext-14 hard-depends on consumers and cannot ship before them | `TestDecodeFallbackNoDecoder`, build with only ext-1 present | unvalidated |
| A-6 | The generic `snapshot_views.go` web adapter renders the new opaque/TE/SR database snapshots without a bespoke template (it forwards a `show` command and renders JSON) | `handler_ospf.go`, `snapshot_views.go`, `handler_isis.go` (the dupl-marked parallel adapter) | each new web view needs custom templating; more work | `TestOSPFOpaqueWebView`, web view e2e | unvalidated |
| A-7 | A Private-Use Opaque Type (128-255, RFC 5250 §9) for debug injection cannot collide with a standards-track consumer (TE=1, grace=3, RI=4, Extended-Prefix/Link, SR) | `rfc/short/rfc5250.md` §9 registry | an injected test LSA is mis-delivered to a real consumer | `TestInjectUsesPrivateOpaqueType` | unvalidated |
| A-8 | The inject path inherits ext-1/RFC-2328 MinLSInterval pacing (no faster-than-5s re-origination) and the ext-1 consumer-callback recover wrapper (a bad inject cannot crash the engine) | ext-1 origination reuse; RFC 5250 §8 rate limits; `rfc/short/rfc2328.md` §B MinLSInterval | a debug inject loop can DoS the LSDB or crash the engine | `TestInjectRespectsMinLSInterval`, `TestInjectMalformedBodyRecovered` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The inject path is reachable by an unprivileged operator or enabled by default -> a crafted LSA is flooded into the live AS | a read-only user injects; a fresh router floods a test LSA | TWO independent gates (read-only authz `deny debug` + engine `debug` enablement off by default), LOCAL-only, Private-Use Opaque Type; `TestInjectDeniedReadOnly`, `TestInjectRequiresDebugEnabled`; doctor Warning when left enabled |
| R-2 | A malformed opaque body (decode) or a bad inject TLV crashes the engine | fuzz crash; a panic in a database view | decode uses the bound-checked ext-1 TLV iterator (never panics); inject reuses the ext-1 recover wrapper; `ze_ospf_debug_decode_errors_total`; `TestOpaqueDecodeMalformed`, `TestInjectMalformedBodyRecovered` |
| R-3 | The SPF-explain view forces an SPF re-run or mutates the installed result | route churn correlated with running `show ip ospf spf detail`; a benchmark regression | the explain view is strictly read-only over the last result + candidate data; `TestSPFExplainNoRecompute` asserts the route table is untouched and SPF run-count unchanged |
| R-4 | A new command exists but its dispatch key is undiscoverable (reproduces the known CLI dispatch-discovery gap) | the command works only if you already know the RPC name; help shows the RPC name not the dispatch key | each command's YANG node carries operator help naming the dispatch key; the new commands appear in completion + the dispatch-key listing; `TestNewCommandsDiscoverable` |
| R-5 | A typed decoder names a consumer body format inside ext-14 generic code -> removing the consumer breaks the build | a grep finds `te`/`sr`/`grace` body structs referenced in generic ext-14 files | decoders register through the ext-14 registry from the consumer's own package; generic code only calls the registry interface + the ext-1 iterator; `TestDecodeFallbackNoDecoder` + a self-containment grep |
| R-6 | The inject command is surfaced on the web (a remote, possibly unauthenticated, write path) | an `/ospf/inject` route appears; a web test exercises injection | the web adapter wires ONLY read-only `viewSpec` rows; inject is CLI + authz only; a test asserts no inject web route exists |
| R-7 | A pipe operator (e.g. `resolve`/`origin`) is unsupported on a new command -> inconsistent operator experience | `show ip ospf database te \| json` works but `\| resolve` errors | every new command routes through `ApplyPipes`; `TestNewCommandsPipeComplete` exercises each operator |
| R-8 | The injected LS-ID is not tracked, so `withdraw` cannot find the instance and a test LSA lingers | a `withdraw` returns "not found" while the LSA is still in the database | ext-14 records each injected `(scope, opaque-type, opaque-id)`; `withdraw` re-originates at MaxAge via the ext-1 purge path; `TestInjectWithdrawFlushes` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show ip ospf database opaque-area detail` from the CLI | -> | central proxy -> engine arm -> opaque snapshot enriched via the decoder registry / ext-1 TLV iterator | `TestShowOSPFDatabaseDetailWired` (unit) + `test/ospf/ospf-debug-decode.ci` |
| `show ip ospf spf detail` from the CLI | -> | engine arm -> SPF-explain snapshot from the `spf` computer's last result + candidates | `TestSPFExplainWired` (unit) + `test/ospf/ospf-debug-spf-explain.ci` |
| `debug ip ospf inject opaque scope area id 1 hex ...` as an authorized debug-enabled operator | -> | authz allow -> `ze-debug:ospf-inject` proxy -> engine arm -> debug gate -> ext-1 `OnOriginate` -> install + flood | `TestDebugInjectWired` (unit) + `test/ospf/ospf-debug-inject.ci` |
| `debug ip ospf inject ...` as a read-only operator | -> | authz `deny debug` rejects before the engine is reached | `TestInjectDeniedReadOnly` (unit) |
| GET `/ospf/database/opaque` (web) | -> | generic snapshot-view adapter forwards `show ip ospf database opaque-area` and renders JSON | `TestOSPFOpaqueWebView` (unit) + web e2e |
| `ze` OSPF decode of opaque-LSA hex (offline) | -> | `cli/decode.go` -> ext-1 codec -> decoder registry / generic TLV iterator | `test/ospf/ospf-debug-decode-offline.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show ip ospf database opaque-area detail` with a registered typed decoder for the LSA's Opaque Type | each opaque LSA's body is rendered as named typed TLVs; the LSA header (LS age, Options incl. O-bit, LS type, Opaque Type/ID, Advertising Router, seq, checksum, length) is shown |
| AC-2 | The same command with NO decoder registered for the Opaque Type | the body is rendered via the ext-1 generic TLV iterator (type/length/value-hex); a malformed body renders as raw hex, increments `ze_ospf_debug_decode_errors_total`, and never panics |
| AC-3 | `show ip ospf database te` after ext-2 registers its TE decoder | TE opaque LSAs (Opaque Type 1) are listed and decoded into Router-Address / Link sub-TLVs; before ext-2 lands, the view is empty (no error) |
| AC-4 | `show ip ospf database segment-routing` after ext-3/ext-4/ext-5 land | SR-related content (SR-Algorithm / SRGB / Prefix-SID / Adjacency-SID) is summarised; before they land, the view is empty (no error) |
| AC-5 | `show ip ospf spf detail` for an area | the candidate vertices/paths considered, the winning path per prefix, the cost composition, and the §16.x tie-break that selected it are shown; the route table and SPF run-count are UNCHANGED (read-only) |
| AC-6 | `show ip ospf neighbor detail` | per-neighbor full state (DD seq, options incl. O-bit, retransmission/request/summary list sizes, last event, timers) beyond the summary view |
| AC-7 | `show ip ospf interface detail` | per-interface full state (ISM, DR/BDR election detail, timers, opaque-capable neighbour count) beyond the summary view |
| AC-8 | `debug ip ospf inject opaque scope area id <id> hex <body>` as an authorized operator with debug enabled | a crafted opaque LSA (Private-Use Opaque Type, scope = area, the given opaque-id + body) is originated into the local LSDB via the ext-1 seam, installed, and flooded per scope; `ze_ospf_debug_injections_total` and `ze_ospf_debug_injected_lsas` update |
| AC-9 | `debug ip ospf inject opaque scope area id <id> withdraw` for a previously injected LSA | the instance is MaxAge-flushed via the ext-1 purge path so peers withdraw it; `ze_ospf_debug_injected_lsas` decrements |
| AC-10 | `debug ip ospf inject ...` as a read-only-profile user | the command is DENIED by authz (read-only profile `deny debug`) before the engine is reached |
| AC-11 | `debug ip ospf inject ...` while the engine `debug` enablement is off (the default) | the command is rejected with a clear "debug injection not enabled" error; no LSA is originated |
| AC-12 | Any new show/debug command piped (`\| json`, `\| ndjson`, `\| table`, `\| text`, `\| yaml`, `\| match`, `\| count`, `\| resolve`, `\| origin`, `\| log`, `\| no-more`) | every operator is supported; `resolve`/`origin` decorate the IP-bearing fields |
| AC-13 | The offline `ze` OSPF decode subcommand on opaque-LSA hex | renders Opaque Type/ID + typed TLVs (or generic TLV/hex) with no running engine |
| AC-14 | GET the opaque/TE/SR database web views (and their SSE streams) | read-only snapshots render and stream; NO web route exposes injection |
| AC-15 | An injected debug LSA exists and the operator runs `show ip ospf database opaque-*` | the injected LSA appears in the database view with its Private-Use Opaque Type, marked as locally-originated |
| AC-16 | A malformed inject body or a panicking decode | the ext-1 recover wrapper isolates it; the engine continues; the relevant `ze_ospf_debug_*` error metric increments |
| AC-17 | The engine `debug` enablement is left on | `ze doctor` emits a Warning (debug injection enabled) via a new ext-14 doctor code; the two existing OSPF doctor codes are unaffected |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Inspects a received TE opaque LSA decoded into its sub-TLVs | wire -> ext-1 reception -> LSDB; `show ip ospf database te` -> engine snapshot -> ext-2 TE decoder -> rendered | `test/ospf/ospf-debug-decode.ci` (+ `ospf-debug-te-frr` interop) |
| 2 | Asks why a route won | `show ip ospf spf detail` -> engine -> `spf` computer last result + candidates -> per-prefix explanation | `test/ospf/ospf-debug-spf-explain.ci` |
| 3 | Injects a test opaque LSA to exercise flooding without a second router | `debug ip ospf inject opaque scope area id 1 hex ...` -> authz + debug gate -> ext-1 `OnOriginate` -> install + flood; `show ip ospf database opaque-area` shows it | `test/ospf/ospf-debug-inject.ci` |
| 4 | Withdraws the injected test LSA | `debug ip ospf inject opaque scope area id 1 withdraw` -> ext-1 MaxAge flush -> peers purge | `test/ospf/ospf-debug-inject.ci` (withdraw step) + `ospf-debug-inject-frr` interop |
| 5 | A read-only operator is blocked from injecting | `debug ip ospf inject ...` -> authz `deny debug` -> rejected | `TestInjectDeniedReadOnly` + `test/ospf/ospf-debug-authz.ci` |
| 6 | Decodes a captured opaque LSA offline | `ze` OSPF decode of opaque hex -> ext-1 codec -> decoder/generic TLV -> rendered | `test/ospf/ospf-debug-decode-offline.ci` |
| 7 | Views the opaque database in the web UI | GET `/ospf/database/opaque` -> generic snapshot adapter -> `show ip ospf database opaque-area` -> JSON/SSE | web e2e + `TestOSPFOpaqueWebView` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowOSPFDatabaseDetailWired` | `internal/plugins/ospf/cmd_show_test.go` | AC-1, A-3: the detail proxy + engine arm are registered and reachable | |
| `TestOpaqueDecodeTypedDecoder` | `internal/plugins/ospf/decode_view_test.go` | AC-1: a registered decoder renders named TLVs | |
| `TestOpaqueDecodeGenericTLV` / `TestDecodeFallbackNoDecoder` | `internal/plugins/ospf/decode_view_test.go` | AC-2, A-5, R-5: no decoder -> generic TLV iterator fallback | |
| `TestOpaqueDecodeMalformed` | `internal/plugins/ospf/decode_view_test.go` | AC-2, AC-16, R-2: malformed body -> raw hex, error metric, no panic | |
| `TestTEDatabaseView` | `internal/plugins/ospf/decode_view_test.go` | AC-3: TE view empty pre-ext-2, decoded post-ext-2 (stub decoder) | |
| `TestSRDatabaseView` | `internal/plugins/ospf/decode_view_test.go` | AC-4: SR view empty pre-ext-5, summarised post (stub decoder) | |
| `TestSPFExplainCandidateList` / `TestSPFExplainTieBreak` | `internal/plugins/ospf/spf/explain_test.go` | AC-5, A-2: candidate list + §16.x tie-break rationale | |
| `TestSPFExplainNoRecompute` | `internal/plugins/ospf/spf/explain_test.go` | AC-5, R-3: route table + SPF run-count unchanged by the explain view | |
| `TestNeighborDetailSnapshot` | `internal/plugins/ospf/neighbor_detail_test.go` | AC-6: full per-neighbor state incl. O-bit option | |
| `TestInterfaceDetailSnapshot` | `internal/plugins/ospf/interface_detail_test.go` | AC-7: full per-interface state incl. opaque-capable count | |
| `TestDebugInjectWired` / `TestDebugInjectOpaqueFloods` | `internal/plugins/ospf/inject_test.go` | AC-8, A-1: inject -> ext-1 `OnOriginate` -> install + flood | |
| `TestInjectWithdrawFlushes` | `internal/plugins/ospf/inject_test.go` | AC-9, R-8: tracked LS-ID -> MaxAge flush | |
| `TestInjectRequiresDebugEnabled` | `internal/plugins/ospf/inject_test.go` | AC-11, A-4, R-1: rejected when debug disabled | |
| `TestInjectUsesPrivateOpaqueType` | `internal/plugins/ospf/inject_test.go` | A-7: injected LSA uses a Private-Use Opaque Type (128-255) | |
| `TestInjectRespectsMinLSInterval` / `TestInjectMalformedBodyRecovered` | `internal/plugins/ospf/inject_test.go` | A-8, AC-16, R-2: MinLSInterval pacing; recover wrapper isolates a bad inject | |
| `TestInjectDeniedReadOnly` | `internal/component/authz/authz_test.go` | AC-10, A-4, R-1: read-only profile denies `debug` | |
| `TestNewCommandsDiscoverable` | `internal/plugins/ospf/yang/cmd_schema_test.go` | R-4: new commands self-document dispatch keys + appear in completion | |
| `TestNewCommandsPipeComplete` | `internal/plugins/ospf/pipe_test.go` | AC-12, R-7: every pipe operator on each new command | |
| `TestOSPFOpaqueWebView` / `TestNoInjectWebRoute` | `internal/component/web/handler_ospf_test.go` | AC-14, A-6, R-6: read-only web views exist; no inject web route | |
| `TestDebugEnabledDoctorWarning` | `internal/plugins/ospf/doctor_test.go` | AC-17: doctor Warning when debug left enabled; existing codes untouched | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Inject scope (LS type) | {9,10,11} | 11 | a non-opaque type rejected | a non-opaque type rejected |
| Inject Opaque ID (24-bit) | 0-16777215 | 16777215 | N/A | 16777216 rejected (exceeds 24 bits) |
| Inject Opaque Type (Private-Use) | 128-255 | 255 | 127 rejected (standards-track) | N/A (1 byte) |
| Inject body / TLV value length | 0-65515 | within LSA max length | N/A | a length pushing past LSA max length rejected |
| SPF-explain area selector | valid area IDs | any configured area | an undeclared area -> empty result | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-debug-decode` | `test/ospf/ospf-debug-decode.ci` | `show ip ospf database opaque-* detail` decodes TLVs (typed + generic) | |
| `ospf-debug-decode-offline` | `test/ospf/ospf-debug-decode-offline.ci` | offline `ze` decode of opaque hex renders Opaque Type/ID + TLVs | |
| `ospf-debug-spf-explain` | `test/ospf/ospf-debug-spf-explain.ci` | `show ip ospf spf detail` explains the winning route + tie-break | |
| `ospf-debug-detail` | `test/ospf/ospf-debug-detail.ci` | `show ip ospf neighbor detail` / `interface detail` full state | |
| `ospf-debug-inject` | `test/ospf/ospf-debug-inject.ci` | inject + observe + withdraw a test opaque LSA (debug enabled, authorized) | |
| `ospf-debug-authz` | `test/ospf/ospf-debug-authz.ci` | read-only user denied inject; debug-disabled router rejects inject | |
| `ospf-debug-pipes` | `test/ospf/ospf-debug-pipes.ci` | each new command honours all pipe operators | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-debug-inject-frr` | `test/interop/scenarios/ospf-debug-inject-frr/` | FRR `ospfd` (opaque on) | a Ze-injected Private-Use opaque LSA floods to FRR, appears in FRR's `show ip ospf database opaque-area`, and is purged on withdraw; FRR's adjacency is unaffected | |
| `ospf-debug-te-frr` | `test/interop/scenarios/ospf-debug-te-frr/` | FRR `ospfd` (TE on) | a TE opaque LSA originated by FRR is decoded by Ze's `show ip ospf database te` into the same sub-TLVs FRR shows (cross-decode parity) | |

> Interop is required: injection changes wire behaviour (a new opaque LSA is
> flooded) and the TE decode must match FRR's interpretation. The raw-IP /
> multicast paths are Linux-only and run as QEMU integration tests
> (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPF interop set.
> `ospf-debug-te-frr` is gated on ext-2's TE decoder landing; until then it is
> skipped with a justification referencing ext-2.

### Future (if deferring any tests)
- `ospf-debug-te-frr` runs only once ext-2 registers the TE decoder; recorded here, not silently dropped. All other ACs are covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/cmd_show.go` -- new `ze-show:ospf-*` proxies (database detail/te/segment-routing, neighbor-detail, interface-detail, spf-detail) + a new `ze-debug:ospf-inject` proxy; the inject proxy forwards only when authz allows
- `internal/plugins/ospf/register.go` -- new `OnExecuteCommand` arms returning the new typed snapshots; the inject arm (debug-gate + ext-1 seam); new `sdk.CommandDecl` rows
- `internal/plugins/ospf/show_database.go` -- extend `dbSubviewType`/`databaseSnapshotByType` with opaque detail/te/segment-routing filters + the decode enrichment hook
- `internal/plugins/ospf/cli/decode.go` -- offline opaque-TLV rendering (Opaque Type/ID + typed/generic TLVs)
- `internal/plugins/ospf/spf/route.go` -- retain/re-expose the candidate set + winning reason for the explain snapshot (without altering the installed result)
- `internal/plugins/ospf/spf/computer.go` -- a read-only SPF-explain snapshot method built from the last result
- `internal/plugins/ospf/doctor.go` -- a NEW debug-enabled-sanity doctor code (Warning when injection left enabled); the two existing codes untouched
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- command-tree nodes for the new show subcommands + the `debug ip ospf inject opaque ...` tree, each with operator help naming the dispatch key
- `internal/component/authz/authz.go` -- `BuiltinReadOnlyProfile` gains a `deny "debug"` entry
- `internal/component/web/handler_ospf.go` -- new read-only `viewSpec` rows + handlers for the opaque/TE/SR database web/SSE views (no inject route)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new commands) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- new `show`/`debug` nodes; read `ai/rules/cli-grammar.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | the inject leaves (scope enum {link,area,as}; opaque-id 24-bit range; opaque-type Private-Use range; body hex pattern) use native `enumeration`/`range`/`pattern` |
| YANG custom validators | [ ] yes | a `CompleteFn` for the injected opaque-id / registered decoder types (dynamic completion of known Opaque Types) |
| CLI commands/flags | [ ] yes | the offline `ze` OSPF decode flag for opaque hex in `cli/decode.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ip ospf database type <opaque-type>`, `debug ip ospf inject opaque scope <s> id <id> ...` |
| Editor autocomplete | [ ] yes | automatic for the new YANG enums + `CompleteFn` for dynamic Opaque Types |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-debug-*.ci` |
| Pipe completeness | [ ] yes | each new command routes through `ApplyPipes`; `resolve`/`origin` on IP fields (`ai/rules/pipe-completeness.md`) |
| Env var registration | [ ] no | the `debug` enablement is operational runtime state, not an `environment/` leaf (a runtime `debug ip ospf` toggle, not config) |
| Doctor check for runtime dependencies | [ ] yes | a debug-enabled-sanity Warning code (no new socket/port/binary/cert; the inject path adds no runtime dependency) per `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_debug_injected_lsas` | gauge | `scope` (link/area/as) |
| `ze_ospf_debug_injections_total` | counter | `scope`, `action` (originate/withdraw) |
| `ze_ospf_debug_decode_errors_total` | counter | `opaque_type` |

> These follow the ext-0 `ze_ospf_<ext>_*` contract (here `ze_ospf_debug_*`),
> registered by ext-14's owner code. They are added to the ext-0 metrics mapping
> when this spec lands; no existing OSPF series is renamed.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF debug & introspection tooling |
| 2 | Config syntax changed? | [ ] no | inject is a runtime debug command, not config; no YANG config leaf added |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- the new `show ip ospf ... detail/te/segment-routing` + `debug ip ospf inject opaque ...` |
| 4 | API/RPC added/changed? | [ ] yes | document the `ze-show:ospf-*` / `ze-debug:ospf-inject` RPCs under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains the debug/introspection surface + decoder registry |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a debug/introspection section (decode, explain, gated inject) |
| 7 | Wire format changed? | [ ] no | no new wire format; injected LSAs are RFC 5250 opaque LSAs |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document the ext-14 decoder-registry interface for ext-2/3/4/5/9 authors |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5250.md` -- note the inject/observe debug surface; `rfc/short/rfc2328.md` -- the explain view surfaces §16 preference |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- Ze's in-process inject/observe vs FRR's ospfclient socket |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the decoder registry + the gated inject path |
| 13 | Route metadata keys added/changed? | [ ] no | introspection installs no routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the three `ze_ospf_debug_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` -- the new commands + the read-only profile `deny debug` |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `cmd_show.go`, `show_database.go`, `authz.go`, `handler_ospf.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF CLI examples against the new subcommands |

## Files to Create
- `internal/plugins/ospf/decode_view.go` -- the opaque/extension decode view: the decoder registry (keyed by Opaque Type), the typed-vs-generic rendering, the te/segment-routing aggregations
- `internal/plugins/ospf/inject.go` -- the guarded inject API: the debug-enablement flag, scope/opaque-id/body validation, the Private-Use Opaque Type, the injected-LS-ID tracking, the ext-1 `OnOriginate` call + withdraw
- `internal/plugins/ospf/neighbor_detail.go` -- the full per-neighbor state snapshot
- `internal/plugins/ospf/interface_detail.go` -- the full per-interface state snapshot
- `internal/plugins/ospf/spf/explain.go` -- the read-only SPF-explain snapshot (candidates + tie-break) built from the last result
- `internal/plugins/ospf/decode_view_test.go`, `inject_test.go`, `neighbor_detail_test.go`, `interface_detail_test.go`, `pipe_test.go`, `doctor_test.go` (new cases)
- `internal/plugins/ospf/spf/explain_test.go`
- `test/ospf/ospf-debug-decode.ci`, `ospf-debug-decode-offline.ci`, `ospf-debug-spf-explain.ci`, `ospf-debug-detail.ci`, `ospf-debug-inject.ci`, `ospf-debug-authz.ci`, `ospf-debug-pipes.ci`
- `test/interop/scenarios/ospf-debug-inject-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-debug-te-frr/` -- `ze.conf`, `frr.conf`, `check.py` (gated on ext-2)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm ext-1 seams (registry, `OnOriginate`, TLV iterator, LS-ID split) exist |
| 3. Wiring phase | Wiring Test table -- proxies + engine arms + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the new proxies + engine arms as stubs + failing wiring tests
   - Tests: `TestShowOSPFDatabaseDetailWired`, `TestSPFExplainWired`, `TestDebugInjectWired`, `TestInjectDeniedReadOnly`, `test/ospf/ospf-debug-decode.ci` (stub)
   - Files: `cmd_show.go` (new RPCRegistrations), `register.go` (new arms + CommandDecls), `yang/ze-ospf-cmd.yang` (new nodes), `authz.go` (`deny debug`), stub snapshot/inject functions
   - Verify: each command is reachable; read-only is denied inject; the deeper tests still fail because the snapshots/inject are stubs
2. **Phase: Decode view + decoder registry** -- generic + typed opaque rendering
   - Tests: `TestOpaqueDecodeGenericTLV`, `TestDecodeFallbackNoDecoder`, `TestOpaqueDecodeMalformed`, `TestOpaqueDecodeTypedDecoder`, `TestTEDatabaseView`, `TestSRDatabaseView`
   - Files: `decode_view.go`, `show_database.go` (enrichment), `cli/decode.go` (offline)
   - Verify: generic TLV fallback works with no decoder; a stub typed decoder renders named TLVs; malformed bodies never panic
3. **Phase: SPF-explain** -- candidates + tie-break, read-only
   - Tests: `TestSPFExplainCandidateList`, `TestSPFExplainTieBreak`, `TestSPFExplainNoRecompute`
   - Files: `spf/explain.go`, `spf/route.go` (retain candidates), `spf/computer.go` (snapshot)
   - Verify: the explanation matches the installed winner; SPF run-count + route table untouched
4. **Phase: Neighbor / interface detail** -- the deep-state dumps
   - Tests: `TestNeighborDetailSnapshot`, `TestInterfaceDetailSnapshot`
   - Files: `neighbor_detail.go`, `interface_detail.go`
   - Verify: full state incl. O-bit / opaque-capable counts
5. **Phase: Guarded inject** -- the ospfclient-equivalent
   - Tests: `TestDebugInjectOpaqueFloods`, `TestInjectWithdrawFlushes`, `TestInjectRequiresDebugEnabled`, `TestInjectUsesPrivateOpaqueType`, `TestInjectRespectsMinLSInterval`, `TestInjectMalformedBodyRecovered`, `ospf-debug-inject.ci`, `ospf-debug-authz.ci`
   - Files: `inject.go`, `register.go` (inject arm), `doctor.go` (debug-enabled Warning)
   - Verify: inject floods through ext-1, withdraw flushes, both gates enforced, Private-Use type, paced, recovered
6. **Phase: Pipes + web + discovery** -- the surface
   - Tests: `TestNewCommandsPipeComplete`, `TestOSPFOpaqueWebView`, `TestNoInjectWebRoute`, `TestNewCommandsDiscoverable`, `TestDebugEnabledDoctorWarning`, `ospf-debug-pipes.ci`, `ospf-debug-detail.ci`, `ospf-debug-decode-offline.ci`, `ospf-debug-spf-explain.ci`
   - Files: `pipe` routing in each command, `handler_ospf.go` (web views), `yang/ze-ospf-cmd.yang` (help text/dispatch keys), metric registration
   - Verify: all pipes; read-only web views; no inject web route; commands discoverable; metrics + doctor
7. **Functional tests** -> the seven `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 5250 Section X` / `// RFC 2328 Section 16.x` comments on the inject scope/Opaque-Type gating and the explain preference rendering
9. **Interop** -> `ospf-debug-inject-frr` (and `ospf-debug-te-frr` once ext-2 lands) QEMU scenarios
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; the inject/observe value matches FRR's ospfclient (inject, observe, withdraw) without a separate daemon; decode parity with FRR for TE |
| Correctness | scope/opaque-id/opaque-type validation; Private-Use type; MinLSInterval pacing; explain tie-break matches §16.x; decode fallback exact |
| Naming | `ze_ospf_debug_*` metrics; `show ip ospf ...`/`debug ip ospf ...` nouns; JSON keys kebab-case; YANG kebab-case |
| Data flow | reads are read-only over existing snapshots; inject goes through ext-1 only; no consumer body format in generic code |
| CLI grammar | keyword-before-value on every command; typed selectors (`type`/`scope`/`id`); inject is an operational verb, not config mutation |
| Doctor checks | the debug-enabled Warning code registered; the two existing codes untouched |
| YANG validation | inject leaves use native enum/range/pattern; `CompleteFn` for dynamic Opaque Types |
| Prometheus counters | the three `ze_ospf_debug_*` series defined, registered, listed; ext-0 mapping updated |
| Rule: plugin-self-containment | decoders register from their owning consumer; removing a consumer removes its decoder + view; generic fallback remains |
| Rule: pipe-completeness | every new command routes through `ApplyPipes`; `resolve`/`origin` on IP fields |
| Rule: authz | inject denied by read-only profile AND off by default; never on the web |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| New show subcommands wired | `grep -rn 'ze-show:ospf-' internal/plugins/ospf/cmd_show.go` |
| Inject proxy wired | `grep -rn 'ze-debug:ospf-inject' internal/plugins/ospf` |
| Decoder registry + generic fallback | `go test ./internal/plugins/ospf -run 'Decode'` |
| SPF-explain read-only | `go test ./internal/plugins/ospf/spf -run 'Explain'` |
| Inject gated (authz + debug) | `go test ./internal/component/authz -run TestInjectDeniedReadOnly && go test ./internal/plugins/ospf -run TestInjectRequiresDebugEnabled` |
| Three metric series registered | `grep -rn 'ze_ospf_debug_' internal/plugins/ospf` |
| No inject web route | `go test ./internal/component/web -run TestNoInjectWebRoute` |
| Functional + interop tests present | `ls test/ospf/ospf-debug-*.ci test/interop/scenarios/ospf-debug-*-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | scope ∈ {9,10,11}; opaque-id ≤ 24 bits; opaque-type ∈ Private-Use; body within LSA max length; hex parse bound-checked; reject otherwise |
| Privilege / gate | inject denied by the read-only authz profile AND off by default; both required; fail-closed |
| Surface minimization | inject is CLI + authz only, never web/SSE; LOCAL-only (no remote injection); a doctor Warning flags an accidentally-left-on debug enablement |
| Resource exhaustion | inject inherits MinLSInterval pacing + the ext-1/area LSA caps; an inject loop cannot grow memory unbounded or out-pace flooding |
| Decoder isolation | typed-decoder + inject callbacks run under the ext-1 recover wrapper; a bad decoder/inject cannot crash OSPF or wedge the LSDB lock; errors counted, not surfaced to peers |
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
The genuinely useful part of FRR's `ospfclient` is not its Unix socket -- it is
the ability to inject and observe Opaque LSAs for testing and research. Once the
ext-1 carrier exists, that capability is just a registered consumer driving the
existing origination seam, plus a decode/inspect/explain surface over snapshots
Ze already produces. ext-14 therefore needs no new wire format and no SPF change:
it is a read surface plus one guarded write, both riding ext-1's seams, with the
external trust boundary (a socket) replaced by two in-process gates (authz +
debug enablement).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Inject in-process on the ext-1 registry | a standalone `ospfclient` Unix-socket daemon | ext-0 rested the daemon; the useful inject/observe value fits in-process with no new socket or external trust boundary; the carrier already owns origination |
| Two independent gates (read-only authz `deny debug` + engine `debug` enablement off by default) | a single authz rule | defence in depth: an unprivileged user is denied, AND a default router cannot inject even for an admin until debug is explicitly enabled |
| Private-Use Opaque Type (128-255) for injected test LSAs | reuse a standards-track type (TE=1, etc.) | RFC 5250 §9 reserves 128-255 for Private Use; a test LSA must never be mis-delivered to a real consumer |
| Typed decoders registered by their owning consumer, runtime-resolved | bake TE/RI/SR awareness into ext-14 | plugin-self-containment: removing ext-2/3/4/5/9 removes its decoder + view; ext-14 ships and works on generic opaque rendering before any consumer lands |
| SPF-explain reads the last result read-only | re-run SPF for the explanation | re-running could change install timing and waste CPU; the explanation must reflect the installed winner exactly |
| Inject never on the web | a web inject form | a remote write path into the LSDB is unjustified surface; the web carries only read-only views, matching the guide's read-only operational-hooks intent |

## Known Limitations
- Injection is LOCAL-only (this router's LSDB, then normal flooding); there is no remote injection into a peer's database.
- Typed TE/RI/SR/Grace decoding is empty until the owning consumer (ext-2/3/4/5/9) registers its decoder; before then the generic opaque hex/TLV view is the only rendering.
- No SNMP / OSPF MIB; the equivalents are exposed via CLI/JSON/web only (ext-0 rested the MIB).
- The SPF-explain view reflects the LAST computed result; it does not replay historical SPF runs.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above the enforcing code:
- RFC 5250 §3 / App A.2 -- the inject command's LS-ID split (Opaque Type / Opaque ID) and the decode view's rendering of both subfields.
- RFC 5250 §9 -- the Private-Use Opaque Type range gate on injection.
- RFC 5250 §3.1 -- injected opaque LSAs flood only within scope and only to opaque-capable neighbours (enforced by ext-1, relied on here).
- RFC 5250 §8 / RFC 2328 §B -- the MinLSInterval pacing the inject path inherits.
- RFC 2328 §13.1 / §14 -- the freshness/age fields the database decode view renders.
- RFC 2328 §16.1/§16.2/§16.4 -- the path-preference tie-break the SPF-explain view surfaces.

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
| Deep LSDB inspection with full opaque DECODE | functional test | `test/ospf/ospf-debug-decode.ci` (typed + generic TLV rendering) |
| SPF computation trace/explain | functional test | `test/ospf/ospf-debug-spf-explain.ci` (winning route + tie-break) |
| TE and SR database views | unit + interop | `TestTEDatabaseView`/`TestSRDatabaseView`, `ospf-debug-te-frr` (cross-decode parity) |
| Neighbor/interface deep-state dump | functional test | `test/ospf/ospf-debug-detail.ci` |
| Guarded LSA injection (ospfclient-equivalent) behind authz | functional + interop | `test/ospf/ospf-debug-inject.ci`, `ospf-debug-authz.ci`, `ospf-debug-inject-frr` |
| Structured JSON + web/LG surfacing | unit + functional | `TestOSPFOpaqueWebView`, `test/ospf/ospf-debug-pipes.ci` |
| Discovery/dispatch excellent | unit | `TestNewCommandsDiscoverable` |

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
