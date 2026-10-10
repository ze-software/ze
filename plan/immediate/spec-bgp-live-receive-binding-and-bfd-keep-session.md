# Spec: bgp-live-receive-binding-and-bfd-keep-session

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
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
   BFD client opens or closes, and the BGP session stays up. For a dynamic peer
   the owner decides (Open owner question OQ-1).
5. A change that involves strict mode on either side still restarts, because
   strict mode changes the OPEN (capability 74) and the OpenConfirm gate
   (draft-ietf-idr-bgp-bfd-strict-mode Sections 6 and 8).
6. Everything else stays as it is: the swap set is a subtraction from the whole
   `PeerSettings` struct, and an unclassified field still restarts.

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

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6811.md` - origin validation on newly available data
  → Constraint: RFC 6811 Section 4 (read in `rfc/full/rfc6811.txt`): "When a mapping is added or deleted, the implementation MUST re-validate any affected prefixes and run the BGP decision process if needed." The RFC asks for re-validation, never for a session reset; FRR implements it by re-running `bgp_update` over `adj_in` (`revalidate_bgp_node`, `bgpd/bgp_rpki.c`).
- [ ] `rfc/short/rfc2918.md` - ROUTE-REFRESH as the catch-up for a gained consumer
  → Constraint: RFC 2918 Section 4: "A BGP speaker may send a ROUTE-REFRESH message to its peer only if it has received the Route Refresh Capability from its peer." Without it, a gained consumer of a peer that already announced routes still restarts (`feedCatchUpImpossible`).
- [ ] `rfc/short/rfc5882.md` - BFD client behaviour on configuration and deconfiguration
  → Constraint: RFC 5882 Section 8 (read in `rfc/full/rfc5882.txt`): "the system on which BFD is being deconfigured SHOULD put the session into AdminDown state and maintain this state for a Detection Time to ensure that the remote system is aware of it." Removing the opt-in live goes through `stopBFDClient` -> `ReleaseSession`, the path the Established exit already uses; the spec does not change what the BFD plugin sends on release (A-5).
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
  → Decision: `bfd` is the last field of `struct bgp_config`, after `password`, and the byte comparison that decides whether the configuration is the same stops at the `password` offset, so it never sees `bfd`. A BFD change therefore keeps the session, and `bgp_update_bfd` requests, updates or frees the BFD session in place. That function does nothing for a dynamic BGP instance (`bgp_is_dynamic`): BIRD does not run BFD for dynamic BGP at all (input to the open owner question on AC-9).

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
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` `dynamicTemplateChanged` - a dynamic peer's template diff
  → Constraint: it calls `onlyDerivedFeedsDiffer` and `derivedFeedsDeliverable` directly; the renamed predicate reaches dynamic peers through the same calls, so a dynamic group's receive-only attach change also stays up, through the same ordered bind delivery as a configured peer (I1, I2).
  → Constraint (I7): BFD on a dynamic peer has no design yet. `setDynamicGroupsLocked` writes a dynamic peer's bindings directly (`peer.settings.ProcessBindings = feeds`), outside the settings copier and the reload journal, and the reconcile loop in `reactor_api.go` states `applyHotSwappableSettings` must not run on a dynamic peer because `resolveDynamicPeerSettings` writes settings from the establishment goroutine (`plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md`). Any other template change, a BFD opt-in included, restarts the dynamic peer today (`dynamicTemplateChanged`). The BFD half of AC-9 waits on the open owner question below.
- [ ] `internal/component/bgp/reactor/peer_bfd.go` - `bfdStrict`, `startBFDClient`, `stopBFDClient`, `bfdRequestFor`, `runBFDSubscriber`
  → Constraint: `startBFDClient` is idempotent and returns early on a nil or disabled opt-in; `stopBFDClient` is idempotent. The live swap calls them; it adds no third way to open or close a BFD client.
  → Constraint: `startBFDClient` with no published service (`api.GetService() == nil`) logs a warning and leaves a non-strict peer without BFD. In a commit that adds the `bfd` root AND the opt-in, the bfd plugin publishes its service in `OnStarted` (`internal/component/bfd/bfd.go`, `api.SetService`), so the order of the plugin start and the BGP apply decides whether the swap finds a service (A-4).
  → Constraint (I6): a published service is not enough. `pluginService.EnsureSession` (`internal/component/bfd/bfd.go`) refuses with `errBfdNotConfigured` while the bfd plugin's config is not applied, and with `errBfdDisabled` while `bfd > enabled` is false. When the bfd plugin is ALREADY running, `test/reload/bgp-bfd-profile-reload-peer-and-profile.ci` proves bfd applies its root before bgp asks (no "profile ... is not defined" refusal). The only open case is a bfd plugin STARTED by the same commit: A-4 covers both the service and the applied config for it.
- [ ] `internal/component/bgp/reactor/session_bfd_strict.go` `applyBFDStrictNegotiation`, `raiseBFDStrictConfigChanged`, `bfdStrictConfigChanged`; `internal/component/bgp/reactor/reactor_api.go` (the peer summary that sets `BFDEnabled`, and the restart branch of `reconcilePeersJournaled`)
  → Constraint (I5, verified by grep of `.BFD` under `internal/component/bgp/reactor/`): the unlocked readers of `PeerSettings.BFD` are `Peer.bfdStrict` (`peer_bfd.go`, reached from `peer_run.go` at the strict pre-FSM start and at the Established exit), `startBFDClient` (`peer_bfd.go`), `bfdRequestFor` (`peer_bfd.go`, reads `s.BFD` fields), `Session.applyBFDStrictNegotiation` and `Session.raiseBFDStrictConfigChanged` (`session_bfd_strict.go`, under `s.mu`, which does NOT exclude a writer holding `p.mu`), and the peer summary's `BFDEnabled` (`reactor_api.go`, for `show`). `verifyPeerBFDProfiles` and `bfdStrictConfigChanged` read candidate settings, not a running peer's, and stay unlocked. The swap runs on peers in OpenSent or OpenConfirm concurrently with `applyBFDStrictNegotiation`, so every running-peer reader moves to the p.mu-guarded accessor.
  → Constraint (NOTE 3, verified): Event 35 (`raiseBFDStrictConfigChanged`) is raised only on the restart branch of `reconcilePeersJournaled`; a non-strict swap never raises it.
- [ ] `internal/component/bfd/engine/engine.go` `Loop.EnsureSession`, `reviveReleasedLocked`
  → Constraint (NOTE 4, verified): a request for a session that is live and shared with another client joins it through `Acquire` and its new parameters are ignored; a request for a released session still inside its Detection Time is revived WITH the new request's parameters. So a non-strict profile edit (release then ensure) takes effect when BGP was the session's only client, and leaves the shared session on its current parameters when another client (static route, OSPF) holds it. That is the BFD plugin's existing sharing rule (RFC 5882 Section 4.4), not something this spec changes; R-12 and AC-15 record it.
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
- BFD on a dynamic peer: owner question open (AC-9).
- FRR and BIRD both keep the session for both changes; their behaviour is described above.

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
- A non-strict BFD opt-in change is swapped live on a configured peer; the BFD client opens or closes on the running session. A dynamic peer's BFD opt-in change keeps restarting until the owner answers the open question.
- A plugin that replays toward a peer on up skips the replay on an `attached` up.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config commit (SSH editor `commit`, `ze config` apply, or SIGHUP reload) carrying a changed `bgp peer <name> attach process <p>` block or `bgp peer <name> connection bfd` block, and possibly a new `bfd` root.
- Format at entry: the resolved BGP tree, parsed into `PeerSettings` (`parseProcessBindingsFromTree`, the BFD block parser in `config.go`).

### Transformation Path
1. Config transaction: verify (`VerifyPeerBFDProfiles` for a BFD profile name), then apply; plugins named by a new root or a new `attach`/receive type start during the transaction.
2. Reactor reconcile (`reconcilePeersJournaled`, `reactor_api.go`): per peer, `peerSettingsSwapPlan(current, next, session)` returns a copier or a restart reason.
3. Swap: `swapPeerSettingsJournaled` records the undo from the running peer, then `applyHotSwappableSettings` runs the ordered bind delivery below. The dynamic path (`SetDynamicGroups`) runs the same steps through the same helper, so neither caller republishes before the up.
   1. Under p.mu, read the current session S and whether the peer is Established on it; compute gained and lost from the running bindings and next. No write yet.
   2. Locks released: when S is Established and the gained list is not empty, deliver the targeted up (reason `attached`, `InitialReplay` 0, no barrier, unheld roles over the full fed set), then the targeted negotiated to the gained processes granted it. Delivery is synchronous (`deliverPeerState` waits for each result), so every gained process has acknowledged its up before step 3. The graph still lacks the gained edges, so no UPDATE reaches them.
   3. Take r.mu then p.mu. Check that the peer's current session is still S and that its Established state is the one step 1 saw. If yes: write the copied fields, republish the graph when bindings changed, release. If no (S closed, or the peer reached Established inside the window): write and republish all the same, release, deliver a targeted down to the gained processes that step 2 told up (so none holds an up with no down), and restart the session with Cease subcode 6. That window is a reload racing an FSM transition, rare, and a restart is the answer that leaves no process with a double or a missing up (I2).
   4. Locks released: targeted down to the lost processes (`OnPeerUnbound`), then the catch-up for the gained processes whose grant carries routes, re-decided now: sent direction, `hasAdvertised` true restarts; received direction, `feedCatchUp` gives nothing, one ROUTE-REFRESH per negotiated family, or restart.
   5. A kept binding with a changed grant is lost-then-gained for that one process, and its edge must be absent from the graph while it is told down and then up, or a live UPDATE could reach it between the two. So, before step 2, the apply writes the running binding list with that process's binding left out (under r.mu then p.mu, the same write discipline), republishes, releases the locks and delivers the targeted down to it; from step 2 on it is a gained process. The journal's undo is the snapshot taken before the apply, so a rollback never restores the intermediate list. With S not Established at step 1 the process gets no targeted down or up: the FSM's own up reaches it later through the graph.
   6. With S not Established at step 1 and still not Established at step 3, no targeted up is delivered: the graph carries the edge before the FSM's up, which reaches the gained process like any other.
4. BFD transition, after the binding steps, locks released: opt-in gained or edited and the peer Established: `stopBFDClient` (when one is open and the request changed) then `startBFDClient`; opt-in lost or suspended: `stopBFDClient`.
5. Restart branch unchanged: the peer is removed and re-added with a Cease subcode 6.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | `PeerSettings` diff in `reconcilePeersJournaled` | No |
| Reactor -> Plugin server | `EventDispatcher.OnPeerUnbound` and the new targeted bound event, delivery graph republish | No |
| Reactor -> BFD component | `api.GetService().EnsureSession` / `ReleaseSession` through `startBFDClient` / `stopBFDClient` | No |
| Ze -> BGP peer (wire) | no NOTIFICATION; ROUTE-REFRESH per negotiated family when a gained consumer needs the peer's routes | No |

### Integration Points
- `hotSwappableSettings` - gains the BFD field (conditional on non-strict both sides) and the widened binding predicate.
- `peerSettingsSwapPlan` - unchanged structure; the neutralization now covers the new members.
- `EventDispatcher` - gains the targeted up (`OnPeerBound`), sibling of `OnPeerUnbound`; `deliverPeerState` gains the barrier knob, not a second delivery function.
- `SetDynamicGroups` - uses the same ordered bind delivery as `applyHotSwappableSettings`.
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
| A-6 | FRR, as the interop peer, answers ROUTE-REFRESH and keeps `connectionsEstablished` unchanged across a Ze reload that sends one | FRR `peer_on_policy_change` and RFC 2918 support; the `rtr-stayrtr` checker reads the same counter | the continuity check would be measuring something else | AC-7 scenario run, counter read before and after | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Writing `PeerSettings.BFD` from the reload goroutine races the FSM callback, the subscriber, the strict negotiation and `show`, which read it with no lock today | `-race` failure in the new swap unit test | move every running-peer reader to a p.mu-guarded accessor before the field joins the swap set: `bfdStrict` (and through it `peer_run.go` at the strict pre-FSM start and the Established exit), `startBFDClient`, `bfdRequestFor`'s caller, `applyBFDStrictNegotiation` and `raiseBFDStrictConfigChanged` (`session_bfd_strict.go`, which hold `s.mu` only), and the peer summary's `BFDEnabled` (`reactor_api.go`). A `-race` test runs the swap concurrently with `startBFDClient` and with `applyBFDStrictNegotiation` on a peer in OpenConfirm |
| R-2 | The apply opens BFD while the FSM callback for Established also opens it | two `EnsureSession` calls, an orphaned subscriber | `startBFDClient` already serialises on `p.bfd.starting`; the swap reuses it and adds no second open path |
| R-3 | A bfd plugin started by the same commit is not ready when the BGP apply runs (A-4) | nil-service warning or `errBfdNotConfigured` refusal in the AC-5 test log, empty `show bfd sessions` | fixed in the transaction's apply order for a newly started plugin that bgp reads, never by a watcher; a boot where BGP reaches Established before bfd is out of scope (main-thread decision on I6) and stays as it is |
| R-4 | A gained process receives an UPDATE (live, or from the ROUTE-REFRESH) before its targeted up | process sees UPDATE for an unknown peer | the up is delivered and acknowledged before the graph carries the edge (Data Flow step 3.2), and the ROUTE-REFRESH is sent after the republish; AC-4 asserts the order |
| R-5 | A changed grant delivered as down-then-up makes the consumer drop the peer and, for `bgp-rib` and `bgp-rr`, withdraw that peer's routes from EVERY OTHER peer on the down (`emitPurgedWithdraws`, `rib/rib_bestchange.go`; `handleStateDown`, `rr/rr.go`), then re-announce them only as the ROUTE-REFRESH re-sends them. Other sessions see a flap of this peer's routes | AC-8 `.ci` reads a second peer's wire | accepted as the price of not restarting: a restart flaps the same routes on the same sessions AND bounces this peer. The spec does not hide it: section 18a and `docs/guide/config-reload.md` state that changing the grant of a route-consuming process withdraws and re-learns that peer's routes downstream; AC-8 asserts the withdraw and the re-announce on the other peer's wire |
| R-9 | A peer moving to Established between the plan and the republish gives a gained process two ups (FSM's plus targeted) or none | `rr` `ReplayGen` bumped twice, `bmp` peer-up recorded twice; or a process with routes and no up | the targeted up is bound to the session current and Established at step 1, and step 3 restarts when that changed (Data Flow); A-7; `TestBindRacesEstablished` |
| R-10 | A gained process granted updates in the sent direction misses what Ze already advertised (B1) | process's Adj-RIB-Out view lacks routes Ze sent | sent-direction arm: `hasAdvertised` true restarts; AC-13 |
| R-11 | A targeted up makes a plugin replay toward the peer or report readiness for an initial update it never saw (B2) | "peer-up replay failed" log, `bgp-rs` End-of-RIB barrier re-armed, a session-ready signal mid-session | targeted up carries reason `attached`, `InitialReplay` 0, no barrier, unheld roles over the full fed set; replaying plugins skip on `attached`; AC-16 |
| R-12 | A non-strict profile edit on a BFD session shared with another client (static route, OSPF) keeps the session's old parameters (`Acquire` ignores the new request, `engine.go`) | `show bfd sessions` still shows the old timers after a profile edit | the BFD plugin's existing sharing rule (RFC 5882 Section 4.4), unchanged by this spec; documented in `docs/guide/bfd.md` beside the live-edit paragraph; AC-15 asserts the unshared case takes the new profile |
| R-6 | Existing functional tests expect two connections for a receive-only attach change and now fail | `attach-process-reload.ci` red | update their expectations and comments in the same change (listed in Files to Modify); `attach-process-runtime-subscribe.ci` gains `open-received` and keeps two connections |
| R-7 | The rollback undo of a BFD swap leaves a BFD client open or closed against the restored settings | after a rolled-back commit, `show bfd sessions` disagrees with the config | the undo goes through the same copier and the same BFD transition step as the apply; a unit test rolls back a swap and checks the client state |
| R-8 | A peer without Route Refresh that already announced routes still restarts when RPKI is attached live | two connections against such a peer | intended: the catch-up rule is unchanged and documented; the AC-3 test uses a peer that advertises Route Refresh, and AC-10 pins the restart for one that does not |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a consumer process holds an incomplete view of a peer (missing routes, missing or doubled up, a partial copy of what Ze sent), a plugin replays toward a peer it may not send to, or a BFD client is left open or closed against the config; at worst a session left running where it should have restarted |
| What changes even when it is right? | a changed grant on a kept route-consuming binding (`bgp-rib`, `bgp-rr`) withdraws that peer's routes from every other peer on the targeted down and re-announces them as the ROUTE-REFRESH re-sends them (R-5). The other sessions stay up; their routes from this peer flap once. Today the same edit restarts this peer and flaps the same routes |
| How is it reverted? | single commit revert; no config migration, no wire format change |
| Who else touches this path? | `plan/immediate/spec-bgp-reload-rich-peer-no-bounce.md` (reload equality for rich peers, same `peerSettingsEqual` decision), `plan/spec-terminal-demo-showcase.md` (RPKI and BFD topics wait on this), BFD strict-mode reload work (`bgp-bfd-strict-reload-speaker`) |

## Open owner question

**OQ-1, BFD on dynamic peers (AC-9).** A dynamic peer's group template can gain,
lose or edit a non-strict `connection bfd` opt-in. Today that restarts every
dynamic peer of the group, and this spec has no design that keeps them up: the
dynamic path writes a dynamic peer's settings directly, outside the settings
copier and the reload journal (`setDynamicGroupsLocked`, `reactor_dynamic.go`),
so it has no rollback, and `applyHotSwappableSettings` must not run on a dynamic
peer because `resolveDynamicPeerSettings` writes settings from the establishment
goroutine (`plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md`).

| Option | What it means | What it costs |
|--------|---------------|---------------|
| A: design it in this spec | the dynamic path gains the BFD transition, an undo in the reload journal, and the locked access the deferral asks for, so a dynamic peer's BFD opt-in change keeps the session like a configured peer's | one more implementation phase in this spec: fixing the deferral's unlocked read first, journaling dynamic swaps (today they have no rollback at all), and a dynamic-peer rollback test; the spec grows by roughly the BFD half again |
| B: drop BFD from AC-9 | a dynamic peer's BFD opt-in change keeps restarting that dynamic peer, documented as a known limitation; the receive-only binding half of AC-9 stays | no code; one Known Limitations line and one sentence in `docs/guide/bfd.md`. BIRD has no live BFD change on dynamic BGP either: it does not run BFD for a dynamic instance at all (`bgp_update_bfd` returns early for one) |

Which do you want? Until the owner answers, the BFD half of AC-9 is marked
pending and is neither implemented nor dropped.

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
| SIGHUP reload against FRR | → | the same swap, observed by a foreign speaker | interop `bgp-attach-receive-only-live-frr`, `bgp-bfd-enable-live-frr` |

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
| AC-8 | Peer A (Route Refresh advertised) announced a route that `bgp-rib` selects and sends to peer B; a SIGHUP reload changes A's kept `bgp-rib` binding grant (for example `[ update-received ]` to `[ update-received state ]`) | one TCP connection to A and one to B; the process receives a targeted down then a targeted up (reason `attached`) for A, and no UPDATE between them; no other process receives either; Ze sends A one ROUTE-REFRESH per negotiated family; B's wire shows A's route withdrawn and then announced again (R-5) |
| AC-9 | A dynamic peer whose group template gains a receive-only attach block | the dynamic peer's session stays up and the change is delivered, as for a configured peer. **BFD part pending OQ-1:** a non-strict BFD opt-in gained by the template either keeps the session (option A) or keeps restarting it, documented (option B) |
| AC-10 | A running peer that did NOT advertise Route Refresh and announced routes; a SIGHUP reload adds a receive-only binding granting `update-received` | the peer restarts (Cease subcode 6), unchanged from today; the restart log line names `ProcessBindings` |
| AC-11 | A swapped reload (receive-only attach and BFD opt-in) that the transaction rolls back | the binding set and the BFD client return to their pre-commit state with no restart |
| AC-12 | Interop: FRR peer Established with Ze; Ze is reloaded with a receive-only attach, then with a non-strict BFD opt-in, then with the opt-in's profile edited (different transmit interval), then with the opt-in removed | FRR's `connectionsEstablished` for the Ze neighbor is unchanged after each reload; after the BFD reload FRR `show bfd peers` lists Ze's session Up; after the profile edit the session is still Up and FRR reports Ze's new transmit interval; after the removal FRR's BGP session to Ze is still Established (A-5) |
| AC-13 | A running peer to which Ze has advertised a route; a reload adds a binding granting `receive [ update-sent state ]` (or bare `update`, which is granted in both directions) | the peer restarts (Cease subcode 6): Ze cannot re-send what it sent to the new process's view. The same reload on a session where Ze has advertised nothing keeps one connection |
| AC-14 | A running peer that announced routes and did NOT advertise Route Refresh; a reload adds a binding granting only `receive [ state ]` | one TCP connection; the process receives the targeted up; no ROUTE-REFRESH is sent and no restart happens, because the grant carries no route |
| AC-15 | A running non-strict peer whose BFD session has no other client; a commit changes the opt-in's profile | one TCP connection; `show bfd sessions` shows the session under the new profile's timers. With a second client (a static route's BFD) sharing the session, the session keeps its parameters and the BGP session still stays up (R-12) |
| AC-16 | A gained receive-only binding for `adj-rib-in` on a peer, mid-session, on a daemon with no `bgp-peer-up-replay` claimant, and a reload during that peer's initial sync | the targeted up carries `InitialReplay` 0 and reason `attached`; `adj-rib-in` sends nothing toward the peer and logs no "peer-up replay failed"; no peer-up barrier is armed (`SetPeerUpBarrier` not called) |
| AC-17 | A reload that adds a receive-only binding while the peer is in OpenConfirm and reaches Established inside the apply window | the gained process receives exactly one `state up` for that session, or the session restarts; never two ups and never routes without an up |
| AC-18 | An operator `attach process` block identical in grant to the derived binding it replaces (the config builder no longer adds its own) | no restart and no targeted down or up: `ProcessBinding.Derived` no longer exists to make the two differ |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | binds RPKI to a live peer and enables validation in one commit | SSH editor -> commit -> reconcile -> swap -> targeted up -> rpki revalidates retained routes | `test/reload/bgp-rpki-attach-live.ci` |
| 2 | enables BFD on a live peer | SSH editor -> commit -> bfd plugin start -> reconcile -> swap -> `startBFDClient` | `test/reload/bgp-bfd-added-live.ci` |
| 3 | attaches a monitoring process to a live peer | SIGHUP -> reconcile -> swap -> targeted up -> ROUTE-REFRESH -> peer re-sends -> process | `test/reload/bgp-attach-receive-only-live.ci` |
| 4 | does the same against FRR | SIGHUP -> swap -> FRR keeps its session | interop `bgp-attach-receive-only-live-frr`, `bgp-bfd-enable-live-frr` |

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
| `TestDynamicTemplateReceiveOnlySwap` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-9 binding half, through the same ordered bind delivery (up before republish); the BFD half's test is named after OQ-1 is answered | |
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
| `bgp-attach-grant-change-live` | `test/reload/bgp-attach-grant-change-live.ci` (new; SIGHUP; two peers, the second watches the first's route withdrawn and re-announced) | AC-8 | |
| `bgp-attach-no-route-refresh-restarts` | `test/reload/bgp-attach-no-route-refresh-restarts.ci` (new; SIGHUP; the peer's OPEN carries no Route Refresh; two connections) | AC-10 | |
| `bgp-attach-sent-direction-restarts` | `test/reload/bgp-attach-sent-direction-restarts.ci` (new; SIGHUP after Ze announced a static route; two connections) | AC-13 | |
| `bgp-bfd-profile-edit-live` | `test/reload/bgp-bfd-profile-edit-live.ci` (new; one connection, the BFD session reopened under the new profile) | AC-15 | |
| `attach-process-runtime-subscribe` | `test/plugin/attach-process-runtime-subscribe.ci` (unchanged count: gains `open-received`; comment updated to name the OPEN rule) | AC-2 | |
| `bgp-rpki-added-live` | `test/reload/bgp-rpki-added-live.ci` (unchanged; its PREVENTS comment updated) | pre-bound shape stays green | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-attach-receive-only-live-frr` | `test/interop/scenarios/bgp-attach-receive-only-live-frr/` (`ze.conf`, `ze-reload.conf`, `frr.conf`) | FRR | AC-12 first half: a receive-only attach reload keeps FRR's `connectionsEstablished` unchanged; FRR receives a ROUTE-REFRESH and re-sends | |
| `bgp-bfd-enable-live-frr` | `test/interop/scenarios/bgp-bfd-enable-live-frr/` | FRR (bgpd + bfdd) | AC-12 second half: BFD opt-in added, its profile edited, then removed by reload; BGP stays on one session; FRR `show bfd peers` Up after the add and after the edit, with Ze's new transmit interval | |

Each scenario's red phase is recorded per `ai/rules/interop-and-goal-validation.md`: revert the swap (restore `derivedFeedsOnly` with its `Derived` condition, or drop BFD from the copier), rebuild the Ze image, observe the counter advance (red), restore, observe green.

## Files to Modify
- `internal/component/bgp/reactor/peer_derived_bindings.go` - predicate widened and renamed; catch-up widened (route-bearing grants only, sent-direction arm on `hasAdvertised`); the ordered bind delivery (up acknowledged before the edge is published, session check, intermediate publish for a changed grant); comments rewritten (stale-comments rule)
- `internal/component/bgp/reactor/peer_settings_apply.go` - `applyHotSwappableSettings` runs the ordered bind delivery instead of publish-then-catch-up; BFD joins `hotSwappableSettings` when non-strict on both sides; apply runs the BFD transition after locks are released; the long comment's field list updated
- `internal/component/bgp/reactor/peer_settings.go` - `ProcessBinding.Derived` and its comment deleted (I11)
- `internal/component/bgp/reactor/config.go` - `EnsureProcessBinding` stops setting `Derived`; its comment no longer calls the result a distinct kind of binding
- `internal/component/bgp/reactor/peer_bfd.go` - locked accessor for the opt-in; every reader moved to it; a transition helper reusing `startBFDClient`/`stopBFDClient`
- `internal/component/bgp/reactor/session_bfd_strict.go` - `applyBFDStrictNegotiation` and `raiseBFDStrictConfigChanged` read the opt-in through the locked accessor (I5)
- `internal/component/bgp/reactor/peer.go` - `Peer.Settings` immutability comment names `BFD` as a mutable field
- `internal/component/bgp/reactor/reactor_dynamic.go` - `SetDynamicGroups` / `setDynamicGroupsLocked` use the ordered bind delivery instead of publish-then-catch-up (I1, I2); the BFD half waits on OQ-1
- `internal/component/bgp/reactor/reactor_api.go` - the peer summary's `BFDEnabled` reads the locked accessor (I5); the dispatcher interface gains the targeted up, if the interface lives here
- `internal/component/bgp/server/event_dispatcher.go`, `internal/component/bgp/server/events.go` - `OnPeerBound`: targeted up (reason `attached`, `InitialReplay` 0, barrier off, unheld roles over the full fed set) and negotiated to named processes; `deliverPeerState` gains the barrier knob
- `pkg/plugin/rpc/enums.go` - the `attached` session reason beside `ReasonPeerRemoved`
- `internal/component/bgp/plugins/adj_rib_in/rib.go`, `internal/component/bgp/plugins/rr/rr.go`, and every further plugin the up-handler audit finds replaying toward the peer - skip the replay on an `attached` up
- `internal/component/bgp/reactor/peer_derived_bindings_test.go` - `TestOperatorBindingChangeStillRestarts` replaced (see Required Reading)
- `test/reload/bgp-bfd-added-live.ci` - one connection
- `test/reload/bgp-rpki-added-live.ci` - PREVENTS comment no longer says an attach restarts
- `test/plugin/attach-process-reload.ci` - one connection, comment
- `test/plugin/attach-process-runtime-subscribe.ci` - comment names the OPEN rule as the restart cause
- `internal/le/interoplab/bgp/` - checkers for the two scenarios (register file plus bespoke bodies and pure predicates)
- `docs/architecture/core-design.md` section 18a - the new rule, the ordered bind delivery, the sent-direction catch-up, the downstream flap of a changed grant (R-5), the BFD member
- `docs/architecture/api/process-protocol.md` - the `attached` up (Documentation row 8)
- `docs/architecture/api/ipc_protocol.md` - "State events" and the text format: an up can carry reason `attached`
- `docs/architecture/plugin/rib-storage-design.md` - an `attached` up triggers no peer-up replay
- `features/bfd-liveness-detection.md` - Real-path tests and Interop rows
- `docs/guide/config-reload.md` - Change/Effect rows, including "changing a route-consuming process's grant withdraws and re-learns that peer's routes downstream"
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
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/rpki.md` troubleshooting row (line 597) and binding paragraph (line 241): say the attach takes effect without a session reset; `docs/guide/bfd.md` line 195 paragraph: both entry points, the live edit on a shared session (R-12), and the OQ-1 outcome for dynamic peers |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the three `.ci` entry tests fail for the right reason
   - Tests: `bgp-rpki-attach-live`, `bgp-bfd-added-live` at one connection, `bgp-attach-receive-only-live`
   - Files: the three `.ci` files
   - Verify: each fails because the second TCP connection is refused (the restart), not for a fixture reason
   - The wiring phase also writes `bgp-attach-grant-change-live`, `bgp-attach-no-route-refresh-restarts` (green today, kept green), `bgp-attach-sent-direction-restarts` (green today, kept green) and `bgp-bfd-profile-edit-live`
2. **Phase: receive-only bindings** -- widen the predicate, delete `Derived`, route-bearing and sent-direction catch-up, the ordered bind delivery (up acknowledged before the edge is published, session check, intermediate publish for a changed grant) on both the configured and the dynamic path, the `attached` reason and the plugins that skip replay on it
   - Tests: `TestOperatorReceiveOnlyBindingSwaps`, `TestOperatorSendOrOpenBindingStillRestarts`, `TestChangedReceiveGrantIsDownThenUp`, `TestGainedReceiveOnlyBindingGetsUpBeforeRefresh`, `TestReceiveOnlyGainWithoutRouteRefreshRestarts`, `TestSentDirectionGrantNeedsNothingAdvertised`, `TestRouteFreeGrantNeedsNoCatchUp`, `TestUpAcknowledgedBeforeGraphEdge`, `TestBindRacesEstablished`, `TestIdenticalOperatorBindingReplacingDerived`, `TestOnPeerBoundTargetsNamedProcesses`, `TestPeerBoundCarriesNoReplayOrBarrier`, `TestAdjRIBInSkipsReplayOnAttachedUp`, `TestRRSkipsReplayOnAttachedUp`, `TestDynamicTemplateReceiveOnlySwap`
   - Files: `peer_derived_bindings.go`, `peer_settings_apply.go`, `peer_settings.go`, `config.go`, `event_dispatcher.go`, `events.go`, `reactor_dynamic.go`, `pkg/plugin/rpc/enums.go`, `adj_rib_in/rib.go`, `rr/rr.go` and the audit's further plugins
   - Verify: first record the up-handler audit (which plugins replay toward the peer on up); unit tests red then green; `bgp-rpki-attach-live`, `bgp-attach-receive-only-live`, `bgp-attach-grant-change-live` green; A-1, A-2 and A-7 validated and recorded; section 18a, `process-protocol.md`, `ipc_protocol.md` and `rib-storage-design.md` edited in this phase
3. **Phase: BFD opt-in** -- locked accessor and every reader moved to it, BFD in the copier when non-strict both sides, transition after lock release, rollback
   - Tests: `TestNonStrictBFDOptInSwaps`, `TestStrictBFDChangeRestarts`, `TestBFDSwapOpensAndClosesClient`, `TestBFDSwapRacesEstablishedStart`, `TestBFDSwapRacesStrictNegotiation`, `TestNonStrictBFDProfileEditReopens`, `TestBFDSwapRollbackRestoresClient`
   - Files: `peer_bfd.go`, `session_bfd_strict.go`, `reactor_api.go`, `peer_settings_apply.go`, `peer.go`; `reactor_dynamic.go` only if OQ-1 is answered with option A
   - Verify: first record A-4 (a bfd plugin started by the same commit is ready, service and config, before the BGP apply, or not); if not, fix the transaction's apply order, never a watcher; `bgp-bfd-added-live`, `bgp-bfd-removed-live`, `bgp-bfd-profile-edit-live` green; `docs/guide/bfd.md`, `docs/guide/config-reload.md` and `features/bfd-liveness-detection.md` edited in this phase
4. **Phase: existing tests and docs** -- `attach-process-reload.ci`, `attach-process-runtime-subscribe.ci`, `bgp-rpki-added-live.ci` comments, `features/configuration-reload.md`, `docs/guide/rpki.md`
5. **Phase: interop** -- both FRR scenarios with checkers, red phase recorded by reverting each half and rebuilding the image
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
| Rule: no-layering | `derivedFeedsOnly` is replaced, not kept beside the new predicate |
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
| Delete `ProcessBinding.Derived` | keep it as information | once the predicate ignores it, it is write-only and still makes `reflect.DeepEqual` report a change for identical grants (I11); no-layering |

## Known Limitations
- A peer that did not advertise Route Refresh and already announced routes still restarts when a receive-only consumer is added (AC-10). This is the RFC 2918 Section 4 boundary, the same answer BIRD gives for a channel that is not reloadable, not outstanding work.
- A change that involves BFD strict mode still restarts (AC-7), by design (draft-ietf-idr-bgp-bfd-strict-mode Sections 6 and 8).
- A receive grant that includes the OPEN exchange still restarts (AC-2), by design (see Key Design Decisions).
- A grant that covers updates Ze sent (`update-sent`, bare `update`, `*`) still restarts once Ze has advertised on the session (AC-13): Ze cannot re-send its own UPDATEs to a new consumer without fabricating them.
- Changing the grant of a route-consuming process (`bgp-rib`, `bgp-rr`) flaps that peer's routes once on the other sessions (R-5), instead of restarting this peer.
- A BFD profile edit on a session another client shares keeps the session's current parameters (R-12), the BFD plugin's sharing rule.
- BFD on dynamic peers: decided by OQ-1.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The new catch-up call site carries `// RFC 2918 Section 4` (the quote stays on `requestRouteRefreshAll`); the BFD removal call site carries `// RFC 5882 Section 8`; the restart branch keeps its RFC 4486 Section 4 reference, and so do the two new restart decisions inside the apply (sent direction already advertised, session changed inside the window).

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
- [ ] AC-1..AC-18 all demonstrated (AC-9's BFD half as OQ-1 decides)
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
