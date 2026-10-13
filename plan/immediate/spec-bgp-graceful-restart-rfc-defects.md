# Spec: bgp-graceful-restart-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The strict RFC re-read of `spec-rfc-requirement-quote-hand-backfill`
found that Ze's Graceful Restart (RFC 4724) and Long-Lived Graceful Restart
(RFC 9494) helper keeps routes it must drop and forwards stale routes it must
depreference or hold back. An operator meets both as routing: stale routes
survive a NOTIFICATION teardown, and a route a neighbour marked LLGR_STALE is
preferred and advertised as if fresh.

Each row was confirmed open at its producer in HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 4724 Section 4 (RFC4724-4-2) | "It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message." | `internal/component/bgp/plugins/gr/gr.go::handleStructuredState` and `handleStateEvent` | Both test `reason == "notification"`, but the only producer (`internal/component/bgp/reactor/peer_run.go`, through `notifyPeerClosed`) sends "session closed" or "connection lost", so GR retains and marks stale after a NOTIFICATION teardown | weak; no journal row |
| D2 | RFC 9494 Section 4.2 (RFC9494-4.2-8) | "the F bit for a specific address family is not set in the newly received LLGR Capability, or" / "a specific address family is not included in the newly received LLGR Capability, or" | `internal/component/bgp/plugins/gr/gr_state.go::onSessionReestablished` | Keeps a family when EITHER the new GR or the new LLGR capability sets its F bit, so a family with F clear in LLGR, or absent from LLGR, is not purged when GR carries it with F set | unimplemented finding; row has no gap marker |
| D3 | RFC 9494 Section 4.3 (RFC9494-4.3-1) | "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2:" and "Treat the route as the least preferred in route selection" | `internal/component/bgp/plugins/rib/bestpath.go::comparePair`, `internal/component/bgp/plugins/rib/storage/familyrib.go` insert | Only a route the helper marks itself is least preferred; a route received carrying LLGR_STALE (0xFFFF0006) is inserted fresh and competes normally. No non-test code reads the community on receipt | unimplemented finding |
| D4 | RFC 9494 Section 4.3 (RFC9494-4.3-3) | "The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received." | `internal/component/bgp/plugins/gr/gr_egress.go::LLGREgressFilter` | Decides on the local stale level only, so a received LLGR_STALE route is advertised unchanged to a neighbour that never sent the LLGR capability | unimplemented finding |
| D5 | RFC 9494 Section 4.2 (RFC9494-4.2-6) with Section 4.3 (RFC9494-4.3-2) | "The LLGR_STALE community MUST NOT be removed when the route is further advertised." | `internal/component/bgp/reactor/peer_forward_facts.go::applyFactsSendCommunity`, `internal/component/bgp/plugins/filter_community` `removeValues` | The recorded 4.3-2 gap: the community can be stripped on egress, so Section 4.3 is not performed in full and 4.2-6 is claimed on one bullet | unimplemented finding; 4.3-2 carries `{gap}` |

Related: `plan/spec-gr-advanced.md` designs N-bit (RFC 8538) retention on top of
the `wasNotification` branch and assumes it works; D1 makes it work first.

### Re-confirmation at HEAD, 2026-10-13 (design session)

Ownership: the spec's last commit, `af69577952` (2026-10-05, "wip: checkpoint"),
changed only AC-4 (added RFC9494-4.3-3). No `tmp/session/*/state/` file names this
spec, and no other spec claims it. GR plugin commits since then (`a18900e263`,
`e711f2e9bb`, `6e063fb498`, `7d69d6a669`) touch configs, CLI names, untagged test
assertions and generated imports, none of the producers below. No defect is fixed.

| ID | Still open? | Evidence at HEAD |
|----|-------------|------------------|
| D1 | Open | `gr.go::handleStructuredState` and `gr.go::handleStateEvent` both compute `wasNotification` as the reason equal to the literal "notification". The only producer of a teardown reason, the FSM callback in `peer_run.go`, sets "session closed", or "connection lost" when the target state is Idle, then calls `announceClosed` -> `notifyPeerClosed`. It already knows the answer: `p.notificationExchanged` (set by `incrNotificationSent` / `incrNotificationReceived` in `peer_stats.go`, cleared at session start in `peer_run.go`) is read two lines earlier, but only for `raiseSessionDropped`. The other reason producer, `removePeer`, emits `rpc.ReasonPeerRemoved` ("removed"). |
| D2 | Open | `gr_state.go::onSessionReestablished` builds one `newForwarding` set from the GR F bits, then, when the peer was in LLGR, ADDS every family whose LLGR F bit is set. A family is kept when either bit is set. A family absent from the new LLGR capability, or with its LLGR F bit clear, survives when GR lists it with F set. The untagged `TestOnSessionReestablished_DuringLLGR_NoLLGRCap` asserts that wrong outcome ("GR F-bit=1, stale kept until EOR") under a VALIDATES line that says the opposite ("delete all stale"). |
| D3 | Open | `StaleLevel` is set only by `mark-stale` and `attach-community` commands (`rib_commands.go`, `rib_commands_community.go`). Every insert path in `storage/familyrib.go` resets an entry to `StaleLevelFresh`. No non-test code outside the GR community-name registration reads `attribute.CommunityLLGRStale` on receipt. The reactor computes ingress route metadata (`reactor_notify.go`, `ingressMeta` -> `routeMeta` -> `ReceivedUpdate.Meta`), but the received `bgptypes.RawMessage` handed to plugins carries no `Meta`, so the RIB never sees it. |
| D4 | Open | `gr_egress.go::LLGREgressFilter` returns at once when `meta["stale"]` is zero. A received LLGR_STALE route carries no stale metadata on the forward rails (`reactor_api_forward.go`, `forward_rs.go`) or the RIB replay rail (`rib_replay.go` copies the zero `StaleLevel`), so it is advertised unchanged to an EBGP neighbour that never sent LLGR. |
| D5 | Open | `applySendCommunityMask` (`peer_forward_facts.go`) records `AttrModSuppress` on code 8 for `send-community none` or a list without `standard`. `filter_community/handler.go::genericCommunityHandler` drops the whole attribute on Suppress and drops every Remove-matched value with no exemption. A third path the 2026-09-27 table missed: an import or export policy delta that removes COMMUNITIES records Suppress through `filter_delta.go` (the "Removed attributes" branch), and an AttrModSet that omits LLGR_STALE replaces the list. All three end in the one code-8 handler. The `{gap}` annotation in `rfc/short/rfc9494.md` cites line numbers that have since moved. |
| D6 (new) | Open | `LLGREgressFilter` treats ANY nonzero stale level as LLGR-stale. `mark-stale` defaults to level 1 and the GR session-down sequence calls it with no level (`gr.go`), so a route in its conventional GR period (level 1) that `rib_replay.go` replays to a newly established EBGP neighbour without LLGR is turned into a withdraw. RFC 4724 Section 4.2: "The router MUST NOT differentiate between stale and other routing information during forwarding." RFC 9494 Section 4.3 binds only routes carrying, or given, LLGR_STALE. `docs/guide/graceful-restart.md` ("Readvertising a Stale Route") states the current behaviour as intended ("a route staled by `request bgp rib mark-stale` takes the same treatment"), so this is an owner question (Q-2), not a silent fix. |

Verdicts, read from `rfc/audit/rfc4724.json` and `rfc/audit/rfc9494.json` on 2026-10-13:

| Requirement | Verdict | Tagged units today | What the verdict note says is missing |
|-------------|---------|--------------------|----------------------------------------|
| RFC4724-4-2 | weak | `gr_state_test.go::TestGRStateManagerNotificationBypass`, `::TestGRStateManagerRouteRetention` | both inject `wasNotification`; production never produces it |
| RFC9494-4.2-6 | weak | `rib/rfc9494_test.go::TestRFC9494_LLGRStaleRouteIsLeastPreferred`, `::TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred` | only one Section 4.3 procedure (self-attached least preferred) is proven |
| RFC9494-4.2-8 | weak | `gr_state_test.go::TestOnSessionReestablished_DuringLLGR`, `::TestOnSessionReestablished_DuringLLGR_NoCaps` | bullets 1 and 2 (LLGR F bit clear, family absent from LLGR) unasserted |
| RFC9494-4.3-1 | weak | `rib/bestpath_test.go::TestComparePair_LLGRStale`, `::TestComparePair_GRStaleCompetesNormally` | `TestComparePair_LLGRStale` passes on the address tie-break with Step 0 removed; receive path unasserted |
| RFC9494-4.3-2 | no verdict; `{gap}` row | none | the gap annotation in `rfc/short/rfc9494.md` |
| RFC9494-4.3-3 | weak | `gr/rfc9494_gr_egress_test.go::TestLLGREgressFilter_NilStateWithdrawsEBGP`, `::TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer`, `test/plugin/llgr-egress-state-unloaded.ci` | no carrier receives an actual LLGR_STALE UPDATE; the functional carrier checks NO_EXPORT only |

`./le rfc check` at HEAD reports 2 violations, both RFC 4456 SHIFTED verdicts
from another session's `check_special.go` edit; none for RFC 4724 or 9494.
`plan/pre-release/spec-rfc-verdict-fix-bgp.md` routes all five weak verdicts
here in its "Blocked by" table (AC-4).

### RFC text read for this design (`rfc/full/`)

| Defect | Section | Whole sentence quoted |
|--------|---------|-----------------------|
| D1 | RFC 4724 Section 4 | "It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message." |
| D1 (bound) | RFC 8538 Section 2 | "When the "N" bit has been exchanged by two peers, NOTIFICATION messages other than Hard Reset messages are referred to as "Graceful", since such messages invoke Graceful Restart semantics." Ze never sets the N bit in the capability it sends (`reactor/peer_gr_flags.go::restartFlagsFor` sets R and F only; `gr.go` decodes the peer's N bit and acts on nothing), so the N bit is never exchanged and RFC 4724 Section 4 governs every NOTIFICATION teardown. `rfc/short/rfc8538.md` is `Enrolment: blocked`, `Support status: Unsupported`; no audit file exists. |
| D2 | RFC 9494 Section 4.2 | "Similar to [RFC4724], once the LLGR Period begins, the Helper MUST immediately remove all the stale routes from the peer that it is retaining for that address family if any of the following occur:" followed by the three bullets in the Defects table. |
| D2 (bound) | RFC 9494 Section 4.2 | "Once the session is re-established, the procedures specified in [RFC4724] apply for the stale routes irrespective of whether the stale routes are retained during the Restart Time period or the Long-Lived Stale Time period." |
| D2 (bound) | RFC 4724 Section 4.2 | "Once the session is re-established, if the "Forwarding State" bit for a specific address family is not set in the newly received Graceful Restart Capability, or if a specific address family is not included in the newly received Graceful Restart Capability, or if the Graceful Restart Capability is not received in the re-established session at all, then the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is retaining for that address family." |
| D3, D4, D5 | RFC 9494 Section 4.3 | the lead sentence and three bullets quoted in the Defects table |
| D3 | RFC 9494 Section 4.4 | "A least preferred route MUST be treated as less preferred than any other route that is not also least preferred." |
| D6 | RFC 4724 Section 4.2 | "The router MUST NOT differentiate between stale and other routing information during forwarding." |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | One unit test per row, red against HEAD |
| Both polarities | D1: NOTIFICATION teardown flushes, TCP loss retains. D2: each of the three bullets purges, a family with both F bits set is kept. D3/D4: received LLGR_STALE is least preferred and withheld from a non-LLGR neighbour, a fresh route is not |
| Discrimination | Every RFC-tagged unit carries a `./le rfc discriminate-record` record |
| Real entry point | `.ci` tests driving a peer through NOTIFICATION and reconnect |
| Interop | The existing GR interop scenario against FRR (`test/interop/scenarios/bgp-graceful-restart-frr`) extended with a NOTIFICATION teardown, and an LLGR scenario with a peer sending LLGR_STALE |

## Owner decisions

| Row | Question |
|-----|----------|
| - | None open: every row follows its RFC sentence and no reading is in doubt (2026-09-27; superseded by the 2026-10-13 design questions below) |

### Owner questions raised in design (2026-10-13)

Only Q-4 is answered (owner, 2026-10-13, recorded in its row). Q-1, Q-2, Q-3
and Q-5 stay open: the design below proceeds under each "Recommended" reading,
and nothing else here records an approval.

| Q | Defect | Question | Recommended | If the other answer |
|---|--------|----------|-------------|---------------------|
| Q-1 | D2 | On reconnect, a family whose LLGR period has begun: keep its stale routes only when the new GR capability lists it with F set AND the new LLGR capability lists it with F set? | Yes, AND. RFC 9494 Section 4.2 adds its three LLGR conditions and also says RFC 4724 procedures apply "irrespective" of which period retained the routes. Consequence an operator sees: a family the peer omits from GR but carries in LLGR (LLGR-only) is purged at reconnect unless the new GR lists it with F set. | LLGR-only reading: an LLGR-period family obeys only the three Section 4.2 bullets; AC-2 drops its GR-side rows |
| Q-2 | D6 | Should `LLGREgressFilter` act only on LLGR-stale routes (stale level at or above `storage.DepreferenceThreshold`, the level LLGR entry and receipt of LLGR_STALE set), letting a conventional GR-stale route (level 1) be advertised like any other? | Yes. RFC 4724 Section 4.2 forbids differentiating stale routes during forwarding; RFC 9494 Section 4.3 binds LLGR_STALE routes only. `docs/guide/graceful-restart.md` changes with it. | Keep today's "any stale level" behaviour; D6 leaves this spec and the guide keeps its sentence |
| Q-3 | D5 | Preserve LLGR_STALE on every further advertisement of a route whose received COMMUNITIES carries it, whatever `send-community`, `community-remove` or a policy Set/Suppress says? | Yes, unconditional on presence: the Section 4.3 bullet is stated without exception, and the one code-8 handler sees the source attribute but not the route's metadata. A `send-community none` destination then receives COMMUNITIES holding LLGR_STALE alone. No config refusal for `community-remove ffff0006`: the runtime exemption makes it inert, and the guide says so. | Preserve only for routes Ze holds as LLGR-stale (meta stale level 2): needs the stale level plumbed into the attribute plan, a wider change |
| Q-4 | all | Approve the RFC-tagged test changes listed in "RFC-tagged tests this spec changes" (TDD Test Plan)? | Approve: each change strengthens an assertion the audit called weak; none relaxes one. **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** It covers the two units listed in the TDD plan; the `TestOnSessionReestablished_DuringLLGR` prose is written for whichever Q-1 answer is recorded, and this does not answer Q-1. Commands under "Owner approval commands" in the TDD plan | The affected verdicts stay weak and AC-4 cannot close |
| Q-5 | - | RFC 9494 gaps RFC9494-4.2-7, 4.2-9 and 4.2-10 (consecutive-drop purge and LLST re-arm, LLST expiry during synchronization) sit in the same `gr_state.go` and are owned by no spec (grep of `plan/` 2026-10-13). Fold them into this spec, or open a separate spec? | Separate spec in `plan/immediate/`: they change timer ownership across reconnect, a different mechanism from the six rows here, and folding them would double this spec's surface | Fold in: add D7 to D9 and their ACs before approval |
| Q-7 | D5 / R-6 | Toward an IBGP neighbour without LLGR whose `send-community` suppresses standard communities, the Section 4.6 NO_EXPORT the GR egress filter adds is dropped by the Suppress. Fold the fix into D5 (an egress filter's Add of a value survives a send-community Suppress), or record it as a journal row for a separate fix? | Fold into D5: same handler, same phase, and it is the Section 4.6 MUST on the very routes D4 starts sending | journal row in `plan/journal/`, this spec unchanged |
| Q-6 | D1 | The teardown reason after a NOTIFICATION becomes the new declared constant `rpc.ReasonNotification`, spelt "notification", in state events, `show` FSM history and the session-closed log line, replacing "session closed" / "connection lost" for those teardowns. Accept that operator-visible string? | Yes: GR already expects that spelling, and BMP reads the NOTIFICATION from its own cache, not the reason | Choose another spelling; only the constant's value changes |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/graceful-restart.md` - the page `ai/CODE-TO-DOCS.md` names for every `plugins/gr/` file
  → Constraint: its "Peer Restarts" list, LLGR "How It Works" step 5 ("families with F-bit=0 or missing from the new OPEN are purged", silent on the LLGR capability) and "Readvertising a Stale Route" table change in the same phase as D2, D4 and D6
  → Decision: stale levels are 0 fresh, 1 GR-stale (normal selection), 2+ LLGR-stale (loses to any level below 2); D3 reuses level 2 for a received LLGR_STALE route, so no new level is invented
- [ ] `docs/architecture/api/architecture.md` "Close Reasons" - the page that lists the down reasons
  → Constraint: D1 adds a third reason row there; the table already omits `removed` and its JSON example shows a "hold-timer-expired" reason no producer emits, both corrected in the D1 phase
- [ ] `docs/architecture/testing/interop.md` - the `bgp-graceful-restart-frr` paragraph
  → Constraint: that paragraph says the scenario "does not cover NOTIFICATION, reconnect F-bit or received-stale-policy behavior"; the interop phase rewrites it
  → Constraint: a new scenario directory is named, never numbered
- [ ] `docs/architecture/route-selection.md` - the page for `bestpath.go`
  → Constraint: Step 0 stale depreference is unchanged; only the producer of `StaleLevel` on receipt changes

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md`, `rfc/full/rfc9494.txt` Sections 4.2 to 4.4
  → Constraint: RFC9494-4.3-1 binds "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor": a received LLGR_STALE route is depreferenced only when Ze sent LLGR to that neighbour, which the GR plugin records per peer (`gr_llgr_exchange.go::recordSentLLGR`)
  → Constraint: the `Support remaining` Meta row of `rfc/short/rfc9494.md` names RFC9494-4.3-2 among its five gaps; removing the `{gap}` changes that row and so the generated `docs/features/rfc-status.md`
  → Constraint: RFC scope is frozen (2026-10-08): no enrolment, and `rfc/short/rfc8538.md` stays blocked
- [ ] `rfc/full/rfc4724.txt` Section 4 and 4.2, `rfc/full/rfc8538.txt` Section 2
  → Decision: D1 flushes on every NOTIFICATION because Ze never sends the N bit; N-bit retention stays with `plan/spec-gr-advanced.md`

**Key insights:**
- The reactor already knows a NOTIFICATION crossed the wire (`notificationExchanged`); D1 is a missing hand-off, not missing detection.
- The reactor already has a received-route metadata channel (in-process ingress filters write `ingressMeta`, which rides `ReceivedUpdate.Meta` to every egress filter); D3 and D4 need one ingress filter and one plumb of that map to plugin events, not a new channel.
- Every removal of COMMUNITIES, whatever its origin, ends in the one code-8 attribute handler, so D5 has one choke point.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr.go` - session-down handlers
  → Constraint: two parallel handlers, `handleStructuredState` (DirectBridge) and `handleStateEvent` (JSON fallback), carry the same `wasNotification` line; both change together
  → Constraint: subscriptions are "open direction received", "open direction sent", "state", "eor"; the plugin does not subscribe to UPDATE or NOTIFICATION events and must not start to (per-UPDATE plugin delivery for a peer-level fact)
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` (616L) - `onSessionDownDeferred`, `onSessionReestablished`, `enterLLGRLocked`, `handleLLSTExpired`
  → Constraint: `onSessionReestablished` calls `stopLLSTTimersLocked` (which nils `llgrFamilies`) BEFORE deciding purges, so the per-family "LLGR period has begun" set must be captured first; `inLLGR` is peer-wide and is not that set
  → Constraint: a family still in its conventional GR period follows the RFC 4724 rule alone; only a family present in `llgrFamilies` gets the RFC 9494 bullets
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter`, `staleFromMeta`
  → Constraint: fast path is "meta stale is zero"; D6 moves the threshold to `storage.DepreferenceThreshold` without adding a lock to the fresh-route path
- [ ] `internal/component/bgp/plugins/gr/gr_llgr_exchange.go` - `exchangedLLGRLocked`, `recordSentLLGR`
  → Decision: the per-peer "Ze advertised LLGR" fact for D3 is derived from what `recordSentLLGR` stores, published as an atomic snapshot so the ingress filter takes no mutex per UPDATE
- [ ] `internal/component/bgp/plugins/gr/register.go` - registers `LLGREgressFilter` (Stage Annotation, Readvertise) and the two community names
  → Decision: the D3 ingress filter registers in the same `filterapi.Filter` entry (its `Ingress` field)
- [ ] `internal/component/bgp/reactor/peer_run.go` - the Established-exit branch of the FSM callback
  → Constraint: reason chosen there feeds `announceClosed`, the FSM history `transitionReason` and the "session closed" log line; one variable, one choice
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyPeerClosed`, the received-UPDATE path (`ingressMeta`, `routeMeta`, `ReceivedUpdate.Meta`)
  → Constraint: the received `RawMessage` is built BEFORE the ingress steps; its `Meta` must be set after them, from the same `routeMeta`
- [ ] `pkg/plugin/rpc/enums.go` - `ReasonPeerRemoved`, `ReasonAttached`
  → Decision: `ReasonNotification` is declared beside them, the one spelling GR compares against
- [ ] `pkg/plugin/rpc/bridge.go` - `StructuredEvent.Meta` documented "sent events only"
  → Constraint: that comment changes with the plumb
- [ ] `internal/component/bgp/server/events.go` - `getStructuredEvent` copies `msg.Meta` into `se.Meta`
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - `handleReceivedStructured`; reads `se.Meta` keys replay, rib-lifecycle, config-static
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go`, `storage/routeentry.go` - insert resets `StaleLevel` to `StaleLevelFresh`; `DepreferenceThreshold` is 2
  → Constraint: a refresh of the same NLRI WITHOUT LLGR_STALE must still reset to fresh; a refresh WITH it must land at level 2
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `compareBeforeMED` Step 0, unchanged
- [ ] `internal/component/bgp/plugins/rib/rib_replay.go`, `rib_commands.go` - copy `StaleLevel` into `meta["stale"]` for replay and readvertise
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `IngressFilterFunc`, `EgressFilterFunc`, `Filter`, `LastSetOrSuppress`, `RegisterAttrModHandler`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `sendCommunitySuppression`, `applySendCommunityMask`
- [ ] `internal/component/bgp/reactor/filter_delta.go` - the "Removed attributes" Suppress branch
- [ ] `internal/component/bgp/plugins/filter_community/handler.go`, `register.go` - `genericCommunityHandler` serves codes 8, 16, 32; Suppress drops the attribute, Remove drops matching values
  → Constraint: zero-copy retained runs (`p.Op` fragments) must survive the change; LLGR_STALE re-insertion must not allocate per destination

**Behavior to preserve:**
- TCP loss (no NOTIFICATION) of a GR peer still retains, marks stale and starts the Restart Time timer.
- `removed` teardown still releases GR state and metrics without retention.
- BMP Peer Down reason selection (it reads its NOTIFICATION cache, not the reason string).
- A family still in conventional GR at reconnect: RFC 4724 rule unchanged.
- A fresh route never touches the GR egress filter beyond the zero-level check.
- Self-attached LLGR_STALE (LLGR entry) keeps level 2 and today's readvertise rail.
- Section 4.6 IBGP partial deployment (NO_EXPORT plus LOCAL_PREF 0) unchanged.

**Behavior to change:** the rows D1 to D5, and D6 if Q-2 is answered yes.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a session teardown (NOTIFICATION or TCP loss), a reconnect OPEN, an UPDATE carrying LLGR_STALE

### Transformation Path
1. reactor close reason to the GR plugin as a structured state event
2. GR state machine retains or purges
3. RIB best path and egress filters

Per defect, after the fix:

| Defect | Path |
|--------|------|
| D1 | NOTIFICATION sent or received -> `peer_stats.go` sets `notificationExchanged` -> FSM leaves Established (`peer_run.go`) -> reason is `rpc.ReasonNotification` when the flag is set -> `announceClosed` -> `notifyPeerClosed` -> observers -> state event to GR -> `handleStructuredState` / `handleStateEvent` compare against `rpc.ReasonNotification` -> `onSessionDownDeferred` clears state, no retain, no mark-stale -> RIB drops the peer's routes by normal DOWN handling. Site of the reason producer: the FSM leave-Established callback is today's site; `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` Phase 2b moves it to `Peer.runOnce` after `<-deliveryDone` (Blast Radius, "Shared producer with the rs spec") |
| D2 | reconnect OPEN -> GR `handleStructuredOpen` stores caps -> state up -> `onSessionReestablished` captures the set of families with an LLGR timer BEFORE stopping timers -> per family: purge if the RFC 4724 rule fails, and, for a family in that set, if any RFC 9494 Section 4.2 bullet holds -> `purge-stale` per family |
| D3 | UPDATE received -> `reactor_notify.go` in-process ingress steps -> GR ingress filter: source in the "Ze sent LLGR" snapshot AND COMMUNITIES carries 0xFFFF0006 -> writes stale level 2 into `ingressMeta` -> `routeMeta` -> (a) `ReceivedUpdate.Meta` for the forward rails, (b) NEW: received `RawMessage.Meta` -> `se.Meta` -> RIB `handleReceivedStructured` inserts with `StaleLevel` 2 -> Step 0 of best path makes it least preferred |
| D4 | forward rails: `update.Meta` stale 2 -> `LLGREgressFilter` per destination (withdraw EBGP non-LLGR, Section 4.6 IBGP, unchanged to LLGR); replay rail: RIB `StaleLevel` 2 -> `meta["stale"]` -> same filter |
| D5 | any COMMUNITIES op (send-community Suppress, `community-remove`, policy delta Suppress or Set) -> code-8 handler -> when the source attribute holds a value registered as preserved, the emitted attribute keeps it (a Suppress emits an attribute holding only the preserved values) |
| D6 | `LLGREgressFilter` early return extends from "level 0" to "level below `storage.DepreferenceThreshold`" (only if Q-2 = yes) |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ GR plugin | structured state event with a close reason | No |
| Reactor ↔ GR ingress filter | in-process `filterapi.IngressFilterFunc`, writes `meta["stale"]` (uint8) | No |
| Reactor ↔ RIB plugin | received UPDATE `StructuredEvent.Meta` (new for received events) | No |
| Reactor ↔ filter_community handler | `filterapi.AttrPlan` for code 8; preserved-value set read from `filterapi` | No |

### Integration Points
- `notifyPeerClosed`, `apiStateObserver.OnPeerClosed`
- `filterapi.Register` (GR adds `Ingress`), a new `filterapi` preserved-community registration that GR calls at `init()` and `genericCommunityHandler` reads
- `rpc.ReasonNotification` in `pkg/plugin/rpc/enums.go`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | D3 uses the existing ingress metadata channel; D1 uses the existing reason field |
| No unintended coupling (components stay isolated) | Yes (design) | RIB learns no LLGR fact: it reads a stale level from metadata; filter_community learns no LLGR fact: it reads a registered preserved set |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | no second LLGR-capability store; the snapshot is derived from `recordSentLLGR` |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | the community handler keeps its retained-run fragments; a preserved value is emitted from the source bytes |
| Registration over hardcoding, outbound | Yes (design) | GR registers its ingress filter and its preserved community; no core switch learns LLGR |
| Registration over hardcoding, inbound | Yes (design) | `genericCommunityHandler` gains no LLGR constant; the reason spelling is one declared constant compared by GR |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer sends NOTIFICATION | → | GR session-down handler | `test/plugin/gr-notification-flushes.ci` (new) |
| Ze sends NOTIFICATION (hold timer expiry on its side) | → | `peer_run.go` reason choice -> GR session-down handler | `TestPeerClosedReasonNotificationWhenExchanged` (reactor, new) plus `test/plugin/gr-notification-flushes.ci` second connection |
| TCP close, no NOTIFICATION | → | GR retention (unchanged) | `test/plugin/gr-mark-stale.ci` (existing, unchanged) |
| reconnect OPEN during LLGR | → | `onSessionReestablished` per-family rule | `test/plugin/llgr-reconnect-fbit-purge.ci` (new) |
| UPDATE with LLGR_STALE from an LLGR peer | → | GR ingress filter -> RIB insert -> best path | `test/plugin/llgr-stale-received-depreferenced.ci` (new) |
| UPDATE with LLGR_STALE forwarded to EBGP non-LLGR, LLGR and IBGP non-LLGR peers | → | `LLGREgressFilter` on the forward rail | `test/plugin/llgr-stale-received-egress.ci` (new) |
| `send-community none` / `community-remove ffff0006` destination | → | code-8 handler preserved set | `test/plugin/llgr-stale-community-preserved.ci` (new) |
| real peer daemons | → | all of the above | interop scenarios in the Interop Tests table |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GR peer torn down by a NOTIFICATION sent or received | routes flushed, no stale retention |
| AC-2 | reconnect during LLGR with each Section 4.2 purge condition | that family's stale routes removed |
| AC-3 | route received with LLGR_STALE from an LLGR neighbour | least preferred; not advertised to a non-LLGR neighbour; community kept on further advertisement |
| AC-4 | the `weak` verdicts of RFC4724-4-2, RFC9494-4.2-6, RFC9494-4.2-8, RFC9494-4.3-1 and RFC9494-4.3-3, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28; RFC9494-4.3-3 transfer completed 2026-10-05) |
| AC-5 | GR peer whose TCP session is lost with no NOTIFICATION exchanged (D1 negative polarity) | routes retained and marked stale level 1, Restart Time timer armed, exactly as today |
| AC-6 | any Established -> non-Established transition after a NOTIFICATION was sent or received in that session | the state-down event's reason is `rpc.ReasonNotification` ("notification") on both the structured and the JSON plugin paths and in FSM history; a session without one keeps "session closed" / "connection lost"; peer removal keeps "removed" |
| AC-7 | reconnect, family F in its LLGR period, new GR lists F with F bit set, new LLGR lists F with F bit clear | F's stale routes purged at once (RFC9494-4.2-8 bullet 1) |
| AC-8 | reconnect, family F in its LLGR period, new GR lists F with F bit set, new LLGR omits F (or no LLGR capability at all) | F's stale routes purged at once (bullet 2) |
| AC-9 | reconnect during LLGR with neither GR nor LLGR capability | every stale family purged, peer state released (bullet 3; behaviour already correct, kept) |
| AC-10 | reconnect, family F in its LLGR period, both new capabilities list F with F bit set | F kept until its EOR (negative polarity) |
| AC-11 | (Q-1 = AND) reconnect, family F in its LLGR period, new LLGR lists F with F set, new GR omits F or clears its F bit | F purged at once |
| AC-12 | reconnect, family G still in its conventional GR period (no LLGR timer), new GR lists G with F set, new LLGR omits G | G kept until EOR: the RFC 9494 bullets apply only "once the LLGR Period begins" |
| AC-13 | UPDATE carrying LLGR_STALE received from a peer Ze sent the LLGR capability to, competing with the same prefix from another peer at a LOWER LOCAL_PREF and a HIGHER peer address | the LLGR_STALE route is stored at stale level 2 and loses (`show bgp rib` best is the other route, best-path explain names the stale step) |
| AC-14 | the same UPDATE received from a peer Ze did NOT send LLGR to | stored at level 0; wins on LOCAL_PREF (negative polarity: the obligation binds only "A BGP speaker that has advertised" LLGR) |
| AC-15 | the LLGR_STALE route is refreshed by an UPDATE for the same NLRI without LLGR_STALE | entry returns to level 0 and competes normally |
| AC-16 | a received LLGR_STALE route (AC-13 conditions) is forwarded | EBGP neighbour without LLGR: not advertised (withdrawn if previously sent); neighbour with LLGR: advertised with LLGR_STALE intact; IBGP neighbour without LLGR: advertised with NO_EXPORT and LOCAL_PREF 0. Same three outcomes on the RIB replay rail when such a neighbour comes up later |
| AC-17 | a route whose COMMUNITIES holds LLGR_STALE and other values, sent to a destination with `send-community none`, one with `community-remove ffff0006`, and one whose export policy removes the COMMUNITIES attribute | each destination receives COMMUNITIES containing LLGR_STALE; the other values are removed as configured (Q-3) |
| AC-18 | a route WITHOUT LLGR_STALE to the same three destinations | today's output byte for byte: no COMMUNITIES for `send-community none`, the configured removals otherwise (negative polarity) |
| AC-19 | (Q-2 = yes) a route at stale level 1 (conventional GR period) replayed to an EBGP neighbour without LLGR | advertised unchanged; a level-2 route to the same neighbour is still withheld |
| AC-20 | `./le rfc check` after the change | RFC9494-4.3-2 carries no `{gap}`, has positive and negative tagged tests with discrimination records; the `Support remaining` row of `rfc/short/rfc9494.md` no longer names 4.3-2; no new violation for RFC 4724 or 9494 |
| AC-21 | the interop scenarios in the Interop Tests table | each passes against its named peer daemon, and each was seen red with its producer fix reverted and the Ze image rebuilt |

### AC proofs

| AC | Proof (test that fails if the AC breaks) |
|----|------------------------------------------|
| AC-1 | `TestHandleStructuredState_NotificationReasonFlushes`, `TestHandleStateEvent_NotificationReasonFlushes` (gr, new); `test/plugin/gr-notification-flushes.ci`; interop `bgp-graceful-restart-frr` NOTIFICATION phase |
| AC-2 | AC-7 to AC-12 proofs |
| AC-3 | AC-13 to AC-17 proofs |
| AC-4 | five `./le rfc audit-stamp ... mode rejudge` records by an agent that wrote none of the tests, verdict `enforced` in `rfc/audit/*.json` |
| AC-5 | `TestHandleStructuredState_ConnectionLostRetains` (gr, new); `test/plugin/gr-mark-stale.ci` (existing) |
| AC-6 | `TestPeerClosedReasonNotificationWhenExchanged`, `TestPeerClosedReasonWithoutNotification` (reactor, new) |
| AC-7 | `TestOnSessionReestablished_LLGRFBitClearPurges` (gr, new) |
| AC-8 | `TestOnSessionReestablished_FamilyAbsentFromLLGRPurges` (gr, new); `TestOnSessionReestablished_DuringLLGR_NoLLGRCap` (untagged, assertion corrected) |
| AC-9 | `TestOnSessionReestablished_DuringLLGR_NoCaps` (existing tagged, unchanged body) |
| AC-10 | `TestOnSessionReestablished_DuringLLGR` (existing tagged; see the tagged-test table) |
| AC-11 | `TestOnSessionReestablished_LLGRPeriodGRFBitClearPurges` (gr, new) |
| AC-12 | `TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets` (gr, new) |
| AC-13 | `TestLLGRIngressFilter_MarksReceivedStale` (gr, new); `TestRIBReceivedStaleMetaSetsLevel` (rib, new); `TestComparePair_LLGRStale` (strengthened); `test/plugin/llgr-stale-received-depreferenced.ci`; interop `bgp-llgr-stale-received-frr` |
| AC-14 | `TestLLGRIngressFilter_NoLLGRSentLeavesFresh` (gr, new); same `.ci`, second source peer |
| AC-15 | `TestRIBReceivedRefreshWithoutStaleResetsLevel` (rib, new) |
| AC-16 | `TestLLGREgressFilter_ReceivedStaleMeta` (gr, new, all three destinations); `test/plugin/llgr-stale-received-egress.ci`; interop `bgp-llgr-stale-received-frr` |
| AC-17 | `TestCommunityHandler_PreservedSurvivesSuppress`, `_PreservedSurvivesRemove`, `_PreservedSurvivesSet` (filter_community, new); `test/plugin/llgr-stale-community-preserved.ci` |
| AC-18 | `TestCommunityHandler_NoPreservedValueUnchanged` (filter_community, new); `test/plugin/send-community-suppress.ci` (existing, unchanged) |
| AC-19 | `TestLLGREgressFilter_GRStaleLevelOnePassesThrough` (gr, new) |
| AC-20 | `./le rfc check` output pasted at closure |
| AC-21 | `./le test integration interop` scenario selectors for both scenarios, red and green outputs recorded |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | operator runs `clear bgp` on a GR neighbour (it sends Cease) | peer NOTIFICATION -> reactor reason -> GR flush -> RIB withdraw downstream | interop `bgp-graceful-restart-frr` NOTIFICATION phase |
| 2 | a neighbour in LLGR reconnects having lost forwarding state | OPEN caps -> GR per-family purge | `test/plugin/llgr-reconnect-fbit-purge.ci` |
| 3 | an upstream helper readvertises LLGR_STALE routes to Ze | ingress filter -> RIB least preferred -> egress per destination | interop `bgp-llgr-stale-received-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per row | beside each producer | D1 to D5 | filled by the rows below (2026-10-13) |
| `TestPeerClosedReasonNotificationWhenExchanged` / `TestPeerClosedReasonWithoutNotification` | `internal/component/bgp/reactor/` (beside the existing `peer_stats_test.go`) | AC-6 | new |
| `TestHandleStructuredState_NotificationReasonFlushes`, `TestHandleStateEvent_NotificationReasonFlushes`, `TestHandleStructuredState_ConnectionLostRetains` | `internal/component/bgp/plugins/gr/rfc4724_dispatch_test.go` | AC-1, AC-5; tagged RFC4724-4-2 positive and negative | new |
| `TestOnSessionReestablished_LLGRFBitClearPurges`, `_FamilyAbsentFromLLGRPurges`, `_LLGRPeriodGRFBitClearPurges`, `_GRPeriodFamilyIgnoresLLGRBullets` | `internal/component/bgp/plugins/gr/gr_state_test.go` | AC-7, AC-8, AC-11, AC-12; tagged RFC9494-4.2-8 positive (bullets 1, 2) | new |
| `TestOnSessionReestablished_DuringLLGR_NoLLGRCap` | same | AC-8: assertion flips to "purged" (untagged; a test wrong about what it asserts) | change |
| `TestLLGRIngressFilter_MarksReceivedStale`, `_NoLLGRSentLeavesFresh`, `_NoCommunityLeavesFresh` | `internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go` (new file) | AC-13, AC-14; tagged RFC9494-4.3-1 and RFC9494-4.2-6 | new |
| `TestRIBReceivedStaleMetaSetsLevel`, `TestRIBReceivedRefreshWithoutStaleResetsLevel` | `internal/component/bgp/plugins/rib/rfc9494_test.go` | AC-13, AC-15; tagged RFC9494-4.3-1 positive | new |
| `TestLLGREgressFilter_ReceivedStaleMeta`, `TestLLGREgressFilter_GRStaleLevelOnePassesThrough` | `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | AC-16, AC-19; tagged RFC9494-4.3-3 | new |
| `TestCommunityHandler_PreservedSurvivesSuppress`, `_PreservedSurvivesRemove`, `_PreservedSurvivesSet`, `_NoPreservedValueUnchanged` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-17, AC-18; tagged RFC9494-4.3-2 positive and negative | new |
| `TestReceivedUpdateEventCarriesIngressMeta` | `internal/component/bgp/reactor/` | the plumb: a received UPDATE's plugin event carries `routeMeta` | new |
| `BenchmarkLLGRIngressFilter_NoLLGRPeer` | `internal/component/bgp/plugins/gr/` | the per-UPDATE cost for a peer Ze sent no LLGR is a snapshot load and a map miss, 0 allocs | new |

### RFC-tagged tests this spec changes (owner approval, Q-4)

| Tagged unit | Requirement | Change | Why |
|-------------|-------------|--------|-----|
| `internal/component/bgp/plugins/rib/bestpath_test.go::TestComparePair_LLGRStale` | RFC9494-4.3-1 positive | the stale candidate gets the higher LOCAL_PREF and the lower peer address, so only Step 0 can make it lose | the audit found it passes with Step 0 removed |
| `internal/component/bgp/plugins/gr/gr_state_test.go::TestOnSessionReestablished_DuringLLGR` | RFC9494-4.2-8 negative | body unchanged; tag prose rewritten (it cites `newForwarding` and gr_state.go line ranges the fix replaces) and a fresh discrimination record | a tagged unit whose producer and claim change owes a new record |

Every other tagged unit named in the verdict table keeps its body and tag:
`TestGRStateManagerNotificationBypass`, `TestGRStateManagerRouteRetention`,
`TestOnSessionReestablished_DuringLLGR_NoCaps`, both `rib/rfc9494_test.go` units,
`TestComparePair_GRStaleCompetesNormally`, both 4.3-3 egress units and
`test/plugin/llgr-egress-state-unloaded.ci`. New tagged units are additions, each
with a `./le rfc discriminate-record` record in the same change.

#### Independent audit of these changes (2026-10-13)

Source: `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/rfc-tagged-test-compliance-review.md`, section 5.
Re-checked against the test bodies at HEAD and `rfc/full/rfc9494.txt` Sections 4.2 to 4.4; agreed on every row.

| Tagged unit | Audit verdict | What the change owes |
|-------------|---------------|----------------------|
| `TestComparePair_LLGRStale` | COMPLIANT, prose to narrow | The body never marks a route LLGR_STALE: it builds a `Candidate` with `StaleLevel: 2` and calls `ComparePair`. The tag prose says "a route marked LLGR_STALE"; rewrite it to claim what the body checks: a candidate at `StaleLevel 2` (at `storage.DepreferenceThreshold`) loses Step 0 to one below it, whatever its LOCAL_PREF and peer address. The receive-path marking stays with `TestLLGRIngressFilter_MarksReceivedStale` and `TestRIBReceivedStaleMetaSetsLevel` (AC-13). The both-stale tie case (`stale` vs `stale2`) must keep equal LOCAL_PREF after `stale` is raised, so it still proves RFC 9494 Section 4.4 "normal tiebreaking applies" on the peer address and its "lower peer addr" message stays true |
| `TestOnSessionReestablished_DuringLLGR` | COMPLIANT | Both F bits set means the family is kept under the Q-1 AND reading, which is RFC 4724 Section 4.2 applied "irrespective of whether the stale routes are retained during the Restart Time period or the Long-Lived Stale Time period" (RFC 9494 Section 4.2). Body unchanged; only the prose and the record change, as the row above says |

The audit also confirms unchanged and still correct: `TestGRStateManagerNotificationBypass`
and `TestGRStateManagerRouteRetention` (their claims are scoped to `onSessionDown`),
`TestOnSessionReestablished_DuringLLGR_NoCaps`, and the 4.3-3 egress units (all use
stale level 2, so D6 does not move them).

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-4). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`; both names were checked to exist at HEAD `051876c3f4`. New tagged units need no approval.

2 commands:

```
./le rfc approve unit rib.TestComparePair_LLGRStale reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-graceful-restart-rfc-defects, stale candidate gets higher LOCAL_PREF and lower peer address, tag prose narrowed to StaleLevel 2)'
./le rfc approve unit gr.TestOnSessionReestablished_DuringLLGR reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-graceful-restart-rfc-defects, body unchanged, tag prose rewritten)'
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stale level in route metadata | 0-255 | 1 passes the egress filter (Q-2), 2 is withheld | N/A | N/A |

### Functional Tests
- filled by the table below (2026-10-13)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-notification-flushes` | `test/plugin/gr-notification-flushes.ci` | peer with GR sends routes and EOR, then NOTIFICATION: `show bgp rib received` holds none of its routes before its second connection, no `gr-state` row exists; second connection ends with Ze's hold timer NOTIFICATION, same outcome | new |
| `llgr-reconnect-fbit-purge` | `test/plugin/llgr-reconnect-fbit-purge.ci` | `le test peer` with restart-time 0 and LLST 60 sends two families, closes TCP, reconnects with LLGR F clear for one family and absent for the other (one connection each): both purged before EOR | new; needs the `le test peer` OPEN option to clear the LLGR F bit per family (A-4) |
| `llgr-stale-received-depreferenced` | `test/plugin/llgr-stale-received-depreferenced.ci` | two sources announce one prefix; the LLGR one carries LLGR_STALE and the higher LOCAL_PREF; best path is the other source | new |
| `llgr-stale-received-egress` | `test/plugin/llgr-stale-received-egress.ci` | the AC-16 three destinations | new |
| `llgr-stale-community-preserved` | `test/plugin/llgr-stale-community-preserved.ci` | the AC-17 three destinations, exact COMMUNITIES hex | new |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-graceful-restart-frr` (extended) | `test/interop/scenarios/bgp-graceful-restart-frr/` | FRR | a NOTIFICATION phase: FRR `clear bgp <ze> ` sends Cease; Ze must hold none of FRR's routes within a bound far below FRR's Restart Time, and FRR must see Ze's onward withdrawal. Ze sends no N bit, so FRR's own N bit does not invoke RFC 8538 (A-2) | new phase |
| `bgp-llgr-stale-received-frr` | `test/interop/scenarios/bgp-llgr-stale-received-frr/` (new) | FRR (sender, LLGR negotiated, route-map sets `llgr-stale` and a higher local preference) and GoBGP (EBGP receiver without LLGR) | Ze depreferences the received LLGR_STALE route against a GoBGP-sourced copy, does not advertise it to the GoBGP EBGP neighbour, and a second FRR instance with LLGR negotiated receives it with LLGR_STALE intact | new |

D2 is proven by `le test peer` rather than interop: no daemon in the lab lets a
test choose the LLGR F bit per family on reconnect (A-4). Goal validation for
D2 is the `.ci`, which drives the real reconnect path.

Goal validation plan:

| Goal | Defect | Evidence | Red run |
|------|--------|----------|---------|
| A NOTIFICATION teardown flushes the peer's routes | D1 | the NOTIFICATION phase of `bgp-graceful-restart-frr`: FRR `clear bgp` sends Cease, Ze holds none of FRR's routes within the bound | red with the `peer_run.go` reason change reverted and the Ze image rebuilt |
| An LLGR-period family with its LLGR F bit clear, or absent from the new LLGR capability, is purged on reconnect | D2 | `test/plugin/llgr-reconnect-fbit-purge.ci` | red with the `gr_state.go::onSessionReestablished` change reverted |
| A received LLGR_STALE route is least preferred | D3 | `bgp-llgr-stale-received-frr`: Ze's best path is the GoBGP-sourced copy despite FRR's higher local preference | red with `LLGRIngressFilter` unregistered and the Ze image rebuilt |
| A received LLGR_STALE route is withheld from an EBGP neighbour without LLGR | D4 | the same scenario: GoBGP's RIB lacks the prefix | the same red run: GoBGP's RIB holds the prefix |
| LLGR_STALE is kept on further advertisement | D5 | `test/plugin/llgr-stale-community-preserved.ci` (exact COMMUNITIES hex), plus the second FRR instance in `bgp-llgr-stale-received-frr` showing `llgr-stale` on the route | red with the `filter_community/handler.go` preservation reverted (and the Ze image rebuilt for the interop leg) |

## Files to Modify

- the producers in the defect table and their tests; `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md`

| File | Change |
|------|--------|
| `pkg/plugin/rpc/enums.go` | declare `ReasonNotification` |
| `pkg/plugin/rpc/bridge.go` | `StructuredEvent.Meta` comment: received events carry ingress metadata too |
| `internal/component/bgp/reactor/peer_run.go` | reason is `rpc.ReasonNotification` when `notificationExchanged` |
| `internal/component/bgp/reactor/reactor_notify.go` | received `RawMessage.Meta` set from `routeMeta` after the ingress steps |
| `internal/component/bgp/plugins/gr/gr.go` | both down handlers compare against `rpc.ReasonNotification`; stale header comment on `handleStateEvent` updated |
| `internal/component/bgp/plugins/gr/gr_state.go` | `onSessionReestablished` per-family rule (capture LLGR-period set before stopping timers) |
| `internal/component/bgp/plugins/gr/gr_llgr_exchange.go` | publish the "Ze sent LLGR" peer snapshot |
| `internal/component/bgp/plugins/gr/gr_ingress.go` (new) | `LLGRIngressFilter` |
| `internal/component/bgp/plugins/gr/gr_egress.go` | D6 threshold (Q-2) |
| `internal/component/bgp/plugins/gr/register.go` | `Ingress: LLGRIngressFilter`; register LLGR_STALE as preserved |
| `internal/component/bgp/filterapi/filterapi.go` | preserved-community registration and read accessor |
| `internal/component/bgp/plugins/filter_community/handler.go` | keep preserved values on Suppress, Remove and Set for code 8 |
| `internal/component/bgp/plugins/rib/rib_structured.go`, `storage/familyrib.go` | insert at the metadata stale level instead of always fresh |
| `rfc/short/rfc9494.md` | drop the 4.3-2 `{gap}`, update `Support remaining` |
| `docs/guide/graceful-restart.md`, `docs/architecture/api/architecture.md`, `docs/architecture/testing/interop.md`, `docs/architecture/meta/` (the `stale` key on received routes) | page edits in the phase that changes the behaviour |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/plugins/gr/gr_ingress.go` | D3 ingress filter |
| `internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go` | its tests |
| `test/plugin/gr-notification-flushes.ci`, `llgr-reconnect-fbit-purge.ci`, `llgr-stale-received-depreferenced.ci`, `llgr-stale-received-egress.ci`, `llgr-stale-community-preserved.ci` | functional proofs |
| `test/interop/scenarios/bgp-llgr-stale-received-frr/` | interop scenario (FRR and GoBGP configs) plus its checker in `internal/le/interoplab/bgp/` |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config or RPC added |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf; Q-3 recommends no refusal of `community-remove ffff0006` |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | Yes | the five `.ci` files above (behaviour, not a new RPC) |
| Pipe completeness | N-A | no output command |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no runtime dependency; the existing `bgp-gr` doctor check (`doctor.go`, out-of-process engine leaves the egress filter blind) now covers the ingress filter too, and its message names both |
| Prometheus counters/metrics | No | none added; A-5 records why no counter for received LLGR_STALE |
| BGP family surface (new SAFI / capability / attribute) | No | no family, capability or attribute added |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope/evidence/level changed? | Yes | the features entry that cites Graceful Restart (`./le feature report` names it; `features/` has no GR file today, so the closure checks the report) |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/architecture.md` Close Reasons (new `notification`, missing `removed`, stale example) |
| 5 | Plugin added/changed? | Yes | `docs/guide/graceful-restart.md` (plugin behaviour), `docs/guide/plugins.md` only if it lists GR filters |
| 6 | Has a user guide page? | Yes | `docs/guide/graceful-restart.md` |
| 7 | Wire format changed? | No | what goes on the wire changes (LLGR_STALE kept), not a format |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` if it documents state-event reasons or event `meta` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md`, generated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Maybe | `docs/functional-tests.md` if `le test peer` gains a per-family LLGR F-bit option (A-4) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (filter pipeline: preserved communities; received event metadata) |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md` and the GR meta page: `stale` now set at ingress |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | GR adds an ingress filter to its existing registration |
| 16 | Changed source files referenced by doc source anchors? | Derived | `./le spec citation anchors spec plan/immediate/spec-bgp-graceful-restart-rfc-defects.md` at implementation |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/graceful-restart.md` stale-level table and readvertise table |

## Discovery (repo-maintenance Mechanical Checklist)

| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` graceful-restart row -> `docs/guide/graceful-restart.md`; `ai/CODE-TO-DOCS.md` for each file above |
| What rule prevents regression? | the RFC-tagged units with discrimination records; `./le rfc check` |
| What registry prevents drift? | `filterapi.Register` (ingress filter) and the preserved-community registration; `rpc` reason constants |
| What verification proves it? | the `.ci` set and the two interop scenarios |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D1: the reactor names the reason (`rpc.ReasonNotification`) from `notificationExchanged` | (b) GR subscribes to NOTIFICATION events and caches per peer, as BMP does | (b) depends on event ordering between two deliveries and duplicates a fact the reactor already holds; (a) is one declared constant and one branch at the single producer |
| D2: per-family rule, LLGR bullets only for families with a running LLGR timer, AND with the RFC 4724 rule (Q-1) | (b) keep one peer-wide `wasInLLGR` switch and intersect instead of union | (b) wrongly applies LLGR bullets to a family still in its conventional GR period (AC-12) |
| D3/D4: GR in-process ingress filter writes stale level 2 into ingress metadata; reactor plumbs that map to the received plugin event; RIB inserts at that level | (b) RIB scans COMMUNITIES for 0xFFFF0006 on every insert; (c) GR subscribes to UPDATEs and dispatches `mark-stale` per route | (b) cannot know whether Ze sent LLGR to the source, puts LLGR knowledge in the RIB, and leaves the forward rails without metadata; (c) is per-UPDATE plugin traffic and races the insert. (a) feeds the RIB, the forward rails and the replay rail from one decision |
| D5: preserved-community registration in `filterapi`, consulted by the code-8 handler | (b) LLGR_STALE constant hardcoded in `genericCommunityHandler`; (c) exempt at each of the three producers | (b) is a central enumeration in a plugin that does not own LLGR; (c) is three copies and misses the next producer. One handler sees every removal |
| D6: threshold at `storage.DepreferenceThreshold` | keep any nonzero level | RFC 4724 Section 4.2 sentence quoted above; pending Q-2 |

## Implementation Steps

1. Failing tests; 2. fixes; 3. discrimination records and verdicts.

Detailed phases (each: failing test, fix, pass, page edit in the same phase):

1. **Phase: Wiring** -- declare `rpc.ReasonNotification`; register the empty GR ingress filter; register the preserved set; plumb received `Meta`. Tests: `TestReceivedUpdateEventCarriesIngressMeta`, wiring `.ci` files red.
2. **Phase: D1** -- reason choice in `peer_run.go`, both GR handlers. Tests: AC-1, AC-5, AC-6 rows. Pages: api/architecture.md Close Reasons, graceful-restart.md Peer Restarts.
3. **Phase: D2** -- `onSessionReestablished`. Tests: AC-7 to AC-12 rows. Page: graceful-restart.md LLGR step 5.
4. **Phase: D3** -- ingress filter, sent-LLGR snapshot, RIB insert level. Tests: AC-13 to AC-15 rows, benchmark. Pages: meta docs, graceful-restart.md stale levels.
5. **Phase: D4/D6** -- egress threshold (Q-2) and received-stale egress proofs. Tests: AC-16, AC-19 rows. Page: graceful-restart.md readvertise table.
6. **Phase: D5** -- code-8 handler. Tests: AC-17, AC-18 rows. Pages: core-design.md filter pipeline; `rfc/short/rfc9494.md` gap and Support row.
7. **Phase: interop** -- extend `bgp-graceful-restart-frr`, add `bgp-llgr-stale-received-frr`, record red with each fix reverted and the image rebuilt. Page: testing/interop.md.
8. **Phase: verdicts** -- discrimination records for every new or changed tagged unit; independent re-judge of the five verdicts (AC-4).

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has a test in AC proofs and code at a named symbol |
| Correctness | the LLGR-period family set is captured before `stopLLSTTimersLocked`; a refresh without LLGR_STALE returns to level 0 |
| Data flow | the RIB reads a stale level, never the community; filter_community reads a registered set, never an LLGR constant |
| Rule: performance | ingress filter: no mutex, no allocation for a peer Ze sent no LLGR; community handler keeps zero-copy fragments |
| Rule: rfc-compliance | every new implementing function quotes its RFC sentence; every caller names the section |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| reason constant used by both GR handlers | grep for the literal "notification" in `plugins/gr/` finds none outside the constant's comparison |
| no `{gap}` on RFC9494-4.3-2 | `./le rfc check` |
| five verdicts enforced | `rfc/audit/rfc4724.json`, `rfc/audit/rfc9494.json` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | a peer can attach LLGR_STALE to any route: Ze depreferences it only when Ze sent LLGR to that peer (RFC 9494 Section 5.2 risk); a malformed COMMUNITIES length is already treat-as-withdraw at receipt, so the ingress scan sees whole values only |
| Policy bypass | the preserved set lets one value through `send-community none`; it must hold LLGR_STALE only, registered by GR |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `notificationExchanged` is reset per session before any NOTIFICATION of the new session can set it | the store of false at session start in `peer_run.go`; setters in `peer_stats.go` | a stale true would flush a later TCP-loss restart | `TestPeerClosedReasonWithoutNotification` after a prior notified session | unvalidated |
| A-2 | Ze never sends the N bit, so RFC 8538 never applies | `peer_gr_flags.go::restartFlagsFor`; `gr_capability.go` leaves the flags nibble zero | D1 would violate RFC 8538 for N-bit peers | grep at implementation; interop FRR phase shows FRR flushing nothing of its own and Ze flushing | unvalidated |
| A-3 | `filter_community`'s code-8 handler is linked in every build that forwards, so every COMMUNITIES op reaches it | `filter_community/register.go` registers at `init()`; `filterapi.go` comment that reactor's generic handler omits community codes because of it | a build without it would bypass D5 | grep of `plugin/all/all.go` at implementation | unvalidated |
| A-4 | `le test peer` can send an LLGR capability with a per-family F bit clear, or can gain that option cheaply | `option=open:value=llgr` exists; per-family flag control not verified | AC-7 `.ci` needs a tool change first | read the peer's OPEN option parser | unvalidated |
| A-5 | no counter is owed for received LLGR_STALE routes | none of the existing GR metrics count routes | an operator cannot see how many routes were depreferenced on receipt | owner review of the design | unvalidated |
| A-6 | the RIB runs in-process, so `se.Meta` reaches it as a map | RIB is an internal plugin; `StructuredEvent.Meta` is a Go map on DirectBridge | an external RIB would need `meta` on its JSON event | `TestReceivedUpdateEventCarriesIngressMeta` and the JSON event encoder check | unvalidated |
| A-7 | FRR can originate a route carrying `llgr-stale` via route-map toward a session where LLGR is negotiated | FRR community keyword list includes llgr-stale | the interop scenario needs a native injector instead | first interop dry run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | changing the reason string breaks a test or consumer that matches "session closed" after a NOTIFICATION | grep of tests for the two strings; reactor tests red | the change is limited to notified sessions; consumers that need the distinction already use BMP's cache |
| R-2 | ingress filter adds per-UPDATE cost on the receive hot path | benchmark; allocs/op above 0 | atomic snapshot lookup first; scan COMMUNITIES only for peers Ze sent LLGR |
| R-3 | plumbing `routeMeta` into received plugin events changes what other plugins see (`src-role` now visible) | plugin tests that assert `se.Meta == nil` on received events | audit `se.Meta` readers (`rib_structured.go` reads three named keys only) |
| R-4 | Q-1 AND reading purges LLGR-only families at reconnect, a visible traffic change | AC-11 `.ci`; operator report | owner decides Q-1 before implementation |
| R-5 | `send-community none` peers start receiving one community (Q-3) | `send-community-suppress.ci` stays green only for routes without LLGR_STALE | documented in the guide; only LLGR-capable or 4.6 IBGP destinations ever receive such routes after D4 |
| R-6 | Section 4.6 IBGP branch adds NO_EXPORT while `send-community none` suppresses code 8. Read from `genericCommunityHandler`: a Suppress as the last Set/Suppress drops the attribute and ignores every Add, so the NO_EXPORT the 4.6 branch adds is lost today for such a destination (RFC 9494 Section 4.6: "The NO_EXPORT community [RFC1997] MUST be attached to the stale routes.") | AC-16 IBGP destination with `send-community none` | owner question Q-7; not fixed silently, because preserving an Added value through a Suppress changes the handler contract for every producer |
| R-7 | concurrent reload changes which peers Ze sent LLGR to while UPDATEs arrive | race test on the snapshot | copy-on-write snapshot published under the GR mutex, read lock-free |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | routes kept or dropped wrongly across restarts; stale routes selected or leaked to neighbours; communities leaking past `send-community none` |
| How is it reverted? | single commit revert per phase; no config or wire format migration |
| Who else touches this path? | `plan/immediate/spec-bgp-llgr-per-family-config.md` (same GR plugin, skeleton); `plan/spec-gr-advanced.md` (N bit on top of D1); `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (AC-4 parent) |
| Shared producer with the rs spec | `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` (Phase 2b, Data Flow 5, Blast Radius "Depends on GR D1") depends on D1 and shares two sites with it: D1's teardown reason producer in `reactor/peer_run.go`, and the no-retention check in `gr.go::handleStructuredState` / `handleStateEvent`. (a) It adds a second input to that producer, `receivePublication.failed`, reported as a second declared close reason `rpc.ReasonAdjRIBInInvalid` beside `rpc.ReasonNotification` (spelling is the rs spec's own Q-6, distinct from this spec's Q-6), and one declared predicate `rpc.ReasonForbidsRetention` in `pkg/plugin/rpc/enums.go`, true for `rpc.ReasonNotification`, `rpc.ReasonAdjRIBInInvalid` and `rpc.ReasonPeerRemoved`, which `gr.go` calls in place of a reason comparison. (b) It moves the down event, and the reason producer with it, from the FSM leave-Established callback to `Peer.runOnce` after `<-deliveryDone`, so a session's buffered UPDATEs reach every plugin before its down event. Whichever spec lands second carries the move: if D1 lands first, rs Phase 2b moves D1's producer; if rs Phase 2b lands first, D1 writes its producer at the `runOnce` site |

## Known Limitations

- RFC 8538 N-bit retention stays in `plan/spec-gr-advanced.md`.
- RFC9494-4.2-7, 4.2-9, 4.2-10 and 5-2 are not in this spec: 5-2 belongs to `plan/immediate/spec-bgp-llgr-per-family-config.md`; the other three await Q-5.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC 4724
Section 4 at both GR down handlers' NOTIFICATION branch, RFC 9494 Section 4.2 at the
per-family reconnect rule, RFC 9494 Section 4.3 at the ingress filter, the RIB
insert level and the code-8 preservation, RFC 4724 Section 4.2 at the D6 threshold.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
