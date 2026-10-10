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
   session is up and learns the routes the peer sent before (ROUTE-REFRESH when the
   peer supports it). The process that loses it is told the peer is down for it alone.
2. A binding that grants a send type still restarts, because the process can put
   routes into the session's initial routing update and the End-of-RIB barrier
   counts it (`ProcessBinding.MayPushRoutes`, `Peer.resetAPISync`).
3. A receive grant that includes the OPEN exchange (`open` in any direction, or the
   `*` wildcard) still restarts: only a new session produces an OPEN the process
   can be shown.
4. A non-strict BFD opt-in (`connection bfd` with `strict false`) is added, removed,
   suspended, resumed or edited on the running session: the BFD client opens or
   closes, and the BGP session stays up.
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
  → Decision: FRR never resets for it. Outbound: `update_group_adjust_peer` then `bgp_announce_route`. Inbound: returns if not established, tries `bgp_soft_reconfig_in`, else "if (CHECK_FLAG(peer->cap, PEER_CAP_REFRESH_RCV)) bgp_route_refresh_send(peer, afi, safi, 0, 0, 0, BGP_ROUTE_REFRESH_NORMAL);". Ze's equivalent of soft-reconfig-in is a consumer that already retains routes; Ze's ROUTE-REFRESH branch is `requestRouteRefreshAll`.
- [ ] FRR `bgpd/bgp_bfd.c` `bgp_peer_configure_bfd` and `bgp_peer_config_apply` - FRR enabling BFD on a peer
  → Decision: FRR allocates `p->bfd_config`, creates the session with `bfd_sess_new(bfd_session_status_update, p)`, and `bgp_peer_config_apply` ends in `bfd_sess_install`; no peer reset anywhere on this path.
  → Constraint: FRR's `bfd_session_status_update` acts on Down only "if (bss->state == BSS_DOWN && bss->previous_state == BSS_UP)": a freshly installed session that has never been Up does not end the BGP session. Ze's start-on-Established path has the same property today (the bfd-opt-in tests pass), and the live start inherits it (A-3).
- [ ] BIRD `nest/proto.c` `channel_reconfigure` (BIRD commit 02d082a7) - BIRD's reload decision per channel
  → Decision: BIRD reloads rather than restarts: "if (import_changed && !channel_reloadable(c)) return 0;" (restart only when not reloadable), otherwise "channel_request_reload(c)". The same function re-subscribes ROA tables when `rpki_reload` changes. Ze's analogue: catch-up by ROUTE-REFRESH, restart only when the peer cannot re-send.
- [ ] BIRD `proto/bgp/bgp.c` `bgp_reconfigure` and `bgp_update_bfd`; `proto/bgp/bgp.h` `struct bgp_config`
  → Decision: `bfd` is the last field of `struct bgp_config`, AFTER `password`, so the `memcmp` that decides `same` stops at `OFFSETOF(struct bgp_config, password)` and never sees it. A BFD change therefore keeps the session, and `bgp_update_bfd(p, new->bfd)` requests, updates or frees the BFD session in place. BIRD skips BFD for a dynamic BGP instance ("!bgp_is_dynamic(p)") in that function.

### Source Code (current behaviour)
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `hotSwappableSettings`, `peerSettingsSwapPlan`, `applyHotSwappableSettings`, `swapPeerSettingsJournaled`
  → Constraint: the mutable set (fields written from the reload goroutine) is safe only because every reader outside the facts snapshot goes through a p.mu-guarded accessor. `BFD` is read today without a lock under the immutability contract on `Peer.Settings`, so it can join the swap set only after every reader moves to a locked accessor.
  → Constraint: the undo for a rollback is read from the RUNNING peer before the apply (`hotSwappableSnapshot`); a BFD swap's undo MUST restore the previous opt-in and reopen or close the BFD client the same way.
- [ ] `internal/component/bgp/reactor/peer_derived_bindings.go` - `derivedFeedsOnly`, `onlyDerivedFeedsDiffer`, `derivedFeedChange`, `derivedFeedsDeliverable`, `Session.feedCatchUp`, `Peer.catchUpDerivedFeeds`, `requestRouteRefreshAll`
  → Decision: the predicate drops its `Derived` condition and becomes "grants no send type and receives no OPEN" (renamed so the name says what it tests). Everything built on it (diff, gained/lost, catch-up, targeted down) is reused as is.
  → Constraint: today a kept feed with a DIFFERENT grant is refused (restart) because the consumer's picture of the old grant cannot be told apart from the new one. The new rule delivers it as lost-then-gained for that one process: a targeted down, then a targeted up and the catch-up, so the consumer starts that peer again from nothing.
  → Constraint: a gained consumer today receives NO session-up event; only the lost side has `OnPeerUnbound`. Plugins that build per-peer state from `state up` exist (`adj_rib_in/rib.go`, `rib/rib.go`, `bmp/bmp_events.go`, `rr/rr.go`, `gr/gr.go`, `watchdog/watchdog.go`, `ls_export/export_plugin.go`, `redistribute_egress/register.go`, `trafficusage/monitor.go`), so a gained receive-only binding needs a targeted up.
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` `dynamicTemplateChanged` - a dynamic peer's template diff
  → Constraint: it calls `onlyDerivedFeedsDiffer` and `derivedFeedsDeliverable` directly; the renamed predicate reaches dynamic peers through the same calls, so a dynamic group's receive-only attach change also stays up. BFD on a dynamic peer follows the same plan (AC-9).
- [ ] `internal/component/bgp/reactor/peer_bfd.go` - `bfdStrict`, `startBFDClient`, `stopBFDClient`, `bfdRequestFor`, `runBFDSubscriber`
  → Constraint: `startBFDClient` is idempotent and returns early on a nil or disabled opt-in; `stopBFDClient` is idempotent. The live swap calls them; it adds no third way to open or close a BFD client.
  → Constraint: `startBFDClient` with no published service (`api.GetService() == nil`) logs a warning and leaves a non-strict peer without BFD. In a commit that adds the `bfd` root AND the opt-in, the bfd plugin publishes its service in `OnStarted` (`internal/component/bfd/bfd.go`, `api.SetService`), so the order of the plugin start and the BGP apply decides whether the swap finds a service (A-4).
- [ ] `internal/component/bgp/reactor/peer_settings.go` `BFDSettings`, `ProcessBinding`, `MayPushRoutes`, `ReceivesPeerState`
  → Constraint: strict mode is `Enabled && Strict` (`Peer.bfdStrict`); `HoldTime` and `HoldDown` only act in strict mode.
- [ ] `internal/component/bgp/reactor/peer_run.go` - `startBFDClient` on entry to Established, `stopBFDClient` on exit for a non-strict peer; `initialUpdateReporters`
  → Constraint: only `MayPushRoutes` bindings join the initial-sync barrier, so a receive-only binding added live leaves the barrier of the running session untouched.
- [ ] `internal/component/bgp/server/event_dispatcher.go` `OnPeerUnbound`, `internal/component/bgp/server/events.go` `onPeerUnbound`, `onPeerNegotiated`
  → Decision: the targeted up mirrors `onPeerUnbound`: `SubscribedProcsNamed` over the gained names, then `deliverPeerState` with `SessionStateUp`; a gained process granted `negotiated` also receives the running session's negotiated event, targeted the same way.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` (structured handler near line 345, `handleEvent` near line 759) and `rpki_reload.go` `replaceConfig`
  → Constraint: bgp-rpki reacts to `state` only on DOWN (`removePeer`); it needs no up. Routes already retained in Adj-RIB-In are re-decided by `revalidateConfigSnapshot` when validation is enabled, so for RPKI the ROUTE-REFRESH is not what makes the states appear; the generic receive-only test (AC-4) is what proves the catch-up.
- [ ] `internal/component/bgp/reactor/peer_derived_bindings_test.go` `TestOperatorBindingChangeStillRestarts`
  → Decision: this test pins the rule the owner changed. It is REPLACED by a test of the new rule (an operator receive-only binding swaps) and a test that keeps the remaining restart cases (an operator binding with a send grant, and one granting `open`, still restart). The replacement is a change of the behaviour under test by owner decision, not a weakening.
- [ ] `test/reload/bgp-rpki-added-live.ci`, `test/reload/bgp-bfd-added-live.ci`, `test/plugin/attach-process-reload.ci`, `test/plugin/attach-process-runtime-subscribe.ci` - the functional tests that pin today's counts
  → Constraint: `bgp-rpki-added-live.ci` passes on one connection only because the base config pre-binds `bgp-rpki`; `bgp-bfd-added-live.ci` and `attach-process-reload.ci` expect two connections; `attach-process-runtime-subscribe.ci` changes a kept binding's grant from `[ keepalive state ]` to `[ keepalive state open-received ]`, which gains `open` and therefore STILL restarts under the new rule (two connections kept).

**Key insights:** (minimal context to resume after compaction)
- One predicate decides the binding half: "grants no send type and receives no OPEN". It replaces `derivedFeedsOnly`; derived and operator bindings follow one rule.
- Gained process: targeted `state up` (plus `negotiated` if granted), then the existing catch-up (nothing / ROUTE-REFRESH / restart). Lost process: existing targeted down. Changed grant: down then up for that process.
- BFD half: a non-strict opt-in on both sides of the diff (nil, disabled, or enabled-non-strict) joins the swap set; the apply opens or closes the BFD client with the existing idempotent functions. Strict on either side restarts.
- `PeerSettings.BFD` must move to locked access before it can be written live.
- The bfd plugin may publish its service after the BGP apply in the same commit (A-4): the live swap must still open the session once the service appears.
- FRR and BIRD both keep the session for both changes; quotes above.

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
- A gained receive-only process receives a targeted session-up (and negotiated, if granted).
- A non-strict BFD opt-in change is swapped live; the BFD client opens or closes on the running session.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config commit (SSH editor `commit`, `ze config` apply, or SIGHUP reload) carrying a changed `bgp peer <name> attach process <p>` block or `bgp peer <name> connection bfd` block, and possibly a new `bfd` root.
- Format at entry: the resolved BGP tree, parsed into `PeerSettings` (`parseProcessBindingsFromTree`, the BFD block parser in `config.go`).

### Transformation Path
1. Config transaction: verify (`VerifyPeerBFDProfiles` for a BFD profile name), then apply; plugins named by a new root or a new `attach`/receive type start during the transaction.
2. Reactor reconcile (`reconcilePeersJournaled`, `reactor_api.go`): per peer, `peerSettingsSwapPlan(current, next, session)` returns a copier or a restart reason.
3. Swap: `swapPeerSettingsJournaled` records the undo from the running peer, then `applyHotSwappableSettings` writes the copied fields under r.mu and p.mu, republishes the delivery graph if bindings changed, then (locks released) runs the binding catch-up and, new, the BFD client transition.
4. Binding catch-up: targeted down to lost processes (`OnPeerUnbound`), new targeted up to gained processes, then ROUTE-REFRESH per negotiated family or nothing or restart (`catchUpDerivedFeeds`).
5. BFD transition: opt-in gained or edited and the peer Established: `stopBFDClient` (when one is open and the request changed) then `startBFDClient`; opt-in lost or suspended: `stopBFDClient`.
6. Restart branch unchanged: the peer is removed and re-added with a Cease subcode 6.

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
- `EventDispatcher` - gains the targeted up, sibling of `OnPeerUnbound`.
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
| A-4 | In a commit that adds the `bfd` root and the opt-in together, the bfd service may not be published when the BGP apply runs | `api.SetService` runs in the bfd plugin's `OnStarted` (`internal/component/bfd/bfd.go` line 821); reactor apply order vs plugin start not read | if the service is always published first, no extra mechanism is needed; if not, the swapped peer stays without BFD (warning logged, silent to the operator) | Phase 3 first step: run `bgp-bfd-added-live.ci` with the swap and no other change, and record whether `startBFDClient` saw a nil service | unvalidated |
| A-5 | `stopBFDClient` -> `ReleaseSession` on the last reference behaves as on today's Established exit (RFC 5882 Section 8 AdminDown handling is the BFD plugin's) | the same function is the non-strict exit path today | a live removal could make the remote BGP speaker tear down on BFD Down | AC-6 interop: FRR keeps BGP Established after Ze removes the opt-in | unvalidated |
| A-6 | FRR, as the interop peer, answers ROUTE-REFRESH and keeps `connectionsEstablished` unchanged across a Ze reload that sends one | FRR `peer_on_policy_change` and RFC 2918 support; the `rtr-stayrtr` checker reads the same counter | the continuity check would be measuring something else | AC-7 scenario run, counter read before and after | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Writing `PeerSettings.BFD` from the reload goroutine races the FSM callback and the subscriber, which read it with no lock today | `-race` failure in the new swap unit test | move every reader (`bfdStrict`, `startBFDClient`, `bfdRequestFor`, the log lines) to a p.mu-guarded accessor before the field joins the swap set; a `-race` test runs the swap concurrently with `startBFDClient` |
| R-2 | The apply opens BFD while the FSM callback for Established also opens it | two `EnsureSession` calls, an orphaned subscriber | `startBFDClient` already serialises on `p.bfd.starting`; the swap reuses it and adds no second open path |
| R-3 | The bfd service is published after the BGP apply in the same commit (A-4) | `bfd configured on peer but BFD plugin not loaded` warning in the AC-5 test log, empty `show bfd sessions` | `bfd/api` notifies registered watchers when a service is published; the reactor registers one watcher (at reactor start, not per peer) that calls `startBFDClient` on every Established non-strict peer with an enabled opt-in and no open handle. Built only if A-4 is confirmed; the same watcher also heals a boot where BGP reaches Established before bfd starts |
| R-4 | A gained process receives routes from the ROUTE-REFRESH before its targeted up | process sees UPDATE for an unknown peer | the targeted up is delivered before `requestRouteRefreshAll`, in the same goroutine; AC-4 asserts the order |
| R-5 | A changed grant delivered as down-then-up makes a consumer drop and relearn a peer's routes (a burst of withdraw/announce inside that consumer only) | consumer metrics churn on a grant edit | acceptable and the same picture a restart gives that consumer, without touching the peer or the other consumers; documented in section 18a |
| R-6 | Existing functional tests expect two connections for a receive-only attach change and now fail | `attach-process-reload.ci` red | update their expectations and comments in the same change (listed in Files to Modify); `attach-process-runtime-subscribe.ci` gains `open-received` and keeps two connections |
| R-7 | The rollback undo of a BFD swap leaves a BFD client open or closed against the restored settings | after a rolled-back commit, `show bfd sessions` disagrees with the config | the undo goes through the same copier and the same BFD transition step as the apply; a unit test rolls back a swap and checks the client state |
| R-8 | A peer without Route Refresh that already announced routes still restarts when RPKI is attached live | two connections against such a peer | intended: the catch-up rule is unchanged and documented; the AC-3 test uses a peer that advertises Route Refresh, and AC-10 pins the restart for one that does not |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a consumer process holds an incomplete view of a peer (missing routes or missing up), or a BFD client is left open or closed against the config; at worst a session left running where it should have restarted |
| How is it reverted? | single commit revert; no config migration, no wire format change |
| Who else touches this path? | `plan/immediate/spec-bgp-reload-rich-peer-no-bounce.md` (reload equality for rich peers, same `peerSettingsEqual` decision), `plan/spec-terminal-demo-showcase.md` (RPKI and BFD topics wait on this), BFD strict-mode reload work (`bgp-bfd-strict-reload-speaker`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH editor commit adding `attach process bgp-rpki` to a running peer | → | `peerSettingsSwapPlan` -> `applyHotSwappableSettings` -> targeted up + catch-up | `test/reload/bgp-rpki-attach-live.ci` |
| SSH editor commit adding the `bfd` root and a non-strict `connection bfd` opt-in | → | `peerSettingsSwapPlan` -> `applyHotSwappableSettings` -> `startBFDClient` | `test/reload/bgp-bfd-added-live.ci` (one connection) |
| SIGHUP reload adding a receive-only `attach process` for an external process | → | targeted up (`EventDispatcher`) then `requestRouteRefreshAll` | `test/reload/bgp-attach-receive-only-live.ci` |
| SIGHUP reload adding a receive-only attach block (kept edge plus added edge) | → | binding swap, delivery graph republish | `test/plugin/attach-process-reload.ci` (one connection) |
| SIGHUP reload against FRR | → | the same swap, observed by a foreign speaker | interop `bgp-attach-receive-only-live-frr`, `bgp-bfd-enable-live-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `peerSettingsSwapPlan` with current and next differing only in an operator binding that grants no send type and receives no OPEN | returns a copier and an empty reason (no restart) |
| AC-2 | `peerSettingsSwapPlan` with next adding an operator binding that grants `send [ update ]`, or one that receives `open` (any direction), or `receive [ * ]` | returns a restart reason naming `ProcessBindings` |
| AC-3 | A running iBGP peer (Route Refresh advertised) that announced Valid, Invalid and NotFound prefixes; an SSH commit load-merges `attach process bgp-rpki { receive [ update-received state ] }` and the `bgp rpki` cache and policy | one TCP connection for the whole test; no NOTIFICATION from Ze; `show bgp adj-rib-in` shows validation-state 1, 2 and 3 for the routes received before the commit |
| AC-4 | A running peer that announced a route; a reload adds `attach process <external> { receive [ update-received state ] }` for an external process started by that reload | one TCP connection; the process receives a `state up` for the peer BEFORE any UPDATE from it; Ze sends one ROUTE-REFRESH per negotiated family; the process receives the route the peer announced before the reload |
| AC-5 | A running non-strict peer; an SSH commit load-merges a `bfd` root with a profile and the peer's `connection bfd { profile <p> }` | one TCP connection; `show bfd sessions` lists the single-hop session to the peer under the committed profile |
| AC-6 | A running non-strict peer with an open BFD session; a commit removes the opt-in (or sets `enabled false`) | one TCP connection; `show bfd sessions` no longer lists the peer's session; the BGP session stays Established |
| AC-7 | A running peer whose opt-in changes between non-strict and strict, in either direction, or whose strict `hold-time`/`hold-down` changes | the peer restarts, and the restart log line names `BFD` |
| AC-8 | A reload that changes a kept receive-only binding's grant (for example `[ update-received ]` to `[ update-received state ]`) | no restart; the process receives a targeted down then a targeted up for that peer; no other process receives either |
| AC-9 | A dynamic peer whose group template gains a receive-only attach block or a non-strict BFD opt-in | the dynamic peer's session stays up and the change is delivered, as for a configured peer |
| AC-10 | A running peer that did NOT advertise Route Refresh and announced routes; a reload adds a receive-only binding | the peer restarts (Cease subcode 6), unchanged from today |
| AC-11 | A swapped reload (receive-only attach and BFD opt-in) that the transaction rolls back | the binding set and the BFD client return to their pre-commit state with no restart |
| AC-12 | Interop: FRR peer Established with Ze; Ze is reloaded with a receive-only attach, then with a non-strict BFD opt-in, then with the opt-in removed | FRR's `connectionsEstablished` for the Ze neighbor is unchanged after each reload; after the BFD reload FRR `show bfd peers` lists Ze's session Up; after the removal FRR's BGP session to Ze is still Established |

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
| `TestDynamicTemplateReceiveOnlyAndBFDSwap` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-9 | |
| `TestOnPeerBoundTargetsNamedProcesses` | `internal/component/bgp/server/events_test.go` (or the file holding `onPeerUnbound` tests) | targeted up reaches only the named processes, and `negotiated` only when granted | |
| `TestBFDServicePublishStartsWaitingPeers` | `internal/component/bgp/reactor/peer_bfd_test.go` | R-3, only if A-4 is confirmed | |

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
| `attach-process-reload` | `test/plugin/attach-process-reload.ci` (changed to one connection; the added process must receive the targeted up) | AC-1, AC-4 | |
| `attach-process-runtime-subscribe` | `test/plugin/attach-process-runtime-subscribe.ci` (unchanged count: gains `open-received`; comment updated to name the OPEN rule) | AC-2 | |
| `bgp-rpki-added-live` | `test/reload/bgp-rpki-added-live.ci` (unchanged; its PREVENTS comment updated) | pre-bound shape stays green | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-attach-receive-only-live-frr` | `test/interop/scenarios/bgp-attach-receive-only-live-frr/` (`ze.conf`, `ze-reload.conf`, `frr.conf`) | FRR | AC-12 first half: a receive-only attach reload keeps FRR's `connectionsEstablished` unchanged; FRR receives a ROUTE-REFRESH and re-sends | |
| `bgp-bfd-enable-live-frr` | `test/interop/scenarios/bgp-bfd-enable-live-frr/` | FRR (bgpd + bfdd) | AC-12 second half: BFD opt-in added then removed by reload; BGP stays on one session; FRR `show bfd peers` Up after the add | |

Each scenario's red phase is recorded per `ai/rules/interop-and-goal-validation.md`: revert the swap (restore `derivedFeedsOnly` with its `Derived` condition, or drop BFD from the copier), rebuild the Ze image, observe the counter advance (red), restore, observe green.

## Files to Modify
- `internal/component/bgp/reactor/peer_derived_bindings.go` - predicate widened and renamed; changed grant as down-then-up; targeted up before catch-up; comments rewritten (stale-comments rule)
- `internal/component/bgp/reactor/peer_settings_apply.go` - BFD joins `hotSwappableSettings` when non-strict on both sides; apply runs the BFD transition after locks are released; the long comment's field list updated
- `internal/component/bgp/reactor/peer_bfd.go` - locked accessor for the opt-in; every reader moved to it; a transition helper reusing `startBFDClient`/`stopBFDClient`
- `internal/component/bgp/reactor/peer.go` - `Peer.Settings` immutability comment names `BFD` as a mutable field
- `internal/component/bgp/reactor/reactor_dynamic.go` - dynamic template diff reaches the same plan for BFD (AC-9)
- `internal/component/bgp/reactor/reactor_api.go` - the dispatcher interface gains the targeted up, if the interface lives here
- `internal/component/bgp/server/event_dispatcher.go`, `internal/component/bgp/server/events.go` - targeted up (and negotiated) to named processes
- `internal/component/bfd/api/registry.go` - service-published watcher, only if A-4 is confirmed (R-3)
- `internal/component/bgp/reactor/peer_derived_bindings_test.go` - `TestOperatorBindingChangeStillRestarts` replaced (see Required Reading)
- `test/reload/bgp-bfd-added-live.ci` - one connection
- `test/reload/bgp-rpki-added-live.ci` - PREVENTS comment no longer says an attach restarts
- `test/plugin/attach-process-reload.ci` - one connection, comment
- `test/plugin/attach-process-runtime-subscribe.ci` - comment names the OPEN rule as the restart cause
- `internal/le/interoplab/bgp/` - checkers for the two scenarios (register file plus bespoke bodies and pure predicates)
- `docs/architecture/core-design.md` section 18a - the new rule, the targeted up, the BFD member
- `docs/guide/config-reload.md` - Change/Effect rows
- `docs/guide/bfd.md` - live enable and removal on an Established peer
- `docs/guide/rpki.md` - attaching rpki live keeps the session
- `docs/architecture/testing/interop.md` - Scenario Inventory rows for both scenarios
- `features/configuration-reload.md` - Real-path tests gain the new `.ci` files
- `plan/spec-terminal-demo-showcase.md` - D-6 and D-8 closed by this work once it lands (the decision itself is already recorded there)

## Files to Create
- `test/reload/bgp-rpki-attach-live.ci` - AC-3
- `test/reload/bgp-bfd-removed-live.ci` - AC-6
- `test/reload/bgp-attach-receive-only-live.ci` - AC-4
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
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/configuration-reload.md` (Real-path tests), `features/bfd-liveness-detection.md` if its evidence names the restart |
| 2 | Config syntax changed? | No | syntax unchanged |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | the targeted up reuses the existing `state` event shape |
| 5 | Plugin added/changed? | No | plugin registrations unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/config-reload.md`, `docs/guide/bfd.md`, `docs/guide/rpki.md` |
| 7 | Wire format changed? | No | no message format change; only whether a Cease or a ROUTE-REFRESH is sent |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`: a process can receive `state up` for a peer whose session was already up when the process was attached |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the RFC 2918 catch-up and RFC 6811 Section 4 re-validation are newly exercised by a live attach; `rfc/short/rfc2918.md` and `rfc/short/rfc6811.md` source anchors if they cite the derived-only rule |
| 10 | Test infrastructure changed? | No | existing `.ci` options and interop harness only |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists reload behaviour: Ze now matches FRR and BIRD here |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` section 18a |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | event types unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md`; at least core-design 18a (source anchors on `peer_settings_apply.go`, `peer_derived_bindings.go`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/rpki.md` troubleshooting row (line 597) and `docs/guide/bfd.md` opt-in examples: check the surrounding text for "restart" claims |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the three `.ci` entry tests fail for the right reason
   - Tests: `bgp-rpki-attach-live`, `bgp-bfd-added-live` at one connection, `bgp-attach-receive-only-live`
   - Files: the three `.ci` files
   - Verify: each fails because the second TCP connection is refused (the restart), not for a fixture reason
2. **Phase: receive-only bindings** -- widen the predicate, changed grant as down-then-up, targeted up before catch-up, dynamic path
   - Tests: `TestOperatorReceiveOnlyBindingSwaps`, `TestOperatorSendOrOpenBindingStillRestarts`, `TestChangedReceiveGrantIsDownThenUp`, `TestGainedReceiveOnlyBindingGetsUpBeforeRefresh`, `TestReceiveOnlyGainWithoutRouteRefreshRestarts`, `TestOnPeerBoundTargetsNamedProcesses`, `TestDynamicTemplateReceiveOnlyAndBFDSwap` (binding half)
   - Files: `peer_derived_bindings.go`, `peer_settings_apply.go`, `event_dispatcher.go`, `events.go`, `reactor_dynamic.go`
   - Verify: unit tests red then green; `bgp-rpki-attach-live` and `bgp-attach-receive-only-live` green; A-1 and A-2 validated and recorded; section 18a and the plugin-protocol page edited in this phase
3. **Phase: BFD opt-in** -- locked accessor, BFD in the copier when non-strict both sides, transition after lock release, rollback
   - Tests: `TestNonStrictBFDOptInSwaps`, `TestStrictBFDChangeRestarts`, `TestBFDSwapOpensAndClosesClient`, `TestBFDSwapRacesEstablishedStart`, `TestBFDSwapRollbackRestoresClient`
   - Files: `peer_bfd.go`, `peer_settings_apply.go`, `peer.go`, `reactor_dynamic.go`
   - Verify: first record A-4 (service published before the apply or not); build R-3's watcher only if not; `bgp-bfd-added-live` and `bgp-bfd-removed-live` green; `docs/guide/bfd.md` and `docs/guide/config-reload.md` edited in this phase
4. **Phase: existing tests and docs** -- `attach-process-reload.ci`, `attach-process-runtime-subscribe.ci`, `bgp-rpki-added-live.ci` comments, `features/configuration-reload.md`, `docs/guide/rpki.md`
5. **Phase: interop** -- both FRR scenarios with checkers, red phase recorded by reverting each half and rebuilding the image
   - Verify: `INTEROP_SCENARIO=<name> ./le test integration interop` green for each, red recorded with the swap reverted

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | the predicate reads the grant (send types, `open`, `*`), never a plugin name; the targeted up precedes the ROUTE-REFRESH; strict BFD on either side restarts |
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
| BFD service watcher only if A-4 shows the ordering problem | always build it; or make BGP's apply wait for the bfd plugin | simplicity: build machinery only for a failure observed; waiting would couple the BGP apply to an optional plugin |

## Known Limitations
- A peer that did not advertise Route Refresh and already announced routes still restarts when a receive-only consumer is added (AC-10). This is the RFC 2918 Section 4 boundary, the same answer BIRD gives for a channel that is not reloadable, not outstanding work.
- A change that involves BFD strict mode still restarts (AC-7), by design (draft-ietf-idr-bgp-bfd-strict-mode Sections 6 and 8).
- A receive grant that includes the OPEN exchange still restarts (AC-2), by design (see Key Design Decisions).

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The new catch-up call site carries `// RFC 2918 Section 4` (the quote stays on `requestRouteRefreshAll`); the BFD removal call site carries `// RFC 5882 Section 8`; the restart branch keeps its RFC 4486 Section 4 reference.

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
- [ ] AC-1..AC-12 all demonstrated
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
