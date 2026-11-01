# Spec: filter-wire-6-stage-filters -- stage-attached plugin filters replace `filterapi.Register`

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P3 |
| Release reason | Child of `spec-filter-wire-0-umbrella` (post-release engineering). It also fixes shipped misbehaviour: `bgp-role` (RFC 9234 Section 5) and `bgp-filter-community` (RFC 7999 Section 3.1, RFC 8195) run forked accept every route with no error, which a release owes fixed whatever bucket the mechanism sits in |
| RFC gaps | - |
| Depends | `spec-filter-wire-0-umbrella` phases 1 and 3 (landed: 1a179d341e Stage 1 reads/sets, 29fc996ece batched `filter-update`, 4ee9ba017c typed bridge slot); the umbrella's D-13 `loop` row (engine step, in the parent session's tree at writing, committed before this spec starts) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-filter-wire-0-umbrella` D-13 merges the in-process-only `filterapi.Register`
family into the one plugin filter mechanism (D-12): every filter owned by its
plugin, called through the typed `DirectBridge` slot when the plugin runs
internal and through the batched `filter-update` message when it runs forked.
D-13 named the outcome for each row, but nobody designed the mechanism four of
the rows need: a filter that is **attached to a fixed direction, stage and
priority and applies to every peer**, with no operator chain reference. Today
only `filterapi.Register` gives that, and only as a Go function pointer in the
daemon.

The defect this closes, read at the producer (umbrella session handoff, D-13
inventory, re-read here): `bgp-filter-community`, `bgp-filter-community-relation`
and `bgp-role` keep their per-peer config (and, for role, the role each peer's
OPEN announced) in package globals their plugin's own callbacks fill. Run forked
(`plugin { external ... }`), those callbacks fill the subprocess's copy, the
daemon's copy stays empty, and the daemon-side filter functions find no config
for any peer and accept every route unchanged. No error, no log line: RFC 9234
Section 5 ingress and egress procedures, the RFC 7999 BLACKHOLE withholding and
the RFC 8195 relation tags silently stop. `bgp-gr` and `bgp-rs` refuse to start
forked for a separate reason (the `retention.Family` same-process call), so their
rows are not silently broken today, but they hold the same function-pointer hooks.

Goal: one filter mechanism. A plugin declares at Stage 1 that a filter is
attached at (direction, stage, priority); the reactor builds its ordered ingress
and egress steps from those declarations, runs each through the existing
external-member path of the library chain (slot internal, batch forked), and
`filterapi.Filter`, `Register`, `IngressOrdered`, `EgressOrdered`,
`ReadvertiseEgressFuncs`, `NotifyPeerRemoved` and `PreserveOpaqueAttributesFor`
are deleted. Each moved filter's existing `.ci` passes with its plugin internal,
and a forked twin of it that is red at HEAD passes after.

## Required Reading

### Design doors

- [ ] `ai/doors/system.md`
  → Constraint: a plugin can run in another process; a plain Go call or package global that only the same-process callback fills is the classic silent defect. A per-route callback reaches DirectBridge as a typed slot of the protocol, and the same evaluator answers the batched message forked.
  → Constraint: `pkg/plugin/rpc` is a published contract: the Stage 1 attachment, any new `FilterAction`, and any new field are wire changes, made on purpose with their pages (`process-protocol.md`, `docs/plugin-development/`).
- [ ] `ai/doors/bgp-engine.md`
  → Constraint: the engine reaches plugin features only through what they register, and never names a plugin package from the reactor. The "filterapi.Register for compiled-in filters" boundary and "A filterapi filter is in-process only" both stay true until the last row moves, and are edited in that commit.
  → Constraint: received bytes are borrowed (`IsAsyncSafe` false); the slot path reads them on the reactor goroutine, the forked path encodes declared fields only (D-12).
- [ ] `ai/doors/bgp-plugins.md`
  → Constraint: a plugin declares twice (`registry.Registration` and the `sdk.Registration` passed to `p.Run`) and both must agree; `./le plugin declarations check` judges it. The attachment is declared in both, from one `filterDecls()` value as `fakefilter` does.
  → Decision: the hook table row "`filterapi.Register` filters -- in-process only" is deleted by this spec, and the typed filter slot with its forked equivalent stays the only filter hook.
- [ ] `ai/doors/plugin-infra.md`
  → Constraint: one engine-op entry per RPC; the SDK stays generic: no new `On*` callback for attached filters, `sdk.OnFilter` already installs one evaluator on both transports. `sdk.CheckFilterConformance` under `-race` is the fast-path contract gate for every evaluator this spec writes.
  → Constraint: one plugin failure stops the startup tier (owner ruling); a declaration refused at Stage 1 fails that plugin.
- [ ] `ai/doors/core.md`
  → Constraint: a registry holds value types and is the only list; `./le arch enumeration check` refuses a literal that restates it. The attached-step list is derived from declarations, never written.
- [ ] `ai/doors/api-surfaces.md`
  → Constraint: no new command; the filter metrics stay named as they are (a Prometheus rename breaks dashboards).
- [ ] `ai/doors/le-tooling.md`
  → Constraint: forked twins are `.ci` files under `test/plugin/` run by `./le test`; a compiled Go fixture returns an error when an assertion fails; scoped package tests carry `-tags ze_bgp` through `./le job run`.

### Architecture Docs

- [ ] `docs/architecture/core-design.md` "Ingress Filter Pipeline", "Policy Filter Chain"
  → Constraint: ingress order is a declared property: stage, then priority, then name (`filterapi.LessOrder`), loop detection at `FilterStageProtocol`, the per-peer chain at `FilterStagePeerChain` (300) after every attached step. The new step list must sort identically.
- [ ] `docs/architecture/api/process-protocol.md` "Runtime hot path", the Stage 1 `declare-registration`
  → Constraint: `FilterDecl` (`name`, `name-pattern`, `direction`, `reads`, `sets`, `on-error`) is the Stage 1 filter contract; `registry.CheckFilterDeclarations` judges it in `runStartupHandshake`. The attachment extends this declaration and this judge, not a second declaration.
- [ ] `docs/plugin-development/handlers.md` "Route filters"
  → Constraint: an evaluator answers from what it holds and never waits on its own data (the batch shares one `FilterUpdateTimeout`). Role's and community's evaluators read only plugin memory and the declared fields: they satisfy this.
- [ ] `docs/architecture/meta/README.md`, `meta/role.md`, `meta/filter-community.md`
  → Constraint: route metadata keys `src-role`, `src-peer-role`, `stale`, `rebuilt` are a cross-plugin contract with fixed types (string, string, uint8, bool); the RIB reads `stale` from the received event's `Meta`. Moving the writers must keep the key, the type, and that the map reaches the received event and the egress steps.
- [ ] `plan/post-release/engineering/spec-filter-wire-0-umbrella.md` D-12, D-13, R-14, AC-37
  → Decision: this spec is the home of D-13's plugin rows (community, relation, role, gr, rs); AC-37 is closed through AC-6 to AC-13 here, and the umbrella's phase 4 table names this spec.
- [ ] `docs/architecture/api/architecture.md` "Attaching a Process to a Peer"
  → Constraint: which process sees which peer's events is the peer's `attach process` block; an attached-filter plugin attached to no peer receives no `state` event today, which is why `PeerRemoved` exists. The scope Q-2 settles becomes that plugin's implicit attach for `state` only, and the page says so.
- [ ] `docs/architecture/api/ipc_protocol.md`
  → Constraint: a request line is framed by its newline and answer lines are width-framed; the attachment and `withdraw` travel inside the existing JSON payloads, so framing is unchanged.
- [ ] `docs/architecture/config/syntax.md`
  → Constraint: no config syntax changes: an attached filter is declared by its plugin, never written in a peer's `filter import`/`filter export` chain (alternative B rejected).
- [ ] `docs/plugin-development/metrics.md` "When ConfigureMetrics Runs"
  → Constraint: `registry.Registration.ConfigureMetrics` is the plugin metrics hook; `filterapi.Filter.ConfigureMetrics` duplicates it for `bgp-filter-community` and goes.

### RFC Summaries (Scope: protocol)

- [ ] `rfc/short/rfc9234.md`, text `rfc/full/rfc9234.txt` Section 5
  → Constraint (Section 5, ingress 1): "If a route with the OTC Attribute is received from a Customer or an RS-Client, then it is a route leak and MUST be considered ineligible (see Section 3)."
  → Constraint (Section 5, ingress 3): "If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS."
  → Constraint (Section 5, egress 1, a line break falls inside "RS-Client" as printed): "If a route is to be advertised to a Customer, a Peer, or an RS- Client (when the sender is an RS), and the OTC Attribute is not present, then when advertising the route, an OTC Attribute MUST be added with a value equal to the AS number of the local AS."
  → Constraint (Section 5, egress 2): "If a route already contains the OTC Attribute, it MUST NOT be propagated to Providers, Peers, or RSes."
  → Constraint (Section 5, last paragraph): "The operator MUST NOT have the ability to modify the procedures defined in this section." So the OTC filter cannot be an operator chain member (an operator can mark a chain reference `inactive:`): it must be attached by the plugin. This decides alternative B below.
- [ ] `rfc/short/rfc9494.md`, text `rfc/full/rfc9494.txt` Sections 4.3 and 4.6
  → Constraint (4.3): "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2:" then "The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received." So the ingress step needs "Ze advertised LLGR on this session" synchronously (today `PeerFilterInfo.LLGRAdvertised`), and the egress step needs "LLGR received from this destination" and a withdraw toward it.
  → Constraint (4.6): "The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors." "The NO_EXPORT community [RFC1997] MUST be attached to the stale routes." "The stale routes MUST have their LOCAL_PREF set to zero." So egress needs the destination's iBGP fact and community add and local-preference set, both already settable fields.
- [ ] `rfc/short/rfc7999.md` Section 3.1, `rfc/short/rfc8195.md` Section 3.2
  → Constraint: the BLACKHOLE withholding toward a non-agreeing eBGP destination is an egress suppress decision (`blackholeReadvertiseRefused`); the relation tag needs the source's iBGP fact and the source peer role `src-peer-role`.

**Key insights:**
- The forked breakage is not a missing transport for a daemon-side function: the state those functions read lives in the plugin process. Running the evaluator in the plugin (`sdk.OnFilter`) puts the reader where the writer is, so role's `OnValidateOpen` learned roles and community's config apply are coherent internal and forked with no state shipped to the daemon.
- What the plugin cannot know itself is reactor state: which peer (name, group, address) and, per route, `LLGRAdvertised`, the iBGP fact, local AS, route-server AS, the source peer on egress, and the ingress metadata. Those become declared reads, encoded only when declared (D-10 rule: the declared set is what is sent).
- Every moved filter already has a library-chain-shaped answer except two: writing a route metadata key, and turning an announce into a withdraw (OTC ingress "ineligible" today rewrites the payload to its withdrawal, LLGR egress calls `ModAccumulator.SetWithdraw`).

## Current Behavior (MANDATORY)

Read at HEAD 357b33c57c plus the parent session's uncommitted `loop` row.

**Source files read:**
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `Filter` (Name, Stage, Priority, Ingress, Egress, Readvertise, PreserveOpaqueAttributes, PeerRemoved, ConfigureMetrics), `Register`, `IngressOrdered`, `EgressOrdered`, `ReadvertiseEgressFuncs`, `IngressFilters`, `EgressFilters`, `NotifyPeerRemoved`, `PreserveOpaqueAttributesFor`, `LessOrder`, the stage constants (Protocol 0, Policy 100, Annotation 200, PeerChain 300), `PeerFilterInfo` (Address, PeerAS, LocalAS, RouterID, ASN4, Name, GroupName, AllowOwnAS, ClusterID, LoopDisabled, RSClient, RouteServerAS, IsIBGP, LLGRAdvertised), `EnableRSForwarding`, `SetMetricsRegistry`.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `buildOrderedIngressSteps` (engine loop step, `IngressOrdered`, peer-chain step, stable sort by `LessOrder`), `buildOrderedEgressSteps`, `runIngressPolicyChain`, `runEgressPolicyChain`, `ingressPolicyWithdrawals`.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - the ingress pass over `orderedIngressSteps`: an in-process step gets one shared `ingressMeta` map, a modified payload rebuilds the `WireUpdate`; the map becomes `msg.Meta` on the received event.
- [ ] `internal/component/bgp/reactor/reactor.go` - `egressFilters` (`filterapi.EgressFilters`), `orderedIngressSteps`, `orderedEgressSteps`, `readvertiseEgressFilters` built at startup; the metrics-enable block calls `filterapi.SetMetricsRegistry`.
- [ ] Egress rails reading `r.egressFilters` directly: `forward_rs.go` (RS fast path, two sites), `reactor_api_forward.go` (one site, plus the ordered `forwardUpdateCore` loop), `reactor_api_forward_batch.go`, `reactor_api_relay.go`; the stale readvertise rail `reactor_api_batch.go` (`readvertiseEgressFilters`). Six rails in all.
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - removal calls `filterapi.NotifyPeerRemoved` with Address, Name, GroupName.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `facts.preserveOpaqueAttributes = filterapi.PreserveOpaqueAttributesFor(facts.filterInfo)` once per snapshot.
- [ ] `internal/component/bgp/reactor/filter_library.go` - `runLibraryChain`: view reset, `ExternalRoute` (direction, peer, peer AS, raw), `libraryCaller.CallFilter` to `CallFilterUpdate` under `FilterUpdateTimeout`, `Chain.Apply` into a `ModAccumulator`, `rewriteLibrary`.
- [ ] `internal/component/bgp/policychain/external.go`, `external_view.go` - `Member{Name, Plugin}`, `ExternalDeclaration{Reads, Sets, OnError}`, `ExternalCaller`, `judgeExternal`, `applyExternalSets`, `memberView` (fields encoded on first `Fields()` call only).
- [ ] `internal/component/bgp/policyview/view.go`, `register.go`, `fields_json.go` - `View`, `Session{EBGP, Sender, SenderKnown}`, the field readers and JSON writers registered from `init()`.
- [ ] `internal/component/policy/fields.go`, `internal/component/bgp/config/register.go` - `MustRegisterField`, `MustRegisterAttachment`; BGP's `carries` and `settable` lists for `bgp import`/`bgp export`.
- [ ] `pkg/plugin/rpc/types.go`, `enums.go`, `filter_slot.go` - `FilterDecl`, `FilterUpdateInput` (filter, direction, peer, peer-as, update, fields, raw), `FilterUpdateOutput` (action, update, raw, sets, teardown, notify codes, error), `FilterSet{field, action, value}` with actions set/increment/decrement/add/remove, `FilterAction` accept/reject/modify, `FilterView`, `FilterRoute`, `DirectBridge.SetFilterSlot`/`CallFilterSlot`.
- [ ] `internal/component/plugin/registry/filter_declaration.go`, `registry.go` - `DeclaredFilter`, `CheckFilterDeclarations`, `SetRuntimeFilterDeclarations`/`RuntimeFilterDeclarations`, `Registration.Filters`.
- [ ] `internal/component/plugin/registration.go` - `FilterRegistration` carries `Overrides` ("Default filters this filter replaces"), copied at `startup.go` and read nowhere else found.
- [ ] `internal/component/bgp/config/peers.go` - `prependDefaultFilters`: config `loop-detection` entries are prepended to every peer's import chain unless the chain already names them.
- [ ] Plugins: `filter_community/register.go`, `filter_community.go` (`ingressFilter`, `egressFilter`, `relationIngressFilter`, globals `definitions`, `peerConfigs`, `agreements`), `role/register.go`, `role/otc.go` (`OTCIngressFilter`, `OTCEgressFilter`), `role/role.go` (`getFilterConfig`, `filterRemoteRoles` filled by `OnValidateOpen`, `forgetRemovedPeer`, `pruneLearnedRoles` at commit), `gr/register.go`, `gr/gr_egress.go` (`LLGREgressFilter`, `egressState`), `gr/gr_ingress.go` (`LLGRIngressFilter`), `gr/gr.go` and `rs/server.go` (both refuse `!p.IsInternal()`), `rs/register.go` (`preserveOpaqueAttributes` returns `peer.RSClient`).

**The rows this spec moves:**

| Row | Stage / prio | Hooks | Reads besides the UPDATE | Writes | Forked today |
|-----|--------------|-------|--------------------------|--------|--------------|
| `bgp-filter-community` | Policy 0 | ingress (tag, strip), egress (tag, strip, BLACKHOLE withhold), `ConfigureMetrics` | own config by Name/GroupName; LocalAS, PeerAS, RouteServerAS; dest Name/GroupName/IsIBGP on egress | payload rewrite (ingress), `ModAccumulator` community ops (egress), suppress | silently accepts everything (daemon globals empty) |
| `bgp-filter-community-relation` | Annotation 1 | ingress | own config; `meta["src-peer-role"]`; IsIBGP | RFC 8195 community tag | silently tags nothing |
| `bgp-role` | Annotation 0 | ingress, egress, `PeerRemoved` | own config by Address/GroupName; role learned from OPEN (`OnValidateOpen`, plugin process); src and dest identity on egress; `meta["src-role"]` | `meta["src-role"]`, `meta["src-peer-role"]`, OTC insert, payload-to-withdrawal (ingress leak), suppress (egress) | silently skips every RFC 9234 Section 5 MUST |
| `bgp-gr` LLGR egress | Annotation 0, `Readvertise` | egress on the forward rails and the stale readvertise rail | `meta["stale"]`, `meta["rebuilt"]`, `src.LLGRAdvertised`; `egressState` (LLGR caps from OPEN, plugin memory); dest Address, PeerAS, LocalAS | community add NO_EXPORT, LOCAL_PREF 0, `SetWithdraw` | refuses to start forked (`gr.go` `runGRPlugin`, `test/plugin/gr-external-refuses.ci`) |
| `bgp-gr-llgr-ingress` | Protocol 0 | ingress | `src.LLGRAdvertised`, COMMUNITIES | `meta["stale"] = 2` | refuses forked (same plugin) |
| `bgp-rs` | none | `PreserveOpaqueAttributes` only | `peer.RSClient` | none (forwarding choice) | refuses forked (`rs/server.go`) |

**Behavior to preserve:**
- The ingress order: loop (Protocol), `bgp-gr-llgr-ingress` (Protocol), `bgp-filter-community` (Policy 0), `bgp-role` (Annotation 0), `bgp-filter-community-relation` (Annotation 1), the per-peer chain (PeerChain); egress likewise. `TestOrderedIngressCarriesTheEngineLoopStep` and the order tests in `unified_filter_order_test.go` keep their expectations.
- Every `.ci` of the moved filters, unchanged expectations, with the plugin internal: `role-otc-*.ci`, `community-tag.ci`, `community-strip.ci`, `community-relation-tag.ci`, `community-blackhole-noexport.ci`, `rfc7999-blackhole-*.ci`, `bgp-filter-community-reload.ci`, `bgp-role-export-reload.ci`, `role-learned-cleared-on-peer-delete.ci`, `llgr-*.ci`, `graceful-restart-llgr.ci`, the `bgp-rs-*` forwarding tests.
- Route metadata keys, types, and their presence on the received event (`RawMessage.Meta`, JSON `route-meta`).
- The stale readvertise rail runs only readvertise-opted egress steps, never the full egress set.
- `bgp-role` frees a removed peer's learned role before the removal returns (internal) and before a re-created peer's OPEN (both transports).
- The filter_community metric names and labels.

**Behavior to change:**
- `bgp-role`, `bgp-filter-community`, `bgp-filter-community-relation` run forked apply their procedures exactly as internal.
- `filterapi.Register` and the hooks it carries no longer exist; the doors' "in-process only" filter row is deleted.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `filterapi` ordered stages | `filterapi/filterapi.go` `Filter`, `Register`, `IngressOrdered`, `EgressOrdered`, `ReadvertiseEgressFuncs`, `LessOrder`, `FilterStage*`; `reactor/filter_ordered.go` `buildOrderedIngressSteps`/`buildOrderedEgressSteps` | Go function pointers registered from `init()` at (stage, priority), sorted with the engine loop step and the per-peer chain step into one pass; six egress rails | **Replaced**, per `ai/rules/no-layering.md`: the registry and its function types are deleted first. `LessOrder` and the stage constants **stay** (the sort key and the stage vocabulary the attachment names). `buildOrdered*Steps` are **extended**: their step kinds become engine loop, attached plugin filter, peer chain; their input becomes the runtime filter declarations instead of `IngressOrdered` |
| Stage 1 filter declarations | 1a179d341e; `rpc.FilterDecl`; `registry.Registration.Filters`; `registry.CheckFilterDeclarations`, `SetRuntimeFilterDeclarations`/`RuntimeFilterDeclarations`; `process-protocol.md` Stage 1 | A plugin declares each filter's name or pattern, direction, reads, sets and on-error; the engine judges them at the handshake and judges chain references against them | **Extended**: `FilterDecl` gains an optional attachment (stage, priority, readvertise opt-in); an attached declaration names one filter (no pattern), and is judged by the same check plus the attachment rules. The reactor reads attached declarations from the same runtime store the chain check reads, so a compiled-in and a forked plugin attach the same way |
| Typed DirectBridge filter slot | 4ee9ba017c; `rpc.DirectBridge.SetFilterSlot`/`CallFilterSlot`, `rpc.FilterView`, `rpc.FilterRoute`; `sdk.OnFilter`; `ipc.PluginConn.CallFilter`; `server.CallFilterUpdate` | One evaluator per filter, called on the reactor goroutine with a lazy view internal; `ErrBridgeClosed` answers unavailable and the member's on-error applies | **Reused unchanged** as the internal transport of every attached step. The moved evaluators are written once against `sdk.OnFilter` and pass `sdk.CheckFilterConformance` |
| Batched `filter-update` | 29fc996ece; `rpc.FilterUpdateBatchInput`/`Output`, `ipc/filter_batch.go` `filterBatcher`, `FilterUpdateBatchMax`, `FilterUpdateTimeout` | One exchange per plugin connection carries what is queued, up to 64 routes, one verdict each, per-source FIFO by construction (the reactor judges each source synchronously) | **Reused unchanged** as the forked transport. An attached step is one more caller of it |
| Library chain and external member | `policychain.Chain`, `Member{Plugin}`, `ExternalCaller`, `judgeExternal`, `applyExternalSets`, `memberView`; `reactor/filter_library.go` `runLibraryChain`, `libraryCaller`, `rewriteLibrary` | Runs a chain whose external members are plugin filters: encodes declared reads lazily, calls the filter, checks and applies its sets through `ModAccumulator` and the library rewrite, applies on-error | **Extended**: an attached step is run as a one-member external chain compiled from its declaration, through `runLibraryChain`, so verdicts, sets, on-error, metrics and the rewrite are the chain's own. New: sets on route metadata fields write the run's metadata map, and a `withdraw` verdict |
| `policyview` field registry | `policyview/register.go` `MustRegisterReader`, `fields_json.go` `mustRegisterWriter`; `policy.MustRegisterField`; `bgp/config/register.go` `carries`/`settable` | Per-field readers and JSON writers over the BGP view, registered from `init()`; the attachments list which fields a chain member may read and set | **Extended**: session facts (`peer-name`, `peer-group`, `peer-address`, `local-as`, `route-server-as`, `ibgp`, `llgr-advertised`, and the source peer's facts on egress) and route metadata (`src-role`, `src-peer-role`, `stale`, `rebuilt`) become fields with readers; `otc` becomes a readable and settable attribute field with its wire handler. Each is registered by its owner: session facts by the BGP view, metadata keys by the plugin that writes them, `otc` by `bgp-role` (as its `AttrModHandler` and `attribute.RegisterName` already are). Q-4 asks whether BGP's hand-written `carries`/`settable` lists become registrations in the same change |
| `ModAccumulator` and attribute mod handlers | `filterapi.ModAccumulator`, `RegisterAttrModHandler`; `reactor/forward_build.go` `buildModifiedPayload` | Per-attribute ops, a withdraw flag, NLRI rewrite; `role` registers the OTC handler, `filter_community` the three community handlers | **Kept**: sets from attached steps reach it through `Chain.Apply`, as chain sets do; `SetWithdraw` is reached by the new egress `withdraw` verdict |
| Route metadata map | `reactor_notify.go` `ingressMeta` to `msg.Meta`; `docs/architecture/meta/README.md` | One map per received UPDATE shared by the in-process ingress steps, published on the received event and read on egress | **Extended**: the map is fed by metadata-field sets of attached steps instead of direct writes; egress steps read it as declared fields |
| Default chain filters | `bgp/config/peers.go` `prependDefaultFilters`; `FilterDecl.Overrides` / `FilterRegistration.Overrides` | Config `loop-detection` entries are prepended to every peer's import chain; `Overrides` is declared and copied but read nowhere found | **Does not apply**: an operator can mark a chain reference `inactive:`, which RFC 9234 Section 5 forbids for OTC, and an export chain member takes a destination off the RS fast path (`hasActiveFilter`, `forward_rs.go`). `Overrides` looks dead and is checked, not reused (A-6) |
| Peer removal notice | `filterapi.NotifyPeerRemoved` / `Filter.PeerRemoved`; the peer `state` event with reason `removed` (`process-protocol.md`), delivered to attached processes only; `role.pruneLearnedRoles` at commit | Frees per-peer filter state on every removal path, internal only; forked, only the commit prune runs | **Replaced**: a plugin that declares an attached filter receives the `state` event of every peer in its scope (the attachment is its attach), so the removal reaches it on both transports. Q-2 settles the scope |
| Plugin metrics hook | `registry.Registration.ConfigureMetrics`, `registry.InjectPluginMetrics` | Hands an in-process plugin the metrics registry | **Reused**: `filter_community` moves its counters from `filterapi.Filter.ConfigureMetrics` to its `Registration`. Forked counters are A-5 |
| RS forwarding activation | `filterapi.EnableRSForwarding`, `RSForwardingEnabled`; reactor `rsForwardingEnabled` | `bgp-rs` activates the reactor RS fast path from `init()` | **Extended**: `peer_forward_facts.go` derives preserve-opaque-attributes from `rsForwardingEnabled && RSClient`; `rs`'s `PreserveOpaqueAttributes` hook and `PreserveOpaqueAttributesFor` are deleted (umbrella D-13 inventory) |
| Forked test launch | `le mock plugin-external <name>` (`internal/test/cli/cmd_plugin_external.go`; `test/plugin/gr-external-refuses.ci`) | Runs a registered engine's `RunEngine` over a real TLS connect-back | **Reused** for every forked `.ci` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Stage 1 `declare-registration` from a plugin (in-process `registry.Registration.Filters` and the `sdk.Registration`, or a forked plugin's line): the attached `FilterDecl`.
- Per route: a received UPDATE on a session read goroutine (ingress), and each destination of a forward on the six egress rails.

### Transformation Path
1. Handshake: `CheckFilterDeclarations` judges each declaration, including attachment rules (named filter, known stage, direction import/export/both, priority bound, readvertise only on export); the runtime store records it.
2. Reactor start and every change of the runtime store (plugin start, respawn, stop): the ordered ingress and egress step lists are rebuilt from the engine loop step, the attached declarations and the peer-chain step, sorted by `LessOrder`, and published by atomic pointer (today they are built once at `startAPIServer`).
3. Ingress, per route: each attached step runs `runLibraryChain` over a one-member chain; the view's session carries the source facts; metadata-field sets land in the run's metadata map, attribute sets are applied, `reject` drops, `withdraw` keeps the withdrawal half (`ingressPolicyWithdrawals`).
4. The map becomes `msg.Meta` as today.
5. Egress, per destination: each attached export step runs the same way with the destination's facts and the source facts as declared reads and the ingress map as metadata reads; `reject` suppresses for this destination, `withdraw` sets `SetWithdraw`, sets accumulate.
6. Stale readvertise: only readvertise-opted export steps run.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to internal plugin | typed `DirectBridge` filter slot, lazy view, on the reactor goroutine | No |
| Engine to forked plugin | batched `filter-update`, declared fields only, `FilterUpdateTimeout` | No |
| Engine to plugin, peer lifecycle | peer `state` event with reason `removed` | No |

### Integration Points
- `runLibraryChain` / `policychain.Chain` - attached steps are one-member chains.
- `registry.RuntimeFilterDeclarations` - the source of attached steps.
- `policyview` readers and writers - the new fields.
- `ModAccumulator` - egress sets and withdraw.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to prove: attached steps call only `CallFilterUpdate` |
| No unintended coupling (components stay isolated) | No | to prove: the reactor names no plugin package; `./le arch tier check` |
| No duplicated functionality (extends existing, does not recreate) | No | to prove: no second call path beside `runLibraryChain` |
| Zero-copy preserved where applicable (refs, not copies) | No | to prove: slot path encodes nothing undeclared (`BenchmarkFilterFastPath` allocations) |
| Registration over hardcoding, outbound | No | to prove: new fields registered by their owners from `init()` |
| Registration over hardcoding, inbound | No | to prove: the reactor holds no list of attached filter names; grep of `bgp-role`, `bgp-filter-community`, `bgp-gr` under `internal/component/bgp/reactor` is empty |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Forked role and community accept every route unchanged today | umbrella handoff D-13 inventory; globals read in `filter_community.go` `ingressFilter`, `role.go` `getFilterConfig`, filled by plugin callbacks | the forked `.ci` twins are not red at HEAD and the journal row overstates | each forked twin observed red at HEAD before the move | unvalidated |
| A-2 | Every state the moved evaluators read lives in the plugin process or is a reactor fact a declared read can carry | role learns roles in `OnValidateOpen` (plugin process); community config from its config apply; gr caps from OPEN events in the plugin | a filter needs state only the daemon holds and no field can carry | the Current Behavior table, re-read per row at implementation | unvalidated |
| A-3 | `LLGRAdvertised` must stay a synchronous reactor fact, never an event the plugin records | `gr_ingress.go` `llgrStaleFromSource` comment: the sent-OPEN event is asynchronous, so first UPDATEs could be judged before the record | the ingress step reads a stale answer on a session's first UPDATEs | the declared `llgr-advertised` field is read from the session on the read goroutine | unvalidated |
| A-4 | The library chain's external member answers everything the moved filters need once `withdraw` and metadata sets exist | `judgeExternal`, `applyExternalSets`; the Writes column | a filter needs a payload rewrite the sets cannot express (OTC ingress leak withdrawal is the `withdraw` verdict) | each evaluator's unit tests through `CheckFilterConformance` | unvalidated |
| A-5 | Forked plugins have no metrics export path, so a forked `bgp-filter-community` counts nothing visible | `docs/plugin-development/metrics.md`: `ConfigureMetrics` is injected in process | forked counters already reach the registry and nothing is owed | read `registry.InjectPluginMetrics` for the forked branch | unvalidated |
| A-6 | `FilterDecl.Overrides` is dead | `startup.go` copies it, no reader found by grep | it is live and an attached filter interacts with it | grep `Overrides` under `internal/` and `pkg/` | unvalidated |
| A-7 | Rebuilding the ordered steps on plugin start, stop and respawn is safe against a concurrent pass | steps are read without a lock today because they are built once | a pass sees a torn list | atomic-pointer publish; `-race` test with a respawn during traffic | unvalidated |
| A-8 | `le mock plugin-external bgp-role` and `bgp-filter-community` start those engines forked (`filter_community`'s `CLIHandler` returns 0, so `ze plugin bgp-filter-community` does not) | `gr-external-refuses.ci` uses the mock launcher | the forked twins need another launcher | first forked twin starts and logs the plugin's handshake | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A forked attached filter adds a round trip to every route of every peer, and a slow or hung plugin stalls ingress for all peers up to `FilterUpdateTimeout` | functional run time of the forked twins; `BenchmarkFilterForkedBatch` | batching already shares one exchange across sources; Q-2 decides whether a peer scope bounds it; the docs say what forked costs |
| R-2 | An attached filter's plugin dies: fail-closed drops every route, fail-open skips an RFC MUST | AC-33-style `.ci` with a stopped plugin | each plugin declares its on-error: role and community declare reject (an RFC 9234 Section 5 MUST cannot fail open); stated in the plugin pages |
| R-3 | Ordering drifts because steps now come from runtime declarations that arrive in tier order | order tests read a different list | the sort key is unchanged; `TestAttachedStepsSortLikeTheRegistry` pins the six-step order |
| R-4 | Egress rails that read `r.egressFilters` (RS fast path, relay, batch) miss the attached steps after the switch | an RS fast-path `.ci` (`role-otc-rs-client-dest-stamp.ci`) passes internal only by accident | one egress step list consumed by all six rails; a wiring test per rail |
| R-5 | Metadata reads on egress for a rebuilt route (`rebuilt`) lose the gr recovery path | `llgr-readvertise*.ci` red | `rebuilt` and the source's `llgr-advertised` are declared reads of the gr egress step |
| R-6 | Peer identity: role keys config by Address then group key, community by Name then GroupName; dynamic peers are named `dyn-<addr>` | `dynamic-peer-gets-group-role-capability.ci` red forked | `peer-name`, `peer-group`, `peer-address` are all declared reads; the plugins keep their lookup code |
| R-7 | A third-party plugin claims `FilterStageProtocol` priority 0 ahead of the RFC checks | review | Q-1 |
| R-8 | `bgp-gr` and `bgp-rs` cannot be proven forked here because they refuse to start forked for `retention.Family` | the forked twin for gr cannot start | Q-3 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes mis-filtered on every BGP peer: OTC leaks accepted or valid routes dropped, BLACKHOLE leaked, LLGR stale routes advertised to non-LLGR eBGP peers |
| How is it reverted? | Per plugin row, each a commit; the mechanism commit reverts alone while no row uses it |
| Who else touches this path? | `spec-filter-wire-0-umbrella` phase 4 rows (filter_family, filter_irr, the five match providers), phase 5 deletion of the text machinery; any session editing `reactor/filter_ordered.go`, `reactor_notify.go`, `pkg/plugin/rpc/types.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Stage 1 attached `FilterDecl` from a compiled-in plugin | → | `CheckFilterDeclarations` attachment rules, runtime store | `TestAttachedFilterDeclarationJudged` (`internal/component/plugin/registry/filter_declaration_test.go`) |
| Runtime declarations | → | `buildOrderedIngressSteps`/`buildOrderedEgressSteps` from declarations | `TestAttachedStepsSortLikeTheRegistry` (`internal/component/bgp/reactor/unified_filter_order_test.go`) |
| Received UPDATE, plugin forked | → | attached step through `runLibraryChain` and the batch | `test/plugin/stage-filter-forked.ci` with `fakefilter` declaring an attachment |
| Each of the six egress rails | → | the one egress step list | `TestEveryEgressRailRunsAttachedSteps` (`internal/component/bgp/reactor/filter_attached_test.go`) |
| Peer removal | → | `state` removed delivered to an attached-filter plugin | `test/plugin/role-learned-cleared-on-peer-delete-forked.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin declares a filter with an attachment (direction, stage, priority) at Stage 1, internal or forked | Its evaluator is called for every route at that position of the ingress or egress pass, for every peer in scope (Q-2), with no chain reference in config |
| AC-2 | An attached declaration with a name pattern, an unknown stage, `FilterStagePeerChain`, readvertise on import, or a duplicate (stage, priority, name) | The plugin is refused at Stage 1 with a message naming the filter, the rule and the fix |
| AC-3 | The six positions registered today (loop, llgr-ingress, community, role, relation, peer chain) | The built ingress and egress step lists are in the same order as at HEAD |
| AC-4 | An attached step answers reject, modify with sets, withdraw, or fails (timeout, unavailable, malformed) | Ingress: drop, apply, keep the withdrawal half, on-error; egress: suppress for that destination, apply, withdraw toward it, on-error; identical to a chain member's handling |
| AC-5 | An attached ingress step sets a metadata field (`src-role`, `src-peer-role`, `stale`) | The key is in the received event's `Meta` with its registered type, and later ingress and egress steps read it as a declared field |
| AC-6 | `bgp-role` forked: route with OTC from a customer; route without OTC from a provider; export to a customer without OTC; export with OTC to a provider | Ineligible (withdrawn); OTC added with the remote AS; OTC added with the local AS; not propagated; each the same internal and forked (forked twins of `role-otc-ingress-reject.ci`, `role-otc-egress-stamp.ci`, `role-otc-egress-filter.ci`, `role-otc-rs-client-dest-stamp.ci`, `role-otc-unicast-scope.ci`), each twin observed red at HEAD |
| AC-7 | `bgp-filter-community` forked: ingress tag and strip, egress tag and strip, BLACKHOLE toward a non-agreeing eBGP destination | Same wire result as internal (forked twins of `community-tag.ci`, `community-strip.ci`, `community-blackhole-noexport.ci`, `rfc7999-blackhole-readvertise-agreement.ci`), each red at HEAD |
| AC-8 | `bgp-filter-community-relation` forked, eBGP source with a resolved peer role, and an iBGP source | Tag written for the eBGP source, none for iBGP (forked twin of `community-relation-tag.ci`), red at HEAD |
| AC-9 | `bgp-role` forked, peer deleted then re-created under the same name before its OPEN | The re-created peer reads no learned role (forked twin of `role-learned-cleared-on-peer-delete.ci`) |
| AC-10 | `bgp-gr` LLGR ingress and egress as attached filters, internal | Every `llgr-*.ci` and `graceful-restart-llgr.ci` passes unchanged, the readvertise rail runs only the LLGR egress step; the forked proof is as Q-3 decides |
| AC-11 | `bgp-rs` | Preserve-opaque-attributes is derived from RS forwarding enabled and the client flag; `bgp-rs-*` tests unchanged; `TestPreserveOpaqueAttributesUsesRegisteredSelector` and `forward_role_cache_test.go` changed only with the `ze-test-judge` approval |
| AC-12 | Every evaluator moved here | Passes `sdk.CheckFilterConformance` under `-race`, including transport equality |
| AC-13 | After the last row | `filterapi.Filter`, `Register`, `IngressOrdered`, `EgressOrdered`, `ReadvertiseEgressFuncs`, `IngressFilters`, `EgressFilters`, `NotifyPeerRemoved`, `PreserveOpaqueAttributesFor`, `IngressFilterFunc`, `EgressFilterFunc` and `filterapi.SetMetricsRegistry` do not exist; `r.egressFilters` and `readvertiseEgressFilters` are gone; grep proves it |
| AC-14 | A plugin restart or respawn under traffic | The step list is republished; no pass sees a torn list (`-race`); the respawned plugin's slot answers |
| AC-15 | The named interop scenario `bgp-role-otc-withdraw-frr` | Passes with `bgp-role` internal, the OTC wire behaviour unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator runs `bgp-role` forked and sets `role import customer` | Stage 1 attachment, ingress step, batch, OTC verdict | forked twin of `role-otc-ingress-reject.ci` |
| 2 | Operator runs `bgp-filter-community` forked with a tag on a peer | config apply in the plugin, ingress step, community set | forked twin of `community-tag.ci` |
| 3 | Third-party author attaches a filter through the SDK | `sdk.OnFilter` plus attached `FilterDecl`, both transports | `test/plugin/stage-filter-forked.ci` and its internal twin |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAttachedFilterDeclarationJudged` | `internal/component/plugin/registry/filter_declaration_test.go` | AC-2 refusals and acceptance | |
| `TestAttachedStepsSortLikeTheRegistry` | `internal/component/bgp/reactor/unified_filter_order_test.go` | AC-3 | |
| `TestAttachedStepVerdicts` | `internal/component/bgp/reactor/filter_attached_test.go` | AC-4 ingress and egress | |
| `TestAttachedStepMetadataSets` | same | AC-5 | |
| `TestEveryEgressRailRunsAttachedSteps` | same | R-4, six rails | |
| `TestAttachedStepsRepublishOnRespawn` | same | AC-14 under `-race` | |
| `TestWithdrawVerdict` | `pkg/plugin/rpc`, `internal/component/bgp/policychain` | the new action, both transports | |
| `TestMetadataFieldRegistration`, `TestSessionFieldReaders` | `internal/component/bgp/policyview` | readers, writers, types; an absent key answers absent | |
| conformance per evaluator | `role`, `filter_community`, `gr` test files | AC-12 | |
| existing `otc_test.go`, `filter_community_test.go`, `gr_egress_test.go`, `gr_ingress_test.go` | plugin packages | the procedures, re-pointed at the evaluators; any assertion change goes to `ze-test-judge` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| attachment stage | the declared stage names (Protocol, Policy, Annotation) | Annotation | N/A (named) | PeerChain refused |
| attachment priority | 0-9999 | 9999 | -1 | 10000 |
| `stale` metadata value | 0-255 (uint8) | 255 | N/A | 256 refused as a set |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `stage-filter-forked`, `stage-filter-internal` | `test/plugin/` | `fakefilter` attaches an ingress filter and judges every peer's route | |
| `role-otc-ingress-reject-forked`, `role-otc-egress-stamp-forked`, `role-otc-egress-filter-forked`, `role-otc-rs-client-dest-stamp-forked`, `role-otc-unicast-scope-forked` | `test/plugin/` | AC-6 | |
| `role-learned-cleared-on-peer-delete-forked` | `test/plugin/` | AC-9 | |
| `community-tag-forked`, `community-strip-forked`, `community-blackhole-noexport-forked`, `rfc7999-blackhole-readvertise-agreement-forked` | `test/plugin/` | AC-7 | |
| `community-relation-tag-forked` | `test/plugin/` | AC-8 | |
| the existing internal tests in Behavior to preserve | `test/plugin/` | unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-role-otc-withdraw-frr` | `test/interop/scenarios/` | FRR | AC-15, OTC on the wire unchanged | |

## Files to Modify
- `pkg/plugin/rpc/types.go`, `enums.go` - attachment on `FilterDecl`; `withdraw` action
- `internal/core/ipc/yang/ze-plugin-engine.yang`, `ze-plugin-callback.yang` - the attachment and the action
- `internal/component/plugin/registry/filter_declaration.go`, `internal/component/plugin/registration.go`, `internal/component/plugin/server/startup.go` - judge and store attachments; deliver `state` to attached-filter plugins
- `internal/component/bgp/filterapi/filterapi.go` - delete the registry and function types (AC-13); keep `LessOrder`, stages, `ModAccumulator`
- `internal/component/bgp/reactor/filter_ordered.go`, `reactor_notify.go`, `reactor.go`, `forward_rs.go`, `reactor_api_forward.go`, `reactor_api_forward_batch.go`, `reactor_api_relay.go`, `reactor_api_batch.go`, `reactor_peers.go`, `peer_forward_facts.go`, `filter_library.go`
- `internal/component/bgp/policychain/external.go`, `sets.go` - metadata sets, withdraw
- `internal/component/bgp/policyview/register.go`, `readers.go`, `fields_json.go`, `view.go` - session facts
- `internal/component/bgp/config/register.go` - attachment fields
- `internal/component/bgp/plugins/role/register.go`, `otc.go`, `role.go`; `filter_community/register.go`, `filter_community.go`, `filter.go`, `metrics.go`; `gr/register.go`, `gr_egress.go`, `gr_ingress.go`; `rs/register.go`
- `internal/test/plugins/fakefilter/fakefilter.go` - an attached declaration
- `docs/architecture/core-design.md` (Ingress Filter Pipeline), `docs/architecture/api/process-protocol.md`, `docs/plugin-development/handlers.md`, `docs/plugin-development/protocol.md`, `docs/architecture/meta/README.md`, `docs/architecture/meta/role.md`, `docs/architecture/meta/filter-community.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/patterns/plugin.md` "Route Filters"

## Files to Create
- `internal/component/bgp/reactor/filter_attached.go`, `filter_attached_test.go`
- the forked `.ci` twins named in Functional Tests, under `test/plugin/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-plugin-engine.yang` (attachment in the declaration), `ze-plugin-callback.yang` (`withdraw`); no config leaf |
| YANG validation constraints | Yes | stage enumeration, priority range in the declaration schema |
| YANG custom validators | N-A | no config leaf |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no config leaf |
| Functional test for new RPC/API | Yes | `test/plugin/stage-filter-forked.ci` |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | Yes | filter_community counters keep their names, registered through `Registration.ConfigureMetrics`; attached steps count through the chain's existing member metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | OTC already registered by `bgp-role`; only its view field is new |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the `features/` entries for BGP role and community filters gain forked evidence (`./le feature report` names them) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (role, community run forked) |
| 6 | Has a user guide page? | No | none changes |
| 7 | Wire format changed? | No | BGP wire unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `process-protocol.md`, `docs/plugin-development/handlers.md`, `protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9234.md`, `rfc9494.md`: forked proof; tags moved with their tests carry discrimination records |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if `fakefilter`'s attachment adds an env key |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` Ingress Filter Pipeline |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md`, `meta/role.md`, `meta/filter-community.md` (writers become field sets) |
| 14 | Prometheus counters added/changed? | No | names unchanged |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived by `./le spec citation anchors spec plan/post-release/engineering/spec-filter-wire-6-stage-filters.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/plugin-development/handlers.md` filter examples |

Discovery (`ai/rules/repo-maintenance.md`): an agent finds the mechanism from the `ai/INDEX.md` filter rows pointing at `core-design.md` and `handlers.md`; `./le plugin declarations check` keeps the two declarations agreeing; the runtime filter declaration store is the one registry; `./le mock plugin-external` and the forked twins prove it.

## Implementation Steps

1. **Phase: Wiring** -- `FilterDecl` attachment, the judge, `fakefilter` attached, the reactor builds steps from declarations; `stage-filter-forked.ci` written failing
   - Tests: `TestAttachedFilterDeclarationJudged`, `TestAttachedStepsSortLikeTheRegistry`, `stage-filter-forked`
   - Files: rpc, YANG, registry, `filter_ordered.go`, `filter_attached.go`
   - Verify: the forked `.ci` fails because the step is not yet called
2. **Phase: Attached step execution** -- one-member chain through `runLibraryChain` on ingress and on all six egress rails; `withdraw`; metadata-field sets and reads; session fact fields; republish on plugin lifecycle
   - Tests: `TestAttachedStepVerdicts`, `TestAttachedStepMetadataSets`, `TestEveryEgressRailRunsAttachedSteps`, `TestAttachedStepsRepublishOnRespawn`, `TestWithdrawVerdict`
3. **Phase: `bgp-role`** -- evaluators on `sdk.OnFilter`, `otc` and metadata fields registered, attachment declared, `state` removed replaces `PeerRemoved`; forked twins red at HEAD then green. One commit
4. **Phase: `bgp-filter-community` and relation** -- same; metrics to `Registration.ConfigureMetrics`. One commit
5. **Phase: `bgp-gr`** -- both LLGR filters attached, readvertise opt-in, `stale`/`rebuilt`/`llgr-advertised` fields. One commit; forked proof per Q-3
6. **Phase: `bgp-rs`** -- fold into RS forwarding; judge for the two tests. One commit
7. **Phase: Delete** -- the `filterapi` registry and function types, `r.egressFilters`, `readvertiseEgressFilters`; doors and pages in the same commit (AC-13)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path |
| Correctness | each forked twin observed red at HEAD and green after; order identical; readvertise rail runs only opted steps |
| Naming | new fields and the attachment JSON keys kebab-case; metric names unchanged |
| Data flow | the reactor names no plugin; attached steps reach plugins only through `CallFilterUpdate` |
| Rule: no-layering | no attached row keeps a `filterapi.Register` beside its plugin filter, and the registry is deleted, not left empty |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| registry gone | grep `filterapi.Register(` and `IngressOrdered` under `internal/` is empty |
| forked twins | the named `.ci` pass, with recorded reds at HEAD |
| conformance | `-race` package tests of role, filter_community, gr |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a forked plugin's attachment, metadata set values and types, and `withdraw` on an unexpected direction are refused, never applied |
| Fail-open | an unavailable RFC-MUST filter does not silently accept (R-2) |
| Resource exhaustion | a plugin cannot attach unbounded steps (one per declared filter, judged at Stage 1) |

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

- The forked breakage is a reader in the wrong process, not a missing transport: moving the evaluator to the plugin makes the plugin's own state coherent on both transports, and only reactor facts need to cross.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `process-global-read-across-the-plugin-boundary.md` | 2026-11-01 | `filterapi.Register` filters of bgp-role and bgp-filter-community read config their forked plugin fills |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A (recommended): an attachment on the Stage 1 `FilterDecl`; the reactor runs each attached declaration as a one-member external chain through `runLibraryChain`, on both transports | B: BGP config prepends each plugin-declared filter to every peer's chain at load, as `prependDefaultFilters` does for `loop-detection`. C: keep `filterapi.Register` and give its function a forked transport | B lets an operator mark the reference `inactive:`, which RFC 9234 Section 5 forbids ("The operator MUST NOT have the ability to modify the procedures defined in this section."), puts an export member on every destination and so takes every peer off the RS fast path, and leaves the readvertise rail without the LLGR step. C is two filter mechanisms, the layering D-13 refuses. A reuses the declaration, the judge, the slot, the batch and the chain's verdict handling, adding only the attachment, `withdraw`, and fields |
| Reactor facts cross as declared fields of the existing view, not as new header members of `FilterUpdateInput` | widen the header with every `PeerFilterInfo` field | fields are encoded only when declared (D-10), the forked message stays small, and a third party reads facts by the same vocabulary as route fields |
| Route metadata keys become fields with readers and settable writers registered by the key's owner | a `meta` member in the verdict | one answer shape (sets) for every change, judged by the same declared-sets rule |
| A metadata set is visible to later ingress steps of the same pass and on egress, as the shared map is today | per-step maps | preserves `bgp-role` writing `src-peer-role` at Annotation 0 for the relation filter at Annotation 1 |
| `bgp-gr-llgr-ingress` becomes a plugin filter reading `llgr-advertised`, not engine code | engine-side like `loop` | `loop` is RFC 4271/4456 for every session; LLGR is gr's feature, and removing gr must remove it (`ai/rules/plugins.md`) |

## Open Design Questions (for Thomas)

| ID | Question | Options | What it changes |
|----|----------|---------|-----------------|
| Q-1 | May any plugin, a third-party one included, attach a filter that applies to every peer with no operator reference? | (a) any plugin the operator configured; (b) only compiled-in plugins; (c) any plugin, but the operator lists attached filters under the plugin block | (a) is the uniform contract (in-tree plugins hold no privilege). (b) keeps a privilege the doors say in-tree plugins do not hold. (c) adds a config leaf and makes OTC operator-modifiable, against RFC 9234 Section 5 |
| Q-2 | Peer scope of an attached filter | (a) every peer, the evaluator accepts unconfigured peers cheaply; (b) the plugin declares the peers it judges at runtime through a new engine RPC | (a) is simplest, but a forked role adds a round trip to every route of every peer (R-1). (b) bounds forked cost and the `state` delivery, at the price of a new RPC and a reconcile on reload |
| Q-3 | `bgp-gr` (and `bgp-rs`) refuse to start forked because of `retention.Family`, so the gr rows cannot get a forked `.ci` here | (a) this spec proves gr internal plus transport equality (`CheckFilterConformance`), and a new spec moves retention onto the bus and owes the forked twins; (b) this spec also moves retention | (a) keeps this spec to the filter mechanism and leaves the gr forked proof owed in a named spec. (b) widens scope into graceful-restart retention for rr and rs |
| Q-4 | BGP's `carries`/`settable` lists in `bgp/config/register.go` are hand-written; the new fields must join them | (a) add the new fields to the lists; (b) turn the lists into registrations each field's owner makes, in this spec | (a) is one line per field but grows a central list (`ai/rules/principles.md`). (b) is the registration the rules ask for, touching every existing field once |

## Known Limitations

- Forked metrics (A-5): if a forked plugin's counters reach no registry today, that is a plugin-infrastructure limit for every plugin, owed by a spec of its own if A-5 holds; this spec does not hide it.

## RFC Documentation (Scope: protocol)
Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The moved evaluators keep their RFC 9234 Section 5, RFC 9494 Sections 4.3 and 4.6, RFC 7999 Section 3.1 and RFC 8195 Section 3.2 quotes; each new call site names its RFC and section.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
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
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-filter-wire-6-stage-filters.md` only, in the same `./le commit create` script
