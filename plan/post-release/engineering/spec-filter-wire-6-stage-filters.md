# Spec: filter-wire-6-stage-filters -- every `filterapi.Register` row becomes a plugin filter on both transports

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P3 |
| Release reason | Child of `spec-filter-wire-0-umbrella` (post-release engineering). It also fixes shipped misbehaviour: `bgp-role` (RFC 9234 Section 5) and `bgp-filter-community` (RFC 7999 Section 3.1, RFC 8195) run forked accept every route with no error, which a release owes fixed whatever bucket the mechanism sits in |
| RFC gaps | - |
| Depends | `spec-filter-wire-0-umbrella` phases 1 and 3 (landed: 1a179d341e Stage 1 reads/sets, 29fc996ece batched `filter-update`, 4ee9ba017c typed bridge slot); the umbrella's D-13 `loop` row (engine step, committed before this spec starts) |
| Phase | 1/8 |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-filter-wire-0-umbrella` D-13 merges the in-process-only `filterapi.Register`
family into the one plugin filter mechanism (D-12): every filter owned by its
plugin, called through the typed `DirectBridge` slot when the plugin runs
internal and through the batched `filter-update` message when it runs forked.

Thomas's decisions of 2026-11-01 (Key Design Decisions, "owner, 2026-11-01")
fix the shape:

1. **Everything is a filter.** Every row that used `filterapi.Register`
   (`bgp-filter-community`, `bgp-filter-community-relation`, `bgp-role`, both
   `bgp-gr` LLGR rows, `bgp-rs`) becomes a plugin filter served on both
   transports.
2. **Every route-processing step is a filter in the peer's filter list** (owner,
   2026-11-01; `ai/META-DECISIONS.md`, "Goal and scope": "Every route-processing
   step is a filter in the peer's filter list"). There is no "every peer"
   attachment, no runtime peer-declaration RPC and no step beside the chain.
   Two kinds of member:
   - **Operator policy** (`bgp-filter-community` tag and strip, the community
     relation tag): a member the operator names in the peer's `filter import` /
     `filter export` list, run only where named.
   - **Procedures a feature's config implies** (`bgp-role` RFC 9234 Section 5
     OTC; the RFC 7454 Section 9 transit-leak check a role brings; `bgp-gr`
     LLGR ingress and egress, RFC 9494; `bgp-rs` transparency): the owning
     plugin declares an **auto-insertion** at Stage 1 (a condition on the
     peer's config, the reference, the direction, the position, required or
     not), and the config loader inserts the member into each peer's list
     where the condition holds. The member is visible like any other (show
     config, dry-run). A **required** one (role: RFC 9234 Section 5, "The
     operator MUST NOT have the ability to modify the procedures defined in
     this section.") refuses at verify an operator edit that names, moves or
     deactivates it, with the plugin's declared reason. The engine and config
     code never name a feature, a plugin or a config path (D-5a).
3. **`bgp-gr` and `bgp-rs` become real filters too, forked included.** This spec
   moves graceful-restart retention (`retention.Family`, the same-process call
   that makes `bgp-gr`, `bgp-rr` and `bgp-rs` refuse to run forked) onto the
   event bus, so all three run on both transports with forked-run tests.

The defect this closes, read at the producer (umbrella D-13 inventory, re-read
here): `bgp-filter-community`, `bgp-filter-community-relation` and `bgp-role`
keep their per-peer config (and, for role, the role each peer's OPEN announced)
in package globals their plugin's own callbacks fill. Run forked, those
callbacks fill the subprocess's copy, the daemon's copy stays empty, and the
daemon-side filter functions accept every route unchanged. No error, no log
line: RFC 9234 Section 5 ingress and egress procedures, the RFC 7999 BLACKHOLE
withholding and the RFC 8195 relation tags silently stop. `bgp-gr`, `bgp-rr` and
`bgp-rs` refuse to start forked (the `retention.Family` same-process call), so
their rows are not silently broken today, but they cannot run forked at all.

Goal: one filter mechanism, and no BGP plugin that must run in the daemon's
process for its procedures to hold. `filterapi.Filter`, `Register`,
`IngressOrdered`, `EgressOrdered`, `ReadvertiseEgressFuncs`, `NotifyPeerRemoved`,
`PreserveOpaqueAttributesFor`, `retention.Publish` and `retention.Family` are
deleted. Each moved filter's existing `.ci` passes with its plugin internal, and
a forked twin of it that is red at HEAD (or, for gr, rr and rs, refused at
HEAD) passes after.

## Required Reading

### Design doors

- [ ] `ai/doors/system.md`
  → Constraint: a plugin can run in another process; a plain Go call or package global that only the same-process callback fills is the classic silent defect. A per-route callback reaches DirectBridge as a typed slot of the protocol, and the same evaluator answers the batched message forked. One-to-many notification with no answer goes on the EventBus: retention is that shape once the DOWN order is used.
  → Constraint: `pkg/plugin/rpc` is a published contract: the Stage 1 binding, the `withdraw` `FilterAction`, and any new field are wire changes, made on purpose with their pages (`process-protocol.md`, `docs/plugin-development/`).
- [ ] `ai/doors/bgp-engine.md`
  → Constraint: the engine reaches plugin features only through what they register, and never names a plugin package from the reactor. The "filterapi.Register for compiled-in filters" boundary and "A filterapi filter is in-process only" both stay true until the last row moves, and are edited in that commit.
  → Constraint: received bytes are borrowed (`IsAsyncSafe` false); the slot path reads them on the reactor goroutine, the forked path encodes declared fields only (D-12).
- [ ] `ai/doors/bgp-plugins.md`
  → Constraint: a plugin declares twice (`registry.Registration` and the `sdk.Registration` passed to `p.Run`) and both must agree; `./le plugin declarations check` judges it. The binding is declared in both, from one `filterDecls()` value as `fakefilter` does.
  → Constraint: "Another plugin" depends on "the dependency order on state events (`bgp-gr` before `bgp-rib`)": the retention event relies on that order and the page names it.
  → Decision: the hook table row "`filterapi.Register` filters -- in-process only" is deleted by this spec, and the typed filter slot with its forked equivalent stays the only filter hook.
- [ ] `ai/doors/plugin-infra.md`
  → Constraint: one engine-op entry per RPC; the SDK stays generic: no new `On*` callback for bound filters, `sdk.OnFilter` already installs one evaluator on both transports. `sdk.CheckFilterConformance` under `-race` is the fast-path contract gate for every evaluator this spec writes.
  → Constraint: one plugin failure stops the startup tier (owner ruling); a declaration refused at Stage 1 fails that plugin.
  → Constraint: a name off the wire is looked up, never registered: a forked plugin's `bound-by` path is checked against the compiled schema, never added to it.
- [ ] `ai/doors/core.md`
  → Constraint: a registry holds value types and is the only list; `./le arch enumeration check` refuses a literal that restates it. The bound-step list is derived from declarations and peer config, never written. Events register their payload type once (`events.Register[T]`).
- [ ] `ai/doors/api-surfaces.md`
  → Constraint: no new command; the filter metrics stay named as they are (a Prometheus rename breaks dashboards).
- [ ] `ai/doors/config-yang.md`
  → Constraint: a backend that cannot apply a config exactly refuses it at verify: a peer whose config carries `filter ingress community` / `filter egress community` settings while its chain names no `bgp-filter-community` member is refused at verify, never accepted as a no-op.
  → Constraint: values arrive as strings; a `bound-by` boolean leaf is read in its string form (`./le config coercion check`).
- [ ] `ai/doors/le-tooling.md`
  → Constraint: forked twins are `.ci` files under `test/plugin/` run by `./le test`; a compiled Go fixture returns an error when an assertion fails; scoped package tests carry `-tags ze_bgp` through `./le job run`.

### Architecture Docs

- [ ] `docs/architecture/core-design.md` "Ingress Filter Pipeline", "Policy Filter Chain"
  → Constraint: ingress order is a declared property: stage, then priority, then name (`filterapi.LessOrder`), loop detection at `FilterStageProtocol`, the per-peer chain at `FilterStagePeerChain` (300) after every bound step. Bound procedures keep their HEAD stages; community and relation leave the stage list for the chain (D-6 below).
- [ ] `docs/architecture/api/process-protocol.md` "Runtime hot path", the Stage 1 `declare-registration`, "emit-event"
  → Constraint: `FilterDecl` (`name`, `name-pattern`, `direction`, `reads`, `sets`, `on-error`) is the Stage 1 filter contract; `registry.CheckFilterDeclarations` judges it in `runStartupHandshake`. The binding extends this declaration and this judge, not a second declaration.
- [ ] `docs/plugin-development/handlers.md` "Route filters"
  → Constraint: an evaluator answers from what it holds and never waits on its own data (the batch shares one `FilterUpdateTimeout`). Role's, community's, gr's and rs's evaluators read only plugin memory and the declared fields: they satisfy this.
- [ ] `docs/architecture/meta/README.md`, `meta/role.md`, `meta/filter-community.md`
  → Constraint: route metadata keys `src-role`, `src-peer-role`, `stale`, `rebuilt` are a cross-plugin contract with fixed types (string, string, uint8, bool); the RIB reads `stale` from the received event's `Meta`. Moving the writers must keep the key, the type, and that the map reaches the received event and the egress steps.
- [ ] `plan/post-release/engineering/spec-filter-wire-0-umbrella.md` D-12, D-13, R-14, AC-37
  → Decision: this spec is the home of D-13's plugin rows (community, relation, role, gr, rs); AC-37 is closed through AC-6 to AC-13 here, and the umbrella's phase 4 row 4g names this spec.
- [ ] `docs/architecture/api/architecture.md` "Attaching a Process to a Peer"
  → Constraint: which process sees which peer's events is the peer's `attach process` block. A bound procedure's plugin receives the `state` event of every peer its filter is bound to (an implicit attach for `state` only), so the removal reaches it on both transports; the page says so.
- [ ] `docs/architecture/api/ipc_protocol.md`
  → Constraint: a request line is framed by its newline and answer lines are width-framed; the binding, `withdraw` and the retention event travel inside existing JSON payloads, so framing is unchanged.
- [ ] `docs/architecture/config/syntax.md`
  → Constraint: no config grammar changes. A bound procedure is never written in a chain; an operator-policy filter is named with the existing chain reference form.
- [ ] `docs/features/ai-first.md` "diagnostic code registry" (the design doc `internal/core/diagnostic/codes.go` and `registry.go` declare)
  → Constraint: a diagnostic code is registered once; deleting `doctor-bgp-gr-out-of-process` removes its registry entry and its `ze show diagnostic` text in the same commit, and the page's code list is regenerated, never hand-edited.
- [ ] `docs/plugin-development/metrics.md` "When ConfigureMetrics Runs"
  → Constraint: `registry.Registration.ConfigureMetrics` is the plugin metrics hook; `filterapi.Filter.ConfigureMetrics` duplicates it for `bgp-filter-community` and goes.
- [ ] `docs/guide/graceful-restart.md` "Keep the bgp-gr engine in the daemon process", the route-server and reflector DOWN paragraph
  → Constraint: rr and rs decide retention while they handle the DOWN, synchronously, so a reconnect or Restart Time expiry that follows cannot change which families that DOWN withdraws. `bgp-gr` orders itself before both (`OptionalDependencies: bgp-rs`, `StartAfter: bgp-rr`, `gr/register.go`). The retention event keeps both properties.

### Source read for the owner's decisions

- [ ] `internal/component/bgp/retention/retention.go` - `Owner`, `Publish`, `Close`, `Family`: one process-global `atomic.Pointer` to gr's `familyRetained` lookup; `Family` answers false with no owner, a guard the comment names and that the three refusals depend on.
  → Constraint: callers are exactly `rr/rr.go` `handleStateDown` (line ~433) and `rs/server_handlers.go` `handleStateDown` (line ~184), each once per distinct family of the departed source; gr publishes in `runGRPlugin` (`gr.go`). Nothing else reads it.
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - `familyRetained` reads `peers[addr].staleFamilies`, set in `onSessionDownDeferred` while gr handles the DOWN.
  → Constraint: the answer rr and rs need is fixed when gr's DOWN handler returns: it is the set gr computed for that DOWN.
- [ ] `internal/component/bgp/server/events.go` `onPeerStateChange`, `deliverPeerState`, `sortByReverseDependencyTier`
  → Constraint: "Deliver sequentially in dependency order — each process must complete before the next starts": the engine waits for each process's `EventResult` before delivering the same state event to the next, dependents first. gr is delivered the DOWN, and returns, before rr and rs are delivered it.
- [ ] `internal/component/plugin/process/delivery.go` `Deliver`
  → Constraint: every delivery to a process (state events and emitted bus events alike) goes into that process's one `eventChan`, FIFO.
- [ ] `internal/component/plugin/server/dispatch.go` `deliverEvent`
  → Constraint: `emit-event` enqueues into each subscriber's `eventChan` (`p.Deliver`) before the RPC returns, so an event gr emits inside its DOWN handler is queued to rr and rs before the engine delivers them the DOWN. An event with no peer address is not peer-scoped and reaches every subscriber with no `attach` grant; a peer-scoped one goes through `PeerScopedProcs`.
- [ ] `internal/component/bgp/config/peers.go` `prependDefaultFilters`, `filterChainContains`
  → Decision (owner, 2026-11-01): `prependDefaultFilters` is the mechanism this spec generalises. Its prepend and its `filterChainContains` presence rule become the generic auto-insertion step, fed by the plugins' declared insertions instead of a literal; a required insertion adds the refusal of an operator reference to it (plain, moved or `inactive:`), which replaces the earlier "beside the chain" argument (D-5). An export member still takes a destination off the RS fast path (`hasActiveFilter`, `forward_rs.go`): A-13.
- [ ] `internal/component/bgp/reactor/peer_settings.go` `PeerSettings` (`ImportPolicy`, `ExportPolicy`, `RSClient`, `RawCapabilityConfig`, `CapabilityConfigJSON`); `reactor/config.go` (`session rs-client` read from the peer's resolved map)
  → Constraint: PeerSettings is built from the peer's resolved config map (bgp, group and peer levels merged). It already keeps derived forms of that map for plugin delivery (`CapabilityConfigJSON`); the binding reads the same resolved map, so group inheritance binds the same way it configures.
- [ ] `internal/component/bgp/plugins/role/yang/ze-role.yang` - `container role` augmented at `/bgp/peer`, `/bgp/group/peer`, `/bgp/group`; "A peer with no role container sends no" role capability.
  → Decision: `role` presence in the peer's resolved config binds `bgp-role`'s ingress and egress procedures.
- [ ] `internal/component/bgp/plugins/gr/yang/` - `leaf long-lived-stale-time` under `session capability graceful-restart` at the three scopes: "When this leaf is set, Ze advertises the LLGR capability (code 71)".
  → Decision: that leaf's presence binds the LLGR procedures; RFC 9494 Section 4.3 obliges exactly a speaker "that has advertised the Long-Lived Graceful Restart Capability to a neighbor".
- [ ] `internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang` - `filter ingress community` / `filter egress community` (`tag`, `strip`, `relation-tag`, `relation-function`, `scrub-own-ga`, `scrub-keep-function`) at bgp, group and peer.
  → Constraint: these settings stay where they are; what changes is that they apply only on a peer whose chain names the filter, and a peer carrying them without the member is refused at verify.
- [ ] `internal/component/bgp/plugins/filter_community/blackhole.go` `blackholeReadvertiseRefused`; `internal/component/bgp/blackholecfg`; the `blackhole` container is `bgp-rib`'s YANG (`rib/yang/ze-rib.yang`)
  → Constraint: the withholding refuses BLACKHOLE toward every eBGP, non-RS-client destination with no agreement. Its population is the peers WITHOUT the feature config, so no config-presence binding reaches it: Q-5.
- [ ] `internal/component/bgp/plugins/rs/register.go`, `rs/server.go`, `rs/server_validation.go`
  → Constraint: besides the retention refusal, `rs` reads `bgp-rib/validation-change` through `validationBus`, a `ze.EventBus` handle that `Registration.ConfigureEventBus` fills in the daemon only. Forked, `validationBus.Load()` is nil and the subscription is skipped with no log line (journal row below). `EnableRSForwarding` and `RelaysOnRSFastPath` are read by the daemon from the compiled registration and stay valid forked.

### RFC Summaries (Scope: protocol)

- [ ] `rfc/short/rfc9234.md`, text `rfc/full/rfc9234.txt` Section 5
  → Constraint (Section 5, ingress 1): "If a route with the OTC Attribute is received from a Customer or an RS-Client, then it is a route leak and MUST be considered ineligible (see Section 3)."
  → Constraint (Section 5, ingress 3): "If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS."
  → Constraint (Section 5, egress 1, a line break falls inside "RS-Client" as printed): "If a route is to be advertised to a Customer, a Peer, or an RS- Client (when the sender is an RS), and the OTC Attribute is not present, then when advertising the route, an OTC Attribute MUST be added with a value equal to the AS number of the local AS."
  → Constraint (Section 5, egress 2): "If a route already contains the OTC Attribute, it MUST NOT be propagated to Providers, Peers, or RSes."
  → Constraint (Section 5, last paragraph): "The operator MUST NOT have the ability to modify the procedures defined in this section." So the OTC member is a REQUIRED auto-insertion: the loader inserts it where `role` is configured, and verify refuses an operator reference to it (plain, moved or `inactive:`), quoting this sentence; removing the `role` container removes the role, and with it the obligation and the member.
- [ ] `rfc/short/rfc9494.md`, text `rfc/full/rfc9494.txt` Sections 4.3 and 4.6
  → Constraint (4.3): "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2:" then "The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received." So the ingress step is bound on the receiving peer and needs `llgr-advertised` synchronously; the egress step is keyed on the route's SOURCE (the neighbor Ze advertised LLGR to), runs toward every destination, and needs "LLGR received from this destination" and a withdraw toward it.
  → Constraint (4.6): "The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors." "The NO_EXPORT community [RFC1997] MUST be attached to the stale routes." "The stale routes MUST have their LOCAL_PREF set to zero." So egress needs the destination's iBGP fact, community add and local-preference set, both already settable fields.
- [ ] `rfc/short/rfc4724.md`, text Section 4.2
  → Constraint: the receiving speaker "MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability" (quoted at `rr.go` `handleStateDown`). Forked rr and rs must withhold the same families internal ones do.
- [ ] `rfc/short/rfc7999.md`, text Section 3.1; `rfc/short/rfc8195.md` Section 3.2
  → Constraint (7999 Section 3.1): "In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it." It binds every bilateral eBGP destination, configured or not (Q-5).
  → Constraint: the relation tag needs the source's iBGP fact and the source peer role `src-peer-role`.
- [ ] `rfc/short/rfc7947.md`, text Section 2.2
  → Constraint: "Optional recognized and unrecognized BGP attributes, whether transitive or non-transitive, SHOULD NOT be updated by the route server (unless enforced by local IXP operator configuration) and SHOULD be passed on to other route server clients." This is `bgp-rs`'s egress procedure toward RS-client destinations, bound by `session rs-client`.

**Key insights:**
- The forked breakage is a reader in the wrong process, not a missing transport for a daemon-side function. Running the evaluator in the plugin (`sdk.OnFilter`) puts the reader where the writer is, so role's `OnValidateOpen` learned roles, community's config apply and gr's LLGR capability state are coherent internal and forked with no state shipped to the daemon.
- What the plugin cannot know itself is reactor state: which peer, and per route `llgr-advertised`, the iBGP fact, local AS, route-server AS, the source peer on egress, the ingress metadata. Those become declared reads, encoded only when declared (D-10).
- Two filter kinds, one mechanism: an operator member and an auto-inserted procedure are both members of the peer's list and run through the chain machinery that already reaches every egress rail (`runLibraryChain`, the external member, `filter-update`). They differ only in who places them: the operator, or the loader acting on the owning plugin's declared insertion.
- Retention needs no request and response: the state DOWN is delivered sequentially in reverse dependency order and every delivery to a process is FIFO, so a retention event gr emits inside its DOWN handler is already queued to rr and rs when their DOWN arrives.

## Current Behavior (MANDATORY)

Read at HEAD 357b33c57c plus the parent session's `loop` row; the owner-decision sources re-read at ff15eecba0.

**Source files read:**
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `Filter` (Name, Stage, Priority, Ingress, Egress, Readvertise, PreserveOpaqueAttributes, PeerRemoved, ConfigureMetrics), `Register`, `IngressOrdered`, `EgressOrdered`, `ReadvertiseEgressFuncs`, `IngressFilters`, `EgressFilters`, `NotifyPeerRemoved`, `PreserveOpaqueAttributesFor`, `LessOrder`, the stage constants (Protocol 0, Policy 100, Annotation 200, PeerChain 300), `PeerFilterInfo`, `EnableRSForwarding`, `SetMetricsRegistry`.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `buildOrderedIngressSteps` (engine loop step, `IngressOrdered`, peer-chain step, stable sort by `LessOrder`), `buildOrderedEgressSteps`, `runIngressPolicyChain`, `runEgressPolicyChain`, `ingressPolicyWithdrawals`.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - the ingress pass over `orderedIngressSteps`: an in-process step gets one shared `ingressMeta` map; the map becomes `msg.Meta` on the received event.
- [ ] `internal/component/bgp/reactor/reactor.go` - `egressFilters`, `orderedIngressSteps`, `orderedEgressSteps`, `readvertiseEgressFilters` built once at startup; `filterapi.SetMetricsRegistry`.
- [ ] Egress rails reading `r.egressFilters` directly: `forward_rs.go` (RS fast path, two sites), `reactor_api_forward.go` (one site, plus the ordered `forwardUpdateCore` loop), `reactor_api_forward_batch.go`, `reactor_api_relay.go`; the stale readvertise rail `reactor_api_batch.go`. Six rails in all.
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - removal calls `filterapi.NotifyPeerRemoved`.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `facts.preserveOpaqueAttributes = filterapi.PreserveOpaqueAttributesFor(facts.filterInfo)` once per snapshot, part of the fan-out dedup identity and the body cache key in `forward_rs.go` and `reactor_api_forward.go`.
- [ ] `internal/component/bgp/reactor/filter_library.go` - `runLibraryChain`, `libraryCaller.CallFilter` to `CallFilterUpdate` under `FilterUpdateTimeout`, `Chain.Apply` into a `ModAccumulator`, `rewriteLibrary`.
- [ ] `internal/component/bgp/policychain/external.go`, `external_view.go` - `Member{Name, Plugin}`, `ExternalDeclaration{Reads, Sets, OnError}`, `ExternalCaller`, `judgeExternal`, `applyExternalSets`, `memberView`.
- [ ] `internal/component/bgp/policyview/view.go`, `register.go`, `fields_json.go` - `View`, `Session{EBGP, Sender, SenderKnown}`, the field readers and JSON writers registered from `init()`.
- [ ] `internal/component/policy/fields.go`, `internal/component/bgp/config/register.go` - `MustRegisterField`, `MustRegisterAttachment`; BGP's `carries` and `settable` lists.
- [ ] `pkg/plugin/rpc/types.go`, `enums.go`, `filter_slot.go` - `FilterDecl`, `FilterUpdateInput`, `FilterUpdateOutput`, `FilterSet`, `FilterAction` accept/reject/modify, `FilterView`, `FilterRoute`, `DirectBridge.SetFilterSlot`/`CallFilterSlot`.
- [ ] `internal/component/plugin/registry/filter_declaration.go`, `registry.go` - `DeclaredFilter`, `CheckFilterDeclarations`, `SetRuntimeFilterDeclarations`/`RuntimeFilterDeclarations`, `Registration.Filters`.
- [ ] `internal/component/plugin/registration.go` - `FilterRegistration.Overrides`, copied at `startup.go`, read nowhere else found.
- [ ] Plugins: `filter_community/register.go`, `filter_community.go`, `blackhole.go`; `role/register.go`, `role/otc.go`, `role/role.go`; `gr/register.go`, `gr/gr.go` (`runGRPlugin` refusal, `retention.Publish`, `setEgressState`), `gr/gr_egress.go`, `gr/gr_ingress.go`, `gr/gr_state.go`, `gr/doctor.go` (`doctor-bgp-gr-out-of-process`); `rr/rr.go` (refusal, `retention.Family`); `rs/server.go` (refusal, `validationBus`), `rs/server_handlers.go` (`retention.Family`), `rs/register.go` (`preserveOpaqueAttributes` returns `peer.RSClient`, `ConfigureEventBus`).

**The rows this spec moves:**

| Row | Stage / prio at HEAD | Hooks | Reads besides the UPDATE | Writes | Forked today |
|-----|----------------------|-------|--------------------------|--------|--------------|
| `bgp-filter-community` | Policy 0 | ingress (tag, strip, scrub), egress (tag, strip, BLACKHOLE withhold), `ConfigureMetrics` | own config by Name/GroupName; LocalAS, PeerAS, RouteServerAS; dest Name/GroupName/IsIBGP/RSClient on egress | payload rewrite (ingress), community ops (egress), suppress | silently accepts everything |
| `bgp-filter-community-relation` | Annotation 1 | ingress | own config; `meta["src-peer-role"]`; IsIBGP | RFC 8195 community tag | silently tags nothing |
| `bgp-role` | Annotation 0 | ingress, egress, `PeerRemoved` | own config by Address/GroupName; role learned from OPEN (plugin process); src and dest identity on egress; `meta["src-role"]` | `meta["src-role"]`, `meta["src-peer-role"]`, OTC insert, payload-to-withdrawal (ingress leak), suppress (egress) | silently skips every RFC 9234 Section 5 MUST |
| `bgp-gr` LLGR egress | Annotation 0, `Readvertise` | egress on the forward rails and the stale readvertise rail | `meta["stale"]`, `meta["rebuilt"]`, `src.LLGRAdvertised`; `egressState` (plugin memory, published to the daemon by `setEgressState`); dest Address, PeerAS, LocalAS | community add NO_EXPORT, LOCAL_PREF 0, `SetWithdraw` | refuses to start forked (`gr-external-refuses.ci`) |
| `bgp-gr-llgr-ingress` | Protocol 0 | ingress | `src.LLGRAdvertised`, COMMUNITIES | `meta["stale"] = 2` | refuses forked (same plugin) |
| `bgp-rs` | none | `PreserveOpaqueAttributes` only | `peer.RSClient` | none (forwarding treatment) | refuses forked (`rs-external-refuses.ci`) |
| retention (`bgp-rr`, `bgp-rs` asking `bgp-gr`) | - | `retention.Family` on DOWN | gr's `staleFamilies` for the departed peer | which families the DOWN withdraws | rr, rs and gr each refuse forked |

**Behavior to preserve:**
- The order of the engine and procedure steps: loop (Protocol), `bgp-gr-llgr-ingress` (Protocol), `bgp-role` (Annotation 0), then the per-peer chain (PeerChain); egress likewise with role and gr egress at Annotation 0 and rs transparency before the chain. `TestOrderedIngressCarriesTheEngineLoopStep` keeps its expectation.
- Every `.ci` of the moved filters with the plugin internal, expectations unchanged: `role-otc-*.ci`, `community-tag.ci`, `community-strip.ci`, `community-relation-tag.ci`, `community-blackhole-noexport.ci`, `community-scrub-own-ga.ci`, `rfc7999-blackhole-readvertise-agreement.ci`, `bgp-filter-community-reload.ci`, `bgp-role-export-reload.ci`, `role-learned-cleared-on-peer-delete.ci`, `llgr-*.ci`, `graceful-restart-llgr.ci`, `addpath-departure-rr-gr-retained.ci`, `addpath-departure-rr-gr-expiry.ci`, `bgp-rs-withdrawal-replacement-gr.ci`, the `bgp-rs-*` forwarding tests. The community tests' configs gain the chain member the operator now names (D-6); that config edit goes to `ze-test-judge` with the owner decision as its source.
- Route metadata keys, types, and their presence on the received event (`RawMessage.Meta`, JSON `route-meta`).
- The stale readvertise rail runs only readvertise-opted egress steps, never the full egress set.
- `bgp-role` frees a removed peer's learned role before the removal returns (internal) and before a re-created peer's OPEN (both transports).
- rr and rs decide retention synchronously while handling the DOWN, from gr's decision for that DOWN, and a reconnect or expiry after it cannot change it.
- The BLACKHOLE withholding toward every non-agreeing bilateral eBGP destination (Q-5 reading).
- The filter_community metric names and labels.

**Behavior to change:**
- `bgp-role`, `bgp-filter-community`, `bgp-filter-community-relation`, `bgp-gr`, `bgp-rr` and `bgp-rs` run forked and apply their procedures exactly as internal. The three refusals, `doctor-bgp-gr-out-of-process` and the three `*-external-refuses.ci` go.
- Community tag, strip, scrub and relation run only on peers whose chain names the filter; settings without the member are refused at verify.
- `filterapi.Register`, the hooks it carries and `retention.Publish`/`Family` no longer exist; the doors' "in-process only" filter row is deleted.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `filterapi` ordered stages | `filterapi/filterapi.go` `Filter`, `Register`, `IngressOrdered`, `EgressOrdered`, `ReadvertiseEgressFuncs`, `LessOrder`, `FilterStage*`; `reactor/filter_ordered.go` `buildOrdered*Steps` | Go function pointers registered from `init()` at (stage, priority), sorted with the engine loop step and the per-peer chain step; six egress rails | **Replaced** (`ai/rules/no-layering.md`): the registry and its function types are deleted first. `LessOrder` and the stage constants **stay**. `buildOrdered*Steps` are **extended**: step kinds become engine loop, bound procedure, peer chain; their input becomes the runtime filter declarations and the peer's bindings |
| Stage 1 filter declarations | 1a179d341e; `rpc.FilterDecl`; `registry.CheckFilterDeclarations`, `RuntimeFilterDeclarations`; `process-protocol.md` Stage 1 | A plugin declares each filter's name, direction, reads, sets and on-error; judged at the handshake | **Extended**: the Stage 1 registration gains `filter-insertions` (D-5), judged by the same handshake check and recorded in the same store; `registry.Registration.FilterInsertions` is the compiled twin, both from one plugin function |
| Default chain filters | `bgp/config/peers.go` `prependDefaultFilters`, `filterChainContains` | Config `loop-detection` entries are prepended to every peer's import chain unless named, plainly or `inactive:` | **Extended** (owner, 2026-11-01): generalised into the auto-insertion step (D-5). The prepend and the presence rule are shared; the inserted references come from the declared insertions, never a literal; required insertions add the verify refusal. Loop-detection's entries are operator policy config and keep feeding the same prepend |
| Peer resolved config | `PeerSettings` built from the merged bgp/group/peer map (`reactor/config.go`); `peerRoles` (`bgp/config/peers.go`) reads the resolved peer map | The loader derives settings and plugin-delivery forms from the peer's resolved map | **Extended**: the insertion step judges each declared condition against the same resolved map, so group inheritance inserts as it configures and a reload re-inserts with the settings. `peerRoles`, `declaredRole` and `leakFilterByRole`, which name `role` in config code, are deleted with the obligation machinery (D-15) |
| Typed DirectBridge filter slot | 4ee9ba017c; `rpc.DirectBridge.SetFilterSlot`/`CallFilterSlot`; `sdk.OnFilter`; `server.CallFilterUpdate` | One evaluator per filter, called on the reactor goroutine internal | **Reused unchanged** as the internal transport of every bound and chain step |
| Batched `filter-update` | 29fc996ece; `ipc/filter_batch.go` `filterBatcher` | One exchange per plugin connection, up to 64 routes, one verdict each | **Reused unchanged** as the forked transport |
| Library chain and external member | `policychain.Chain`, `Member{Plugin}`, `judgeExternal`, `applyExternalSets`; `reactor/filter_library.go` `runLibraryChain` | Runs a chain whose external members are plugin filters | **Extended**: a bound step runs as a one-member external chain compiled from its declaration through `runLibraryChain`. New: metadata-field sets and a `withdraw` verdict |
| `policyview` field registry | `policyview/register.go`, `fields_json.go`; `policy.MustRegisterField`; `bgp/config/register.go` | Per-field readers and JSON writers registered from `init()` | **Extended**: session facts (`peer-name`, `peer-group`, `peer-address`, `local-as`, `route-server-as`, `ibgp`, `rs-client`, `llgr-advertised`, the source peer's facts on egress), route metadata (`src-role`, `src-peer-role`, `stale`, `rebuilt`), `otc`, and the forwarding treatment `preserve-opaque-attributes` become fields, each registered by its owner from `init()`. BGP's `carries`/`settable` become those registrations (Q-4 rule, D-9) |
| `ModAccumulator` and attribute mod handlers | `filterapi.ModAccumulator`, `RegisterAttrModHandler`; `forward_build.go` `buildModifiedPayload` | Per-attribute ops, a withdraw flag, NLRI rewrite | **Kept**: sets reach it through `Chain.Apply`; `SetWithdraw` is reached by `withdraw`; `preserve-opaque-attributes` replaces the per-snapshot `facts.preserveOpaqueAttributes` in the dedup identity and body cache key |
| Route metadata map | `reactor_notify.go` `ingressMeta`; `meta/README.md` | One map per received UPDATE shared by in-process steps | **Extended**: fed by metadata-field sets; egress steps read it as declared fields |
| Peer removal notice | `filterapi.NotifyPeerRemoved`; peer `state` with reason `removed`; `role.pruneLearnedRoles` | Frees per-peer filter state, internal only | **Replaced**: a bound procedure's plugin receives the `state` event of every peer it is bound to (implicit attach for `state`) |
| State delivery order | `bgp/server/events.go` `deliverPeerState` (sequential, reverse dependency tier, waits for each `EventResult`); `process/delivery.go` one FIFO `eventChan` per process | gr handles a DOWN before rr and rs are delivered it | **Reused** as the ordering guarantee of the retention event (D-11) |
| Retention query | `bgp/retention` `Publish`, `Family`; callers rr and rs `handleStateDown`; publisher gr `runGRPlugin` | A process-global function pointer, answering false with no owner | **Replaced** by the `bgp-gr/retention` bus event: `Publish`, `Family` and `Owner` are deleted first, the package keeps the event's name and its registered payload type (D-11) |
| EventBus emit and subscribe | `sdk.EmitEvent`, `server.deliverEvent`, `events.Register[T]`, `RegisterNamespace`/`RegisterEventType` | Plugin-emitted events reach subscribers on both transports | **Extended** with one event type; rs's `bgp-rib/validation-change` moves from the in-process `ConfigureEventBus` handle to the same SDK subscription, so it holds forked |
| Plugin metrics hook | `registry.Registration.ConfigureMetrics`, `registry.InjectPluginMetrics` | Hands an in-process plugin the metrics registry | **Reused**: `filter_community` moves its counters there. Forked counters are A-5 |
| RS forwarding activation | `filterapi.EnableRSForwarding`; `registry.RelaysOnRSFastPath` | `bgp-rs`'s `init()` activates the reactor RS fast path in the daemon binary | **Kept**: both are read in the daemon from the compiled registration, which holds whether rs runs internal or forked |
| Forked test launch | `le mock plugin-external <name>` (`internal/test/cli/cmd_plugin_external.go`) | Runs a registered engine's `RunEngine` over a real TLS connect-back | **Reused** for every forked `.ci` |
| Doctor check for gr placement | `gr/doctor.go` `checkGRInProcess`, `doctor-bgp-gr-out-of-process` | Reports a forked gr from the config | **Replaced** by nothing: the arrangement becomes valid; the check, its diagnostic code and its doc text are deleted with the refusal |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Stage 1 `declare-registration` (in-process `registry.Registration.FilterInsertions` and the `sdk.Registration`, or a forked plugin's line): `FilterDecl` and `filter-insertions`.
- Config load and reload: each peer's resolved map, against which the loader judges each declared insertion condition.
- Per route: a received UPDATE on a session read goroutine (ingress), and each destination of a forward on the six egress rails.
- Per DOWN: the engine's sequential `state` delivery.

### Transformation Path
1. Handshake: `CheckFilterDeclarations` judges each declaration, and the insertion rules (a reference the declaring plugin serves, one direction, priority 0-9999, exactly one condition, a `config` path that is a compiled schema node under `bgp peer`, a `session` fact some owner registered, a reason on a required one); the runtime store records it.
2. Config load and reload: after the chains are canonical, the insertion step prepends each declared insertion whose condition the peer meets to that peer's import or export list, in ascending priority, unless the operator named the reference (plain or `inactive:`); for a required insertion that operator reference is refused at verify with the declared reason. The lists then compile exactly as operator lists do (D-5a: judged generically from the declarations).
3. Ingress, per route: the inserted members run where they sit in the chain through `runLibraryChain`; metadata-field sets land in the run's map, attribute sets apply, `reject` drops, `withdraw` keeps the withdrawal half (`ingressPolicyWithdrawals`). Community and relation run among the operator's members where named.
4. The map becomes `msg.Meta` as today.
5. Egress, per destination: the destination-keyed bound steps of the destination and the source-keyed bound steps of the route's source run, with destination and source facts and the ingress map as declared reads; `reject` suppresses for this destination, `withdraw` sets `SetWithdraw`, sets accumulate (`preserve-opaque-attributes` included); then the destination's export chain.
6. Stale readvertise: only readvertise-opted bound steps run.
7. DOWN: gr's handler computes the retained families and emits `bgp-gr/retention` {peer address, retained families} before returning, for every DOWN it handles (an empty list included); rr and rs, delivered that event first by FIFO, record it, then consume it when their own DOWN for that peer arrives.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to internal plugin | typed `DirectBridge` filter slot, lazy view, on the reactor goroutine | No |
| Engine to forked plugin | batched `filter-update`, declared fields only, `FilterUpdateTimeout` | No |
| Engine to plugin, peer lifecycle | peer `state` event with reason `removed`, implicit for bound plugins | No |
| gr to rr and rs | `emit-event` `bgp-gr/retention`, not peer-scoped, FIFO ahead of the DOWN | No |

### Integration Points
- `runLibraryChain` / `policychain.Chain` - bound steps are one-member chains.
- `registry.RuntimeFilterDeclarations` - the source of bound steps.
- `PeerSettings` - the peer's present `bound-by` paths.
- `policyview` readers and writers - the new fields.
- `ModAccumulator` - egress sets, withdraw, preserve-opaque.
- `server.deliverEvent`, `deliverPeerState` - the retention ordering.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to prove: bound steps call only `CallFilterUpdate` |
| No unintended coupling (components stay isolated) | No | to prove: the reactor names no plugin package; `./le arch tier check` |
| No duplicated functionality (extends existing, does not recreate) | No | to prove: no second call path beside `runLibraryChain`; no retention query beside the event |
| Zero-copy preserved where applicable (refs, not copies) | No | to prove: slot path encodes nothing undeclared (`BenchmarkFilterFastPath` allocations) |
| Registration over hardcoding, outbound | No | to prove: new fields registered by their owners from `init()` |
| Registration over hardcoding, inbound | No | to prove (D-5a): grep of `bgp-role`, `bgp-filter-community`, `bgp-gr`, `bgp-rs`, `bgp-rr`, `role`, `long-lived-stale-time`, `rs-client`, `retention`, `reject-asn`, `transit-leak` as insertion or dispatch literals under `internal/component/bgp/reactor`, `bgp/config`, `bgp/server`, `policyview` and `plugin/server` is empty; `./le arch enumeration check` passes; `fakefilter` declares an insertion with no engine edit (AC-1) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Forked role and community accept every route unchanged today | umbrella D-13 inventory; globals read in `filter_community.go`, `role.go` | the forked twins are not red at HEAD | each forked twin observed red at HEAD before the move | unvalidated |
| A-2 | Every state the moved evaluators read lives in the plugin process or is a reactor fact a declared read can carry | role learns roles in `OnValidateOpen`; community config from its apply; gr caps from OPEN events in the plugin | a filter needs daemon-only state | the Current Behavior table, re-read per row | unvalidated |
| A-3 | `llgr-advertised` must stay a synchronous reactor fact | `gr_ingress.go` `llgrStaleFromSource` comment | first UPDATEs judged before the record | the declared field is read from the session on the read goroutine | unvalidated |
| A-4 | The external member answers everything once `withdraw`, metadata sets and `preserve-opaque-attributes` exist | `judgeExternal`, `applyExternalSets`; the Writes column | a filter needs a rewrite the sets cannot express | each evaluator's tests through `CheckFilterConformance` | unvalidated |
| A-5 | Forked plugins have no metrics export path | `metrics.md`: `ConfigureMetrics` is injected in process | forked counters already reach the registry | read `registry.InjectPluginMetrics` for the forked branch | unvalidated |
| A-6 | `FilterDecl.Overrides` is dead | `startup.go` copies it, no reader found | it is live and interacts with bindings | grep `Overrides` under `internal/` and `pkg/` | unvalidated |
| A-7 | Rebuilding the step lists on plugin lifecycle and peer settings change is safe against a concurrent pass | atomic-pointer publish | a pass sees a torn list | `-race` test with a respawn and a reload during traffic | unvalidated |
| A-8 | `le mock plugin-external` starts each of `bgp-role`, `bgp-filter-community`, `bgp-gr`, `bgp-rr`, `bgp-rs` forked | `gr-external-refuses.ci` uses it | the twins need another launcher | first forked twin starts and logs its handshake | unvalidated |
| A-9 | A forked process's `EventResult` for a state event is returned after its handler returns, so gr's emit precedes the engine's next delivery | `deliverPeerState` comment "each process must complete before the next starts"; `deliveryLoop` | rr or rs could see its DOWN before gr's retention event | read the forked deliver path in `process/delivery.go`; `TestRetentionEventPrecedesDown` over both transports | unvalidated |
| A-10 | The binding paths a declaration names are schema nodes the daemon has compiled (role, gr and rs YANG ship in the daemon) | `config-yang.md`: an external plugin's own YANG is not compiled | a third-party bound procedure cannot bind on its own config | the judge refuses an unknown path with the reason; the limit is written in `handlers.md` | unvalidated |
| A-11 | Nothing besides `retention.Family`, `setEgressState` and `validationBus` keeps gr, rr or rs in the daemon process | the three refusal comments; `rs/register.go` | another in-process reliance breaks a forked run silently | `./le plugin boundary check` and a read of each `register.go` at the start of phase 5; each forked twin | broken (phase 5): rr and rs also read `ribevents.LookupFlowSpecPath` and `ribevents.FlowSpecRoutes`, which only bgp-rib fills in the daemon process (journal row in Owned journal rows); `validationBus` is gone (D-13, both rr and rs), and adj-rib-in and rib take `bgp-rib/validation-change` through the SDK; the receive eligibility gate (`ribevents.RegisterValidationLookup`) is in-process too (journal row); the silent fail-open is closed: adj-rib-in announces `bgp-rib/validation-gate` and a process without its lookup refuses every path and logs (`ribevents.NoteValidationGate`, `TestValidationGateAnnouncedOverTransport`); the forked store's verdicts cross through `request bgp adj-rib-in eligibility` (3250ee22cc, `TestForkedGateServesValidatedVerdicts`); bgp-rib's FlowSpec lookups cross through `request bgp rib flowspec-gate`, announced as `bgp-rib/flowspec-gate` and read by the reactor, rr, rs and adj-rib-in (`ribevents.NoteFlowSpecGate`, `TestRemoteFlowSpecAnswersThroughRIBCommand`, `TestSubscribeValidationGateAppliesFlowSpecGate`); a late subscriber emits `bgp-rib/gate-query` and both owners announce again. bgp-rib's own emissions reach the engine forked through the SDK (`sdkEventBus`, 3fa9f8bd54, `TestBestChangeReachesTheEngineOnBothTransports`); bgp-rib asks the gate once per selection, replay, refresh and FlowSpec reconciliation (`candidateGate`, `replaySourceRefused`, `dropIneligible`, `TestRIBAsksForkedGateInBatches`) and the reactor's stored FlowSpec relay once per section chunk (`flowSpecRuleBatch`, `TestFlowSpecSectionAsksForkedRIBInBatches`); forked rr and rs replay FlowSpec through the gate (`TestForkedRRReflectsFlowSpecThroughGate`, `TestForkedRSReplaysFlowSpecThroughGate`) |
| A-12 | The loader that inserts reads every declaration that can insert: the compiled registrations from `init()`, and for a third-party plugin the Stage 1 runtime store, before peers are built | `ai/digests/hub-engine.md` step 7: peers start after every plugin phase | a third-party insertion arrives after the peers were built and never inserts | read where `PeersFromConfigTree` runs relative to Stage 1 in the daemon boot; the forked `fakefilter` twin | broken, then fixed (Mistake Log): the loader read only `registry.All()`, no Stage 1 insertion was stored, and peers are built before Stage 1. Proven by `TestThirdPartyInsertionAppliesFromStageOne` and `test/plugin/stage-filter-third-party.ci` |
| A-13 | An auto-inserted export member on an RS-client destination need not take it off the RS fast path | `forward_rs.go` `hasActiveFilter` | rs transparency inserted on every RS client disables the fast path | 06b26570b6: no declared field limits the fast path, so the gate is removed for every member, inserted or operator-placed. The fast path runs each destination's active export chain itself through `runEgressPolicyChain`; a reject sends the withdrawal, a received End-of-RIB (IPv4 and MP_UNREACH_NLRI forms) passes unchanged, and a chain that could not run or a refusal needing `withdrawalBySection` returns the destination to bgp-rs. Proof: `TestRSFastPathRunsTheExportChain` and `role-otc-rs-withdraw-eor` red with the gate restored, green without; `--pattern rs-` 47/47, `role-otc` 8/8 | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A forked bound procedure adds a round trip per route of each bound peer, and a slow plugin stalls ingress up to `FilterUpdateTimeout` | functional run time of the twins; `BenchmarkFilterForkedBatch` | binding bounds the cost to configured peers; batching shares one exchange; the docs say what forked costs |
| R-2 | A bound procedure's plugin dies: fail-closed drops routes, fail-open skips an RFC MUST | a `.ci` with a stopped plugin | role, gr and rs declare `on-error reject`; the plugin pages state it |
| R-3 | Ordering drifts because steps come from runtime declarations | order tests | the sort key is unchanged; `TestBoundStepsSortLikeTheRegistry` pins the procedure order |
| R-4 | An egress rail misses bound steps | `role-otc-rs-client-dest-stamp.ci` passes internal only by accident | one per-peer egress step list consumed by all six rails; a wiring test per rail |
| R-5 | `rebuilt` loses the gr recovery path | `llgr-readvertise*.ci` red | `rebuilt` and the source's `llgr-advertised` are declared reads of the gr egress step |
| R-6 | Peer identity drift: role keys by Address then group, community by Name then GroupName; dynamic peers are `dyn-<addr>` | `dynamic-peer-gets-group-role-capability.ci` red forked | `peer-name`, `peer-group`, `peer-address` are declared reads |
| R-7 | A third-party plugin binds a procedure at `FilterStageProtocol` priority 0 ahead of the RFC checks | review | any configured plugin may bind (Q-1 rule); it binds only on peers whose config carries its own path, and ties sort by name |
| R-8 | An operator with community settings forgets the chain member after the change | `bgp-filter-community-reload.ci` | verify refuses settings without the member, naming the peer, the setting and the member to add |
| R-9 | Community moved from Policy 0 to the chain reorders it against role | `unified_filter_order_test.go` | role reads no community and community reads no role output except relation, which stays after role; the order test is changed through `ze-test-judge` with D-6 as source |
| R-10 | A stale retention verdict from an earlier DOWN is consumed at a later one | `addpath-departure-rr-gr-retained.ci` forked | rr and rs consume and clear the verdict at each DOWN; gr emits one for every DOWN it handles, an empty list included |
| R-11 | A peer gr does not receive state for yields no verdict and reads as not retained | a GR peer outside gr's attach | gr already retains only peers it sees; a missing verdict means no gr state, the answer `Family` gave with no owner; the page states it |
| R-12 | rs's per-route `preserve-opaque-attributes` set replaces a per-snapshot fact on the RS fast path | `bgp-rs-fastpath*.ci`, fan-out dedup tests | the field joins the dedup identity and body cache key as the snapshot fact did; internal it is one slot call per route per RS-client destination |
| R-13 | Deleting the three `*-external-refuses.ci` and `doctor-bgp-gr-out-of-process` loses proof | review | each is replaced by a forked twin proving more (the plugin runs and its procedure holds); the deletions go to `ze-test-judge` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes mis-filtered on BGP peers: OTC leaks accepted or valid routes dropped, BLACKHOLE leaked, LLGR stale routes advertised to non-LLGR eBGP peers, a restarting peer's routes withdrawn by a route server or reflector |
| How is it reverted? | Per phase, each a commit; the mechanism commit reverts alone while no row uses it |
| Who else touches this path? | `spec-filter-wire-0-umbrella` phase 4 rows (filter_family, filter_irr, the five match providers), phase 5 deletion of the text machinery; any session editing `reactor/filter_ordered.go`, `reactor_notify.go`, `pkg/plugin/rpc/types.go`, `bgp/plugins/rr`, `bgp/plugins/rs`, `bgp/plugins/gr` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Stage 1 `filter-insertions` from a compiled-in plugin | → | `CheckFilterDeclarations` insertion rules, runtime store | `TestFilterInsertionDeclarationJudged` (`internal/component/plugin/registry/filter_declaration_test.go`) |
| Declared insertions and the peer's resolved config | → | the loader's insertion step, group inheritance, priority order | `TestInsertionFollowsPeerConfig`, `TestInsertionsSortByPriority` (`internal/component/bgp/config/filter_insertion_test.go`) |
| Received UPDATE, plugin forked, peer's config meets the condition | → | the inserted member through `runLibraryChain` and the batch | `test/plugin/stage-filter-forked.ci` with `fakefilter` declaring an insertion |
| Received UPDATE, peer whose config does not meet it | → | no member, no call | `test/plugin/stage-filter-unbound-peer.ci` |
| An operator reference to a required inserted member (plain, moved, `inactive:`) | → | verify refusal with the declared reason | `TestRequiredInsertionRefusesOperatorReference` (`internal/component/bgp/config/filter_insertion_test.go`) |
| Peer removal | → | `state` removed delivered to a bound plugin | `test/plugin/role-learned-cleared-on-peer-delete-forked.ci` |
| DOWN of a GR peer, gr and rr forked | → | `bgp-gr/retention` event before the DOWN | `test/plugin/addpath-departure-rr-gr-retained-forked.ci` |
| Community settings without a chain member | → | verify refusal | `test/reload/community-settings-without-member-refused.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin declares an auto-insertion (reference, direction, priority, one condition: a `config` path of the peer, optionally with values, or a `session` fact its owner registered; required or not) at Stage 1, internal or forked, with no engine edit (`fakefilter` proves it: no file under `reactor`, `bgp/config`, `policyview` or `plugin/server` changes for it) | The member is in the import or export list of exactly the peers whose resolved config (bgp, group, peer) meets the condition, ahead of the operator's members in priority order, visible in the resolved chain; its evaluator is called for every route of those peers and never on other peers |
| AC-2 | An insertion naming a reference the declaring plugin does not serve, no direction or both, a priority outside 0-9999, no condition or two, a `config` path that is not a compiled schema node under `bgp peer`, a `session` fact no owner registered, a required insertion with no reason, or a duplicate (reference, direction) | The plugin is refused at Stage 1 with a message naming the insertion, the rule and the fix |
| AC-3 | The procedure positions (loop, llgr-ingress, role ingress; role egress, gr egress, rs transparency) and the peer chain | The built lists put them in their HEAD order; community and relation run inside the chain at the operator's position, and relation still reads the `src-peer-role` role wrote |
| AC-4 | A bound step answers reject, modify with sets, withdraw, or fails (timeout, unavailable, malformed) | Ingress: drop, apply, keep the withdrawal half, on-error; egress: suppress for that destination, apply, withdraw toward it, on-error; identical to a chain member's handling |
| AC-5 | A bound ingress step sets a metadata field (`src-role`, `src-peer-role`, `stale`) | The key is in the received event's `Meta` with its registered type, and later ingress and egress steps read it as a declared field |
| AC-6 | `bgp-role` forked: route with OTC from a customer; route without OTC from a provider; export to a customer without OTC; export with OTC to a provider | Ineligible (withdrawn); OTC added with the remote AS; OTC added with the local AS; not propagated; each the same internal and forked (forked twins of `role-otc-ingress-reject.ci`, `role-otc-egress-stamp.ci`, `role-otc-egress-filter.ci`, `role-otc-rs-client-dest-stamp.ci`, `role-otc-unicast-scope.ci`), each twin observed red at HEAD |
| AC-7 | `bgp-role` on a peer whose config has no `role` container; an operator who names the role member in a chain, moves it, or writes it `inactive:` on a peer with `role` | No role member on that peer; the reference is refused at verify with the declared RFC 9234 Section 5 reason, naming the peer, the chain and the reference |
| AC-8 | `bgp-filter-community` forked, named in the chain: ingress tag and strip, egress tag and strip, scrub | Same wire result as internal (forked twins of `community-tag.ci`, `community-strip.ci`, `community-scrub-own-ga.ci`), each red at HEAD |
| AC-9 | BLACKHOLE toward a non-agreeing bilateral eBGP destination, `bgp-filter-community` forked | Withheld as internal (forked twins of `community-blackhole-noexport.ci`, `rfc7999-blackhole-readvertise-agreement.ci`), each red at HEAD; bound as the Q-5 reading states until Thomas answers Q-5 |
| AC-10 | `bgp-filter-community-relation` forked, named in the chain, eBGP source with a resolved peer role, and an iBGP source | Tag written for the eBGP source, none for iBGP (forked twin of `community-relation-tag.ci`), red at HEAD |
| AC-11 | A peer carrying `filter ingress community` or `filter egress community` settings whose matching chain names no `bgp-filter-community` member | Refused at verify and at boot with a message naming the peer, the setting and the chain member to add |
| AC-12 | `bgp-role` forked, peer deleted then re-created under the same name before its OPEN | The re-created peer reads no learned role (forked twin of `role-learned-cleared-on-peer-delete.ci`) |
| AC-13 | `bgp-gr` LLGR ingress bound by `long-lived-stale-time` on the receiving peer, LLGR egress keyed on the source, both transports | Every `llgr-*.ci` and `graceful-restart-llgr.ci` passes unchanged internal; forked twins of `graceful-restart-llgr.ci`, `llgr-readvertise.ci`, `llgr-transition.ci`, `llgr-import-no-llgr.ci`, `llgr-stale-received-depreferenced.ci` and `llgr-readvertise-multipeer.ci` pass; the readvertise rail runs only the LLGR egress step |
| AC-14 | A GR peer goes down with `bgp-gr` and `bgp-rr` forked, then with `bgp-gr` and `bgp-rs` forked, and with mixed placements | The retained families are kept and the others withdrawn exactly as internal (forked twins of `addpath-departure-rr-gr-retained.ci`, `addpath-departure-rr-gr-expiry.ci`, `bgp-rs-withdrawal-replacement-gr.ci`); a DOWN after a NOTIFICATION retains nothing |
| AC-15 | `bgp-gr`, `bgp-rr`, `bgp-rs` started forked | Each starts and runs; no refusal, no `doctor-bgp-gr-out-of-process`; `retention.Publish`, `retention.Family` and the three refusals do not exist |
| AC-16 | `bgp-rs` forked and internal, RS-client destination bound by `session rs-client` | Opaque attributes pass to RS clients as at HEAD (forked twins of `bgp-rs-fastpath.ci`, `bgp-rs-relay-aspath-transparency.ci`, `bgp-rs-rpki-revalidate-relay.ci`); `preserve-opaque-attributes` is a set of the rs filter, not a snapshot fact; `TestPreserveOpaqueAttributesUsesRegisteredSelector` and `forward_role_cache_test.go` changed only with `ze-test-judge` approval |
| AC-17 | `bgp-rs` forked receives `bgp-rib/validation-change` | Its revalidation relay runs as internal (the `bgp-rs-rpki-revalidate-relay.ci` twin), through the SDK subscription |
| AC-18 | Every evaluator moved here | Passes `sdk.CheckFilterConformance` under `-race`, including transport equality |
| AC-19 | After the last row | `filterapi.Filter`, `Register`, `IngressOrdered`, `EgressOrdered`, `ReadvertiseEgressFuncs`, `IngressFilters`, `EgressFilters`, `NotifyPeerRemoved`, `PreserveOpaqueAttributesFor`, `IngressFilterFunc`, `EgressFilterFunc`, `filterapi.SetMetricsRegistry`, `retention.Owner`, `Publish` and `Family` do not exist; `r.egressFilters` and `readvertiseEgressFilters` are gone; grep proves it |
| AC-20 | A plugin restart or respawn under traffic, and a reload that adds or removes a peer's `role` | The step lists are republished; no pass sees a torn list (`-race`); the respawned plugin's slot answers; the binding follows the new config |
| AC-21 | The named interop scenario `bgp-role-otc-withdraw-frr` | Passes with `bgp-role` internal and forked, the OTC wire behaviour unchanged |
| AC-22 | A peer with an RFC 9234 role, in the directions the role obliges today (peer, rs, rs-client both; provider import; customer export) | `bgp-filter-path-asn` inserts its transit-leak member (D-15) with no operator entry; with no transit list configured, verify refuses naming the peer, the role, the chain and the exact config to write; `inactive:` on the member runs the session without it; `validateLeakFilterObligations`, `chainDischarges`, `leakFilterRefusal`, `leakFilterByRole`, `peerRoles`, `declaredRole`, `MustDeclareMatchConditionObligation` and `filterapi.ObligationTransitLeak` do not exist |
| AC-23 | The operator and author pages | `docs/guide/bgp-role.md`, `docs/guide/route-policy.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md` where the filter list is shown, `docs/plugin-development/handlers.md`, `docs/architecture/core-design.md`, `ai/doors/bgp-plugins.md`, `ai/doors/plugin-infra.md`, `ai/patterns/plugin.md` describe auto-insertion, required members and the refusal, each in the commit of the code it describes |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator runs `bgp-role` forked and sets `role import customer` | config binds the procedure, ingress step, batch, OTC verdict | forked twin of `role-otc-ingress-reject.ci` |
| 2 | Operator runs `bgp-filter-community` forked and names it in a peer's import chain with a tag | chain member, community set | forked twin of `community-tag.ci` |
| 3 | Operator runs `bgp-gr` and `bgp-rs` forked on an IXP route server and a member restarts | retention event, rs keeps the retained families | forked twin of `bgp-rs-withdrawal-replacement-gr.ci` |
| 4 | Third-party author binds a filter through the SDK | `sdk.OnFilter` plus bound `FilterDecl`, both transports | `test/plugin/stage-filter-forked.ci` and its internal twin |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBoundFilterDeclarationJudged` | `internal/component/plugin/registry/filter_declaration_test.go` | AC-2 | |
| `TestBoundStepsSortLikeTheRegistry` | `internal/component/bgp/reactor/unified_filter_order_test.go` | AC-3 | |
| `TestBindingFollowsPeerConfig` | `internal/component/bgp/reactor/filter_bound_test.go` | AC-1, group inheritance, reload (AC-20) | |
| `TestBoundStepVerdicts` | same | AC-4 | |
| `TestBoundStepMetadataSets` | same | AC-5 | |
| `TestEveryEgressRailRunsBoundSteps` | same | R-4, six rails, destination- and source-keyed | |
| `TestBoundStepsRepublishOnRespawn` | same | AC-20 under `-race` | |
| `TestChainReferenceToBoundFilterRefused` | `internal/component/bgp/config` | AC-7 | |
| `TestCommunitySettingsNeedChainMember` | `internal/component/bgp/plugins/filter_community` | AC-11 | |
| `TestWithdrawVerdict` | `pkg/plugin/rpc`, `internal/component/bgp/policychain` | the new action, both transports | |
| `TestMetadataFieldRegistration`, `TestSessionFieldReaders` | `internal/component/bgp/policyview` | readers, writers, types; an absent key answers absent | |
| `TestRetentionEventPrecedesDown` | `internal/component/bgp/server` | A-9, both transports | |
| `TestRetentionVerdictConsumedPerDown` | `internal/component/bgp/plugins/rr`, `rs` | R-10, R-11 | |
| conformance per evaluator | `role`, `filter_community`, `gr`, `rs` test files | AC-18 | |
| existing `otc_test.go`, `filter_community_test.go`, `gr_egress_test.go`, `gr_ingress_test.go`, `rfc4724_retention_test.go`, `external_refusal_test.go` | plugin packages | re-pointed at the evaluators and the event; any assertion change or deletion goes to `ze-test-judge` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| binding stage | the declared stage names (Protocol, Policy, Annotation) | Annotation | N/A (named) | PeerChain refused |
| binding priority | 0-9999 | 9999 | -1 | 10000 |
| `stale` metadata value | 0-255 (uint8) | 255 | N/A | 256 refused as a set |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `stage-filter-forked`, `stage-filter-internal`, `stage-filter-unbound-peer` | `test/plugin/` | `fakefilter` binds an ingress filter by a config path; only bound peers are judged | |
| `role-otc-ingress-reject-forked`, `role-otc-egress-stamp-forked`, `role-otc-egress-filter-forked`, `role-otc-rs-client-dest-stamp-forked`, `role-otc-unicast-scope-forked` | `test/plugin/` | AC-6 | |
| `role-learned-cleared-on-peer-delete-forked` | `test/plugin/` | AC-12 | |
| `community-tag-forked`, `community-strip-forked`, `community-scrub-own-ga-forked` | `test/plugin/` | AC-8 | |
| `community-blackhole-noexport-forked`, `rfc7999-blackhole-readvertise-agreement-forked` | `test/plugin/` | AC-9 | |
| `community-relation-tag-forked` | `test/plugin/` | AC-10 | |
| `community-settings-without-member-refused` | `test/reload/` | AC-11 | |
| `graceful-restart-llgr-forked` | `test/parse/` | AC-13 | |
| `llgr-readvertise-forked`, `llgr-transition-forked`, `llgr-import-no-llgr-forked` | `test/plugin/` | AC-13: `bgp-gr` runs forked (their outcomes do not depend on gr's filters) | |
| `llgr-stale-received-depreferenced-forked`, `llgr-readvertise-multipeer-forked` | `test/plugin/` | AC-13: forked `bgp-gr:llgr-ingress` depreferences a received LLGR_STALE route, and `llgr-egress` on the readvertise rail depreferences a stale route toward a non-LLGR iBGP destination (NO_EXPORT and LOCAL_PREF 0) | |
| `addpath-departure-rr-gr-retained-forked`, `addpath-departure-rr-gr-expiry-forked`, `addpath-departure-rr-gr-notification-forked`, `bgp-rs-withdrawal-replacement-gr-forked` | `test/plugin/` | AC-14 (the notification twin: a DOWN after a Cease NOTIFICATION retains nothing with `bgp-gr` and `bgp-rr` forked) | |
| `bgp-rs-fastpath-forked`, `bgp-rs-relay-aspath-transparency-forked`, `bgp-rs-rpki-revalidate-relay-forked` | `test/plugin/` | AC-16, AC-17 | |
| the existing internal tests in Behavior to preserve | `test/plugin/` | unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-role-otc-withdraw-frr` | `test/interop/scenarios/` | FRR | AC-21, OTC on the wire unchanged | |

## Files to Modify
- `pkg/plugin/rpc/types.go`, `enums.go` - `binding` on `FilterDecl`; `withdraw` action
- `internal/core/ipc/yang/ze-plugin-engine.yang`, `ze-plugin-callback.yang` - the binding and the action
- `internal/component/plugin/registry/filter_declaration.go`, `internal/component/plugin/registration.go`, `internal/component/plugin/server/startup.go` - judge and store bindings; deliver `state` to bound plugins
- `internal/component/bgp/filterapi/filterapi.go` - delete the registry and function types (AC-19); keep `LessOrder`, stages, `ModAccumulator`, `EnableRSForwarding`
- `internal/component/bgp/reactor/filter_ordered.go`, `reactor_notify.go`, `reactor.go`, `forward_rs.go`, `reactor_api_forward.go`, `reactor_api_forward_batch.go`, `reactor_api_relay.go`, `reactor_api_batch.go`, `reactor_peers.go`, `peer_forward_facts.go`, `peer_settings.go`, `config.go`, `filter_library.go`
- `internal/component/bgp/config/peers.go`, `register.go` - present `bound-by` paths on PeerSettings; refuse a chain reference to a bound filter; field registrations
- `internal/component/bgp/policychain/external.go`, `sets.go` - metadata sets, withdraw
- `internal/component/bgp/policyview/register.go`, `readers.go`, `fields_json.go`, `view.go` - session facts, treatment field
- `internal/component/bgp/retention/retention.go` - delete `Owner`, `Publish`, `Family`; the `bgp-gr/retention` name and payload type, registered once
- `internal/component/bgp/plugins/role/register.go`, `otc.go`, `role.go`; `filter_community/register.go`, `filter_community.go`, `blackhole.go`, `filter.go`, `metrics.go`; `gr/register.go`, `gr.go`, `gr_egress.go`, `gr_ingress.go`, `gr_state.go`, `doctor.go`; `rr/rr.go`; `rs/register.go`, `server.go`, `server_handlers.go`, `server_validation.go`
- `internal/core/diagnostic/codes.go`, `registry.go` - drop `doctor-bgp-gr-out-of-process`
- `internal/test/plugins/fakefilter/fakefilter.go` - a bound declaration
- `test/plugin/` community `.ci` configs - the chain member (judge)
- `docs/architecture/core-design.md` (Ingress Filter Pipeline), `docs/architecture/api/process-protocol.md`, `docs/architecture/api/architecture.md`, `docs/plugin-development/handlers.md`, `docs/plugin-development/protocol.md`, `docs/architecture/meta/README.md`, `meta/role.md`, `meta/filter-community.md`, `docs/guide/graceful-restart.md`, `docs/guide/plugins.md`, `docs/architecture/plugin/plugin-relationships.md` (Cross-Plugin Bus Dependencies), `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/patterns/plugin.md` "Route Filters"

## Files to Create
- `internal/component/bgp/reactor/filter_bound.go`, `filter_bound_test.go`
- the forked `.ci` twins named in Functional Tests, under `test/plugin/`, and `test/reload/community-settings-without-member-refused.ci`

## Files to Delete (through `./le commit create` `remove`, each judged)
- `test/plugin/gr-external-refuses.ci`, `rr-external-refuses.ci`, `rs-external-refuses.ci`; `gr/external_refusal_test.go` if it tests only the refusal

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-plugin-engine.yang` (binding in the declaration), `ze-plugin-callback.yang` (`withdraw`); no config leaf |
| YANG validation constraints | Yes | stage enumeration, priority range in the declaration schema |
| YANG custom validators | Yes | the verify refusals of AC-7 and AC-11, in `bgp/config` and `filter_community` `OnConfigVerify` plus its `InProcessConfigVerifier`, so `ze config validate` agrees with boot |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no config leaf |
| Functional test for new RPC/API | Yes | `test/plugin/stage-filter-forked.ci`, `addpath-departure-rr-gr-retained-forked.ci` |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | Yes | `doctor-bgp-gr-out-of-process` deleted with the arrangement it reported; no new runtime dependency |
| Prometheus counters/metrics | Yes | filter_community counters keep their names, registered through `Registration.ConfigureMetrics` |
| BGP family surface (new SAFI / capability / attribute) | N-A | OTC already registered by `bgp-role`; only its view field is new |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the `features/` entries for BGP role, community filters, graceful restart and route server gain forked evidence (`./le feature report` names them) |
| 2 | Config syntax changed? | Yes | grammar unchanged; community settings now need the chain member: `docs/guide/plugins.md`, `meta/filter-community.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` (binding, `withdraw`, `bgp-gr/retention`) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/guide/graceful-restart.md` (gr, rr, rs run forked) |
| 6 | Has a user guide page? | Yes | `docs/guide/graceful-restart.md` |
| 7 | Wire format changed? | No | BGP wire unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `process-protocol.md`, `docs/plugin-development/handlers.md`, `protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9234.md`, `rfc9494.md`, `rfc4724.md`, `rfc7947.md`, `rfc7999.md`: forked proof; tags moved with their tests carry discrimination records |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if `fakefilter`'s binding adds an env key |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` Ingress Filter Pipeline; `plugin-relationships.md` bus dependencies |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md`, `meta/role.md`, `meta/filter-community.md` |
| 14 | Prometheus counters added/changed? | No | names unchanged |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the `bgp-gr/retention` event type; the diagnostic code removed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived by `./le spec citation anchors spec plan/post-release/engineering/spec-filter-wire-6-stage-filters.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/plugin-development/handlers.md` filter examples; community config examples |

Discovery (`ai/rules/repo-maintenance.md`): an agent finds the mechanism from the `ai/INDEX.md` filter rows pointing at `core-design.md` and `handlers.md`, and retention from `graceful-restart.md`; `./le plugin declarations check` keeps the two declarations agreeing; the runtime filter declaration store is the one registry; `./le mock plugin-external` and the forked twins prove it.

## Implementation Steps

1. **Phase: Wiring** -- the Stage 1 `filter-insertions` (`rpc.FilterInsertion`, the YANG, `registry.Registration.FilterInsertions`), the judge, the loader's generic insertion step generalised from `prependDefaultFilters` for `config` conditions, the required refusal, `fakefilter` declaring an insertion; `stage-filter-forked.ci` written; the AC-23 pages that describe the mechanism
   - Tests: `TestFilterInsertionDeclarationJudged`, `TestInsertionFollowsPeerConfig`, `TestInsertionsSortByPriority`, `TestRequiredInsertionRefusesOperatorReference`, `stage-filter-forked`, `stage-filter-unbound-peer`
   - Verify: the forked `.ci` passes once the member is inserted, since the chain machinery already calls an external member
2. **Phase: Execution vocabulary** -- `withdraw`; metadata-field sets and reads; session fact and treatment fields registered by their owners; `session` conditions and `scope any-peer`; implicit `state` delivery to a plugin whose member a peer carries; re-insertion on reload and on a runtime declaration change; the transit-leak insertion (D-15) replacing the obligation machinery, its tests through `ze-test-judge`
   - Tests: `TestInsertedMemberVerdicts`, `TestInsertedMemberMetadataSets`, `TestWithdrawVerdict`, `TestSessionConditionInsertion`, `TestTransitLeakInsertedForRole`
   - → Status (walk of 2026-11-02 against the code): in the tree: `withdraw` (`3947f295be`, `ff89fc8110`; `TestWithdrawVerdict` in `policychain/withdraw_test.go`); metadata-field sets and reads (`532a2d08ee`, `342aa7523d`; `policyview/metadata.go` `MustRegisterMetadata`, `TestInsertedMemberMetadataSets`, `TestMetadataFieldRegistration`); session facts as view fields (`76c2b9156a`, `policyview/register.go` `registry.RegisterSessionFact`) and `otc` owned by `bgp-role` (`fbe4e03a6e`); `session` conditions (`fa54cd4285`; `TestSessionConditionInsertion`, `TestSessionConditionInsertionPerDynamicPeer`) and `scope any-peer` (`b5f526f88c`; `TestAnyPeerScopeInsertion`); implicit `state` delivery (`7b1757f99a`); re-insertion on a respawn and a reload stop (`da161efadc`; `TestRestartWithAChangedDeclarationRebuildsThePeers`, `TestReloadStoppingAnInsertingPluginRebuildsThePeers`, `TestInsertionChangeIsSerializedWithTransactions`, `TestRebuildOfTheRunningConfigKeepsRuntimeSubscriptions`); the transit-leak insertion as D-19's `bgp-role:upstream-asn` (`6c753c4ed0`, `655d07b59b`, `5300dac021`, `bea8cfa99b`; `TestUpstreamCheckInsertedPerRole` stands for `TestTransitLeakInsertedForRole`, with `role/upstream_test.go` and `test/reload/role-upstream-check.ci`; none of AC-22's obligation symbols remains); `show policy test` and default-originate judged on the session facts (`d3b2389c77`; `TestPolicyDryRunAnswersTheSessionFacts`, `TestDefaultOriginateFilterReadsTheDestinationFacts`); the `fakefilter` `session-judge` member, inserted on a session condition and run internal and forked (`e1e4564054`; `test/plugin/stage-filter-session-{internal,forked}.ci`); AC-4 for an inserted member (`442c6fc2ea`; `TestInsertedMemberVerdicts` in `bgp/config/filter_insertion_verdict_test.go`: accept, reject, modify, withdraw and each failure under both on-error policies, import and export, answer exactly as the operator-named member's at the compiled chain the reactor rails run; the reactor rails read only that outcome, and their handling of a withdraw is `TestWithdrawVerdictReactorRails`). Left: no treatment field is registered yet, and the one the spec names (`preserve-opaque-attributes`, AC-16) is phase 7's
3. **Phase: `bgp-role`** -- evaluators on `sdk.OnFilter`, bound by `role`, `otc` and metadata fields registered, `state` removed replaces `PeerRemoved`; forked twins red at HEAD then green. One commit
4. **Phase: `bgp-filter-community` and relation** -- chain-member filters; the verify refusal; community `.ci` configs gain the member (judge); BLACKHOLE withholding as a bound procedure per the Q-5 reading; metrics to `Registration.ConfigureMetrics`; forked twins red at HEAD then green. One commit
5. **Phase: Retention on the bus** -- `bgp-gr/retention` registered in `bgp/retention`; gr emits it in its DOWN handler; rr and rs record and consume it; `Owner`, `Publish`, `Family` deleted first; rs's validation-change through the SDK subscription; the A-11 sweep. Refusals not yet removed. One commit
   - → Status (walk of 2026-11-02 against the code): retention on the bus, the deletions, rs and rr validation-change through the SDK and the A-11 sweep are in the tree (`retention.Ledger`, gr `emitRetention`, A-11 row); every forked RIB event and gate command decodes through `plugin/server` (`TestForkedRIBEventsDecodeForEngineSubscribers`, `TestForkedRIBGateCommandsCrossTheServer`). Left: `TestRetentionEventPrecedesDown` in `bgp/server` is not written (its internal transport is writable now; its forked transport needs gr forked, phase 6); the AC-14, AC-15 and AC-17 forked twins and refusal removals belong to phases 6 and 7
6. **Phase: `bgp-gr`** -- both LLGR filters bound (ingress on the receiving peer, egress keyed on the source, readvertise opt-in); `setEgressState` gone; the gr refusal, its doctor check and diagnostic code, and `gr-external-refuses.ci` removed (judge); gr forked twins. One commit
7. **Phase: `bgp-rs` and `bgp-rr` forked** -- rs's transparency filter bound by `session rs-client` setting `preserve-opaque-attributes`; the rr and rs refusals and their `.ci` removed (judge); AC-14, AC-16, AC-17 twins; judge for the two rs unit tests. One commit
8. **Phase: Delete** -- the `filterapi` registry and function types, `r.egressFilters`, `readvertiseEgressFilters`; doors and pages in the same commit (AC-19)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path |
| Correctness | each forked twin observed red (or refused) at HEAD and green after; procedure order identical; readvertise rail runs only opted steps; no bound step on an unbound peer |
| Naming | new fields, the binding and the event payload JSON keys kebab-case; metric names unchanged |
| Data flow | the reactor names no plugin and no plugin config path; bound steps reach plugins only through `CallFilterUpdate`; rr and rs reach gr only through the event |
| Rule: no-layering | no row keeps a `filterapi.Register` beside its plugin filter; `retention.Family` is deleted before the event lands; the registry is deleted, not left empty |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| registry gone | grep `filterapi.Register(`, `IngressOrdered`, `retention.Family`, `retention.Publish` under `internal/` is empty |
| forked twins | the named `.ci` pass, with recorded reds at HEAD |
| conformance | `-race` package tests of role, filter_community, gr, rs, rr |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a forked plugin's binding, metadata set values and types, `withdraw` on an unexpected direction, and a retention payload naming an unparseable family are refused, never applied |
| Fail-open | an unavailable RFC-MUST filter does not silently accept (R-2) |
| Resource exhaustion | a plugin cannot bind unbounded steps (one per declared filter, judged at Stage 1) |
| Operator override | no config form reaches a bound procedure (AC-7) |

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
- The binding question "which peers" is answered by the config the operator already wrote, so no runtime peer list crosses the boundary and nothing the operator can edit reaches a procedure.
- Retention needed no query: the order the engine already guarantees for state events carries a one-way event to the askers ahead of the DOWN they decide on.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `process-global-read-across-the-plugin-boundary.md` | 2026-11-01 | `filterapi.Register` filters of bgp-role and bgp-filter-community read config their forked plugin fills |
| `process-global-read-across-the-plugin-boundary.md` | 2026-11-01 | `bgp-rs` `validationBus` filled only by the in-process `ConfigureEventBus` |
| `process-global-read-across-the-plugin-boundary.md` | 2026-11-01 | `bgp-rr` and `bgp-rs` FlowSpec revalidation reads the lookup table only bgp-rib fills in-process |
| `process-global-read-across-the-plugin-boundary.md` | 2026-11-01 | `bgp-adj-rib-in` and `bgp-rib` validation-change bus handle filled only by `ConfigureEventBus` |
| `process-global-read-across-the-plugin-boundary.md` | 2026-11-01 | Receive eligibility gate `ribevents.RegisterValidationLookup` registered only by an in-daemon bgp-adj-rib-in |
| `second-logger-detaches-runtime-level.md` | 2026-11-02 | Forked BGP plugin logger named by the hyphenated plugin name ignores the dotted `ze.log.<subsystem>` key |
| `process-global-read-across-the-plugin-boundary.md` | 2026-11-01 | Forked receive store verdicts: a forked adj-rib-in with validation enabled fails closed and advertises nothing |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-12 held that the loader reads a third-party plugin's Stage 1 insertions before peers are built | `bgpconfig.declaredInsertions` read only `registry.All()`; `engineStartupSink.onRegistration` stored no insertion (`plugin.PluginRegistration` had no field); the daemon builds its peers from the config before any plugin reaches Stage 1, so a third party's judged insertion never landed. fakefilter's insertion hard-coded `fakefilter:judge`, so a copy forked under another name was refused at Stage 1 | reading `filter_insertion.go`, `server/startup.go` `registrationFromRPC` and `bgp/plugin/register.go` `OnPostStartup` | Stored on `PluginRegistration.FilterInsertions`; the seam `registry.RuntimeFilterInsertions`, answered by `Server.thirdPartyFilterInsertions` (running processes no compiled registration describes); `Reactor.RebuildRunningPeers` before `StartPeers` when any exists; fakefilter's reference follows its process name, and a copy conditions on `fakefilter insert-copy <name>` so the compiled registration's own insertion stays off |
| assumption | A-11 held that only `retention.Family`, `setEgressState` and `validationBus` tie gr, rr or rs to the daemon process | `ribevents.RouteEligible` answered true whenever no lookup was registered in the reading process, and only an in-daemon adj-rib-in registers one, so a forked adj-rib-in left the reactor and a forked bgp-rib forwarding and selecting every path while RPKI and ASPA validation was enabled, with no error | the phase 5 sweep of `ribevents` registrations against their readers | adj-rib-in announces `bgp-rib/validation-gate` on enable and disable (`announcedGateCommand`); the reactor (`subscribeValidationGate`) and bgp-rib (`dispatchText`) apply it with `ribevents.NoteValidationGate`, which fails closed and logs where no lookup is registered; serving the verdicts across the boundary is the owned open row |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 (owner, 2026-11-01): everything is a filter. Every row that used `filterapi.Register` (community, relation, role, gr's two LLGR rows, rs) becomes a plugin filter served on both transports, typed slot internal and batched `filter-update` forked | keep some rows engine-side or as a declaration with no verdict (the earlier draft folded rs into `EnableRSForwarding`) | one mechanism; a row left outside it is the second mechanism D-13 refuses |
| D-2 (owner, 2026-11-01): a filter runs only on peers where it is set up; no every-peer attachment, no runtime peer-declaration RPC. This replaces the draft's Q-2 | (a) every peer, the evaluator accepting unconfigured peers; (b) a runtime RPC through which the plugin declares its peers | (a) puts a forked round trip on every route of every peer; (b) adds an RPC and a reload reconcile for a fact the config already states |
| D-3 (owner, 2026-11-01): operator policy (`bgp-filter-community` tag, strip and scrub; the community relation tag) is an ordinary chain member the operator names in the peer's `filter import`/`export` chain, run only where named | keep them stage-attached at Policy 0 and Annotation 1 | policy belongs to the operator's chain; settings without the member are refused at verify (AC-11), so the move cannot become a silent no-op |
| D-4 (owner, 2026-11-01, revised the same day): a procedure a feature's config implies (`bgp-role` OTC, RFC 9234 Section 5; the transit-leak check a role brings, D-15; `bgp-gr` LLGR ingress and egress, RFC 9494; `bgp-rs` transparency) is a member of the peer's filter list, auto-inserted by its own plugin when the peer's config needs it, visible like any member; a required one refuses an operator edit that names, moves or deactivates it (`ai/META-DECISIONS.md`, "Goal and scope": "Every route-processing step is a filter in the peer's filter list."). Owner's words: "The role configuration should auto-insert a filter in the filter list if required. That's how it should be working." | a bound step beside the chain (the earlier D-5, withdrawn by the owner) | one list holds every step, so show config and dry-run show what runs; RFC 9234 Section 5 is met by refusing the operator's reference, not by hiding the member |
| D-5 (owner, 2026-11-01): the insertion mechanism, generalised from `prependDefaultFilters`. The Stage 1 registration carries `filter-insertions`, each an `rpc.FilterInsertion`: `filter` (a reference the declaring plugin serves: `<plugin>:<filter>` of a filter it declares, or `<type>:<instance>` of a filter type it registers), `direction` (`import` or `export`, one), `priority` (0-9999), exactly one condition, `config` (a path of words under a peer's resolved entry: present, or with `values`, equal to one of them) or `session` (a peer fact some owner registered, D-9), and `required` with its `reason` (the text verify prints). The loader, after the chains are canonical, prepends each insertion whose condition the peer's resolved config meets, ahead of the operator's members in ascending priority then reference, unless the operator named that reference (plain or `inactive:`), the presence rule `filterChainContains` already applies to loop-detection. A non-required member is placed by naming it and switched off by `inactive:`, as loop-detection is. A required member named by the operator in any form is refused at verify with its reason, naming the peer, the chain and the reference. A negotiated capability is not a load-time fact: the capability Ze advertises is the config that makes Ze advertise it (`long-lived-stale-time` for LLGR: "When this leaf is set, Ze advertises the LLGR capability"), so a capability condition is a `config` condition, and the per-session negotiated fact stays the evaluator's declared read (`llgr-advertised`, the learned role). A source-keyed procedure (LLGR egress) takes `scope any-peer`: the condition is judged on any peer, and the member goes on every peer's export list. The insertions, each a value in its own plugin's declaration function and none known to the engine (D-5a): `bgp-role` where `role` is present (import and export, required); `bgp-gr` where `long-lived-stale-time` is set (import on that peer; export `scope any-peer`), because RFC 9494 Section 4.3 obliges a speaker "that has advertised the Long-Lived Graceful Restart Capability to a neighbor" for routes from that neighbor; `bgp-rs` where `session rs-client` is set (export); `bgp-filter-path-asn` by `role import` values (D-15) | (a) a bound step beside the chain (withdrawn by the owner); (b) a `ze:` YANG extension on the plugin's augment naming the filter | (a) is the owner's withdrawn option; (b) works only for compiled YANG and splits the declaration from Stage 1. D-5 keeps one Stage 1 declaration a forked and a compiled-in plugin make the same way, judged against the resolved config the loader already builds with group inheritance |
| D-5a (owner, 2026-11-01, revised the same day for D-5): the engine and config code never name a feature, a plugin or a config path (`ai/META-DECISIONS.md`, "Goal and scope": "The plugin declares the insertion; the engine and config code never name a feature, a plugin or a config path to make it happen."). Each insertion is a value in its plugin's own declaration function and the loader judges each one generically. The literals that break this today (`leakFilterByRole`, `declaredRole`, `peerRoles` naming `role` in `bgp/config/peers.go`) go with D-15; the roleless-peer warning and its doctor seam (`rolelessPeers`, `rolelessPeersFromTree`) also read `role` in config code and move to `bgp-role` in phase 3. The text below predates D-5's revision; read "binding" as "insertion". Original: the engine never names which filter binds to which peer feature. The binding of a protocol-procedure filter is DECLARED by the plugin that owns it, in the one Stage 1 `FilterDecl` both transports read (and its `registry.Registration` twin): the condition (`bound-by`: a path or leaf of the peer's config the plugin names, or `bound-by-capability`: a capability code the plugin names, advertised or negotiated on the session), its stage and priority, and `operator-editable: false`, implied by every binding. The engine keeps a registry of declared bindings (the runtime filter declaration store) and evaluates each one generically against `PeerSettings` and the session's capability facts. No switch, field list, path literal or plugin name for `role`, LLGR, community, gr or rs exists in `reactor`, `bgp/config`, `policyview` or `plugin/server`; the bindings D-5 lists are values in each plugin's own `filterDecls()`. Asked of every piece: a new plugin that declares a binding, with nobody editing the engine, is bound and run. The same holds for the view fields the filters read (each owner registers its field, D-9) and for retention (D-11), which uses only the generic `emit-event`, subscription and sequential state delivery: the engine has no gr-, rr- or rs-specific path | the engine maps known features to filters (a table keyed by `role`, `long-lived-stale-time`, `rs-client`) | a central map is a second declaration of what the plugin already owns and refuses a new plugin until edited (`ai/rules/principles.md`, registration over enumeration). `./le arch enumeration check` and the grep row of Architectural Verification prove the absence |
| D-6: community and relation leave the stage list and run inside the chain at the operator's position, after the bound procedures | keep them at Policy 0 and Annotation 1 as engine-placed steps | D-3 makes them chain members; role reads no community, and relation, which reads role's `src-peer-role`, still runs after role. The order tests that pinned them change through `ze-test-judge` with D-3 as the source |
| D-7: reactor facts cross as declared fields of the existing view, not as new header members of `FilterUpdateInput` | widen the header with every `PeerFilterInfo` field | fields are encoded only when declared (D-10), the forked message stays small, and a third party reads facts by the same vocabulary as route fields |
| D-8: route metadata keys become fields with readers and settable writers registered by the key's owner; a metadata set is visible to later ingress steps of the same pass and on egress | a `meta` member in the verdict; per-step maps | one answer shape (sets), judged by the declared-sets rule; preserves role writing `src-peer-role` for relation |
| D-9 (Q-4, settled by rule): BGP's `carries`/`settable` lists become registrations each field's owner makes, in this spec | add the new fields to the hand-written lists | `ai/rules/principles.md`: a feature registers and is discovered; a central list is a second declaration |
| D-10 (Q-1, settled by rule): any plugin the operator configured may declare a bound filter, a third-party one included | only compiled-in plugins | the in-tree plugins hold no privilege on the protocol (`ai/doors/bgp-plugins.md`); a third-party binding binds only where its own path is configured, and A-10 states the compiled-schema limit |
| D-11 (owner Q-3 = (b), 2026-11-01): `bgp-gr` and `bgp-rs` become real filters, forked included, and this spec moves retention onto the bus. `retention.Owner`, `Publish` and `Family` are deleted. `internal/component/bgp/retention` keeps the event's identity: namespace `bgp-gr`, type `retention`, payload {`peer-address`, `families` retained, as `<afi>/<safi>`}, registered once with `events.Register[T]`. gr, in its DOWN handler, emits the event for every DOWN it handles (an empty `families` after a NOTIFICATION or with no GR capability) before it returns. The engine delivers a state event sequentially in reverse dependency order and waits for each process's result (`deliverPeerState`), and every delivery to a process is FIFO on its one `eventChan` (`process.Deliver`, `server.deliverEvent`), so rr and rs have the event queued before their DOWN. On DOWN they consume and clear the recorded verdict for that peer, synchronously, as `handleStateDown` does today; no verdict means gr holds no state for the peer, the answer `Family` gave with no owner. The event carries no peer address, so it is not peer-scoped and needs no `attach` grant. The three refusals, `doctor-bgp-gr-out-of-process` and the three `*-external-refuses.ci` go | (a) keep retention in-process and prove gr internal only (the draft's Q-3 (a)); (b) rr and rs query gr through a text command on DOWN; (c) the engine attaches gr's answer to the DOWN it delivers next | (a) is the owner's rejected option. (b) is a request and response for a fact that is already settled when gr returns, and adds a dispatch round trip inside the DOWN handler. (c) puts GR knowledge in the engine, which holds no policy or GR state (`ai/doors/bgp-engine.md`). The event reuses the order the engine already guarantees and the bus both transports already have |
| D-12: rs's filter is RFC 7947 Section 2.2 transparency toward RS-client destinations: it answers accept with the treatment field `preserve-opaque-attributes` set, which replaces the per-snapshot `facts.preserveOpaqueAttributes` in the dedup identity and the body cache key | derive the treatment in the reactor from `RSClient` (the earlier draft) | D-1 makes rs a filter; the treatment then has one producer, rs, on both transports |
| D-13: rs's `bgp-rib/validation-change` subscription moves from the in-process `ConfigureEventBus` handle to the SDK subscription | leave it in-process | D-11 makes rs run forked, where the handle is nil and the subscription is skipped with no log line |
| D-14: `bgp-gr-llgr-ingress` is a plugin filter reading `llgr-advertised`, not engine code | engine-side like `loop` | `loop` is RFC 4271/4456 for every session; LLGR is gr's feature, and removing gr must remove it (`ai/rules/plugins.md`) |

| D-15 (owner, 2026-11-01): the RFC 7454 Section 9 transit-leak check is auto-inserted too. Owner's words: "the transit-leak check should be auto-inserted too." `bgp-filter-path-asn`, which owns the `reject-asn` list and today declares the obligation (`policy.MustDeclareMatchConditionObligation`), declares two insertions of `reject-asn:transit-leak`: import where the peer's `role import` is `peer`, `provider`, `rs` or `rs-client`, export where it is `peer`, `customer`, `rs` or `rs-client` (the directions `leakFilterByRole` holds today). The obligation machinery (`validateLeakFilterObligations`, `chainDischarges`, `leakFilterRefusal`, `leakFilterByRole`, `peerRoles`, `declaredRole`, `MustDeclareMatchConditionObligation`, `filterapi.ObligationTransitLeak`) is deleted first. Derivation (1), the list: the transit ASNs are operator data, a `policy reject-asn <name>` instance (`ze-filter-path-asn.yang`); the curated transit-free table (`curated.go`) only feeds completion and constrains nothing, so no default list exists. The insertion names a list by a declared name, `transit-leak` (Q-6); when the config holds no such instance, verify refuses naming the peer, the role, the chain and the config to write (`policy { reject-asn transit-leak { indirect [ <ASN> ] } }`), or the opt-out. Derivation (2), required: no. RFC 7454 is a Best Current Practice and Section 9 opens "This section lists the RECOMMENDED practices when processing BGP AS paths." with SHOULD bullets ("Network administrators SHOULD NOT accept prefixes when the first AS number in the AS path is not the one of the peer's unless the peering is done toward a BGP route server"), so the operator removes it explicitly with `inactive:reject-asn:transit-leak` in that chain: the config file records the decision, the insertion stops, and nothing runs | keep refusing a role peer with no discharging match | the owner's rule makes the procedure a member; the refusal asked the operator to write what the plugin can insert |
| D-16 (owner, 2026-11-01, replaces Q-6 and D-15 derivation (1)): the transit ASNs the RFC 7454 Section 9 check compares are DERIVED from config, not read from a named `reject-asn` list (a `reject-asn` list is a position rule, not data). Owner's words: "we should know which transit we have, because they should be defined on the other peers with the role, so those should be added automatically." The derived set, the default and usually enough, is the remote ASN of every configured peer whose RFC 9234 role makes it Ze's provider (Ze's local role toward it is `customer`; read `role.go` for the pairs and quote RFC 9234 Sections 3 and 5). `role { transit-asns <asn-set>; }` is OPTIONAL and exists for transit providers this router does not peer with (for example a provider reached through another router of the network); the check uses the union of both. It names a `policy { asn-set NAME [ asn ... ]; }`, a new plain ASN-set data type (no positions, no action), registered as a policy data type by the package that owns ASN sets, never hand-listed in engine code, with `ze:reference` so `transit-asns` completes and validates. The leaf is declared in the YANG of the plugin that owns the insertion (augmenting `role`, inherited from the group like the rest of `role`), and its YANG description and the operator guide say exactly what this row says about the default and the union. The inserted check, on the role sessions RFC 7454 Section 9 and RFC 9234 make it apply to (customer, peer, RS-client; derive from the text), rejects a route whose AS path carries a set ASN at the positions RFC 7454 Section 9 implies (quote `rfc/full/rfc7454.txt` Section 9 and report the positions chosen), except on the session to that provider itself. The plugin computes the set from the peer configs it already reads (no engine code naming role) and recomputes it on reload when a provider peer is added or removed, as a live swap with no session bounce, like a definition reload. Whether a role peer with no provider peer and no `transit-asns` is refused at verify (with the exact lines to add) or gets no member is decided from the RFC text and reported. No user-facing name says "transit-leak". Owed: unit tests, a `.ci` (add a provider peer: a customer's route through the provider's ASN is rejected; remove the provider on reload: it is accepted), the repo configs that relied on the old refusal or the `inactive:` opt-out converted (judged), and the pages | a named `reject-asn transit-leak` instance (the withdrawn Q-6 reading); `role { transit-asns <reject-asn list> }` (withdrawn the same day) | the operator already told Ze who its providers are by configuring their roles; a second list would be a second declaration of that fact |
| D-18 (phase 1, 2026-11-01, follows D-5a and A-10): an insertion's `config` path, continued from a peer's entry (`rpc.FilterInsertionSchemaPath`), MUST be a node of the compiled schema, judged at Stage 1 and at config load (`config.CheckFilterInsertionPaths`). For a COMPILED-IN registration the node its first word names MUST also be defined by the plugin's own YANG module (`config.SchemaNodeModule` against `config.YANGModuleName(reg.YANG)`), so a compiled plugin inserts only where config it owns is set, and the loader, which reads every compiled registration whether or not it runs, cannot insert a member of a plugin no config of its own asked for. A THIRD-PARTY insertion (no compiled registration) is scoped by the operator's `plugin external` entry, and its own YANG is not compiled, so it may name any compiled path | condition on any compiled path for every plugin | a compiled plugin conditioned on `ze-bgp-conf` config would insert its member on every such peer while the plugin may not run at all |
| D-17 (owner, 2026-11-01, answers Q-7: no): a `reject-asn` member or a `reject-asn` match condition the operator wrote does not stand in for the inserted check; it may stay in the chain beside it. The choice lives only in the inserting plugin's declaration | a `satisfied-by` field on the insertion | the inserted check compares the derived provider set, which a hand-written list does not track |
| D-19 (phase 2, 2026-11-01, derives D-16 from the RFC text): the check is `bgp-role:upstream-asn`, a filter `bgp-role` declares and evaluates (`role/upstream.go`, one `sdk.OnFilter` evaluator on both transports), inserted not-required by `upstreamInsertions` on `role import`. Sessions: import where the local role is `provider` (RFC 7454 Section 9: "filters for specific 2-byte or 4-byte AS paths that must not be accepted if advertised by the customer, such as upstream transit providers or peer ASNs"), `peer` and `rs` (RFC 9234 Section 3.1: "Peer:  MAY propagate any route learned from a Customer, or that is locally originated, to a Peer.  All other routes MUST NOT be propagated.", the same of an RS-Client); export where it is `peer` or `rs-client` (RFC 7454 Section 9: "Network administrators SHOULD NOT advertise prefixes with upstream AS numbers in the AS path to their peering AS unless they intend to provide transit for these prefixes."); never on a `customer` session (the provider's own), and the session's own remote ASN is never compared. Positions: any ASN of the AS path, AS_SET members included (the text says "containing" and names no position). Set: one router-wide union of the remote ASN of every peer whose `role import` is `customer` (RFC 9234 Section 3.1: "Customer:  the local AS is a transit customer of the remote AS;") and every asn-set any `transit-asns` names; a peer with no configured remote ASN (a dynamic group member) adds none. Empty set: no verify refusal, and the member rejects nothing, because Section 9 compares against "upstream transit providers" and a network with none (transit-free, a route server) has nothing that can leak, so a refusal would refuse a correct config. `policy asn-set NAME { asn [ ... ]; }` lives in `bgp-role`'s own module `ze-role-asn-set.yang` (its only reader; a policy list without `ze:filter`, so a chain naming one is refused for what it is) with the `policy-asn-set` reference source; the bracket leaf-list under a keyed entry is the YANG form of the owner's `asn-set NAME [ asn ... ]`. Reload: the set is part of `roleConfig`, so a bgp or policy edit installs a new snapshot (`installUpstreamSet`) and no chain or session changes. The required OTC member (D-4) is phase 3's: OTC is still a `filterapi` stage filter, not a chain member, so no member exists to refuse an edit of until phase 3 moves it onto `sdk.OnFilter` | a named `reject-asn` list (D-15, withdrawn); a per-session set | the derived set is the fact the operator already configured, and a provider is a router-wide fact |

## Open Design Questions (for Thomas)

| ID | Question | Options | What it changes |
|----|----------|---------|-----------------|
| Q-5 | RFC 7999 Section 3.1: "In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it." The withholding (`blackholeReadvertiseRefused`) must therefore run toward every bilateral eBGP destination WITHOUT an agreement, and the agreement is `bgp-rib`'s `blackhole` container. No config-presence binding reaches "peers lacking the config", so D-2 and this MUST pull apart for this one procedure | (a) **the reading this spec proceeds under**: a bound procedure of `bgp-filter-community` declaring `bound-by-session` on the session facts the BGP view registers (`ebgp` true, `rs-client` false), set by every bilateral eBGP peer's own session config, evaluated generically like every other binding (D-5a); (b) make it an operator chain member and accept that a peer whose operator did not name it receives BLACKHOLE unagreed; (c) move the withholding to the `bgp-rib` feature that owns the agreement, bound where `blackhole` is configured on the source, and accept the same leak for unconfigured destinations | (a) keeps the shipped RFC behaviour and is the one binding that covers peers by session kind rather than by a feature path. (b) and (c) open an RFC 7999 MUST leak that a fix cannot ship (`ai/rules/rfc-compliance.md`). Only phase 4's BLACKHOLE step and AC-9 depend on the answer |
Q-6 and Q-7 are answered by the owner: D-16 replaces Q-6, D-17 answers Q-7 (Key Design Decisions).

Q-5 is answered (main thread, 2026-11-01): reading (a) stands on the RFC 7999 Section 3.1 text quoted in its row; under D-5 it is a `session`-condition insertion (`ebgp` true, `rs-client` false) of the withholding member on export.

## Known Limitations

- Forked metrics (A-5): if a forked plugin's counters reach no registry today, that is a plugin-infrastructure limit for every plugin, owed by a spec of its own if A-5 holds; this spec does not hide it.
- A third-party plugin can bind only by a schema path the daemon has compiled (A-10), because an external plugin's own YANG is not compiled yet (`docs/plugin-development/schema.md`).

## RFC Documentation (Scope: protocol)
Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The moved evaluators keep their RFC 9234 Section 5, RFC 9494 Sections 4.3 and 4.6, RFC 7999 Section 3.1, RFC 8195 Section 3.2 and RFC 7947 Section 2.2 quotes; the retention emit and consume keep RFC 4724 Section 4.2; each new call site names its RFC and section.

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
- [ ] AC-1..AC-21 all demonstrated
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
