# Spec: bgp-live-receive-binding-and-bfd-keep-session

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/6 |
| Progress | Phases 1 to 3 committed; AC-8 `.ci` green 10/10 with its discrimination red 3/3 (2026-10-11); Phase 4 next, now carrying AC-25 (owner decision of 2026-10-11). Gates 2026-10-12: scoped `./le go lint run` (reactor, bgp/config, rib, rr, rs, watchdog, redistribute_egress, test/runner) 64 findings, 2 from this spec (unparam `onStateUp`, `establishDynamicPeer`) fixed, 62 blame to other commits; `./le test bgp reload -a` 59/65, reds `bgp-bfd-added-live`, `bgp-bfd-dynamic-toggle-live`, `bgp-bfd-profile-edit-live` (Phase 4 entry tests, not built) and `config-apply-ordering-coarse-root`, `config-apply-ordering-mixed-rollback`, `rsvpte-reload` (darwin: interface plugin has no backend, journaled in `plan/journal/gate-verdict-depends-on-the-machine.md`); reactor `go test -race` with gate tags 20/20 green (runs of 5, 4, 6, 5) |
| Handoff | - |
| Updated | 2026-10-11 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope: protocol. The change is to when Ze ends a BGP session (a NOTIFICATION
     Cease the peer sees, or its absence) and when it sends ROUTE-REFRESH, so it is
     wire-visible and owes an interop test. Bucket plan/immediate/: an operator who
     enables RPKI or BFD on a live peer meets a session reset as a bug. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Enabling RPKI or BFD on an ESTABLISHED BGP peer resets that peer's session in Ze
today. FRR and BIRD keep the session up for both changes. The owner decided on
2026-10-11, answering D-6 and D-8 of `plan/spec-terminal-demo-showcase.md`, in
these words: "yes like them". Ze keeps the session up, like FRR and BIRD do.

Measured 2026-10-10 (showcase spec, D-6 and D-8):

| Change committed live | Today | Goal |
|-----------------------|-------|------|
| `attach process bgp-rpki { receive [ update-received state ] }` added to a running peer | Cease subcode 6, the peer re-opens: two TCP connections | one TCP connection, routes received before the commit carry their RFC 6811 states |
| `connection bfd { profile ... }` (non-strict) added to a running peer, with a new `bfd` root | Cease subcode 6, the BFD session opens on the SECOND Established | one TCP connection, the BFD session opens on the running session |

The goal, stated as the rule the reload follows afterwards:

1. An `attach process` binding that grants the process NO send type (no
   `send` list, no `send [ * ]`) is a receive-only binding, whether the operator
   typed it or the config builder derived it. Gaining, losing or changing one is
   delivered to the running session. The process that gains the feed is told the
   session is up, by an event that starts no initial routing update and arms no
   End-of-RIB barrier, before any message of that session reaches it. It is then
   brought level with the routes the session already carried, in each direction
   its new grant covers: routes the peer sent are asked for again (ROUTE-REFRESH
   when the peer supports it, restart otherwise); routes Ze sent cannot be asked
   for again, so the binding swaps only while Ze has advertised nothing on the
   session (`Peer.hasAdvertised`) and restarts otherwise. A grant that carries no
   route (`state`, `negotiated`, `keepalive`, `notification`, `refresh` and
   nothing else) needs no catch-up at all. The process that loses the feed is
   told the peer is down for it alone.
2. A binding that grants a send type still restarts, because the process can put
   routes into the session's initial routing update and the End-of-RIB barrier
   counts it (`ProcessBinding.MayPushRoutes`, `Peer.resetAPISync`).
3. A receive grant that includes the OPEN exchange (`open` in any direction, or the
   `*` wildcard) still restarts: only a new session produces an OPEN the process
   can be shown.
4. A non-strict BFD opt-in (`connection bfd` with `strict false`) is added, removed,
   suspended, resumed or edited on the running session of a configured peer: the
   BFD client opens or closes, and the BGP session stays up. The same holds for a
   dynamic peer whose group template gains, loses or edits the opt-in. The owner
   decided on 2026-10-11, answering OQ-1, in these words: "A: yes, design it".
   A dynamic peer therefore goes through the same swap as a configured peer: the
   same plan, the same copier, the same ordered delivery, and the same reload
   journal, so a reload the journal rolls back undoes a dynamic swap (Phase 3).
   The owner decided on 2026-10-11, answering OQ-2, in these words: "A". The
   dynamic peer therefore swaps the whole hot-swappable set a configured peer
   swaps: filter chains, `PrefixUpdated`, static routes (written to the wire as
   UPDATEs by `deliverStaticRouteDelta`), and `Capabilities` when the
   negotiation outcome is unchanged (`hotSwappableWithCapabilities`). An
   import-chain swap filters the routes received after the reload; routes
   already accepted are not re-filtered (R-21).
5. A change that involves strict mode on either side still restarts, because
   strict mode changes the OPEN (capability 74) and the OpenConfirm gate
   (draft-ietf-idr-bgp-bfd-strict-mode Sections 6 and 8).
6. Everything else stays as it is: the swap set is a subtraction from the whole
   `PeerSettings` struct, and an unclassified field still restarts.
7. A BFD client that releases a shared BFD session takes its own narrowing of the
   session's identity with it, so the next client for the same remote system
   shares the one session (RFC 5882 Section 4.4). The owner decided on
   2026-10-11 to fold this RFC5882-4.4-1 defect into this spec, in these words:
   "fold it into keep-session" (AC-24).

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/core-design.md` section 18a "BGP Peer Reload: Swap or Restart" - documents today's rule this spec changes
  → Constraint: the swap set is a SUBTRACTION from the whole struct (`hotSwappableSettings`); the new members join it, nothing is turned into an enumeration of what matters. A field added tomorrow still restarts.
  → Constraint: the decision and the apply share one copier (`settingsCopier`), so a field judged swappable is always delivered. The BFD swap and the receive-only swap MUST travel through the same copier, never through a second list beside it.
  → Decision: the catch-up table (no route announced: nothing; Route Refresh advertised: one ROUTE-REFRESH per negotiated family; otherwise restart) is reused unchanged for an operator receive-only binding.
- [ ] `docs/guide/config-reload.md` - the operator page; its "Peer settings changed: Session restarted" row becomes wrong for these changes
  → Constraint: the page gains rows for "receive-only attach added/removed/changed" and "BFD opt-in (non-strict) added/removed/changed" with "session stays up", and keeps "session restarted" for a send grant, an `open` grant and strict mode.
- [ ] `docs/guide/bfd.md` (around the "Established, the reactor calls ... EnsureSession" paragraph, line 195) - when a non-strict peer opens its BFD session
  → Constraint: today a non-strict peer opens BFD only on entry to Established (`peer_run.go`, the FSM callback). The live swap opens it on an ALREADY Established peer through the same `startBFDClient`, so the page states both entry points.
- [ ] `docs/guide/rpki.md` (line 241 binding paragraph, line 597 troubleshooting row) - how an operator binds bgp-rpki
  → Constraint: the troubleshooting fix "Add `attach process rpki ...`" now takes effect without a session reset; the page says so.
- [ ] `docs/architecture/testing/interop.md` "Live RPKI policy reload" and "Scenario Structure" - continuity proof pattern
  → Decision: session continuity is proven with FRR's `connectionsEstablished` counter (`queryFRRSessionGeneration`, `internal/le/interoplab/bgp/check_rfc.go`) read before and after a SIGHUP reload through `ze-reload.conf` (`prepare.go`), as the `rtr-stayrtr` checker does.
  → Constraint: every scenario has one typed or bespoke checker in `internal/le/interoplab/bgp`; a bespoke body MUST NOT call `checkScenario`; the scenario directory is named, no numeric prefix.
- [ ] `docs/architecture/api/ipc_protocol.md` (declared by `pkg/plugin/rpc/enums.go`) - "State events" (line 204: only down events carry a reason) and the text format line (line 1182)
  → Decision: edited in Phase 2: an up event can carry reason `attached`, sent when a process is attached to a session that was already up; both the JSON and the text format paragraphs say so.
- [ ] `docs/architecture/plugin/rib-storage-design.md` (declared by `adj_rib_in/rib.go`) - peer-up replay and the initial token (lines 102 to 114)
  → Decision: edited in Phase 2: an `attached` up carries no initial token and triggers no peer-up replay.
- [ ] `docs/architecture/behavior/fsm.md` (declared by `session_bfd_strict.go`) - the six BFD strict-mode events
  → Decision: no edit. No FSM event or transition changes: Event 35 is still raised only on the restart branch, and moving the opt-in reads behind an accessor changes no behaviour the page describes.
- [ ] `docs/architecture/config/apply-ordering.md` (declared by `loader_create.go`, `reactor/operation.go`, `reactor.go`) - "`verify -> execute -> commit` with rollback, not best-effort apply" and "A modify-peer swaps the running session's settings in place"
  → Decision: edited in Phase 3: verify parses the candidate's dynamic groups and applies none of them; the dynamic population is reconciled by the first journaled step of the BGP apply, a dynamic peer's template change swaps in place under the modify-peer rule, and the step's inverse restores the previous group set and undoes each dynamic swap, while a dynamic peer the step removed comes back only when it reconnects (R-15). The page's rule "A remove-peer's inverse restores the peer the reactor was running" names a dynamic peer as the stated exception, with the reason: a dynamic peer is passive by construction, so a rebuilt idle peer and the next accepted connection differ only in `show` and `ActivePeers` (review r2 N4).
- [ ] `docs/research/bfd-implementation-guide.md` (declared by `bfd/engine/engine.go`) - "Shared Sessions and Refcounting"
  → Decision: no edit. The page describes coalescing requests for the same key into one refcounted session, and the session continuing when one client deregisters; it says nothing about the narrowed `joined` key. Phase 4 step 1 makes the engine match that lifecycle (a client leaving takes its narrowing with it). Re-checked in Phase 4 against the anchor run of Documentation row 16.
- [ ] `docs/architecture/config/syntax.md` (declared by `reactor_dynamic.go`, `reactor_api.go`) - the paragraph on a group's blocks reaching the peers a dynamic group creates (source anchor `buildDynamicPeerSettings`)
  → Decision: no edit expected: the page describes how a dynamic peer inherits its group's blocks, which `buildDynamicPeerSettings` keeps doing; it makes no claim about reload. Re-checked in Phase 3 against the anchor run of Documentation row 16.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6811.md` - origin validation on newly available data
  → Constraint: RFC 6811 Section 4 (read in `rfc/full/rfc6811.txt`): "When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed." The RFC asks for re-validation, never for a session reset; FRR implements it by re-running `bgp_update` over `adj_in` (`revalidate_bgp_node`, `bgpd/bgp_rpki.c`).
- [ ] `rfc/short/rfc2918.md` - ROUTE-REFRESH as the catch-up for a gained consumer
  → Constraint: RFC 2918 Section 4: "A BGP speaker may send a ROUTE-REFRESH message to its peer only if it has received the Route Refresh Capability from its peer." Without it, a gained consumer of a peer that already announced routes still restarts (`feedCatchUpImpossible`).
- [ ] `rfc/short/rfc5882.md` - BFD client behaviour on configuration and deconfiguration
  → Constraint: RFC 5882 Section 8 (read in `rfc/full/rfc5882.txt`): "the system on which BFD is being deconfigured SHOULD put the session into AdminDown state and maintain this state for a Detection Time to ensure that the remote system is aware of it." Removing the opt-in live goes through `stopBFDClient` -> `ReleaseSession`, the path the Established exit already uses; the spec does not change what the BFD plugin sends on release (A-5).
- [ ] `rfc/short/rfc5880.md` - BFD timer changes on a live session (AC-25)
  → Constraint: RFC 5880 Section 6.8.3 (read in `rfc/full/rfc5880.txt`): "If either bfd.DesiredMinTxInterval is changed or bfd.RequiredMinRxInterval is changed, a Poll Sequence MUST be initiated (see section 6.5)." A profile timer edit applied in place (Phase 4 step 3) goes through the machine's existing reconfigure path, which owes that Poll; the summary lists RFC5880-6.8.3-4 as an open gap, which Phase 4 step 3 reads before relying on that path.
- [ ] `rfc/short/rfc4486.md` - Cease subcode 6 "Other Configuration Change"
  → Constraint: the restart branch keeps sending subcode 6; only the set of changes that reach it shrinks.

### Peer implementations (read at the source, 2026-10-11)
- [ ] FRR `bgpd/bgpd.c` `peer_on_policy_change` (FRR commit cd39d029) - what FRR does when an inbound policy changes on an Established peer
  → Decision: FRR never resets for it. Outbound: `update_group_adjust_peer` then `bgp_announce_route`. Inbound: returns if not established, tries `bgp_soft_reconfig_in`, and otherwise, when the peer advertised the Route Refresh capability (`PEER_CAP_REFRESH_RCV`), sends a normal ROUTE-REFRESH for that AFI/SAFI with `bgp_route_refresh_send`. Ze's equivalent of soft-reconfig-in is a consumer that already retains routes; Ze's ROUTE-REFRESH branch is `requestRouteRefreshAll`.
- [ ] FRR `bgpd/bgp_bfd.c` `bgp_peer_configure_bfd` and `bgp_peer_config_apply` - FRR enabling BFD on a peer
  → Decision: FRR allocates `p->bfd_config`, creates the session with `bfd_sess_new(bfd_session_status_update, p)`, and `bgp_peer_config_apply` ends in `bfd_sess_install`; no peer reset anywhere on this path.
  → Constraint: FRR's `bfd_session_status_update` acts on Down only when the session's previous state was Up: a freshly installed session that has never been Up does not end the BGP session. Ze's start-on-Established path has the same property today (the bfd-opt-in tests pass), and the live start inherits it (A-3).
- [ ] BIRD `nest/proto.c` `channel_reconfigure` (BIRD commit 02d082a7) - BIRD's reload decision per channel
  → Decision: BIRD reloads rather than restarts: when the import side changed and the channel is not reloadable, the function reports failure and the protocol restarts; otherwise it requests a channel reload (`channel_request_reload`). The same function re-subscribes ROA tables when `rpki_reload` changes. Ze's analogue: catch-up by ROUTE-REFRESH, restart only when the peer cannot re-send.
- [ ] BIRD `proto/bgp/bgp.c` `bgp_reconfigure` and `bgp_update_bfd`; `proto/bgp/bgp.h` `struct bgp_config`
  → Decision: `bfd` is the last field of `struct bgp_config`, after `password`, and the byte comparison that decides whether the configuration is the same stops at the `password` offset, so it never sees `bfd`. A BFD change therefore keeps the session, and `bgp_update_bfd` requests, updates or frees the BFD session in place.
  → Decision (corrected 2026-10-11, review r2 I5, re-read at BIRD 02d082a7): BIRD DOES run BFD for a dynamic neighbor and keeps its session across a BFD or filter template change. `bgp_is_dynamic` (`proto/bgp/bgp.h`) is `ipa_zero(p->remote_ip)`, true only for the listening parent: `bgp_spawn` (`proto/bgp/bgp.c`) sets `cf->remote_ip` on each spawned child, so the `!bgp_is_dynamic(p)` guard in `bgp_update_bfd` passes for every child and requests its BFD session. On reconfigure `protos_commit` (`nest/proto.c`) clones each spawned child's config from the new parent and calls `proto_reconfigure`; `bgp_reconfigure` copies the child's `remote_ip` and `local_ip` into the clone before the byte comparison, so a BFD-only edit compares equal and a filter edit goes through `channel_reconfigure`; a child that cannot reconfigure is removed (`proto_undef_clone`, then `goto remove`), not restarted. This supports both owner answers (OQ-1 and OQ-2, option A each).

### Source Code (current behaviour)
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `hotSwappableSettings`, `peerSettingsSwapPlan`, `applyHotSwappableSettings`, `swapPeerSettingsJournaled`
  → Constraint: the mutable set (fields written from the reload goroutine) is safe only because every reader outside the facts snapshot goes through a p.mu-guarded accessor. `BFD` is read today without a lock under the immutability contract on `Peer.Settings`, so it can join the swap set only after every reader moves to a locked accessor.
  → Constraint: the undo for a rollback is read from the RUNNING peer before the apply (`hotSwappableSnapshot`); a BFD swap's undo MUST restore the previous opt-in and reopen or close the BFD client the same way.
- [ ] `internal/component/bgp/reactor/peer_derived_bindings.go` - `derivedFeedsOnly`, `onlyDerivedFeedsDiffer`, `derivedFeedChange`, `derivedFeedsDeliverable`, `Session.feedCatchUp`, `Peer.catchUpDerivedFeeds`, `requestRouteRefreshAll`
  → Decision: the predicate drops its `Derived` condition and becomes "grants no send type and receives no OPEN" (renamed so the name says what it tests). The diff, the gained/lost lists and the targeted down are reused; the catch-up decision is widened (next two constraints).
  → Constraint (B1, verified at `internal/core/events/token.go` `SplitTypeToken`): a token naming a registered type with no suffix gets `DirBoth`, so `receive [ update ]` and `receive [ update-sent ]` both grant the UPDATEs Ze SENDS, and `receive [ * ]` (`ReceiveAll`) grants both directions. `Session.feedCatchUp` reads only `Session.receivedRoute`, and ROUTE-REFRESH makes the PEER re-send, never Ze. A gained process granted the sent direction therefore holds a partial Adj-RIB-Out copy if the session already advertised. The catch-up decision gains a sent-direction arm: when the new grant covers updates in the sent direction, `Peer.hasAdvertised` (`peer.go`, reads `Session.advertised` of the current connection) false means nothing to learn, true means restart (Ze cannot re-send what it sent to the process's view). The received-direction arm stays `feedCatchUp`. The binding swaps only when every direction its grant covers is satisfiable; the verdict is the worse of the two arms.
  → Constraint (I4): the catch-up runs only for a grant that carries routes. A grant whose every type is one of `state`, `negotiated`, `keepalive`, `notification`, `refresh` (the base types of `baseReceiveTypes` other than `update` and `open`, `peer_settings.go`) needs none; `test/plugin/attach-process-reload.ci` adds exactly such a `receive [ state ]` binding; under the new rule it swaps whatever the peer announced and whatever its Route Refresh support, because the grant carries no route (AC-14 pins that case directly). Any other type, `update` or a plugin-registered type such as `update-rpki`, counts as route-bearing in the direction it is granted, so an unknown type fails closed into the catch-up rather than out of it.
  → Constraint: today a kept feed with a DIFFERENT grant is refused (restart) because the consumer's picture of the old grant cannot be told apart from the new one. The new rule delivers it as lost-then-gained for that one process: a targeted down, then a targeted up and the catch-up, so the consumer starts that peer again from nothing.
  → Constraint: a gained consumer today receives NO session-up event; only the lost side has `OnPeerUnbound`. Plugins that build per-peer state from `state up` exist (`adj_rib_in/rib.go`, `rib/rib.go`, `bmp/bmp_events.go`, `rr/rr.go`, `gr/gr.go`, `watchdog/watchdog.go`, `ls_export/export_plugin.go`, `redistribute_egress/register.go`, `trafficusage/monitor.go`), so a gained receive-only binding needs a targeted up.
  → Constraint (I1, verified in `applyHotSwappableSettings`, `peer_settings_apply.go`): today the delivery graph is republished under r.mu BEFORE `catchUpDerivedFeeds` runs with the locks released, so the read goroutine can hand a live UPDATE to the gained process in that gap, before any up. The same order exists on the dynamic path (`SetDynamicGroups` republishes inside `setDynamicGroupsLocked`, then catches up). Both callers move to one order (Data Flow step 3).
  → Constraint (I2): a peer in OpenSent or OpenConfirm at swap time that reaches Established between the republish and the `State()` check in `catchUpDerivedFeeds` gives the gained process two ups for one session: the FSM's (the graph already carries the edge) and the targeted one. `rr` bumps `ReplayGen` twice and `bmp` records the peer up twice. The targeted up is delivered only for the session that was current and Established when the apply began, and the republish checks that session is still current and Established (Data Flow step 3).
  → Constraint (I11, verified: `ProcessBinding.Derived` is read only by `derivedFeedsOnly`, line 30, and written only by `EnsureProcessBinding`, `config.go`): once the predicate drops `Derived` the field is write-only, yet `reflect.DeepEqual` in `onlyDerivedFeedsDiffer` and `derivedFeedsDeliverable` still compares it, so an operator attach replacing a derived binding with an identical grant reads as a changed grant and earns a spurious down-then-up. The field is deleted (no-layering), with its comment in `peer_settings.go` and the derived wording on `EnsureProcessBinding`.
- [ ] `internal/component/bgp/server/events.go` `deliverPeerState`, `countPeerUpBarrier`, `onPeerUnbound`; `internal/component/plugin/server/startup_claims.go` `Server.UnheldRoles`; `internal/component/bgp/reactor/reactor_api.go` `establishedPeerInfo`; `internal/component/bgp/reactor/peer.go` `initialReplayToken`
  → Constraint (B2): `deliverPeerState` computes `UnheldRoles` over the procs it is handed, and arms the peer-up barrier (`countPeerUpBarrier` then `SetPeerUpBarrier`) for any barrier-declaring plugin among them. Called with only the gained processes mid-session, it would (a) report `bgp-peer-up-replay` unheld because its holder is not in that set, so `adj-rib-in` would self-replay toward the peer (`adj_rib_in/rib.go`, `replayDrivenElsewhere`), be refused by a receive-only binding and log "peer-up replay failed"; (b) re-arm the barrier for `bgp-rs` after the End-of-RIB already went. `establishedPeerInfo` fills `InitialReplay` from `initialReplayToken`, which is non-zero while the initial update is owed, so a reload during initial sync would let `adj-rib-in` and `rr` report session-ready for an update they never saw.
  → Decision: the targeted up carries `InitialReplay` 0, arms no barrier, computes its unheld roles over the FULL fed set (the processes the published graph feeds this peer's state plus the gained ones), and carries a new session reason, `attached` (a constant beside `ReasonPeerRemoved` in `pkg/plugin/rpc/enums.go`). A plugin that runs a peer-up replay toward the peer skips the replay on that reason, because the binding that delivered it grants no send type and every UPDATE it built would be refused. Two members are verified at the producer: `adj_rib_in/rib.go` (relays the replay routes on up unless `replayDrivenElsewhere`) and `rr/rr.go` (starts `replayForPeer` on every up). The other candidates, files that handle `SessionStateUp` and mention replay, are `rib/rib.go`, `watchdog/watchdog.go`, `bmp/bmp_events.go` and `redistribute_egress/register.go`; the implementer reads each up handler, plus `rs`, and records which of them send toward the peer on up. `deliverPeerState` gains the knob that turns the barrier off rather than a second delivery function.
  → Decision (NOTE 5): a gained process granted `negotiated` receives the running session's negotiated event AFTER its up, the order the FSM uses (`notifyPeerEstablished` then `notifyPeerNegotiated`, `peer_run.go`).
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` `dynamicTemplateChanged`, `setDynamicGroupsLocked`, `SetDynamicGroups`, `resolveDynamicPeerSettings`, `createDynamicPeer` - a dynamic peer's template diff, its direct binding write, and its establishment-time resolution
  → Constraint: `dynamicTemplateChanged` builds the old and the next template settings for the peer's address (`buildDynamicPeerSettings`, both unresolved) and calls `onlyDerivedFeedsDiffer` and `derivedFeedsDeliverable` directly; any other difference restarts the peer. Phase 3 replaces it with `peerSettingsSwapPlan` over the same two template-built settings (no-layering), so the plan, its copier and its restart reason are the configured peer's.
  → Constraint (I7, read at the producer 2026-10-11): `setDynamicGroupsLocked` writes a dynamic peer's bindings directly (`peer.settings.ProcessBindings = feeds`, under `peer.mu` but outside the copier and with no undo), republishes the graph under `r.mu`, then `SetDynamicGroups` catches up after the unlock: the publish-then-catch-up order I1 names.
  → Constraint (read at the producer 2026-10-11): `resolveDynamicPeerSettings` already reads `p.settings.ImportFilters` and `ExportFilters` under `p.mu.RLock` and writes `PeerAS` and both resolved chains under `p.mu`. That is the fix of the deferral row the reconcile comment still cites (`reactor_api.go`, the dynamic branch of `reconcilePeersJournaled`, naming `plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md`); the row was closed on 2026-08-29 against `TestDynamicPeerSettingsFilterReadRacesTheReload` (`reactor_dynamic_race_test.go`), and the file went with `plan/deferrals/` in `6fb9cd8814` on 2026-09-05. What is STILL unlocked is the template pair `p.dynImportFilters` / `p.dynExportFilters`: captured from the running chains on the first establishment and read for every later one, with no lock, under the comment "Those two fields are touched by this function alone". The moment a dynamic swap writes a new template, that claim is false, and the capture can also latch a chain the reload already replaced. Phase 3 captures the pair at construction and moves every later read and write under `p.mu`.
  → Constraint (read at the producer 2026-10-11): `createReloadFunc` (`bgp/config/loader_create.go`) calls `r.SetDynamicGroups(dynGroups)` from inside the ReloadFunc. Every caller of the ReloadFunc therefore APPLIES the candidate's dynamic groups: `loadPeersFullOrTree` (`reactor_api.go`), reached from `VerifyConfig` (documented "without modifying reactor state"), `ApplyConfigDiff`, `peerDiffCount` (the budget estimate) and `ReconcilePeersWithJournal`; and `peerSettingsFromReloadConfig` (`operation.go`). A verify of a candidate that drops a dynamic group tears that group's peers down before the commit is decided, and nothing records an undo. Phase 3 makes the ReloadFunc return the groups and apply nothing. Until it lands, the defect is a row in `plan/journal/check-mode-mutates-the-tree.md` (2026-10-11), so other sessions grepping for it find it.
  → Constraint: the dynamic apply runs before the peer reconcile today and keeps that order, as the first journaled step after the globals in `reconcilePeersJournaled`, so the reconcile loop's dynamic branch still sees the population the dynamic step decided to keep.
  → Constraint: `TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges` (`reactor_api_dynamic_reload_test.go`) pins a restart for a template whose import chain changed, and `TestReloadNeverSwapsSettingsOntoADynamicPeer` pins the restart when a configured entry replaces a dynamic peer. The first flips under the single copier (a filter edit becomes a swap, owner decision OQ-2 option A) and is replaced by a test of the new rule; the second is a different branch (a configured entry naming the address) and stays as it is.
- [ ] `internal/component/bgp/reactor/peer_bfd.go` - `bfdStrict`, `startBFDClient`, `stopBFDClient`, `bfdRequestFor`, `runBFDSubscriber`
  → Constraint: `startBFDClient` is idempotent and returns early on a nil or disabled opt-in; `stopBFDClient` is idempotent. The live swap calls them; it adds no third way to open or close a BFD client.
  → Constraint: `startBFDClient` with no published service (`api.GetService() == nil`) logs a warning and leaves a non-strict peer without BFD. In a commit that adds the `bfd` root AND the opt-in, the bfd plugin publishes its service in `OnStarted` (`internal/component/bfd/bfd.go`, `api.SetService`), so the order of the plugin start and the BGP apply decides whether the swap finds a service (A-4).
  → Constraint (I6): a published service is not enough. `pluginService.EnsureSession` (`internal/component/bfd/bfd.go`) refuses with `errBfdNotConfigured` while the bfd plugin's config is not applied, and with `errBfdDisabled` while `bfd > enabled` is false. When the bfd plugin is ALREADY running, `test/reload/bgp-bfd-profile-reload-peer-and-profile.ci` proves bfd applies its root before bgp asks (no "profile ... is not defined" refusal). The only open case is a bfd plugin STARTED by the same commit: A-4 covers both the service and the applied config for it.
- [ ] `internal/component/bgp/reactor/session_bfd_strict.go` `applyBFDStrictNegotiation`, `raiseBFDStrictConfigChanged`, `bfdStrictConfigChanged`; `internal/component/bgp/reactor/reactor_api.go` (the peer summary that sets `BFDEnabled`, and the restart branch of `reconcilePeersJournaled`)
  → Constraint (I5, verified by grep of `.BFD` under `internal/component/bgp/reactor/`): the unlocked readers of `PeerSettings.BFD` are `Peer.bfdStrict` (`peer_bfd.go`, reached from `peer_run.go` at the strict pre-FSM start and at the Established exit), `startBFDClient` (`peer_bfd.go`), `bfdRequestFor` (`peer_bfd.go`, reads `s.BFD` fields), `Session.applyBFDStrictNegotiation` and `Session.raiseBFDStrictConfigChanged` (`session_bfd_strict.go`, under `s.mu`, which does NOT exclude a writer holding `p.mu`), and the peer summary's `BFDEnabled` (`reactor_api.go`, for `show`). `verifyPeerBFDProfiles` and `bfdStrictConfigChanged` read candidate settings, not a running peer's, and stay unlocked. The swap runs on peers in OpenSent or OpenConfirm concurrently with `applyBFDStrictNegotiation`, so every running-peer reader moves to the p.mu-guarded accessor.
  → Constraint (NOTE 3, verified): Event 35 (`raiseBFDStrictConfigChanged`) is raised only on the restart branch of `reconcilePeersJournaled`; a non-strict swap never raises it.
- [ ] `internal/component/bfd/engine/engine.go` `Loop.EnsureSession`, `reviveReleasedLocked`
  → Constraint (NOTE 4, verified): a request for a session that is live and shared with another client joins it through `Acquire` and its new parameters are ignored; a request for a released session still inside its Detection Time is revived WITH the new request's parameters. So a non-strict profile edit (release then ensure) takes effect when BGP was the session's only client, and leaves the shared session on its current parameters when another client (static route, OSPF) holds it. That is the BFD plugin's existing sharing rule (RFC 5882 Section 4.4), not something this spec changes; R-12 and AC-15 record it.
  → Constraint (RFC5882-4.4-1 defect, verified at the producer by review r2 I10; folded into this spec by the owner on 2026-10-11): RFC 5882 Section 4.4 (read in `rfc/full/rfc5882.txt`): "If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session." `Loop.EnsureSession` narrows the entry's `joined` key on a shared join (`entry.joined = narrowKey(entry.joined, key)`); `Loop.ReleaseSession` only decrements the refcount, and `joined` has no other writer. Sequence: A (no local address) creates S; B pinned to local X joins, so `joined.Local` becomes X; B releases and `joined` stays X; C pinned to Y then fails `sharesSession(C, joined)` in `sharedEntryLocked` and `createSessionLocked` opens a SECOND session to the same remote system. S never ran on X, so the narrowing is a stale claim the moment B leaves; a released entry matched by `sharedEntryLocked` carries the same stale `joined`. This spec's live BFD swap (AC-15's release-then-ensure, AC-6's removal) is exactly the release that leaves the stale key, so Phase 4 fixes it first (AC-24).
- [ ] `internal/component/bgp/reactor/peer_settings.go` `BFDSettings`, `ProcessBinding`, `MayPushRoutes`, `ReceivesPeerState`
  → Constraint: strict mode is `Enabled && Strict` (`Peer.bfdStrict`); `HoldTime` and `HoldDown` only act in strict mode.
- [ ] `internal/component/bgp/reactor/peer_run.go` - `startBFDClient` on entry to Established, `stopBFDClient` on exit for a non-strict peer; `initialUpdateReporters`
  → Constraint: only `MayPushRoutes` bindings join the initial-sync barrier, so a receive-only binding added live leaves the barrier of the running session untouched.
- [ ] `internal/component/bgp/server/event_dispatcher.go` `OnPeerUnbound`, `internal/component/bgp/server/events.go` `onPeerUnbound`, `onPeerNegotiated`
  → Decision: the targeted up (`OnPeerBound`, sibling of `OnPeerUnbound`) finds its recipients like `onPeerUnbound`, by `SubscribedProcsNamed` over the gained names, because the graph does not carry their edge yet when it runs (I1). It calls `deliverPeerState` with `SessionStateUp`, reason `attached`, `InitialReplay` 0, the barrier off and the unheld roles computed over the full fed set (B2). A gained process granted `negotiated` then receives the running session's negotiated event, targeted the same way, after its up.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` (structured handler near line 345, `handleEvent` near line 759) and `rpki_reload.go` `replaceConfig`
  → Constraint: bgp-rpki reacts to `state` only on DOWN (`removePeer`); it needs no up. Routes already retained in Adj-RIB-In are re-decided by `revalidateConfigSnapshot` when validation is enabled, so for RPKI the ROUTE-REFRESH is not what makes the states appear; the generic receive-only test (AC-4) is what proves the catch-up.
- [ ] `internal/component/bgp/reactor/peer_derived_bindings_test.go` `TestOperatorBindingChangeStillRestarts`
  → Decision: this test pins the rule the owner changed. It is REPLACED by a test of the new rule (an operator receive-only binding swaps) and a test that keeps the remaining restart cases (an operator binding with a send grant, and one granting `open`, still restart). The replacement is a change of the behaviour under test by owner decision, not a weakening.
- [ ] `test/reload/bgp-rpki-added-live.ci`, `test/reload/bgp-bfd-added-live.ci`, `test/plugin/attach-process-reload.ci`, `test/plugin/attach-process-runtime-subscribe.ci` - the functional tests that pin today's counts
  → Constraint: `bgp-rpki-added-live.ci` passes on one connection only because the base config pre-binds `bgp-rpki`; `bgp-bfd-added-live.ci` and `attach-process-reload.ci` expect two connections; `attach-process-runtime-subscribe.ci` changes a kept binding's grant from `[ keepalive state ]` to `[ keepalive state open-received ]`, which gains `open` and therefore STILL restarts under the new rule (two connections kept).

**Key insights:** (minimal context to resume after compaction)
- One predicate decides the binding half: "grants no send type and receives no OPEN". It replaces `derivedFeedsOnly`; derived and operator bindings follow one rule.
- Gained process: targeted `state up` (reason `attached`, `InitialReplay` 0, no barrier, unheld roles over the full fed set), then `negotiated` if granted, all acknowledged BEFORE the delivery graph carries the edge; then the catch-up, only for a route-bearing grant: received direction by `feedCatchUp` (nothing / ROUTE-REFRESH / restart), sent direction by `hasAdvertised` (nothing / restart). Lost process: existing targeted down. Changed grant: down then up for that process.
- The targeted up is delivered only for the session that was current and Established when the apply began; a session change in the window restarts rather than risk a double or a missing up.
- `ProcessBinding.Derived` is deleted.
- BFD half: a non-strict opt-in on both sides of the diff (nil, disabled, or enabled-non-strict) joins the swap set; the apply opens or closes the BFD client with the existing idempotent functions. Strict on either side restarts.
- `PeerSettings.BFD` must move to locked access (every reader listed under `session_bfd_strict.go` above) before it can be written live.
- A bfd plugin started by the same commit may not be ready when the BGP apply runs (A-4). If so the fix is the transaction's apply order, never a service watcher: boot behaviour is out of scope (main-thread decision on I6).
- Dynamic peers (owner, OQ-1: "A: yes, design it"): the plan runs over the old and next template-built settings, the copier and the ordered delivery are the configured peer's, the template's `$remote_as` / `$remote_ip` chains are resolved against the running peer before the copier sees them, the `dyn*` template pair is captured at construction and touched only under `p.mu`, the ReloadFunc applies nothing, and the dynamic step is the first journaled step of the reconcile.
- The rest of the hot-swappable set reaches dynamic peers too (owner, OQ-2: "A"), including `Capabilities` under an unchanged negotiation and static routes (written to the wire). An import-chain swap filters only routes received after the reload, for configured and dynamic peers alike; re-import belongs to `plan/immediate/spec-bgp-filtered-route-storage.md` AC-3.
- Phase 2 fails closed on the dynamic path: until Phase 3 replaces it, `dynamicTemplateChanged` restarts on any binding difference.
- "Rolled back" means the reload journal. The reload compensation path re-reads the staged candidate and does not restore `undo.before` for any peer (journal row, `plan/journal/rollback-forgets-partial-apply.md`).
- Phase 4 step 1 fixes the RFC5882-4.4-1 shared-session defect in the BFD engine (owner: "fold it into keep-session"): `joined` is recomputed from the live clients on every acquire and release (AC-24).
- FRR and BIRD both keep the session for both changes, on configured and on dynamic neighbors; their behaviour is described above.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` (346L) - swap-or-restart decision over the whole `PeerSettings`; five-field swap set; `ProcessBindings` swaps only when derived feed-only bindings alone differ
- [ ] `internal/component/bgp/reactor/peer_derived_bindings.go` (273L) - derived feed predicate, gained/lost diff, catch-up decision, targeted down, ROUTE-REFRESH per negotiated family
- [ ] `internal/component/bgp/reactor/peer_bfd.go` (583L) - BFD client start/stop, strict predicate, request builder
- [ ] `internal/component/bgp/reactor/peer_run.go` - BFD start on Established, stop on exit (non-strict), initial-sync reporters
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - dynamic template diff with the same derived-feed calls
- [ ] `internal/component/bgp/server/event_dispatcher.go`, `events.go` - `OnPeerUnbound`, `onPeerUnbound`, `onPeerNegotiated`
- [ ] `internal/component/bfd/api/registry.go` - `SetService`/`GetService`, published from bfd `OnStarted`
- [ ] `internal/component/bgp/plugins/rpki/rpki.go`, `rpki_reload.go` - RPKI's use of state events and retained-route revalidation
- [ ] `internal/core/events/token.go` `SplitTypeToken` - a bare registered token is granted in both directions (B1)
- [ ] `internal/component/bgp/reactor/peer.go` `hasAdvertised`, `initialReplayToken` - the sent-direction catch-up question, and the token a targeted up must not carry
- [ ] `internal/component/bgp/server/events.go` `deliverPeerState`, `countPeerUpBarrier`; `internal/component/plugin/server/startup_claims.go` `UnheldRoles` - what an up delivery computes and arms
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_claims.go`; `internal/component/bgp/plugins/rr/rr.go` - replay toward the peer on every up; withdrawal toward other peers on down (rr `handleStateDown`)
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` `emitPurgedWithdraws` - a down for a peer re-elects and withdraws that peer's routes from the other peers
- [ ] `internal/component/bgp/reactor/session_bfd_strict.go`, `reactor_api.go` - the rest of the unlocked `PeerSettings.BFD` readers; Event 35 only on the restart branch
- [ ] `internal/component/bfd/bfd.go` `EnsureSession`; `internal/component/bfd/engine/engine.go` `EnsureSession`, `reviveReleasedLocked` - refusal before config apply; profile edits on shared sessions
- [ ] `internal/component/bgp/config/loader_create.go` `createReloadFunc` - applies the candidate's dynamic groups from inside the ReloadFunc
- [ ] `internal/component/bgp/reactor/reactor_api.go` `loadPeersFullOrTree`, `VerifyConfig`, `peerDiffCount`, `reconcilePeersJournaled`; `operation.go` `peerSettingsFromReloadConfig` - every caller of the ReloadFunc, and the journal the dynamic step joins
- [ ] `internal/component/bgp/reactor/reactor_api_dynamic_reload_test.go`, `reactor_dynamic_race_test.go` - the tests that pin today's dynamic reload rule and the filter-pair lock
- [ ] `internal/component/plugin/server/reload_compensation.go` `compensateReload`, `restoreReload`; `reload.go` `reloadConfig`; `internal/component/config/storage/source.go` `ReadReloadConfig`; `internal/component/cli/model_commands_commit.go` `commitCandidateAndReload`; `internal/component/web/editor.go` `applyCommittedContent` - the compensation path re-reads the still-staged candidate (R-18)
- [ ] `internal/component/bfd/engine/engine.go` `EnsureSession`, `ReleaseSession`, `sharedEntryLocked`, `sharesSession`, `narrowKey`, the `joined` field; `rfc5882_shared_session_test.go` - the stale narrowed key (AC-24)

**Behavior to preserve:** (unless the user explicitly said to change it)
- The restart branch: one log line naming every field that forced it, Cease subcode 6 (or 3 for a removed peer), and the fail-closed default for any unclassified field.
- A binding with a send grant restarts (End-of-RIB barrier membership changes).
- The derived-feed behaviour proven by the existing tests in `peer_derived_bindings_test.go` other than `TestOperatorBindingChangeStillRestarts` (gained derived feed with/without Route Refresh, removal sends no refresh, catch-up re-decided at apply).
- Strict BFD: opening before the FSM starts, the OpenConfirm gate, capability 74, and every strict reload behaviour covered by `test/interop/scenarios/bgp-bfd-strict-reload-speaker`.
- `test/reload/bgp-rpki-added-live.ci` as it is (pre-bound shape, one connection).
- A rollback of a swapped reload returns the running session to what it enforced before, restarting nothing (`swapPeerSettingsJournaled`).

**Behavior to change:** (only what the user asked for)
- An operator `attach process` binding that grants no send type and no OPEN receive is swapped live, like a derived one.
- A kept receive-only binding with a changed grant is swapped as down-then-up for its process, instead of restarting.
- A gained receive-only process receives a targeted session-up (reason `attached`, then negotiated if granted) before the delivery graph feeds it anything.
- The catch-up runs only for a route-bearing grant, and covers the sent direction (`hasAdvertised`) as well as the received one.
- `ProcessBinding.Derived` is deleted.
- A non-strict BFD opt-in change is swapped live on a configured peer and on a dynamic peer (through its group template); the BFD client opens or closes on the running session.
- A dynamic peer's template change is planned by `peerSettingsSwapPlan` and applied through the configured peer's copier and ordered delivery, instead of `dynamicTemplateChanged` restarting on anything but a derived feed. The template's other hot-swappable fields swap too (owner, OQ-2: "A"): filter chains, `PrefixUpdated`, static routes, and `Capabilities` when `negotiationOutcomeUnchanged` holds (`peerSettingsSwapPlan` returns `hotSwappableWithCapabilities`, `peer_settings_apply.go`, `peer_settings_negotiation.go`). A static-route swap writes the wire: `deliverStaticRouteDelta` (`peer_static_wire.go`) announces the added routes and withdraws the removed ones on the running connection. An import-chain swap applies to UPDATEs received after the reload only (`runIngressPolicyChain` reads `ImportFilters` per UPDATE); routes already accepted are not re-filtered, where today's restart re-learned them all (I3).
- The ReloadFunc returns the candidate's dynamic groups and applies nothing; a verify, a diff count or an operation lookup no longer changes the dynamic population.
- The dynamic step is recorded in the reload journal; a reload the journal rolls back restores the previous group set and undoes every dynamic swap. The reload compensation path (`compensateReload`) is out of this scope: it does not restore `undo.before` for configured peers today either (Phase 3.2 and its journal row).
- The BFD engine's shared session widens its `joined` key again when a pinned client releases, so a later client pinned elsewhere shares the one session (RFC 5882 Section 4.4, AC-24).
- The `dyn*` filter templates are captured when the dynamic peer is built and read or written only under `p.mu`.
- A plugin that replays toward a peer on up skips the replay on an `attached` up.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config commit (SSH editor `commit`, `ze config` apply, or SIGHUP reload) carrying a changed `bgp peer <name> attach process <p>` block or `bgp peer <name> connection bfd` block, or the same blocks changed in a dynamic group's template, and possibly a new `bfd` root.
- Format at entry: the resolved BGP tree, parsed into `PeerSettings` (`parseProcessBindingsFromTree`, the BFD block parser in `config.go`) and into `DynamicGroupConfig` templates (`peersAndDynamicGroups`, `bgp/config/peers.go`).

### Transformation Path
1. Config transaction: verify (`VerifyPeerBFDProfiles` for a BFD profile name), then apply; a registry-internal plugin named by a new config root starts during the transaction (`autoLoadForNewConfigPaths`). -> Correction (2026-10-11, Phase 3): an `external` program the reload newly declares is NEVER started: the auto-load walks `registry.ConfigRootsMap()` only and no reload path updates `Server.config.Plugins`. The two attach `.ci` declare the program in the boot config; journal row in `plan/journal/unwired-feature.md`; owner decision (2026-10-11): a separate spec, `plan/immediate/spec-reload-starts-declared-external-programs.md` (Work Not Done). The ReloadFunc returns the peers, the globals AND the dynamic groups, and applies none of them, so verify and the budget estimate change nothing. The tree route has no ReloadFunc (a reactor built by `LoadReactor`, or a config path of `""` or `-`: `CreateReactor` installs none), and `loadPeersFullOrTree` there parses no groups. It answers "groups not read", a result distinct from an empty group set, and the dynamic step on that answer leaves the group set and every dynamic peer as they are, which is what happens today. An empty set is never read as "the config dropped every group" unless the ReloadFunc returned it (`TestTreeRouteLeavesDynamicGroupsUnchanged`).
2. Reactor reconcile (`reconcilePeersJournaled`, `reactor_api.go`), after the globals:
   1. Dynamic step, journaled, first. Skipped entirely on the "groups not read" answer of the tree route (step 1). Under r.mu, for every dynamic peer, the step only PLANS: removed when its group is gone or its range no longer covers it (Cease subcode 3, as today); otherwise `peerSettingsSwapPlan(oldTemplateBuilt, nextTemplateBuilt, session)`, where both sides come from `buildDynamicPeerSettings` for the peer's address, exactly the inputs `dynamicTemplateChanged` uses today. A restart reason marks the peer for restart (Cease subcode 6) with the reason logged, as for a configured peer. A swap is recorded with the next template's settings and its unresolved chains (for the `dyn*` pair); nothing is resolved against the peer here, because the peer's `PeerAS` can change before the write. The group set is then replaced and the `ActivePeers` counts recomputed from `r.peers`, as today, and r.mu is released. `sync.RWMutex` is not reentrant and step 3 takes r.mu itself (`applyHotSwappableSettings`), so the swaps, like the stops and the catch-ups, run only AFTER r.mu is released, as `SetDynamicGroups` runs its stops and catch-ups today. The step's journal undo restores the previous group set, recounts, and undoes each dynamic swap through the configured peer's undo (step 3) with the `dyn*` pair restored in the same `p.mu` section. A dynamic peer the step removed is not rebuilt by the undo, because nothing names it: its next connection is accepted under the restored group.
   2. Configured peers: per peer, `peerSettingsSwapPlan(current, next, session)` returns a copier or a restart reason.
   3. The reconcile runs twice per transaction reload: the BGP plugin's apply runs `ReconcilePeersWithJournal`, then the server's `reloadConfig` calls `ApplyConfigDiff(newTree)` after the transaction committed (review r2 N5). The second pass compares templates that are now equal, so its dynamic step plans nothing and delivers nothing: no second targeted up, no second ROUTE-REFRESH, no BFD transition (`TestDynamicTemplateReceiveOnlySwap` asserts it).
3. Swap: `swapPeerSettingsJournaled` records the undo from the running peer, then `applyHotSwappableSettings` runs the ordered bind delivery below. The dynamic step calls the same two functions; for a dynamic peer the undo snapshot also holds the `dyn*` pair, and the copier's write section also writes the next unresolved chains into the `dyn*` pair. Neither caller republishes before the up, and the dynamic step adds no second copier and no second delivery order.
   1. Under p.mu, read the current session S and whether the peer is Established on it; compute gained and lost from the running bindings and next. No write yet.
   2. Locks released: when S is Established and the gained list is not empty, deliver the targeted up (reason `attached`, `InitialReplay` 0, no barrier, unheld roles over the full fed set), then the targeted negotiated to the gained processes granted it. Delivery is synchronous (`deliverPeerState` waits for each result), so every gained process has acknowledged its up before step 3. The graph still lacks the gained edges, so no UPDATE reaches them.
   3. Take r.mu then p.mu. For a dynamic peer, resolve the next template's variable-bearing filter chains here, with `resolveFilterVars` against the `PeerAS` and address current under this p.mu, when the peer has learned its AS, and leave them unresolved otherwise (the state `createDynamicPeer` leaves before establishment, which `resolveDynamicPeerSettings` then resolves from the new `dyn*` pair). This is the reload goroutine, not a hot path, so the allocation under p.mu is accepted here; resolving from the AS read at plan time would leave a peer that established inside the window with unresolved chains. Then check that the peer's current session is still S and that its Established state is the one step 1 saw. If yes, or if the swap has no gained process: write the copied fields, republish the graph when bindings changed, release. A session change with no gained process restarts nothing, because no targeted up was or should have been delivered: a filter-only swap racing establishment keeps the session, as today. If no AND the swap has gained processes (S closed, or the peer reached Established inside the window): write and republish all the same, release, deliver a targeted down to the gained processes that step 2 told up (so none holds an up with no down), and restart the session with Cease subcode 6. That window is a reload racing an FSM transition, rare, and a restart is the answer that leaves no process with a double or a missing up (I2).
   4. Locks released: targeted down to the lost processes (`OnPeerUnbound`), then the catch-up for the gained processes whose grant carries routes, re-decided now: sent direction, `hasAdvertised` true restarts; received direction, `feedCatchUp` gives nothing, one ROUTE-REFRESH per negotiated family, or restart.
   5. A kept binding with a changed grant is lost-then-gained for that one process, and its edge must be absent from the graph while it is told down and then up, or a live UPDATE could reach it between the two. So, before step 2, the apply writes the running binding list with that process's binding left out (under r.mu then p.mu, the same write discipline), republishes, releases the locks and delivers the targeted down to it; from step 2 on it is a gained process. The journal's undo is the snapshot taken before the apply, so a rollback never restores the intermediate list. With S not Established at step 1 the process gets no targeted down or up: the FSM's own up reaches it later through the graph.
   6. With S not Established at step 1 and still not Established at step 3, no targeted up is delivered: the graph carries the edge before the FSM's up, which reaches the gained process like any other.
4. BFD transition, after the binding steps, locks released, for a configured and a dynamic peer alike: opt-in gained or edited and the peer Established: `stopBFDClient` (when one is open and the request changed) then `startBFDClient`; opt-in lost or suspended: `stopBFDClient`. In the BFD engine, each release recomputes the shared session's `joined` key from the clients still attached, so a release by this peer never leaves the session pinned to this peer's local address (RFC 5882 Section 4.4, Phase 4 step 1, AC-24).
   - Establishment racing a dynamic swap: `resolveDynamicPeerSettings` reads the `dyn*` pair under `p.mu`, resolves with the lock released (it allocates), and publishes under `p.mu` only when the pair is still the one it read (a template generation on the peer, bumped by every `dyn*` write); otherwise it resolves again from the new pair. A resolution never publishes chains built from a template a swap already replaced.
5. Restart branch unchanged: the peer is removed and re-added with a Cease subcode 6.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | `PeerSettings` diff in `reconcilePeersJournaled` | No |
| Reactor -> Plugin server | `EventDispatcher.OnPeerUnbound` and the new targeted bound event, delivery graph republish | No |
| Reactor -> BFD component | `api.GetService().EnsureSession` / `ReleaseSession` through `startBFDClient` / `stopBFDClient` | No |
| BFD component -> BFD engine | `pluginService.EnsureSession` / `ReleaseSession` -> `Loop.EnsureSession` / `Loop.ReleaseSession`, where the shared entry's `joined` key is recomputed | No |
| Ze -> BGP peer (wire) | no NOTIFICATION; ROUTE-REFRESH per negotiated family when a gained consumer needs the peer's routes | No |

### Integration Points
- `hotSwappableSettings` - gains the BFD field (conditional on non-strict both sides) and the widened binding predicate.
- `peerSettingsSwapPlan` - unchanged structure; the neutralization now covers the new members.
- `EventDispatcher` - gains the targeted up (`OnPeerBound`), sibling of `OnPeerUnbound`; `deliverPeerState` gains the barrier knob, not a second delivery function.
- `SetDynamicGroups` - kept for the initial load only (no journal at boot); the reload reaches the dynamic population through the journaled dynamic step of `reconcilePeersJournaled`, which uses `peerSettingsSwapPlan`, `swapPeerSettingsJournaled` and `applyHotSwappableSettings`. `dynamicTemplateChanged` is deleted.
- `ReloadFunc` (`reactor.go`) - returns the dynamic groups beside the peers and globals; `createReloadFunc` stops calling `SetDynamicGroups`.
- `resolveDynamicPeerSettings` / `createDynamicPeer` - the `dyn*` pair is captured at construction and touched only under `p.mu`, with the generation check above.
- `Peer.startBFDClient` / `stopBFDClient` - called from the apply as well as from the FSM callback.
- `reactorAPIAdapter.swapPeerSettingsJournaled` - the undo covers the BFD field and its client transition.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to fill at implementation: the swap goes through `peerSettingsSwapPlan` and the copier only |
| No unintended coupling (components stay isolated) | No | to fill: the reactor still reaches BFD only through `bfd/api` |
| No duplicated functionality (extends existing, does not recreate) | No | to fill: no second binding diff, no second BFD open path |
| Zero-copy preserved where applicable (refs, not copies) | No | to fill: reload path only, not a hot path |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | to fill: the binding rule reads the grant, never a plugin name; no `bgp-rpki` special case |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | to fill: grep for `bgp-rpki` and `bfd` in `internal/component/bgp/reactor/` shows no new literal |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A receive-only binding has no effect on the running session's OPEN, negotiation or initial-sync barrier | `initialUpdateReporters` and `resetAPISync` count only `MayPushRoutes` bindings (`peer_run.go`, `peer.go`); capabilities come from `PeerSettings.Capabilities`, not from bindings | a binding that changes the OPEN would be swapped while the OPEN is stale | grep every reader of `ProcessBindings` in `internal/component/bgp/` (list in Current Behavior) and confirm none feeds `buildOpen`; record the list | unvalidated |
| A-2 | A process newly started by the commit (its `attach` line auto-loads it) has subscribed before the reactor's targeted up and ROUTE-REFRESH run | none: ordering of plugin start vs reactor apply in the config transaction not yet read | the up and the refreshed routes reach no process; the consumer holds nothing for the peer | AC-4 functional test with an external process that is started by the same commit and must see the up and the refreshed UPDATE | unvalidated |
| A-3 | A BFD session opened on an already Established non-strict peer that starts Down (never Up) does not end the BGP session | the same `startBFDClient` runs on entry to Established today and `test/plugin/bgp-bfd-opt-in.ci` passes; FRR acts only on Up->Down | the live enable would reset the session through BFD instead of through the reload | AC-5 functional test, one connection, BFD session listed | unvalidated |
| A-4 | In a commit that STARTS the bfd plugin (adds the `bfd` root) and adds the opt-in, the bfd plugin has published its service AND applied its config before the BGP apply runs | `api.SetService` runs in the bfd plugin's `OnStarted` (`internal/component/bfd/bfd.go`); `EnsureSession` refuses with `errBfdNotConfigured` until the config is applied. For a bfd plugin already running, `test/reload/bgp-bfd-profile-reload-peer-and-profile.ci` already proves bfd applies before bgp asks; the newly started case is not read | the swapped peer stays without BFD: a nil-service warning or an `errBfdNotConfigured` refusal in the log, `show bfd sessions` empty. The fix is then the transaction's apply order for a newly started plugin that bgp reads (`WantsConfig`), the guarantee the existing `.ci` proves for a running one; never a service watcher, because a watcher would also change boot behaviour, which the main thread removed from scope (I6) | Phase 3 first step: run `bgp-bfd-added-live.ci` with the swap and no other change, and record whether `startBFDClient` met a nil service or `EnsureSession` refused | unvalidated |
| A-7 | The FSM's own `state up` for a session reads the delivery graph after `State()` reports Established, so a graph published before the transition carries the gained edge to that up, and one published after it does not need to | `peer_run.go` sets Established then calls `notifyPeerEstablished`; `onPeerStateChange` reads `PeerScopedProcs` at delivery time (`events.go`) | the step 3 check in Data Flow could pass while the FSM's up went out without the gained edge, leaving the process with no up | `TestBindRacesEstablished` (unit, `-race`): drive the transition inside the window and assert exactly one up per gained process or a restart | unvalidated |
| A-5 | `stopBFDClient` -> `ReleaseSession` on the last reference behaves as on today's Established exit (RFC 5882 Section 8 AdminDown handling is the BFD plugin's) | the same function is the non-strict exit path today | a live removal could make the remote BGP speaker tear down on BFD Down | AC-6 interop: FRR keeps BGP Established after Ze removes the opt-in | unvalidated |
| A-8 | `resolveDynamicPeerSettings` is the only writer of a dynamic peer's settings outside the reload goroutine, and the only reader of the `dyn*` pair | grep of `dynImportFilters` / `dynExportFilters` under `internal/component/bgp/reactor/` (2026-10-11): `peer.go` declares them, `reactor_dynamic.go` `resolveDynamicPeerSettings` is the only reader and writer | a second writer would race the swap's `dyn*` write the generation check does not cover | Phase 3 first step: repeat the grep, plus every write to `p.settings.` reached from the establishment goroutine, and record the list in the spec | unvalidated |
| A-9 | No caller of the ReloadFunc relies on its applying the dynamic groups, other than the reload that then reconciles | callers read 2026-10-11: `loadPeersFullOrTree` (from `VerifyConfig`, `ApplyConfigDiff`, `peerDiffCount`, `ReconcilePeersWithJournal`) and `peerSettingsFromReloadConfig`; test fakes in `internal/component/bgp/reactor/*_test.go` and `bgp/config/*_test.go` | a path that today gets its dynamic groups installed as a side effect would stop getting them: the boot path (`loader_create.go`, which calls `SetDynamicGroups` itself) and `ApplyConfigDiff` (which parses and then reconciles nothing) are the two to check. The tree route (no ReloadFunc: `LoadReactor`, or a config path of `""` or `-`) never yields groups, so `loadPeersFullOrTree` answers "groups not read" there, never an empty set, and the dynamic step leaves the population untouched (Data Flow step 1) | Phase 3: grep every `reloadFunc` / `ReloadFunc` caller and record what each does with the groups; `TestTreeRouteLeavesDynamicGroupsUnchanged` | unvalidated |
| A-6 | FRR, as the interop peer, answers ROUTE-REFRESH and keeps `connectionsEstablished` unchanged across a Ze reload that sends one | FRR `peer_on_policy_change` and RFC 2918 support; the `rtr-stayrtr` checker reads the same counter | the continuity check would be measuring something else | AC-7 scenario run, counter read before and after | unvalidated |

Phase 1 validation, 2026-10-11 (grep of the tree at b095791c0a, plus the Phase 1 `.ci` runs):

  → Constraint (A-1, holds by grep): the non-test readers of `ProcessBindings` under `internal/component/bgp/` are `config/flowspec_binding.go`, `config/loader_create.go`, `config/peers.go` (`validatePeerProcessCaps`), `reactor/send_permission.go` (send grants only), `reactor/config.go` (the parser), `reactor/peer_settings_apply.go`, `reactor/delivery_graph.go` (the graph), `reactor/peer_run.go` `initialUpdateReporters` (counts only `MayPushRoutes() && ReceivesPeerState()` bindings), `reactor/reactor_api.go` `GetPeerProcessBindings`, and `reactor/reactor_dynamic.go`. `Session.buildOpen` (`session_negotiate.go`) reads no binding. One caveat for Phase 2: `validatePeerProcessCaps` reads the bindings to allow Route Refresh and graceful-restart, so a next config that drops the last route-pushing binding of a peer advertising either is refused at parse time, before any swap.
  → Constraint (A-2, unproven, owed in Phase 2): `bgp-attach-receive-only-live` (AC-4) and `bgp-attach-dynamic-receive-only-live` (AC-9) attach an external process started by the same reload; today both restart before the question arises, so the ordering of plugin start against the reactor's targeted up is still unread. Their green run in Phase 2 is the evidence.
  → Constraint (A-3, supported): in `bgp-bfd-profile-edit-live` the BFD session opened on entry to Established is listed under profile fast (script steps 1-3 pass) while ze-peer runs no BFD, so a session that never comes Up left BGP Established. This is today's start-on-Established path; the live start on an already Established peer is proved by AC-5 / AC-15 green in Phase 4.
  → Constraint (A-4, unproven, owed in Phase 4 first step): `bgp-bfd-dynamic-toggle-live` and `bgp-bfd-added-live` both add the `bfd` root and the opt-in in one commit, so they are the carriers; today the restart fires first and nothing about the plugin start order is observable.
  → Constraint (A-5, unproven, owed in Phase 4): the second reload of `bgp-bfd-dynamic-toggle-live` (opt-in removed, `lacks "peer":"127.0.0.1"`) and AC-12 interop are the carriers.
  → Constraint (A-6, unproven, owed with AC-12): no interop scenario was run in Phase 1.
  → Constraint (A-7, unproven, owed in Phase 2): `TestBindRacesEstablished` is the carrier; no `.ci` can order the FSM transition inside the apply window.
  → Constraint (R-22 premise BROKEN, Phase 2 agent 2, 2026-10-11): Implementation Steps 2 says "no unit test and no `.ci` pins a dynamic derived-feed swap on reload". Wrong: `test/reload/reload-dynamic-peer-survives.ci` reloads a config whose second peer is iBGP (remote 65533, local 65533), so the AIGP builder binding reaches the dynamic template, and with `dynamicTemplateChanged` failing closed the dynamic peer reads Cease 6/6 where the test expects Cease 6/2 at shutdown (`bin/le-lrb/le test bgp reload -pattern reload-dynamic-peer-survives`: "error-subcode: expected=2, got=6"). A fail-closed Phase 2 commit regresses live behaviour. Decision owed: land Phase 2 with Phase 3's dynamic step, or give the Phase 2 dynamic path a bindings-only ordered delivery (a copier carrying ProcessBindings alone, so the template's unresolved chains never reach the peer).
  → Constraint (A-8, holds by grep): `dynImportFilters` / `dynExportFilters` are declared in `peer.go` and read and written only in `reactor_dynamic.go` `resolveDynamicPeerSettings` (the lazy capture on first resolve). Phase 3 step 1 still records every write to `p.settings.` reached from the establishment goroutine.
  → Constraint (A-9, holds by grep, and the side effect is real): `createReloadFunc` (`config/loader_create.go`) calls `r.SetDynamicGroups(dynGroups)`, and the boot path in the same file calls it itself. The ReloadFunc's callers are `reactorAPIAdapter.Reload` and `loadPeersFullOrTree` (`reactor_api.go`; the latter reached from `VerifyConfig` and `ApplyConfigDiff`) and `peerSettingsFromReloadConfig` (`operation.go`). So a `VerifyConfig` or an operation's candidate lookup installs the candidate's dynamic groups today, which is AC-23's defect.
  → Decision (Phase 1 harness): `reload/live-stages` (`internal/test/fixture/reload_fixture_live_stages.go`) is a new external-plugin fixture that runs reloads and reads from inside the daemon, so a test can read `show bfd sessions` between two SIGHUP reloads. The dynamic tests use it with the clean-shutdown Cease (subcode 2) as the continuity carrier of `reload-dynamic-peer-survives.ci`.
  → Constraint (dynamic filter race, journal row 2026-10-11 in `plan/journal/false-synchronization-claim.md`): a dynamic session can establish before `bgp-filter-prefix` connects and its first UPDATE is rejected by `on-error=reject`; `bgp-dynamic-filter-swap-live` re-sends its route after Ze's End-of-RIB.
  → Decision (AC-3 entry test, Phase 1 complete): `bgp-rpki-attach-live` waited on Route Refresh being reachable from the SSH editor, which `1f2dd73052` fixed (`route-refresh` is now a `capability-mode` leaf). Run 2026-10-11 through a freshly built `bin/le-lrb/le`: the editor commit lands ("Session committed: 7 change(s) applied and reloaded"), the OPEN carries capability 2, and the peer reads NOTIFICATION Cease 6/6 where the ROUTE-REFRESH is expected, so it is red for the restart, not for a fixture reason. All 10 Phase 1 entry tests are now committed; Phase 2 starts next.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Writing `PeerSettings.BFD` from the reload goroutine races the FSM callback, the subscriber, the strict negotiation and `show`, which read it with no lock today | `-race` failure in the new swap unit test | move every running-peer reader to a p.mu-guarded accessor before the field joins the swap set: `bfdStrict` (and through it `peer_run.go` at the strict pre-FSM start and the Established exit), `startBFDClient`, `bfdRequestFor`'s caller, `applyBFDStrictNegotiation` and `raiseBFDStrictConfigChanged` (`session_bfd_strict.go`, which hold `s.mu` only), and the peer summary's `BFDEnabled` (`reactor_api.go`). A `-race` test runs the swap concurrently with `startBFDClient` and with `applyBFDStrictNegotiation` on a peer in OpenConfirm |
| R-2 | The apply opens BFD while the FSM callback for Established also opens it | two `EnsureSession` calls, an orphaned subscriber | `startBFDClient` already serialises on `p.bfd.starting`; the swap reuses it and adds no second open path |
| R-3 | A bfd plugin started by the same commit is not ready when the BGP apply runs (A-4) | nil-service warning or `errBfdNotConfigured` refusal in the AC-5 test log, empty `show bfd sessions` | fixed in the transaction's apply order for a newly started plugin that bgp reads, never by a watcher; a boot where BGP reaches Established before bfd is out of scope (main-thread decision on I6) and stays as it is |
| R-4 | A gained process receives an UPDATE (live, or from the ROUTE-REFRESH) before its targeted up | process sees UPDATE for an unknown peer | the up is delivered and acknowledged before the graph carries the edge (Data Flow step 3.2), and the ROUTE-REFRESH is sent after the republish; AC-4 asserts the order |
| R-5 | A changed grant delivered as down-then-up makes the consumer drop the peer. A forwarder, `bgp-rs` or `bgp-rr`, then withdraws that peer's routes from EVERY OTHER peer on the down (`handleStateDown`, `rs/server_handlers.go` and `rr/rr.go`) and re-announces them only as the ROUTE-REFRESH re-sends them. `bgp-rib` forwards nothing between peers: its purge (`emitPurgedWithdraws`, `rib/rib_bestchange.go`) publishes best changes on the rib event bus only, so its churn reaches another peer only through a process that consumes those events. The forwarder's withdrawal on the down is built from what `bgp-rib` recorded as sent (`recoveryRoutes`, `rib/rib_recovery.go`, reads `ribOut[destination]`), so it reaches only a destination where `bgp-rib` is bound with the sent direction; a destination without that binding keeps the dead route silently (journal row 2026-10-10, `plan/journal/silent-fall-through.md`). The AC-8 `.ci` binds `bgp-rib` `receive [ update state refresh ] send [ update ]` on both peers for that reason. Other sessions see a flap of this peer's routes | AC-8 `.ci` reads a second peer's wire | accepted as the price of not restarting: a restart flaps the same routes on the same sessions AND bounces this peer. The spec does not hide it: section 18a and `docs/guide/config-reload.md` state that changing the grant of a route-consuming process withdraws and re-learns that peer's routes downstream; AC-8 asserts the withdraw and the re-announce on the other peer's wire |
| R-9 | A peer moving to Established between the plan and the republish gives a gained process two ups (FSM's plus targeted) or none | `rr` `ReplayGen` bumped twice, `bmp` peer-up recorded twice; or a process with routes and no up | the targeted up is bound to the session current and Established at step 1, and step 3 restarts when that changed (Data Flow); A-7; `TestBindRacesEstablished` |
| R-10 | A gained process granted updates in the sent direction misses what Ze already advertised (B1) | process's Adj-RIB-Out view lacks routes Ze sent | sent-direction arm: `hasAdvertised` true restarts; AC-13 |
| R-11 | A targeted up makes a plugin replay toward the peer or report readiness for an initial update it never saw (B2) | "peer-up replay failed" log, `bgp-rs` End-of-RIB barrier re-armed, a session-ready signal mid-session | targeted up carries reason `attached`, `InitialReplay` 0, no barrier, unheld roles over the full fed set; replaying plugins skip on `attached`; AC-16 |
| R-12 | A non-strict profile edit on a BFD session shared with another client (static route, OSPF) keeps the session's old parameters (`Acquire` ignores the new request, `engine.go`). Superseded in part by the owner decision of 2026-10-11 (AC-25): a shared session takes a profile edit in place, by the rule Phase 4 step 3 decides | `show bfd sessions` still shows the old timers after a profile edit | the BFD plugin's existing sharing rule (RFC 5882 Section 4.4), unchanged by this spec; documented in `docs/guide/bfd.md` beside the live-edit paragraph; AC-15 asserts the unshared case takes the new profile |
| R-6 | Existing functional tests expect two connections for a receive-only attach change and now fail | `attach-process-reload.ci` red | update their expectations and comments in the same change (listed in Files to Modify); `attach-process-runtime-subscribe.ci` gains `open-received` and keeps two connections |
| R-7 | The rollback undo of a BFD swap leaves a BFD client open or closed against the restored settings | after a rolled-back commit, `show bfd sessions` disagrees with the config | the undo goes through the same copier and the same BFD transition step as the apply; a unit test rolls back a swap and checks the client state |
| R-13 | A dynamic swap writes the new templates while the establishment goroutine resolves the old ones, and the resolution then publishes chains built from the replaced template | a dynamic peer filtering with the previous group policy after a reload that raced its establishment | the `dyn*` pair is captured at construction and touched only under `p.mu`; `resolveDynamicPeerSettings` publishes only when the template generation it read is still current, and resolves again otherwise (Data Flow step 4); `TestDynamicTemplateSwapRacesEstablishment` under `-race` |
| R-14 | The copier writes a dynamic peer's template chains unresolved onto a running, resolved peer | `$remote_as` in a running peer's filter chain after a dynamic reload | the write section resolves "next"'s chains under p.mu against the `PeerAS` and address current there, before the copier writes (Data Flow step 3.3), so a peer that established between plan and write is resolved too; `TestDynamicFilterSwapResolvesAgainstLearnedAS` |
| R-15 | A rollback cannot rebuild a dynamic peer the dynamic step removed, because no configuration names it | after a rolled-back reload that dropped a dynamic group, the group's peers are gone until they reconnect | accepted and documented: the undo restores the group set, so the next connection is accepted under the previous template; a configured peer's undo also restarts it (re-add). `TestFailingReloadRollsBackDynamicSwap` asserts the group set and the swapped peers; `docs/guide/config-reload.md` states the removed-peer case; `docs/architecture/config/apply-ordering.md`, whose rule reads "A remove-peer's inverse restores the peer the reactor was running", names a dynamic peer as the stated exception (review r2 N4: a dynamic peer is passive by construction, so a rebuilt idle dynamic peer and "accept the next connection under the restored group" differ only in the `show` listing and `ActivePeers`) |
| R-16 | Making the ReloadFunc side-effect free leaves a path without its dynamic groups (A-9) | a dynamic group present in config with no listener or no peers accepted after a commit | A-9's grep; the boot path keeps its own `SetDynamicGroups` call; `test/reload/reload-dynamic-peer-survives.ci` stays green |
| R-17 | The verify-time teardown this spec removes was hiding a test that passed only because verify had already applied the groups | a dynamic reload test going red once verify stops applying | each such red is read at the producer; a test that relied on verify applying is wrong about what it asserts and is corrected, never weakened |
| R-8 | A peer without Route Refresh that already announced routes still restarts when RPKI is attached live | two connections against such a peer | intended: the catch-up rule is unchanged and documented; the AC-3 test uses a peer that advertises Route Refresh, and AC-10 pins the restart for one that does not |
| R-18 | The reload compensation path (`compensateReload` -> `restoreReload` -> `ApplyConfigDiff(undo.before)`) re-applies the candidate instead of restoring the previous config, and after Phase 3 that includes the candidate's dynamic groups | after a reload rejected late (listener migration, an outer caller's refusal), the reactor runs the rejected candidate's peers | out of scope by design and recorded: this spec's "rolled back" is the reload journal (AC-11, AC-20). Read 2026-10-11: `loadPeersFullOrTree` ignores its tree on the file route and calls the ReloadFunc, `storage.ReadReloadConfig` returns the staged candidate first, and the CLI and web commit routes clear the candidate only after the reload returned (`commitCandidateAndReload`, `EditorManager.applyCommittedContent`), so the candidate is still staged while `compensateReload` runs inside the reload. The defect predates this spec and hits configured peers the same way; journal row in `plan/journal/rollback-forgets-partial-apply.md` |
| R-19 | On the tree route (no ReloadFunc) the dynamic step reads "no groups" as "every group dropped" and tears down every dynamic peer | dynamic peers gone after `ApplyConfigDiff` on a reactor built by `LoadReactor` | the tree route answers "groups not read", distinct from an empty set, and the step skips (Data Flow step 1, A-9); `TestTreeRouteLeavesDynamicGroupsUnchanged` |
| R-20 | Recomputing `joined` on release widens a session a still-attached pinned client relies on, or narrows nothing on a join | two sessions to one remote system while B is attached and C is pinned elsewhere is CORRECT (RFC 5883 Section 4.1), so the negative half must stay | `joined` is the session key narrowed by EVERY remaining client's key, recomputed on each acquire and release; AC-24 has both halves; `TestSharedSessionWidensOnRelease` and the existing RFC5882-4.4-1 records |
| R-21 | A dynamic group's import-chain edit, now swapped live (owner, OQ-2: "A"), keeps routes the new chain rejects, where today's restart re-learned every route through the new chain | a route the new policy rejects still in `show bgp adj-rib-in` after the reload | accepted and documented (owner decision A): the configured-peer import swap already behaves so (`test/reload/reload-import-policy-applies.ci`, "enforced on subsequently evaluated routes"); FRR (soft-reconfig or ROUTE-REFRESH) and BIRD (`channel_request_reload`) re-evaluate, and that re-import is owned by `plan/immediate/spec-bgp-filtered-route-storage.md` AC-3. Blast Radius, Known Limitations and `docs/guide/config-reload.md` state it |
| R-22 | A Phase 2 commit on its own ships the old publish-then-catch-up order for OPERATOR receive-only bindings on dynamic peers (no targeted up, graph before up, no sent-direction arm) | a dynamic peer's gained operator binding receiving UPDATEs with no up between the Phase 2 and Phase 3 commits | Phase 2 fails closed on the dynamic path: `dynamicTemplateChanged` restarts on any `ProcessBindings` difference until Phase 3 deletes it (Implementation Steps 2) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a consumer process holds an incomplete view of a peer (missing routes, missing or doubled up, a partial copy of what Ze sent), a plugin replays toward a peer it may not send to, or a BFD client is left open or closed against the config; at worst a session left running where it should have restarted. On the dynamic path: a dynamic peer filtering with an unresolved or a replaced template (R-13, R-14), a rolled-back reload leaving a dynamic swap in place, or a dynamic group missing its listener because the ReloadFunc no longer installs it (R-16). Every dynamic peer of every group is on this path, so an IXP route server is the deployment that meets a defect first |
| What changes even when it is right? | a changed grant on a kept forwarding binding (`bgp-rs`, `bgp-rr`) withdraws that peer's routes from every other peer on the targeted down and re-announces them as the ROUTE-REFRESH re-sends them (R-5). The other sessions stay up; their routes from this peer flap once. Today the same edit restarts this peer and flaps the same routes |
| What changes even when it is right, on the dynamic path? | a verify, a budget estimate or an operation lookup over a candidate no longer tears down dynamic peers before the commit is decided; a dynamic template edit to a BFD opt-in, a receive-only binding, a filter chain, `PrefixUpdated`, static routes, or `Capabilities` that negotiate identically keeps the dynamic sessions up instead of restarting every peer of the group (owner, OQ-2: "A"). A static-route edit reaches each dynamic peer's wire as UPDATEs announcing the added routes and withdrawing the removed ones (`deliverStaticRouteDelta`). An import-chain edit filters only the routes each dynamic peer sends AFTER the reload: routes the old chain already accepted stay in Adj-RIB-In until the peer re-sends them, where today's restart re-learned every one through the new chain (R-21). Configured peers already behave so; re-import is owned by `plan/immediate/spec-bgp-filtered-route-storage.md` AC-3 |
| How is it reverted? | single commit revert; no config migration, no wire format change |
| Who else touches this path? | `plan/immediate/spec-bgp-reload-rich-peer-no-bounce.md` (reload equality for rich peers, same `peerSettingsEqual` decision), `plan/spec-terminal-demo-showcase.md` (RPKI and BFD topics wait on this), BFD strict-mode reload work (`bgp-bfd-strict-reload-speaker`), `plan/spec-bgp-remote-as-auto.md` (plans to reuse `resolveDynamicPeerSettings` for static peers: the `dyn*` capture at construction and the generation check change the function it reuses), and every test that installs a ReloadFunc (the signature gains the dynamic groups) |

## Owner decisions

**OQ-1, BFD on dynamic peers (AC-9): answered 2026-10-11, option A.** The
owner's words: "A: yes, design it". A dynamic peer's non-strict BFD opt-in
change, made through its group template, keeps the session up like a configured
peer's. The design is Phase 3 (dynamic peers join the shared swap) followed by
Phase 4 (the BFD opt-in, which then reaches both kinds of peer through the one
copier). Option B (keep restarting a dynamic peer on a BFD change) is not taken;
BIRD does not take it either: it requests BFD for every spawned dynamic neighbor
and reconfigures the child in place (Required Reading, BIRD `bgp_update_bfd`).

**OQ-2, the rest of the hot-swappable set on dynamic peers: answered 2026-10-11,
option A.** The owner's words: "A". A dynamic peer swaps exactly what a
configured peer swaps, through the one copier (`hotSwappableSettings`, a
subtraction from the whole struct): BFD and receive-only bindings, and also
filter chains, `PrefixUpdated`, static routes, and `Capabilities` when the
negotiation outcome is unchanged. Filter chains are resolved against the running
peer in the write section (R-14). No dynamic-only exclusion set is written.
`TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges` is replaced by
`TestDynamicFilterSwapResolvesAgainstLearnedAS` as a change of the behaviour
under test by owner decision. The consequence the owner accepted with it: an
import-chain edit filters only routes received after the reload (R-21).

**RFC5882-4.4-1, the BFD shared-session defect: decided 2026-10-11.** The
owner's words: "fold it into keep-session". The defect (Required Reading, BFD
engine) is fixed as Phase 4 step 1 and owned by AC-24.

**BFD profile edits on live sessions: decided 2026-10-11.** The owner's words:
"Apply live, in keep-session (Recommended)". Editing a BFD profile (its timers
and its authentication) reaches the live BFD sessions that use it in place,
including pinned sessions and shared sessions. Today neither happens:
`applyPinned` (`internal/component/bfd/bfd.go`) calls `EnsureSession` only for a
key not already in `r.pinned`, and `api.Key` carries no profile, so an edit to a
pinned session's profile never reaches the engine; authentication is set only in
`createSessionLocked` and `replaceAuth`, and `Reconfigure` runs only in
`reviveReleasedLocked`, so a live session never takes new authentication. The
refusal texts of `authMismatchCase` (`bfd/engine/auth_join.go`) tell the
operator to edit a profile, and that edit then changes nothing. Evidence:
`scratch/review-rfc-verdict-bfd-r4.md` ISSUE 1 (session 5620b26f). Owned by
AC-25, built as Phase 4 step 3. This supersedes in part R-12 and AC-15's shared
half ("the session keeps its parameters"): which client's profile a shared
session takes when its clients name different profiles is the first design
question of Phase 4 step 3, answered before code.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH editor commit adding `attach process bgp-rpki` to a running peer | → | `peerSettingsSwapPlan` -> `applyHotSwappableSettings` -> targeted up + catch-up | `test/reload/bgp-rpki-attach-live.ci` |
| SSH editor commit adding the `bfd` root and a non-strict `connection bfd` opt-in | → | `peerSettingsSwapPlan` -> `applyHotSwappableSettings` -> `startBFDClient` | `test/reload/bgp-bfd-added-live.ci` (one connection) |
| SIGHUP reload adding a receive-only `attach process` for an external process | → | targeted up (`EventDispatcher`) then `requestRouteRefreshAll` | `test/reload/bgp-attach-receive-only-live.ci` |
| SIGHUP reload adding a receive-only attach block (kept edge plus added edge) | → | binding swap, delivery graph republish | `test/plugin/attach-process-reload.ci` (one connection) |
| SIGHUP reload changing a kept receive-only grant, with a second peer watching | → | intermediate republish, targeted down, targeted up, republish, ROUTE-REFRESH | `test/reload/bgp-attach-grant-change-live.ci` |
| SIGHUP reload adding a receive-only binding on a peer without Route Refresh | → | `feedCatchUp` impossible -> restart branch | `test/reload/bgp-attach-no-route-refresh-restarts.ci` |
| SIGHUP reload adding `receive [ update-sent state ]` after Ze advertised | → | sent-direction arm (`hasAdvertised`) -> restart | `test/reload/bgp-attach-sent-direction-restarts.ci` |
| SIGHUP reload adding, then a second reload removing, a non-strict `connection bfd` in a dynamic group's template | → | journaled dynamic step -> `peerSettingsSwapPlan` -> `applyHotSwappableSettings` -> `startBFDClient` / `stopBFDClient` | `test/reload/bgp-bfd-dynamic-toggle-live.ci` |
| SIGHUP reload adding a receive-only attach block to a dynamic group's template | → | journaled dynamic step -> ordered bind delivery -> targeted up | `test/reload/bgp-attach-dynamic-receive-only-live.ci` |
| SIGHUP reload changing a dynamic group's import chain that uses `$remote_as` | → | dynamic step -> write section resolves next against the learned AS -> copier | `test/reload/bgp-dynamic-filter-swap-live.ci` |
| SIGHUP reload removing a pinned BGP peer's non-strict BFD opt-in while a static route's unpinned BFD shares the session, then a second pinned client on another local address joins | → | `stopBFDClient` -> `ReleaseSession` -> `Loop.ReleaseSession` recomputes `joined` -> `Loop.EnsureSession` shares | `test/reload/bgp-bfd-shared-session-widens.ci` |
| SIGHUP reload against FRR | → | the same swap, observed by a foreign speaker | interop `bgp-attach-receive-only-live-frr`, `bgp-bfd-enable-live-frr`, `bgp-bfd-dynamic-enable-live-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `peerSettingsSwapPlan` with current and next differing only in an operator binding that grants no send type and receives no OPEN | returns a copier and an empty reason (no restart) |
| AC-2 | `peerSettingsSwapPlan` with next adding an operator binding that grants `send [ update ]`, or one that receives `open` (any direction), or `receive [ * ]` | returns a restart reason naming `ProcessBindings` |
| AC-3 | A running iBGP peer (Route Refresh advertised) that announced Valid, Invalid and NotFound prefixes; an SSH commit load-merges `attach process bgp-rpki { receive [ update-received state ] }` and the `bgp rpki` cache and policy | one TCP connection for the whole test; no NOTIFICATION from Ze; `show bgp adj-rib-in` shows validation-state 1, 2 and 3 for the routes received before the commit |
| AC-4 | A running peer that announced a route; a reload adds `attach process <external> { receive [ update-received state ] }` for an external process started by that reload | one TCP connection; the process receives a `state up` for the peer, reason `attached`, BEFORE any UPDATE from it; Ze sends one ROUTE-REFRESH per negotiated family; the process receives the route the peer announced before the reload |
| AC-5 | A running non-strict peer; an SSH commit load-merges a `bfd` root with a profile and the peer's `connection bfd { profile <p> }` | one TCP connection; `show bfd sessions` lists the single-hop session to the peer under the committed profile |
| AC-6 | A running non-strict peer with an open BFD session; a commit removes the opt-in (or sets `enabled false`) | one TCP connection; `show bfd sessions` no longer lists the peer's session; the BGP session stays Established |
| AC-7 | A running peer whose opt-in changes between non-strict and strict, in either direction, or whose strict `hold-time`/`hold-down` changes | the peer restarts, and the restart log line names `BFD` |
| AC-8 | Peer A (Route Refresh advertised) announced a route that `bgp-rs` forwards to peer B; a SIGHUP reload changes A's kept receive-only `bgp-rs` binding grant (`[ update-received state ]` to `[ update-received state refresh ]`) | one TCP connection to A and one to B; the process receives a targeted down then a targeted up (reason `attached`) for A, and no UPDATE between them; no other process receives either; Ze sends A one ROUTE-REFRESH per negotiated family; B's wire shows A's route withdrawn and then announced again (R-5) |
| AC-9 | A dynamic peer whose group template gains a receive-only attach block (SIGHUP reload) | one TCP connection; the gained process receives the targeted up (reason `attached`) before any UPDATE of that session, through the same ordered bind delivery as a configured peer; the restart reason, when a dynamic change does restart, is logged by field name as for a configured peer |
| AC-10 | A running peer that did NOT advertise Route Refresh and announced routes; a SIGHUP reload adds a receive-only binding granting `update-received` | the peer restarts (Cease subcode 6), unchanged from today; the restart log line names `ProcessBindings` |
| AC-11 | A swapped reload (receive-only attach and BFD opt-in) that the reload journal rolls back (the compensation path is R-18, out of scope) | the binding set and the BFD client return to their pre-commit state with no restart |
| AC-12 | Interop: FRR peer Established with Ze; Ze is reloaded with a receive-only attach, then with a non-strict BFD opt-in, then with the opt-in's profile edited (different transmit interval), then with the opt-in removed | FRR's `connectionsEstablished` for the Ze neighbor is unchanged after each reload; after the BFD reload FRR `show bfd peers` lists Ze's session Up; after the profile edit the session is still Up and FRR reports Ze's new transmit interval; after the removal FRR's BGP session to Ze is still Established (A-5) |
| AC-13 | A running peer to which Ze has advertised a route; a reload adds a binding granting `receive [ update-sent state ]` (or bare `update`, which is granted in both directions) | the peer restarts (Cease subcode 6): Ze cannot re-send what it sent to the new process's view. The same reload on a session where Ze has advertised nothing keeps one connection |
| AC-14 | A running peer that announced routes and did NOT advertise Route Refresh; a reload adds a binding granting only `receive [ state ]` | one TCP connection; the process receives the targeted up; no ROUTE-REFRESH is sent and no restart happens, because the grant carries no route |
| AC-15 | A running non-strict peer whose BFD session has no other client; a commit changes the opt-in's profile | one TCP connection; `show bfd sessions` shows the session under the new profile's timers. With a second client (a static route's BFD) sharing the session, the session keeps its parameters and the BGP session still stays up (R-12) |
| AC-16 | A gained receive-only binding for `adj-rib-in` on a peer, mid-session, on a daemon with no `bgp-peer-up-replay` claimant, and a reload during that peer's initial sync | the targeted up carries `InitialReplay` 0 and reason `attached`; `adj-rib-in` sends nothing toward the peer and logs no "peer-up replay failed"; no peer-up barrier is armed (`SetPeerUpBarrier` not called) |
| AC-17 | A reload that adds a receive-only binding while the peer is in OpenConfirm and reaches Established inside the apply window | the gained process receives exactly one `state up` for that session, or the session restarts; never two ups and never routes without an up |
| AC-18 | An operator `attach process` block identical in grant to the derived binding it replaces (the config builder no longer adds its own) | no restart and no targeted down or up: `ProcessBinding.Derived` no longer exists to make the two differ |
| AC-19 | An Established dynamic peer; a SIGHUP reload adds a non-strict `connection bfd { profile <p> }` to its group's template (with the `bfd` root), then a second reload removes it | one TCP connection for the whole test; after the first reload `show bfd sessions` lists the single-hop session to the dynamic peer under `<p>`; after the second it lists none; the BGP session stays Established throughout. The same template change between non-strict and strict restarts the dynamic peer with a log line naming `BFD` (AC-7's rule) |
| AC-20 | A reload that swaps a dynamic peer (template gains a receive-only binding and a non-strict BFD opt-in) and whose later configured-peer step fails, so the journal rolls back | the dynamic peer is still the same `*Peer` on the same session; its bindings, its BFD client, its `dyn*` pair and the reactor's group set are the pre-reload ones; the gained process received a targeted down for the rolled-back binding; no Cease is sent |
| AC-21 | `resolveDynamicPeerSettings` running on the establishment goroutine while a dynamic swap writes a new template pair, under `-race` | no race report; the chains the peer publishes are built from the template that is current when they are published, never from the replaced one; `dynImportFilters` / `dynExportFilters` are set at construction, and no read or write of them happens outside `p.mu` (the unlocked capture is gone) |
| AC-22 | An Established dynamic peer whose group template's import chain (written with `$remote_as`) changes, by SIGHUP reload | one TCP connection; the running chain is the new template resolved with the peer's learned AS; a route the peer sends AFTER the reload that the new chain rejects is not accepted (the dry-run pattern of `test/reload/reload-import-policy-applies.ci`); a route the old chain accepted before the reload stays accepted until the peer re-sends it (R-21, re-import owned by `plan/immediate/spec-bgp-filtered-route-storage.md` AC-3) |
| AC-23 | A dynamic peer Established under group G; `VerifyConfig`, `peerDiffCount`, and an operation's candidate lookup (`peerSettingsFromReloadConfig`) each run over a candidate that drops G | after each call the dynamic peer is still the same `*Peer` on the same session and the reactor's group set still holds G; only the journaled reconcile applies the candidate's groups. On the tree route (no ReloadFunc), `ApplyConfigDiff` leaves the group set and every dynamic peer unchanged |
| AC-24 | BFD engine, one peer and VRF: client A (no local address) holds session S; client B pinned to local X joins S, then releases; client C pinned to local Y on the same peer and VRF then requests a session. Negative half: C pinned to Y requests while B is still attached | C receives S's handle and the engine holds ONE session to that remote system (RFC 5882 Section 4.4: "all MUST share a single BFD session"); S's `joined` key is back to A's key after B left. Negative half: C gets its own session, because S is pinned to X while B holds it (RFC 5883 Section 4.1, unchanged) |
| AC-25 | A live pinned BFD session under profile P (no authentication); a second client whose request carries authentication is refused (`ErrAuthMismatch`); a reload then edits P to carry that same authentication, and the client joins again. Separately, a reload edits P's transmit interval while the pinned session is Up, and a reload edits the profile of a BFD session shared by two clients | the second join succeeds and the engine holds ONE session to that remote system, now authenticated; the interval edit reaches the live session in place (same session, same local discriminator, the new interval in `show bfd sessions`); the shared session takes the edit by the rule Phase 4 step 3 decides. Owner decision of 2026-10-11 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | binds RPKI to a live peer and enables validation in one commit | SSH editor -> commit -> reconcile -> swap -> targeted up -> rpki revalidates retained routes | `test/reload/bgp-rpki-attach-live.ci` |
| 2 | enables BFD on a live peer | SSH editor -> commit -> bfd plugin start -> reconcile -> swap -> `startBFDClient` | `test/reload/bgp-bfd-added-live.ci` |
| 3 | attaches a monitoring process to a live peer | SIGHUP -> reconcile -> swap -> targeted up -> ROUTE-REFRESH -> peer re-sends -> process | `test/reload/bgp-attach-receive-only-live.ci` |
| 4 | does the same against FRR | SIGHUP -> swap -> FRR keeps its session | interop `bgp-attach-receive-only-live-frr`, `bgp-bfd-enable-live-frr` |
| 5 | an IXP operator enables, then disables, BFD on a dynamic group | SIGHUP -> journaled dynamic step -> swap -> `startBFDClient`, then `stopBFDClient` | `test/reload/bgp-bfd-dynamic-toggle-live.ci`, interop `bgp-bfd-dynamic-enable-live-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOperatorReceiveOnlyBindingSwaps` | `internal/component/bgp/reactor/peer_derived_bindings_test.go` | AC-1, replaces `TestOperatorBindingChangeStillRestarts` | |
| `TestOperatorSendOrOpenBindingStillRestarts` | same | AC-2: send grant, `open` grant, `*` grant each restart | |
| `TestChangedReceiveGrantIsDownThenUp` | same | AC-8: targeted down then up to that process only | |
| `TestGainedReceiveOnlyBindingGetsUpBeforeRefresh` | same | AC-4 order: targeted up delivered before the ROUTE-REFRESH is written | |
| `TestReceiveOnlyGainWithoutRouteRefreshRestarts` | same | AC-10 | |
| `TestNonStrictBFDOptInSwaps` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-5/AC-6 at the plan: nil<->non-strict, enabled toggle, profile edit swap | |
| `TestStrictBFDChangeRestarts` | same | AC-7: reason names `BFD` | |
| `TestBFDSwapOpensAndClosesClient` | same | apply opens the client on an Established peer and closes it on removal (fake `api.Service`) | |
| `TestBFDSwapRacesEstablishedStart` | same | R-1/R-2 under `-race`: swap concurrent with `startBFDClient`, one handle | |
| `TestBFDSwapRollbackRestoresClient` | `internal/component/bgp/reactor/peer_settings_apply_test.go` | AC-11 | |
| `TestDynamicTemplateReceiveOnlySwap` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-9: a dynamic template's receive-only attach swaps through `applyHotSwappableSettings` (up acknowledged before the graph carries the edge); then a second reconcile over the same config (the `ApplyConfigDiff(newTree)` pass that follows the transaction, N5) is a no-op: no second targeted up, no ROUTE-REFRESH, no restart, same `*Peer` | |
| `TestDynamicBindingChangeRestartsBeforeSharedSwap` | `internal/component/bgp/reactor/reactor_api_dynamic_reload_test.go` | R-22, Phase 2 only: once `Derived` is gone, `SetDynamicGroups` restarts a dynamic peer whose template gains any binding, derived or operator. Deleted in Phase 3 step 3 together with `dynamicTemplateChanged`, the function it pins, and replaced by `TestDynamicTemplateReceiveOnlySwap` (the rule it pins is transient by design) | |
| `TestTreeRouteLeavesDynamicGroupsUnchanged` | `internal/component/bgp/reactor/reactor_api_dynamic_reload_test.go` | AC-23 tree half, R-19: a reactor with no ReloadFunc and an Established dynamic peer; `ApplyConfigDiff` over a tree leaves the group set and the peer (same `*Peer`, no Cease) as they were | |
| `TestSharedSessionWidensOnRelease` | `internal/component/bfd/engine/rfc5882_shared_session_test.go` | AC-24, both halves; red at HEAD (C opens a second session). Tagged `RFC requirement: RFC5882-4.4-1 positive` with the Section 4.4 quote | |
| `TestDynamicNonStrictBFDOptInSwaps` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-19 at the reactor: template nil -> non-strict opens the client on the Established dynamic peer, non-strict -> nil closes it (fake `api.Service`), same `*Peer` | |
| `TestDynamicStrictBFDChangeRestarts` | same | AC-19 strict half: the restart reason names `BFD` | |
| `TestFailingReloadRollsBackDynamicSwap` | `internal/component/bgp/reactor/reactor_api_dynamic_reload_test.go` | AC-20: `reconcilePeersJournaled` with a dynamic swap and a later step that fails; `Rollback` restores bindings, BFD client (fake service refcount), `dyn*` pair and group set, no Cease | |
| `TestDynamicTemplateSwapRacesEstablishment` | `internal/component/bgp/reactor/reactor_dynamic_race_test.go`, under `-race` | AC-21, R-13: `resolveDynamicPeerSettings` in a loop against a dynamic swap loop writing alternating templates; no race report, and every published chain resolves the template current at publication | |
| `TestDynamicTemplatesCapturedAtConstruction` | same | AC-21: `createDynamicPeer` sets the `dyn*` pair from the template before the peer starts; an establishment after a filter swap resolves the swapped template, not the first one seen | |
| `TestDynamicFilterSwapResolvesAgainstLearnedAS` | `internal/component/bgp/reactor/reactor_api_dynamic_reload_test.go` | AC-22, R-14: replaces `TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges` (owner decision OQ-2 "A"); same `*Peer`, running chain resolved from the `PeerAS` current in the write section (including a peer that learned its AS between plan and write), `dyn*` pair holds the unresolved new template | |
| `TestDynamicFilterSwapRacingEstablishmentKeepsSession` | same | I8: a filter-only dynamic swap (no gained process) whose session reaches Established between plan and write keeps the session and restarts nothing; the same race with a gained process restarts (Data Flow step 3.3) | |
| `TestReloadFuncAppliesNoDynamicGroups` | same | AC-23: `VerifyConfig`, `peerDiffCount` and `peerSettingsFromReloadConfig` over a candidate dropping the group leave the peer and the group set untouched | |
| `TestOnPeerBoundTargetsNamedProcesses` | `internal/component/bgp/server/events_test.go` (or the file holding `onPeerUnbound` tests) | targeted up reaches only the named processes, reason `attached`, then `negotiated` only when granted and after the up | |
| `TestPeerBoundCarriesNoReplayOrBarrier` | same | AC-16: `InitialReplay` 0, no `SetPeerUpBarrier`, unheld roles computed over the full fed set (a `bgp-peer-up-replay` holder in the fed set but not in the gained set is reported held) | |
| `TestAdjRIBInSkipsReplayOnAttachedUp` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-16: an `attached` up relays nothing; an ordinary up still replays | |
| `TestRRSkipsReplayOnAttachedUp` | `internal/component/bgp/plugins/rr/rr_test.go` | R-11: an `attached` up starts no `replayForPeer` and does not bump `ReplayGen`; one test per further plugin the implementer's up-handler audit adds | |
| `TestSentDirectionGrantNeedsNothingAdvertised` | `internal/component/bgp/reactor/peer_derived_bindings_test.go` | AC-13: `update-sent`, bare `update` and `*` restart when `hasAdvertised`, swap when not | |
| `TestRouteFreeGrantNeedsNoCatchUp` | same | AC-14: `receive [ state ]` on a peer with routes and no Route Refresh swaps, no refresh; a plugin-registered type counts as route-bearing | |
| `TestUpAcknowledgedBeforeGraphEdge` | same | AC-4/R-4: the graph carries the gained edge only after the targeted up returned; a live UPDATE in the window does not reach the process | |
| `TestBindRacesEstablished` | same, under `-race` | AC-17, A-7: Established reached inside the window gives exactly one up or a restart | |
| `TestIdenticalOperatorBindingReplacingDerived` | same | AC-18 | |
| `TestNonStrictBFDProfileEditReopens` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-15 at the reactor: a profile edit releases then ensures with the new profile (fake `api.Service`) | |
| `TestBFDSwapRacesStrictNegotiation` | same, under `-race` | R-1: swap concurrent with `applyBFDStrictNegotiation` on an OpenConfirm peer and with the peer summary read | |
| `TestPinnedProfileAuthEditJoinsAgain` | `internal/component/bfd/bfd_test.go` (or the file holding `applyPinned` tests) | AC-25: a pinned session's profile gains authentication through `applyPinned`; a client refused before the edit joins after it; one session; red at HEAD (the edit does not reach the engine) | |
| `TestPinnedProfileTimerEditInPlace` | same | AC-25: a pinned session's profile transmit interval edited through `applyPinned` reaches the live machine with the same handle and local discriminator; red at HEAD | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | the feature takes no new numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rpki-attach-live` | `test/reload/bgp-rpki-attach-live.ci` (new; the D-6 shape: `attach process bgp-rpki` inside the merged file, `tcp_connections=1`) | AC-3 | |
| `bgp-bfd-added-live` | `test/reload/bgp-bfd-added-live.ci` (changed: `tcp_connections=1`, conn=2 expectation and the restart comment removed) | AC-5 | |
| `bgp-bfd-removed-live` | `test/reload/bgp-bfd-removed-live.ci` (new) | AC-6 | |
| `bgp-attach-receive-only-live` | `test/reload/bgp-attach-receive-only-live.ci` (new; peer expects a ROUTE-REFRESH and re-sends) | AC-4 | |
| `attach-process-reload` | `test/plugin/attach-process-reload.ci` (changed to one connection; the added `receive [ state ]` process must receive the targeted up and no ROUTE-REFRESH is sent) | AC-1, AC-14 | |
| `bgp-attach-grant-change-live` | `test/reload/bgp-attach-grant-change-live.ci` (new; SIGHUP; two peers, `bgp-rs` forwards A's route to B; the grant change on A's receive-only `bgp-rs` binding makes B watch A's route withdrawn and re-announced) | AC-8 | green 2026-10-11, 10/10 (scratch `gc7-1..10.log`). Discrimination: ze built with bin/ze's feature tags and an overlay setting `if len(rebound) > 0 && false` in `peer_settings_apply.go` (no intermediate publish, no targeted down): 3/3 RED, TYPE mismatch on peer B, which receives A's route a second time with no withdraw between (scratch `disc2-1..3.log`). An earlier 3/3 red (`disc-1..3.log`) was vacuous: that overlay binary was built without feature tags and answered `unknown command: start` |
| `bgp-attach-no-route-refresh-restarts` | `test/reload/bgp-attach-no-route-refresh-restarts.ci` (new; SIGHUP; the peer's OPEN carries no Route Refresh; two connections) | AC-10 | |
| `bgp-attach-sent-direction-restarts` | `test/reload/bgp-attach-sent-direction-restarts.ci` (new; SIGHUP after Ze announced a static route; two connections) | AC-13 | |
| `bgp-bfd-profile-edit-live` | `test/reload/bgp-bfd-profile-edit-live.ci` (new; one connection, the BFD session reopened under the new profile) | AC-15 | |
| `attach-process-runtime-subscribe` | `test/plugin/attach-process-runtime-subscribe.ci` (unchanged count: gains `open-received`; comment updated to name the OPEN rule) | AC-2 | |
| `bgp-rpki-added-live` | `test/reload/bgp-rpki-added-live.ci` (unchanged; its PREVENTS comment updated) | pre-bound shape stays green | |
| `bgp-bfd-dynamic-toggle-live` | `test/reload/bgp-bfd-dynamic-toggle-live.ci` (new; SIGHUP twice; the peer connects into a dynamic range; `tcp_connections=1`; `show bfd sessions` checked after each reload) | AC-19 | |
| `bgp-attach-dynamic-receive-only-live` | `test/reload/bgp-attach-dynamic-receive-only-live.ci` (new; SIGHUP; external process started by the reload sees `state up` reason `attached` before any UPDATE; one connection) | AC-9 | |
| `bgp-dynamic-filter-swap-live` | `test/reload/bgp-dynamic-filter-swap-live.ci` (new; SIGHUP; a `$remote_as` import chain changes; one connection; a route the peer sends AFTER the reload that the new chain rejects is not accepted, asserted with the dry-run pattern of `test/reload/reload-import-policy-applies.ci`; the route accepted before the reload stays, which the test asserts as the documented limitation R-21) | AC-22 | |
| `bgp-bfd-shared-session-widens` | `test/reload/bgp-bfd-shared-session-widens.ci` (new; a static route's unpinned BFD holds the session to the peer; a BGP peer with `connection local ip` X gains then loses a non-strict opt-in by reload (this spec's swap); then a second client pinned to local Y joins; `show bfd sessions` lists ONE session to that remote system) | AC-24 at the real entry point | |
| `reload-dynamic-peer-survives` | `test/reload/reload-dynamic-peer-survives.ci` (unchanged, kept green: the ReloadFunc no longer installs the groups, R-16) | regression | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-attach-receive-only-live-frr` | `test/interop/scenarios/bgp-attach-receive-only-live-frr/` (`ze.conf`, `ze-reload.conf`, `frr.conf`) | FRR | AC-12 first half: a receive-only attach reload keeps FRR's `connectionsEstablished` unchanged; FRR receives a ROUTE-REFRESH and re-sends | |
| `bgp-bfd-enable-live-frr` | `test/interop/scenarios/bgp-bfd-enable-live-frr/` | FRR (bgpd + bfdd) | AC-12 second half: BFD opt-in added, its profile edited, then removed by reload; BGP stays on one session; FRR `show bfd peers` Up after the add and after the edit, with Ze's new transmit interval | |
| `bgp-bfd-dynamic-enable-live-frr` | `test/interop/scenarios/bgp-bfd-dynamic-enable-live-frr/` | FRR (bgpd + bfdd), connecting into a Ze dynamic range | AC-19 against a foreign speaker: Ze's dynamic group template gains the non-strict opt-in, then loses it, by reload; FRR's `connectionsEstablished` unchanged after each; FRR `show bfd peers` Up after the add | |

Each scenario's red phase is recorded per `ai/rules/interop-and-goal-validation.md`: revert the swap (restore `derivedFeedsOnly` with its `Derived` condition, or drop BFD from the copier; for the dynamic scenario, make the dynamic step restart on any template difference, as `dynamicTemplateChanged` did), rebuild the Ze image, observe the counter advance (red), restore, observe green.

### RFC discrimination (RFC5882-4.4-1, AC-24)
| Step | What |
|------|------|
| Tag | `TestSharedSessionWidensOnRelease` carries `RFC requirement: RFC5882-4.4-1 positive` and the Section 4.4 quote, read in `rfc/full/rfc5882.txt` |
| Record | `./le rfc discriminate stem rfc5882 report <path>` proposes the break; `./le rfc discriminate-record id RFC5882-4.4-1 polarity positive unit TestSharedSessionWidensOnRelease route <unit-route> producer <the function that recomputes joined>` observes the unit red under the break (the recompute disabled, `joined` left narrowed) and writes `rfc/discrimination/rfc5882.json` |
| Existing records | the RFC5882-4.4-1 records already in `rfc/discrimination/rfc5882.json` (`TestBFDSharedSessionSameKey`, `TestBFDDistinctSessionsDifferentKey`) stay valid, or are re-recorded when their producer moves (`narrowKey` is rewritten) |
| Gate | `./le rfc check` green over the new tag and record |
| Re-judge | an independent reader (not the implementer) re-judges the RFC5882-4.4-1 verdict on the new evidence, as the parent spec's AC-9 requires |

## Files to Modify
- `internal/component/bgp/reactor/peer_derived_bindings.go` - predicate widened and renamed; catch-up widened (route-bearing grants only, sent-direction arm on `hasAdvertised`); the ordered bind delivery (up acknowledged before the edge is published, session check, intermediate publish for a changed grant); comments rewritten (stale-comments rule)
- `internal/component/bgp/reactor/peer_settings_apply.go` - `applyHotSwappableSettings` runs the ordered bind delivery instead of publish-then-catch-up; BFD joins `hotSwappableSettings` when non-strict on both sides; apply runs the BFD transition after locks are released; the long comment's field list updated
- `internal/component/bgp/reactor/peer_settings.go` - `ProcessBinding.Derived` and its comment deleted (I11)
- `internal/component/bgp/reactor/config.go` - `EnsureProcessBinding` stops setting `Derived`; its comment no longer calls the result a distinct kind of binding
- `internal/component/bgp/reactor/peer_bfd.go` - locked accessor for the opt-in; every reader moved to it; a transition helper reusing `startBFDClient`/`stopBFDClient`
- `internal/component/bgp/reactor/session_bfd_strict.go` - `applyBFDStrictNegotiation` and `raiseBFDStrictConfigChanged` read the opt-in through the locked accessor (I5)
- `internal/component/bgp/reactor/peer.go` - `Peer.Settings` immutability comment names `BFD` as a mutable field (the `dyn*` edits are listed below)
- `internal/component/bgp/reactor/reactor_dynamic.go` - `dynamicTemplateChanged` deleted; the reload's dynamic reconcile (remove, restart, swap through `peerSettingsSwapPlan` / `swapPeerSettingsJournaled` / `applyHotSwappableSettings`, with "next" resolved against the running peer) becomes the journaled dynamic step; `SetDynamicGroups` keeps only the group install the boot path needs, shared with the dynamic step; `createDynamicPeer` captures the `dyn*` pair; `resolveDynamicPeerSettings` reads and writes the pair under `p.mu` with the generation check, and its "touched by this function alone" comment is rewritten (stale-comments rule)
- `internal/component/bgp/reactor/peer.go` - the `dyn*` pair's comment names its two writers and its lock; the template generation field beside it
- `internal/component/bgp/reactor/reactor.go` - `ReloadFunc` returns the dynamic groups; `ReconcilePeersWithJournal` passes them to the reconcile
- `internal/component/bgp/config/loader_create.go` - `createReloadFunc` returns the groups and stops calling `SetDynamicGroups`; the boot path's call stays
- `internal/component/bgp/reactor/operation.go` - `peerSettingsFromReloadConfig` takes the new ReloadFunc result and ignores the groups
- `internal/component/bgp/reactor/reactor_api.go` - the peer summary's `BFDEnabled` reads the locked accessor (I5); the dispatcher interface gains the targeted up, if the interface lives here; `loadPeersFullOrTree` returns the groups; `reconcilePeersJournaled` runs the journaled dynamic step first after the globals; the dynamic branch's comment loses its citation of the deleted `plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md` and names the reason that still holds (a configured entry replacing a dynamic peer restarts because the running settings were resolved at establishment)
- `internal/component/bfd/engine/engine.go` - RFC5882-4.4-1 (Phase 4 step 1): the shared entry records each live client's requested key and each handle carries its client's key; `joined` is recomputed on every acquire and release as the session key narrowed by the remaining clients' keys, back to the session key when none remain (also for a released entry `sharedEntryLocked` matches); `narrowKey`'s comment and the `joined` field comment rewritten (stale-comments rule)
- `internal/component/bfd/engine/rfc5882_shared_session_test.go` - `TestSharedSessionWidensOnRelease`
- `internal/component/bfd/bfd.go` - AC-25 (Phase 4 step 3): `applyPinned` reconfigures an existing pinned key whose resolved request changed (timers, authentication) instead of skipping it
- `internal/component/bfd/engine/engine.go` - AC-25 (Phase 4 step 3): a live session takes new timers and authentication in place (today `Reconfigure` runs only in `reviveReleasedLocked`, and authentication is set only in `createSessionLocked` and `replaceAuth`); the shared-session rule Phase 4 step 3 decides
- `internal/component/bfd/bfd_test.go` (or the file holding `applyPinned` tests) - `TestPinnedProfileAuthEditJoinsAgain`, `TestPinnedProfileTimerEditInPlace`
- `rfc/discrimination/rfc5882.json` - the new record, and the existing RFC5882-4.4-1 records re-recorded if their producer moved
- `internal/component/bgp/reactor/reactor_api_dynamic_reload_test.go` - `TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges` replaced by `TestDynamicFilterSwapResolvesAgainstLearnedAS` (a change of the behaviour under test by owner decision, not a weakening); the other `TestSetDynamicGroups*` tests move to the dynamic step's entry point; the new rollback and ReloadFunc tests
- every test that installs a ReloadFunc (`reload_test.go`, `peer_settings_swap_test.go`, `peer_settings_reload_race_test.go`, `operation_test.go`, `peer_metrics_cleanup_test.go`, `reactor_metrics_behavioral_test.go`, `reactor_peers_takeover_test.go`, `reactor_api_dynamic_reload_test.go`; `bgp/config/loader_test.go`, `asn_notation_reload_test.go`) - the new signature
- `internal/component/bgp/server/event_dispatcher.go`, `internal/component/bgp/server/events.go` - `OnPeerBound`: targeted up (reason `attached`, `InitialReplay` 0, barrier off, unheld roles over the full fed set) and negotiated to named processes; `deliverPeerState` gains the barrier knob
- `pkg/plugin/rpc/enums.go` - the `attached` session reason beside `ReasonPeerRemoved`
- `internal/component/bgp/plugins/adj_rib_in/rib.go`, `internal/component/bgp/plugins/rr/rr.go`, and every further plugin the up-handler audit finds replaying toward the peer - skip the replay on an `attached` up
- `internal/component/bgp/reactor/peer_derived_bindings_test.go` - `TestOperatorBindingChangeStillRestarts` replaced (see Required Reading)
- `test/reload/bgp-bfd-added-live.ci` - one connection
- `test/reload/bgp-rpki-added-live.ci` - PREVENTS comment no longer says an attach restarts
- `test/plugin/attach-process-reload.ci` - one connection, comment
- `test/plugin/attach-process-runtime-subscribe.ci` - comment names the OPEN rule as the restart cause
- `internal/le/interoplab/bgp/` - checkers for the two scenarios (register file plus bespoke bodies and pure predicates)
- `docs/architecture/core-design.md` section 18a - the new rule, the ordered bind delivery, the sent-direction catch-up, the downstream flap of a changed grant (R-5), the BFD member; the sentence "A dynamic peer takes the same rule through its group template (`dynamicTemplateChanged`)" rewritten for the journaled dynamic step
- `docs/architecture/config/apply-ordering.md` - verify applies no dynamic group; the journaled dynamic step and its inverse (R-15); a dynamic peer named as the stated exception to "A remove-peer's inverse restores the peer the reactor was running" (N4)
- `docs/architecture/api/process-protocol.md` - the `attached` up (Documentation row 8)
- `docs/architecture/api/ipc_protocol.md` - "State events" and the text format: an up can carry reason `attached`
- `docs/architecture/plugin/rib-storage-design.md` - an `attached` up triggers no peer-up replay
- `features/bfd-liveness-detection.md` - Real-path tests and Interop rows
- `docs/guide/config-reload.md` - Change/Effect rows, including "changing a route-consuming process's grant withdraws and re-learns that peer's routes downstream"; for a dynamic group's import-chain edit, "session stays up; the new chain applies to routes received after the reload, and routes already accepted stay until the peer re-sends them" (R-21, naming `plan/immediate/spec-bgp-filtered-route-storage.md` AC-3 as the owner of re-import); a dynamic group's static-route edit is sent to each dynamic peer as UPDATEs
- `docs/guide/bfd.md` - live enable and removal on an Established peer
- `docs/guide/rpki.md` - attaching rpki live keeps the session
- `docs/architecture/testing/interop.md` - Scenario Inventory rows for both scenarios
- `features/configuration-reload.md` - Real-path tests gain the new `.ci` files
- `plan/spec-terminal-demo-showcase.md` - D-6 and D-8 closed by this work once it lands (the decision itself is already recorded there)

## Files to Create
- `test/reload/bgp-rpki-attach-live.ci` - AC-3
- `test/reload/bgp-bfd-removed-live.ci` - AC-6
- `test/reload/bgp-attach-receive-only-live.ci` - AC-4
- `test/reload/bgp-attach-grant-change-live.ci` - AC-8
- `test/reload/bgp-attach-no-route-refresh-restarts.ci` - AC-10
- `test/reload/bgp-attach-sent-direction-restarts.ci` - AC-13
- `test/reload/bgp-bfd-profile-edit-live.ci` - AC-15
- `test/interop/scenarios/bgp-attach-receive-only-live-frr/` - AC-12
- `test/interop/scenarios/bgp-bfd-enable-live-frr/` - AC-12
- `test/reload/bgp-bfd-dynamic-toggle-live.ci` - AC-19
- `test/reload/bgp-attach-dynamic-receive-only-live.ci` - AC-9
- `test/reload/bgp-dynamic-filter-swap-live.ci` - AC-22
- `test/reload/bgp-bfd-shared-session-widens.ci` - AC-24
- `test/interop/scenarios/bgp-bfd-dynamic-enable-live-frr/` - AC-19 interop

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new leaf: the existing `attach process` and `connection bfd` blocks change effect, not shape |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | `VerifyPeerBFDProfiles` already checks the profile name at commit |
| CLI commands/flags | N-A | no command added |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no new value |
| Functional test for new RPC/API | Yes | the targeted up is an event, proven by `test/reload/bgp-attach-receive-only-live.ci` |
| Pipe completeness | N-A | no output added |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency: BFD ports and the RTR cache are existing dependencies with their own checks |
| Prometheus counters/metrics | N-A | the existing route-refresh sent counter (`incrRefreshSent`) already counts the catch-up |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/configuration-reload.md` (Real-path tests gain every new `.ci`); `features/bfd-liveness-detection.md`: its Real-path tests row gains `test/reload/bgp-bfd-added-live.ci`, `bgp-bfd-removed-live.ci`, `bgp-bfd-profile-edit-live.ci`, and its Interop row gains `bgp/bgp-bfd-enable-live-frr` (the page makes no restart claim to correct) |
| 2 | Config syntax changed? | No | syntax unchanged |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | the `state` event shape is unchanged, but an up can now carry reason `attached`: `docs/architecture/api/ipc_protocol.md` "State events" and the text-format line, `pkg/plugin/rpc/enums.go` |
| 5 | Plugin added/changed? | No | plugin registrations unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/config-reload.md`, `docs/guide/bfd.md`, `docs/guide/rpki.md` |
| 7 | Wire format changed? | No | no message format change; only whether a Cease or a ROUTE-REFRESH is sent |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, "Event Delivery" section: a process attached to a running peer receives `state up` with reason `attached` (then `negotiated` if granted) before any message of that session, and a plugin that replays on up skips it for that reason |
| 9 | RFC behavior implemented, changed, or newly proven? | No page edit | the RFC 2918 catch-up and RFC 6811 Section 4 re-validation are newly exercised by a live attach, but neither `rfc/short/rfc2918.md` nor `rfc/short/rfc6811.md` cites the derived-only rule or a file this spec renames (grep, 2026-10-11); row 16's anchor run catches any anchor that moves |
| 10 | Test infrastructure changed? | No | existing `.ci` options and interop harness only |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` line 348 "Hot reconfiguration (no restart)" already reads Yes for Ze, FRR and BIRD; the row has no per-change granularity, so this spec makes it truer without changing a cell |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` section 18a |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | event types unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md`; at least core-design 18a (source anchors on `peer_settings_apply.go`, `peer_derived_bindings.go`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/rpki.md` troubleshooting row (line 597) and binding paragraph (line 241): say the attach takes effect without a session reset; `docs/guide/bfd.md` line 195 paragraph: both entry points, the live edit on a shared session (R-12), and that a dynamic group's opt-in change keeps its peers' sessions (OQ-1, option A); `docs/guide/config-reload.md`: rows for a dynamic group template change (swap like a configured peer; a removed or out-of-range dynamic peer still ends with Cease subcode 3), for a dynamic group's import-chain edit (routes received after the reload only, R-21) and for a rolled-back reload that had removed dynamic peers (R-15) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the three `.ci` entry tests fail for the right reason
   - Tests: `bgp-rpki-attach-live`, `bgp-bfd-added-live` at one connection, `bgp-attach-receive-only-live`
   - Files: the three `.ci` files
   - Verify: each fails because the second TCP connection is refused (the restart), not for a fixture reason
   - The wiring phase also writes `bgp-attach-grant-change-live`, `bgp-attach-no-route-refresh-restarts` (green today, kept green), `bgp-attach-sent-direction-restarts` (green today, kept green), `bgp-bfd-profile-edit-live`, and the three dynamic entry tests `bgp-bfd-dynamic-toggle-live`, `bgp-attach-dynamic-receive-only-live` and `bgp-dynamic-filter-swap-live` (each red today on the second connection a dynamic restart causes)
2. **Phase: receive-only bindings (configured peers)** -- widen the predicate, delete `Derived`, route-bearing and sent-direction catch-up, the ordered bind delivery (up acknowledged before the edge is published, session check, intermediate publish for a changed grant) in `applyHotSwappableSettings`, the `attached` reason and the plugins that skip replay on it. The dynamic path is left on `dynamicTemplateChanged` until Phase 3 deletes it, so this phase writes no dynamic delivery code that Phase 3 would replace. It FAILS CLOSED there (review r2 I2, R-22): `dynamicTemplateChanged` stops calling the binding predicate and restarts on any `ProcessBindings` difference, derived or operator. Without that, deleting `Derived` would let `setDynamicGroupsLocked` deliver operator receive-only bindings to dynamic peers through the old publish-then-catch-up order (no targeted up, graph before up, no sent-direction arm) in the Phase 2 commit. Chosen over landing Phases 2 and 3 in one commit because Phase 3 changes the ReloadFunc signature across a dozen test files and adds the journaled step, and a combined commit would carry two reviews' worth of risk; the fail-closed edit removes a few lines from a function Phase 3 deletes. Grep 2026-10-11: no unit test and no `.ci` pins a dynamic derived-feed swap on reload (`attach-process-dynamic-group.ci` runs no reload), so nothing goes red. The cost between the two commits is that a dynamic group's derived-feed change restarts its peers, which is today's answer for every other dynamic template change
   - Tests: `TestOperatorReceiveOnlyBindingSwaps`, `TestOperatorSendOrOpenBindingStillRestarts`, `TestChangedReceiveGrantIsDownThenUp`, `TestGainedReceiveOnlyBindingGetsUpBeforeRefresh`, `TestReceiveOnlyGainWithoutRouteRefreshRestarts`, `TestSentDirectionGrantNeedsNothingAdvertised`, `TestRouteFreeGrantNeedsNoCatchUp`, `TestUpAcknowledgedBeforeGraphEdge`, `TestBindRacesEstablished`, `TestIdenticalOperatorBindingReplacingDerived`, `TestOnPeerBoundTargetsNamedProcesses`, `TestPeerBoundCarriesNoReplayOrBarrier`, `TestAdjRIBInSkipsReplayOnAttachedUp`, `TestRRSkipsReplayOnAttachedUp`, `TestDynamicBindingChangeRestartsBeforeSharedSwap`
   - Files: `peer_derived_bindings.go`, `peer_settings_apply.go`, `peer_settings.go`, `config.go`, `event_dispatcher.go`, `events.go`, `pkg/plugin/rpc/enums.go`, `adj_rib_in/rib.go`, `rr/rr.go` and the audit's further plugins, `reactor_dynamic.go` (`dynamicTemplateChanged` fails closed)
   - Verify: first record the up-handler audit (which plugins replay toward the peer on up); unit tests red then green; `bgp-rpki-attach-live`, `bgp-attach-receive-only-live`, `bgp-attach-grant-change-live` green; A-1, A-2 and A-7 validated and recorded; section 18a, `process-protocol.md`, `ipc_protocol.md` and `rib-storage-design.md` edited in this phase
3. **Phase: dynamic peers join the shared swap (OQ-1 option A)** -- runs before the BFD phase, so the BFD member reaches dynamic peers through the copier with no dynamic-specific BFD code. In this order, each step red then green:
   1. The unlocked template pair: validate A-8 by grep and record it; `createDynamicPeer` captures `dynImportFilters` / `dynExportFilters` from the template before the peer starts; `resolveDynamicPeerSettings` reads and writes them only under `p.mu`, resolves with the lock released, and publishes only when the template generation is unchanged (Data Flow step 4); the "touched by this function alone" comment is rewritten. Tests: `TestDynamicTemplatesCapturedAtConstruction`, `TestDynamicTemplateSwapRacesEstablishment` (red under `-race` with the capture left unlocked and a swap writer in place), `TestDynamicPeerSettingsFilterReadRacesTheReload` kept green.
   2. The side-effect-free ReloadFunc: validate A-9 by grep and record it; `ReloadFunc` returns the groups, `createReloadFunc` stops calling `SetDynamicGroups`, every caller and test fake takes the new signature; the tree route answers "groups not read" (Data Flow step 1). Tests: `TestReloadFuncAppliesNoDynamicGroups` (red today: verify tears the peer down), `TestTreeRouteLeavesDynamicGroupsUnchanged`. Rollback scope: "rolled back" in this spec means the reload journal (`reconcilePeersJournaled`'s journal, AC-11, AC-20). The reload compensation path was read on 2026-10-11 (R-18): `compensateReload` -> `restoreReload` calls `ApplyConfigDiff(undo.before)`, `loadPeersFullOrTree` ignores that tree on the file route and calls the ReloadFunc, `storage.ReadReloadConfig` returns the staged candidate first, and the CLI and web commit routes clear the candidate only after the reload returned, so compensation re-applies the candidate. That is a pre-existing defect for configured peers, not covered here: passing `undo.before` through would change the file route's parse pipeline for every peer (`loadPeersFullOrTree`'s comment says the tree alone lacks YANG defaults, group inheritance and static routes), which is outside this spec. It is recorded as a row in `plan/journal/rollback-forgets-partial-apply.md`.
   3. The journaled dynamic step: `reconcilePeersJournaled` runs it first after the globals; removal and range checks as today; `peerSettingsSwapPlan` over the two template-built settings replaces `dynamicTemplateChanged` (deleted); the step plans under r.mu and runs its swaps, stops and catch-ups after r.mu is released; a swap goes through `swapPeerSettingsJournaled` and `applyHotSwappableSettings`, with "next"'s chains resolved in the write section under p.mu and the `dyn*` pair in the undo snapshot; the group install shared with the boot path's `SetDynamicGroups`. Tests: `TestDynamicTemplateReceiveOnlySwap` (with the second-pass no-op, N5), `TestFailingReloadRollsBackDynamicSwap`, `TestDynamicFilterSwapResolvesAgainstLearnedAS` (it replaces `TestSetDynamicGroupsRemovesThePeerWhenTheTemplateChanges`), `TestDynamicFilterSwapRacingEstablishmentKeepsSession`, the remaining `TestSetDynamicGroups*` tests moved to the step's entry point and kept green; `TestDynamicBindingChangeRestartsBeforeSharedSwap` deleted with `dynamicTemplateChanged`.
   - Files: `reactor_dynamic.go`, `peer.go`, `reactor.go`, `reactor_api.go`, `operation.go`, `bgp/config/loader_create.go`, the ReloadFunc test fakes, `reactor_api_dynamic_reload_test.go`, `reactor_dynamic_race_test.go`, `reactor_dynamic_test.go`
   - Verify: `bgp-attach-dynamic-receive-only-live`, `bgp-dynamic-filter-swap-live` and `reload-dynamic-peer-survives` green; A-8, A-9 recorded; `docs/architecture/config/apply-ordering.md`, the dynamic sentence of section 18a, and the dynamic rows of `docs/guide/config-reload.md` edited in this phase
4. **Phase: BFD opt-in (configured and dynamic peers)** -- locked accessor and every reader moved to it, BFD in the copier when non-strict both sides, transition after lock release, rollback. Dynamic peers take it through the Phase 3 step; no dynamic-only BFD path is written
   1. FIRST, RFC5882-4.4-1 in the BFD engine (owner: "fold it into keep-session"), before the BFD copier, because this phase's release-then-ensure (AC-15) and removal (AC-6) are the releases that leave a stale `joined` key. The shared entry records each live client's requested key, and each handle carries its client's key; `joined` is recomputed on every acquire and release as the session key narrowed by the remaining clients' keys, back to the session key when none remain, including for a released entry `sharedEntryLocked` matches. `narrowKey`'s comment and the `joined` field comment are rewritten. Test: `TestSharedSessionWidensOnRelease` red at HEAD, then green; `TestBFDSharedSessionSameKey` and `TestBFDDistinctSessionsDifferentKey` kept green. Then the discrimination record and the re-judge (RFC discrimination table), and `bgp-bfd-shared-session-widens.ci` once the BFD swap below exists. File: `internal/component/bfd/engine/engine.go`.
   2. The BFD opt-in swap.
   3. BFD profile edits reach live sessions (owner decision 2026-10-11, AC-25): first decide and record which client's profile a shared session takes when its clients name different profiles; then `applyPinned` reconfigures an existing pinned key whose resolved request changed, and the engine applies new timers and authentication to a live session in place. Tests: `TestPinnedProfileAuthEditJoinsAgain`, `TestPinnedProfileTimerEditInPlace`, each red at HEAD. Files: `internal/component/bfd/bfd.go`, `internal/component/bfd/engine/engine.go`. Not built yet.
   - Tests: `TestSharedSessionWidensOnRelease`, `TestNonStrictBFDOptInSwaps`, `TestStrictBFDChangeRestarts`, `TestBFDSwapOpensAndClosesClient`, `TestBFDSwapRacesEstablishedStart`, `TestBFDSwapRacesStrictNegotiation`, `TestNonStrictBFDProfileEditReopens`, `TestBFDSwapRollbackRestoresClient`, `TestDynamicNonStrictBFDOptInSwaps`, `TestDynamicStrictBFDChangeRestarts`; `TestFailingReloadRollsBackDynamicSwap` extended with the BFD opt-in (AC-20)
   - Files: `bfd/engine/engine.go`, `bfd/engine/rfc5882_shared_session_test.go`, `rfc/discrimination/rfc5882.json`, `peer_bfd.go`, `session_bfd_strict.go`, `reactor_api.go`, `peer_settings_apply.go`, `peer.go`
   - Verify: first record A-4 (a bfd plugin started by the same commit is ready, service and config, before the BGP apply, or not); if not, fix the transaction's apply order, never a watcher; `bgp-bfd-added-live`, `bgp-bfd-removed-live`, `bgp-bfd-profile-edit-live`, `bgp-bfd-dynamic-toggle-live`, `bgp-bfd-shared-session-widens` green; `./le rfc check` green with the RFC5882-4.4-1 record; `docs/guide/bfd.md`, `docs/guide/config-reload.md` and `features/bfd-liveness-detection.md` edited in this phase
5. **Phase: existing tests and docs** -- `attach-process-reload.ci`, `attach-process-runtime-subscribe.ci`, `bgp-rpki-added-live.ci` comments, `features/configuration-reload.md`, `docs/guide/rpki.md`
6. **Phase: interop** -- the three FRR scenarios with checkers, red phase recorded by reverting each half and rebuilding the image
   - Verify: `INTEROP_SCENARIO=<name> ./le test integration interop` green for each, red recorded with the swap reverted

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | the predicate reads the grant (send types, `open`, `*`), never a plugin name; the targeted up is acknowledged before the graph carries the edge and before the ROUTE-REFRESH; one up per session per process; the catch-up covers both directions and only route-bearing grants; strict BFD on either side restarts |
| Correctness | the targeted up carries reason `attached`, `InitialReplay` 0, arms no barrier, and its unheld roles come from the full fed set |
| Correctness | a binding judged swappable is always delivered: decision and apply share one copier, the BFD transition reads what the copier wrote |
| Data flow | the reactor reaches BFD only through `bfd/api`; no plugin learns a reload happened except through the targeted events |
| Rule: no-layering | `derivedFeedsOnly` is replaced, not kept beside the new predicate; `dynamicTemplateChanged` is deleted, not kept beside the dynamic step; `SetDynamicGroups` keeps no reconcile logic once the dynamic step owns it |
| Correctness (dynamic) | no ReloadFunc caller changes the running population; the dynamic step is recorded in the journal before it applies; no `dyn*` access outside `p.mu`; the copier never writes an unresolved `$remote_as` chain onto a peer that has learned its AS |
| Rule: stale-comments | the long comments in `peer_settings_apply.go` and `peer_derived_bindings.go`, and every `.ci` header that says an attach or BFD change restarts, are rewritten |
| Rule: goroutine-lifecycle | no new goroutine per peer or per reload; the BFD subscriber goroutine is the existing one started by `startBFDClient` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| receive-only attach keeps the session | `./le test functional reload` (selecting `bgp-rpki-attach-live` and `bgp-attach-receive-only-live`; the selector syntax is in `docs/functional-tests.md`) green with `tcp_connections=1` |
| BFD enable/remove keeps the session | `.ci` `bgp-bfd-added-live`, `bgp-bfd-removed-live` green with one connection |
| old pin replaced | `grep -n TestOperatorBindingChangeStillRestarts internal/` returns nothing; the two replacement tests exist |
| interop | both scenarios green, red recorded |
| docs | section 18a and the three guide pages carry the new rule with source anchors |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization | a binding that grants a send type can never be swapped live: the predicate tests `SendAll` and every `Send` key, and a send grant added to a kept binding restarts |
| Resource exhaustion | a reload loop that toggles a receive-only binding sends one ROUTE-REFRESH per family per reload, not per process; the BFD toggle releases its session each time (no handle leak: `TestBFDSwapOpensAndClosesClient` checks the fake service's refcount returns to zero) |
| Fail closed | any field or grant the plan cannot classify still restarts |

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

- The derived/operator distinction was standing in for the real question, which is whether the binding can change what the running session sends or was promised at OPEN time. The grant answers that directly.
- RPKI needs no up event (it acts only on down), but the generic receive-only class does: nine plugins build per-peer state from `state up`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Classify by grant (no send, no OPEN receive), for operator and derived bindings alike | special-case `bgp-rpki`; add a registry flag per plugin ("live attachable") | the grant is already the declaration of what the binding can do; a plugin-name case is a central enumeration (`ai/rules/principles.md`); a per-plugin flag would be a second declaration of what the grant says |
| Fix the reset in Ze rather than avoid it in the showcase | pre-bind every protocol plugin in the showcase base config (D-6 option a); show the BFD restart on a card (D-8 option a) | both alternatives rejected by the owner on 2026-10-11 ("yes like them"): it hides the reset rather than removing it, and cannot work for BFD where the opt-in is the feature |
| Changed grant on a kept binding = targeted down then up for that process | keep restarting (today's derived rule) | a restart resets every consumer and the peer to fix one consumer's view; down-then-up gives that consumer a clean start and touches nothing else |
| `open` or `*` receive grant restarts | replay the stored `peerOpen` to the gained process | a replayed OPEN is a fabricated event the process cannot tell from a real one; the restart is the honest answer and stays rare |
| BFD swaps only when non-strict on both sides | swap strict changes too | strict mode changes the OPEN (capability 74) and the OpenConfirm gate, which only a new session can apply; BIRD and FRR have no strict mode to compare with |
| If A-4 fails, fix the transaction's apply order for a newly started plugin bgp reads; no service watcher | a reactor-wide watcher on `api.SetService` | a watcher fires at boot too, which changes startup behaviour the owner did not ask for (main-thread decision on I6), and it would not cover `errBfdNotConfigured` anyway; the apply order is the guarantee the existing `bgp-bfd-profile-reload-peer-and-profile.ci` already proves for a running plugin |
| Targeted up is acknowledged before the delivery graph carries the gained edge, and is bound to the session current and Established when the apply began | republish then up (today's order); hold r.mu across the delivery | republish-first lets a live UPDATE reach the process before its up (I1); holding r.mu across a plugin delivery can deadlock against a plugin calling back into the reactor; a session change in the window restarts, which is rare and leaves no double or missing up (I2) |
| Targeted up carries reason `attached`, `InitialReplay` 0, no barrier, unheld roles over the full fed set; replaying plugins skip on `attached` | reuse the establishment up unchanged | the establishment up starts an initial routing update: replay toward the peer, barrier, readiness token. None of that belongs mid-session, and a receive-only binding cannot send the replay anyway (B2) |
| A grant covering updates Ze sent swaps only while Ze has advertised nothing; otherwise restart | ignore the sent direction (today); replay Ze's Adj-RIB-Out to the process | ignoring it leaves the process a partial copy (B1); a synthetic replay of sent UPDATEs is a fabricated event the process cannot tell from the wire, the same reason an `open` grant restarts |
| A changed grant is delivered with the process's edge removed from the graph between its down and its up | down and up with the edge in place | with the edge in place a live UPDATE can land between the down and the up |
| A dynamic peer's template change goes through the configured peer's plan, copier, ordered delivery and journal (owner, OQ-1: "A: yes, design it") | keep `dynamicTemplateChanged` and add a BFD arm to it; journal only the BFD part | a second diff beside `peerSettingsSwapPlan` is the layering the swap set's single copier exists to prevent, and a partial journal leaves the bindings it already writes with no undo |
| The plan compares the old and next TEMPLATE-built settings; "next" is resolved against the running peer before the copier runs | compare the running (resolved) settings with the next template | the running settings carry the learned `PeerAS` and resolved chains, so every comparison would report a change; the template pair is what `dynamicTemplateChanged` already compares |
| The `dyn*` template pair is captured at construction and touched only under `p.mu`, with a generation check on publication | keep the first-establishment latch; hold `p.mu` across `resolveFilterVars` | the latch can capture a chain a reload already replaced, and its "touched by this function alone" claim stops being true once a swap writes the pair; holding the lock across an allocating resolution is what the current code deliberately avoids |
| The ReloadFunc returns the dynamic groups and applies nothing; the dynamic step is the first journaled step of `reconcilePeersJournaled` | journal the `SetDynamicGroups` call where it is, inside the ReloadFunc | the ReloadFunc also runs at verify, at budget estimation and at an operation's candidate lookup, none of which may change the running population; a journal entry recorded at verify would undo a commit that never happened |
| A rolled-back dynamic step restores the group set and undoes swaps, and does not rebuild a removed dynamic peer | rebuild the removed peer from its old settings | no configuration names a dynamic peer and no connection exists to rebuild it on; the restored group accepts its next connection, which is what a dynamic peer's lifecycle already is |
| Delete `ProcessBinding.Derived` | keep it as information | once the predicate ignores it, it is write-only and still makes `reflect.DeepEqual` report a change for identical grants (I11); no-layering |

## Known Limitations
- A peer that did not advertise Route Refresh and already announced routes still restarts when a receive-only consumer is added (AC-10). This is the RFC 2918 Section 4 boundary, the same answer BIRD gives for a channel that is not reloadable, not outstanding work.
- A change that involves BFD strict mode still restarts (AC-7), by design (draft-ietf-idr-bgp-bfd-strict-mode Sections 6 and 8).
- A receive grant that includes the OPEN exchange still restarts (AC-2), by design (see Key Design Decisions).
- A grant that covers updates Ze sent (`update-sent`, bare `update`, `*`) still restarts once Ze has advertised on the session (AC-13): Ze cannot re-send its own UPDATEs to a new consumer without fabricating them.
- Changing the grant of a forwarding process (`bgp-rs`, `bgp-rr`) flaps that peer's routes once on the other sessions (R-5), instead of restarting this peer.
- A BFD profile edit on a session another client shares keeps the session's current parameters (R-12), the BFD plugin's sharing rule.
- A rolled-back reload that had removed dynamic peers (group dropped, range narrowed) does not bring them back; they return when they reconnect, under the restored group (R-15).
- An import-chain edit, on a configured peer as on a dynamic group (owner decision OQ-2 "A"), applies to routes received after the reload only. Routes the old chain already accepted stay in Adj-RIB-In until the peer re-sends them; for a dynamic group this replaces today's restart, which re-learned every route through the new chain (R-21, AC-22). FRR and BIRD re-evaluate. Re-import is owned by `plan/immediate/spec-bgp-filtered-route-storage.md` AC-3.
- "Rolled back" means the reload journal. The reload compensation path (`compensateReload`) re-applies the staged candidate rather than the previous config, for configured peers today and for dynamic groups after Phase 3 (R-18); recorded in `plan/journal/rollback-forgets-partial-apply.md`, not fixed here.

## Work Not Done

- A reload that newly declares an `external` program does not start it, so a receive-only binding to it delivers to no process. Owner decision (Thomas, 2026-10-11): a separate spec, not this one. Homed in `plan/immediate/spec-reload-starts-declared-external-programs.md`; journal row in `plan/journal/unwired-feature.md`. The two attach `.ci` declare their fixture in the boot config until it lands.

## Work Inherited From a Deferral Row

`plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md` (recorded
2026-08-08, read from `6fb9cd8814^`): "the dynamic-peer resolver reads the shared
settings struct unlocked, on a sole-writer claim the reload swap falsified". Its
row was closed on 2026-08-29 for the filter pair, which
`resolveDynamicPeerSettings` now reads under `p.mu.RLock`
(`TestDynamicPeerSettingsFilterReadRacesTheReload`), and the file went with the
whole directory on 2026-09-05. Its "Next step" asked a second question that the
closure did not answer: whether the `dyn*` template capture moves under `p.mu`
or is captured once at construction, with the sole-writer comment corrected
either way. That half is still open in the tree, and the dynamic swap this spec
adds is exactly the second writer that would falsify the comment. Phase 3
step 1 answers it (both: captured at construction, then touched only under
`p.mu`), and this spec retires the row. The reconcile comment in `reactor_api.go`
that still cites the deleted file is rewritten in the same phase.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The new catch-up call site carries `// RFC 2918 Section 4` (the quote stays on `requestRouteRefreshAll`); the BFD removal call site carries `// RFC 5882 Section 8`; the BFD engine function that recomputes `joined` carries the RFC 5882 Section 4.4 quote ("If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session."), and its callers in `EnsureSession` and `ReleaseSession` carry `// RFC 5882 Section 4.4`; the restart branch keeps its RFC 4486 Section 4 reference, and so do the two new restart decisions inside the apply (sent direction already advertised, session changed inside the window).

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
- [ ] AC-1..AC-24 all demonstrated
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
- [ ] Boundary tests for all numeric inputs (N-A: no numeric input)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md` only, in the same `./le commit create` script
