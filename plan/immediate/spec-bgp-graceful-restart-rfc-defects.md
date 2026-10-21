# Spec: bgp-graceful-restart-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

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
| D5 | RFC 9494 Section 4.2 (RFC9494-4.2-6) with Section 4.3 (RFC9494-4.3-2) | "The LLGR_STALE community MUST NOT be removed when the route is further advertised." | `internal/component/bgp/reactor/peer_forward_facts.go::applyFactsSendCommunity`, `internal/component/bgp/plugins/filter_community/handler.go::genericCommunityHandler` (the old `removeValues` helper no longer exists) | The recorded 4.3-2 gap: the community can be stripped on egress, so Section 4.3 is not performed in full and 4.2-6 is claimed on one bullet | unimplemented finding; 4.3-2 carries `{gap}` |

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
| D1 | Open | `gr.go::handleStructuredState` and `gr.go::handleStateEvent` both compute `wasNotification` as the reason equal to the literal "notification". The only producer of a teardown reason, the FSM callback in `peer_run.go`, sets "session closed", or "connection lost" when the target state is Idle, then calls `announceClosed` -> `notifyPeerClosed`. It already knows the answer: `p.notificationExchanged` (set by `incrNotificationSent` / `incrNotificationReceived` in `peer_stats.go`, cleared at session start in `peer_run.go`) is read two lines earlier, but only for `raiseSessionDropped`. The other reason producer, `removePeer`, emits `rpc.ReasonPeerRemoved` ("removed"). Two readings D1 states (design review r1, N-5): a NOTIFICATION Ze built but could not write is recorded by `peer_stats.go::recordNotificationUnsent`, which leaves `notificationExchanged` false, so that teardown keeps "session closed" / "connection lost" and GR still retains (no NOTIFICATION crossed the wire; the rs spec's publication-failure reason is a separate input). Ze's own hold-timer expiry sends a NOTIFICATION through `session_write.go::sendNotificationWithin`; when the write succeeds (`delivered` true) `incrNotificationSent` sets the flag, so that teardown now FLUSHES the peer's routes, because Ze sends no N bit (A-2). |
| D2 | Open | `gr_state.go::onSessionReestablished` builds one `newForwarding` set from the GR F bits, then, when the peer was in LLGR, ADDS every family whose LLGR F bit is set. A family is kept when either bit is set. A family absent from the new LLGR capability, or with its LLGR F bit clear, survives when GR lists it with F set. The untagged `TestOnSessionReestablished_DuringLLGR_NoLLGRCap` asserts that wrong outcome ("GR F-bit=1, stale kept until EOR") under a VALIDATES line that says the opposite ("delete all stale"). |
| D3 | Open | `StaleLevel` is set only by `mark-stale` and `attach-community` commands (`rib_commands.go`, `rib_commands_community.go`). Every insert path in `storage/familyrib.go` resets an entry to `StaleLevelFresh`. No non-test code outside the GR community-name registration reads `attribute.CommunityLLGRStale` on receipt. The reactor computes ingress route metadata (`reactor_notify.go`, `ingressMeta` -> `routeMeta` -> `ReceivedUpdate.Meta`), but the received `bgptypes.RawMessage` handed to plugins carries no `Meta`, so the RIB never sees it. |
| D4 | Open | `gr_egress.go::LLGREgressFilter` returns at once when `meta["stale"]` is zero. A received LLGR_STALE route carries no stale metadata on any of the three rails that reach the filter, so it is advertised unchanged to an EBGP neighbour that never sent LLGR. (1) Live forward rails, `reactor_api_forward.go` and `forward_rs.go`: `update.Meta` holds no stale level. (2) RIB rails, `rib_replay.go::replayRoutesWithCursor` and `resendRoutesWithCursor` (through `collectGroupedRibOutRoutesFiltered`) and `rib_commands.go::sendRoutes` (through `ribout_entry.go::collectRibOutRoutes` -> `reconstructRoute`): each copies `ribOutEntry.StaleLevel`, which `handleSentStructured` and `rib.go::handleSent` store as 0 and only `markStaleCommand` and `attachSentSourceCommunityLocked` write; the reactor runs the readvertise egress filters only when the batch stale level is above 0 (`reactor_api_batch.go`, "the common Stale==0 path is untouched"). (3) Relay rails, the `bgp-adj-rib-in` peer-up replay `reactor_api_relay.go::RelayStoredRoute`, plus `relay_recovery.go` and `aigp_readvertise.go`, all build through `buildRelayUpdate`, which sets `Meta` nil under the comment "RFC 9494 staleness is genuinely false for a replay", false once D3 lands (design review r2, B-1). |
| D5 | Open | `applySendCommunityMask` (`peer_forward_facts.go`) records `AttrModSuppress` on code 8 for `send-community none` or a list without `standard`. `filter_community/handler.go::genericCommunityHandler` drops the whole attribute on Suppress and drops every Remove-matched value with no exemption. A third path the 2026-09-27 table missed: an import or export policy delta that removes COMMUNITIES records Suppress through `filter_delta.go` (the "Removed attributes" branch), and an AttrModSet that omits LLGR_STALE replaces the list. Those three end in the one code-8 handler. Three more do NOT (design review r2, I-1, read at the producers): `filter_community`'s in-process ingress (`filter_community.go::ingressFilter` -> `filter.go::applyIngressFilter` -> `applyIngressOps` -> `ingressStripCommunities`) rewrites the payload bytes itself, and a raw-override policy response (`filter_chain.go::decodeFilterRawOverride`) replaces the whole UPDATE body, terminally, on import (`filter_ordered.go::runIngressPolicyChain`) and on export (`filter_ordered.go::runEgressPolicyChainASN4`). The `{gap}` annotation in `rfc/short/rfc9494.md` cites line numbers that have since moved. |
| D6 (new) | Open | `LLGREgressFilter` treats ANY nonzero stale level as LLGR-stale. `mark-stale` defaults to level 1 and the GR session-down sequence calls it with no level (`gr.go`), so a route in its conventional GR period (level 1) that `rib_replay.go` replays to a re-established EBGP neighbour without LLGR, or resends on its route refresh (`sendRoutes`), is turned into a withdraw. The RIB replay rail never reaches a neighbour's first session: `collectPeerUpReplay` returns nothing when `seenBefore` is false (design review r2, N-4). The withholding obligation is RFC 9494 Section 4.3, and its scope is set by its lead sentence: "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2:". A route in its conventional GR period neither arrived with LLGR_STALE nor was given it, so Section 4.3 does not reach it, and no sentence of RFC 4724 or RFC 9494 obliges withholding it. RFC 4724 is not cited here: its Section 4.2 sentence "The router MUST NOT differentiate between stale and other routing information during forwarding." concerns the data plane (Section 4.1 pairs the same sentence with retaining "the forwarding state"), so it neither obliges nor forbids withholding a route from a neighbour. D6 is therefore a behaviour choice inside what the RFCs leave open. `docs/guide/graceful-restart.md` ("Readvertising a Stale Route") states the current behaviour as intended ("a route staled by `request bgp rib mark-stale` takes the same treatment"), so this is an owner question (Q-2), not a silent fix. |

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
The BGP child spec-rfc-verdict-fix-bgp routed all five weak
verdicts here from its "Blocked by" table (AC-4), which now owns them alone.

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
| D6 | RFC 9494 Section 4.3 | the lead sentence quoted in the Defects table (D3): it binds routes received with LLGR_STALE or given it per Section 4.2, and no other. RFC 4724 Section 4.2 "The router MUST NOT differentiate between stale and other routing information during forwarding." was read and is not cited for D6: it concerns forwarding state (Section 4.1), not readvertisement |
| D3 (bound, B-1) | RFC 4724 Section 4.2 | "When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information." and "The Receiving Speaker MUST replace the stale routes by the routing updates received from the peer." A route received with LLGR_STALE is one of "the routes received from the peer": deleting it at TCP loss breaks the first sentence, and deleting it at the EOR after the peer re-sent it treats a replacement as still stale, which breaks the second |
| Q-1 (counter-text) | RFC 9494 Section 4.2 | "If the Graceful Restart Capability that was received does not list all AFIs/SAFIs supported by the session, then the GR Restart Time shall be deemed zero for those AFIs/SAFIs that are not listed." and "For example, if the Restart Time is zero and the Long-Lived Stale Time is nonzero, only the procedures particular to LLGR would apply." |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | One unit test per row, red against HEAD |
| Both polarities | D1: NOTIFICATION teardown flushes, TCP loss retains. D2: each of the three bullets purges, a family with both F bits set is kept. D3/D4: received LLGR_STALE is least preferred and withheld from a non-LLGR neighbour, a fresh route is not; a received LLGR_STALE route is retained at TCP loss and kept after its refresh and EOR, a retention-stale route the peer did not re-send is purged |
| Discrimination | Every RFC-tagged unit carries a `./le rfc discriminate-record` record |
| Real entry point | `.ci` tests driving a peer through NOTIFICATION and reconnect |
| Interop | The existing GR interop scenario against FRR (`test/interop/scenarios/bgp-graceful-restart-frr`) extended with a NOTIFICATION teardown, and an LLGR scenario with a peer sending LLGR_STALE |

## Owner decisions

| Row | Question |
|-----|----------|
| - | None open: every row follows its RFC sentence and no reading is in doubt (2026-09-27; superseded by the 2026-10-13 design questions below) |

### Owner questions raised in design (2026-10-13)

Every question is answered. Q-4 by the owner on 2026-10-13 (recorded in its
row). Q-1, Q-2, Q-3, Q-5, Q-6 and Q-7 by the owner on 2026-10-14 (Thomas: "LDP,
BFD, GR, Prefix-SID, accept"): **Owner, 2026-10-14: accept (recommendation)**
on each, recorded in the Answer column. The "If the other answer" column is kept
as the rejected branch and nothing builds it.

| Q | Defect | Question | Recommended | If the other answer | Answer |
|---|--------|----------|-------------|---------------------|--------|
| Q-1 | D2 | On reconnect, a family whose LLGR period has begun: keep its stale routes only when the new GR capability lists it with F set AND the new LLGR capability lists it with F set? | Yes, AND. RFC 9494 Section 4.2 adds its three LLGR conditions and also says RFC 4724 procedures apply "irrespective" of which period retained the routes. Counter-text the owner weighs against it, same section: "If the Graceful Restart Capability that was received does not list all AFIs/SAFIs supported by the session, then the GR Restart Time shall be deemed zero for those AFIs/SAFIs that are not listed." and "For example, if the Restart Time is zero and the Long-Lived Stale Time is nonzero, only the procedures particular to LLGR would apply." So a family the peer omits from GR but carries in LLGR (LLGR-only) is a configuration the RFC contemplates. Ze supports it today: it retains such a family at DOWN (`gr_state.go`, the `grPeerState.restartDeadline` comment on omitted GR families) and keeps it at reconnect (`onSessionReestablished` unions the F bits). Consequence an operator sees under AND: every reconnect of such a peer purges its LLGR-only families at once, unless the new GR lists them with F set, which by definition it does not. That is a behaviour regression for an RFC-contemplated configuration, not only a visible traffic change. | LLGR-only reading: an LLGR-period family obeys only the three Section 4.2 bullets; AC-2 drops its GR-side rows | **Owner, 2026-10-14: accept (recommendation)**: AND, with the stated cost accepted: a peer's LLGR-only families are purged on every reconnect (R-4) |
| Q-2 | D6 | Should `LLGREgressFilter` act only on LLGR-stale routes (stale level at or above `storage.DepreferenceThreshold`, the level LLGR entry and receipt of LLGR_STALE set), letting a conventional GR-stale route (level 1) be advertised like any other? | Yes. This is a behaviour choice, not an RFC obligation either way: RFC 9494 Section 4.3 obliges withholding only for a route "with the LLGR_STALE community" received from an LLGR neighbour or one Ze attached it to "per Section 4.2", and a conventional GR-stale route is neither; no sentence of RFC 4724 or RFC 9494 asks Ze to withhold it, so today's withdraw is a restriction Ze added. RFC 4724 Section 4.2 "MUST NOT differentiate ... during forwarding" is not the ground: it concerns forwarding state. `docs/guide/graceful-restart.md` changes with it. | Keep today's "any stale level" behaviour; D6 leaves this spec and the guide keeps its sentence | **Owner, 2026-10-14: accept (recommendation)**: yes, `LLGREgressFilter` acts only on LLGR-stale routes (level at or above `storage.DepreferenceThreshold`); D6 stays in this spec |
| Q-3 | D5 | Preserve LLGR_STALE on every further advertisement of a route whose received COMMUNITIES carries it, whatever `send-community`, `community-remove`, a `filter_community` `community { strip }`, a policy Set/Suppress or a raw-override policy response says (the last two sites added by design review r2, I-1, after the owner's answer; the owner confirms this widened text at approval, design review r3, N-4, because R-10 changes what an operator's raw filter does)? | Yes, unconditional on presence: the Section 4.3 bullet is stated without exception, and the one code-8 handler sees the source attribute but not the route's metadata. A `send-community none` destination then receives COMMUNITIES holding LLGR_STALE alone. No config refusal for `community-remove ffff0006`: the runtime exemption makes it inert, and the guide says so. | Preserve only for routes Ze holds as LLGR-stale (meta stale level 2): needs the stale level plumbed into the attribute plan, a wider change | **Owner, 2026-10-14: accept (recommendation)**: unconditional LLGR_STALE preservation, on the widened text (all four D5 sites, raw-override included, R-10 accepted); no config refusal of `community-remove ffff0006` |
| Q-4 | all | Approve the RFC-tagged test changes listed in "RFC-tagged tests this spec changes" (TDD Test Plan)? | Approve: each change strengthens an assertion the audit called weak; none relaxes one. **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** It covers the two units listed in the TDD plan; the `TestOnSessionReestablished_DuringLLGR` prose is written for whichever Q-1 answer is recorded, and this does not answer Q-1. Commands under "Owner approval commands" in the TDD plan | The affected verdicts stay weak and AC-4 cannot close | answered 2026-10-13 (see Recommended) |
| Q-5 | - | RFC 9494 gaps RFC9494-4.2-7, 4.2-9 and 4.2-10 (consecutive-drop purge and LLST re-arm, LLST expiry during synchronization) sit in the same `gr_state.go` and are owned by no spec (grep of `plan/` 2026-10-13). Fold them into this spec, or open a separate spec? | Separate spec in `plan/immediate/`: they change timer ownership across reconnect, a different mechanism from the six rows here, and folding them would double this spec's surface | Fold in: add D7 to D9 and their ACs before approval | **Owner, 2026-10-14: accept (recommendation)**: separate spec, `plan/immediate/spec-bgp-llgr-reconnect-timers.md` (skeleton, created 2026-10-14) |
| Q-6 | D1 | The teardown reason after a NOTIFICATION becomes the new declared constant `rpc.ReasonNotification`, spelt "notification", in state events, `show` FSM history and the session-closed log line, replacing "session closed" / "connection lost" for those teardowns. Accept that operator-visible string? | Yes: GR already expects that spelling, and BMP reads the NOTIFICATION from its own cache, not the reason | Choose another spelling; only the constant's value changes | **Owner, 2026-10-14: accept (recommendation)**: `rpc.ReasonNotification`, spelt "notification" |
| Q-7 | D5 / R-6 | Toward an IBGP neighbour without LLGR whose `send-community` suppresses standard communities, the Section 4.6 NO_EXPORT the GR egress filter adds is dropped by the Suppress. Fold the fix into D5 (an egress filter's Add of a value survives a send-community Suppress), or record it as a journal row for a separate fix? | Fold into D5: same handler, same phase, and it is the Section 4.6 MUST on the very routes D4 starts sending | journal row in `plan/journal/`, this spec unchanged | **Owner, 2026-10-14: accept (recommendation)**: fold into D5. Mechanism designed in round 6: the new action `filterapi.AttrModAttach`, recorded only by the GR Section 4.6 branch (Data Flow, "Q-7: which Adds survive a Suppress") |
| Q-8 | D5 (Q-7) | Approve the change to two RFC9494-4.6-2 tagged units in `gr/rfc9494_gr_egress_test.go` (`TestLLGREgressFilter_NilStateDepreferencesIBGP`, `TestLLGREgressFilter_IBGPPartial`): each asserts the NO_EXPORT op's action is `AttrModAdd`, and after round 6 the GR filter records it as `AttrModAttach`. The body asserts `AttrModAttach` (value 0xFFFFFF01, 4 bytes, unchanged), and the `IBGPPartial` tag prose, which quotes `mods.Op(attrCodeCommunity, AttrModAdd, communityNoExport)`, is rewritten to the new call (design round 6) | Approve: the assertion is strengthened, never relaxed: it now pins the form a `send-community` Suppress cannot drop, and a revert to `AttrModAdd` turns both red | the two units stay red against the Q-7 fix, so D5 cannot land | **open: owner** (an RFC-tagged test change needs the owner's `./le rfc approve unit`, `ai/rules/rule-precedence.md`) |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/graceful-restart.md` - the page `ai/CODE-TO-DOCS.md` names for every `plugins/gr/` file
  → Constraint: its "Peer Restarts" list, LLGR "How It Works" step 5 ("families with F-bit=0 or missing from the new OPEN are purged", silent on the LLGR capability) and "Readvertising a Stale Route" table change in the same phase as D2, D4 and D6
  → Decision: stale levels are 0 fresh, 1 GR-stale (normal selection), 2+ LLGR-stale (loses to any level below 2). `StaleLevel` is the RETENTION mark: `mark-stale` sets it (`FamilyRIB.MarkStale` overwrites every entry's level), and `purge-stale` deletes every entry above fresh (`FamilyRIB.PurgeStale`, `rib_commands.go::purgeStaleCommand`). D3 does NOT store a received LLGR_STALE route in it (design review r1, B-1): the GR down sequence runs `purge-stale` -> retain -> `mark-stale` (`gr.go::handleStructuredState` / `handleStateEvent`), and `onEORReceived` -> `purge-stale <family>` (`gr.go::handleEOREvent`), so a level-2 received route would be deleted at TCP loss, deleted again at the EOR after the peer refreshed it, and reset to level 1 by `mark-stale` if it survived. "Received with LLGR_STALE" is its own per-entry fact, `IngressStaleLevel` (see Data Flow D3)
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
- The reactor already has a received-route metadata channel (in-process ingress filters write `ingressMeta`, which rides `ReceivedUpdate.Meta` to the egress filters of the LIVE forward rails); D3 needs one ingress filter and one plumb of that map to plugin events, not a new channel. That channel does not reach the other two rails into `LLGREgressFilter` (design review r2, B-1): the RIB rails send from Adj-RIB-Out copies, and the relay rails rebuild a route from stored bytes with no metadata. D4 names how each rail gets the fact (Data Flow).
- Most removals of COMMUNITIES end in the one code-8 attribute handler, and three do not: `filter_community`'s in-process ingress strip, and a raw-override policy response on import and on export (design review r2, I-1). D5 therefore owns four sites: the handler, `ingressStripCommunities`, and the two raw-override branches, all reading the one registered preserved set.

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
  → Decision (design review r1, N-6): the D3 ingress filter registers as its OWN `filterapi.Filter` entry at `filterapi.FilterStageProtocol` (0), not in the egress entry, whose one `Stage` is `FilterStageAnnotation` (200). The reactor runs in-process ingress steps in stage order, each on the payload the previous step produced (`reactor_notify.go`, the `orderedIngressSteps` loop), and the per-peer policy chain sorts last at `FilterStagePeerChain` (300). `filter_community` registers an in-process `Ingress` at `FilterStagePolicy` (100). At stage 0 the GR step reads COMMUNITIES as the neighbour sent it, which is the fact RFC 9494 Section 4.3 binds ("upon receiving a route from that neighbor with the LLGR_STALE community"), before any import policy can add or remove the value
  → Decision (design review r2, N-1): the new entry carries its own `Name`, `bgp-gr-llgr-stale-ingress`, because `filterapi.Register` refuses a second entry under the same Name (`ErrDuplicateFilterName`) and GR's `init()` then exits; the existing egress entry keeps `grPluginName`. `filter_community` sets the precedent with "bgp-filter-community-relation"
  → Constraint: marking is on the received community, not the post-policy one. An import policy that removes LLGR_STALE cannot make the stored community and the level disagree, because D5 preserves the value at every site that can remove it on import: the code-8 handler (per-peer chain delta), `ingressStripCommunities` (`filter_community` `community { strip }`), and the raw-override branch of `runIngressPolicyChain` (design review r2, I-1); an import policy that ADDS LLGR_STALE yields a route carrying the community at `IngressStaleLevel` 0, which is neither "receiving a route from that neighbor with the LLGR_STALE community" nor attaching it "per Section 4.2", so Section 4.3 does not bind it (stated in `docs/guide/graceful-restart.md`)
- [ ] `internal/component/bgp/reactor/peer_run.go` - the Established-exit branch of the FSM callback
  → Constraint: reason chosen there feeds `announceClosed`, the FSM history `transitionReason` and the "session closed" log line; one variable, one choice
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyPeerClosed`, the received-UPDATE path (`ingressMeta`, `routeMeta`, `ReceivedUpdate.Meta`)
  → Constraint: the received `RawMessage` is built BEFORE the ingress steps; its `Meta` must be set after them, from the same `routeMeta`
- [ ] `pkg/plugin/rpc/enums.go` - `ReasonPeerRemoved`, `ReasonAttached`
  → Decision: `ReasonNotification` is declared beside them, the one spelling GR compares against
- [ ] `pkg/plugin/rpc/bridge.go` - `StructuredEvent.Meta` documented "sent events only"
  → Constraint: that comment changes with the plumb
- [ ] `internal/component/bgp/server/events.go` - `getStructuredEvent` copies `msg.Meta` into `se.Meta`
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - `handleReceivedStructured` reads no `se.Meta` today; the `se.Meta` reads (keys replay, rib-lifecycle, config-static) are all in `handleSentStructured` (design review r1, I-3)
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go`, `storage/routeentry.go` - insert resets `StaleLevel` to `StaleLevelFresh`; `DepreferenceThreshold` is 2
  → Constraint: every insert path (`FamilyRIB.Insert`, `FamilyRIB.InsertEntry`, `insertMultiNoOp`, `pathSet.refresh`, and in `storage/familyrib_opaque.go` `insertOpaqueNoOp` and the equal-attributes branch of `insertOpaque`, both of which keep the OLD entry; design review r2, I-2) resets `StaleLevel` to fresh and stays so; the same paths set `IngressStaleLevel` from the received event's metadata on every insert, so a refresh WITHOUT LLGR_STALE lands at 0 and a refresh WITH it at 2. Opaque storage holds every family `IsCIDRFamily` rejects, VPN and EVPN among them, which are LLGR's main users. Simplest single carrier: the level rides on the `parsed` entry the insert receives, and each no-op or equal branch writes it onto the kept entry; the labelled path (`rib_structured.go::insertLabeledEntry`) writes it on its own, because it runs no post-insert callback
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib_opaque.go` - `insertOpaqueNoOp`, `insertOpaque` (design review r2, I-2)
  → Constraint: the no-op branch resets `StaleLevel` when it or the message id differs and keeps the old entry; `IngressStaleLevel` joins that comparison, so a re-announce with equal attributes whose level changed between sessions (AC-14 polarity after a reconfigure and reconnect) is written, not skipped
  → Constraint: `MarkStale`, `PurgeStale`, `StaleCount` and the per-family purge in `purgeStaleCommand` read and write `StaleLevel` only; they never touch `IngressStaleLevel`
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `compareBeforeMED` Step 0, unchanged
- [ ] `internal/component/bgp/plugins/rib/rib_replay.go`, `rib_commands.go`, `ribout_entry.go` - copy `ribOutEntry.StaleLevel` into `meta["stale"]` for replay (`replayRoutesWithCursor`, `resendRoutesWithCursor`, both through `collectGroupedRibOutRoutesFiltered`, whose group key carries the level) and readvertise (`sendRoutes`, through `collectRibOutRoutes` -> `reconstructRoute`); `extractCandidate` copies the received entry's level into the best-path `Candidate`; `rib_sent_lifecycle.go::attachSentSourceCommunityLocked` copies a received entry's level to the sent owners; `rib_attr_format.go` shows it as `stale-level`
  → Constraint: the readers that start from a RECEIVED entry (`extractCandidate`, `rib_attr_format.go`) read the effective level, the higher of `StaleLevel` and `IngressStaleLevel`, so Step 0 and `show` agree
  → Decision (design review r2, B-1): the RIB rails start from a `ribOutEntry`, not a received entry, so they read the ingress fact from its one holder: the `IngressStaleLevel` of the SOURCE received entry the `ribOutEntry` names (design review r3, I-1, read at the producer: the lookup that exists is `sent_owner.go::sentSourceCurrent`, which parses `SourcePeer`, takes `bgpPeers[source]`, and asks it by key, `AppendPrefixPathsRetained` for a CIDR family (the labelled family's prefix through `parsePrevKey`) or `AppendKeyPathsRetained` otherwise, into an inline four-path buffer it releases. The round-2 text named `sentReceivedOwner` then `PeerRIB.Lookup`, which do not compose: `PeerRIB.Lookup` needs the source's framed NLRI bytes, `sentReceivedOwner` returns an owner key, and `ribOutEntry.NativeNLRI` is empty for a CIDR family). `sentSourceCurrent` becomes one function that answers two questions from that one lookup: whether the copy is current, and the source's ingress level. The two answers match differently (r3, N-1, main-thread decision). "Current" keeps today's test unchanged: the path whose `SourceAddPath`, `SourcePath` AND `SourceMessageID` all match. The ingress level is read from the source's CURRENT entry found by key alone, the path whose add-path mode and path id match, whatever its MsgID, because a source re-announced while still LLGR_STALE carries a new MsgID before its new send reaches this copy, and that copy must still be withheld from a non-LLGR EBGP neighbour; the early "not current" exits on a zero `SourceOwner` or `SourceMessageID` therefore do not skip the level read. The peer-up rail (`collectGroupedRibOutRoutesFiltered` with `peerUp`, which already calls `sentSourceCurrent` per copy) does that one lookup per copy, not two; the refresh rails (`collectRibOutRoutes` for `sendRoutes`, and `rib_commands.go::outboundResend`, which reaches `collectGroupedRibOutRoutesFiltered` with `peerUp` false), which apply no current filter today and still apply none, call the same function for the level alone. They do not use its current answer to skip a non-current copy (design review r4, N-2, read at the producer): `sentSourceCurrent` answers false for a zero `SourceOwner` or `SourceMessageID`, an unparsable `SourcePeer`, no `bgpPeers` entry, or no path at all, none of which means a newer send is pending, and on the refresh rail a copy left out between BoRR and EoRR is purged by the destination (RFC 7313 Section 4: "When a BGP speaker receives an EoRR message from a peer, it MUST immediately remove any routes from the peer that are still marked as stale for that <AFI, SAFI>."), a withdrawal nobody decided. The window that leaves open is risk R-11. The `meta["stale"]` each RIB rail writes, and the replay group key, is the higher of `ribOutEntry.StaleLevel` and that source level; a `ribOutEntry` with no source entry found by key (`LocalOrigin`, an unparsable `SourcePeer`, no `bgpPeers` entry, or no path with that add-path mode and path id) contributes its own `StaleLevel` only. `ribOutEntry` gains no field, so `markStaleCommand` and `attachSentSourceCommunityLocked`, which write `ribOutEntry.StaleLevel`, cannot overwrite the ingress fact. Every caller already holds `peerMu` (r3 verified: `outboundResend` and both refresh handlers under RLock, `collectPeerUpReplay` under the state handler's Lock), and the order `peerMu` then `PeerRIB.mu` is the one `sentSourceCurrent` and `attachSentSourceCommunityLocked` already take. `sentSourceCurrent` holds no discrimination record (r3: `rfc/discrimination/` searched for the symbol and for `sent_owner.go`, with `ingressStripCommunities` as the control that is found)
  → Constraint (design review r2, N-3): these `StaleLevel` readers are retention-only and MUST NOT read `IngressStaleLevel`: `rib_commands.go::autoExpireStale` (`RemoveFamilyMatching` on `StaleLevel` above fresh), `rib_commands_community.go::attachCommunityCommand` and `deleteWithCommunityCommand` (both skip fresh entries), with `MarkStale`, `PurgeStale`, `StaleCount` and `purgeStaleCommand`. The implementer greps every non-test `StaleLevel` read in `plugins/rib/` and names each one, effective or retention-only, in the phase that changes it
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `sendStaleReadvertise`, `decideStaleReadvertise` (design review r2, B-1)
  → Constraint: the readvertise egress filters run only when the batch's stale level is above 0, and `decideStaleReadvertise` hands them a zero `src` and a meta built from that level alone. The RIB rails therefore deliver the fact as `meta["stale"]` or not at all; a byte reading inside `LLGREgressFilter` never runs on this rail. Unchanged by this spec
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` - `RelayStoredRoute`, `buildRelayUpdate`; `relay_recovery.go`, `aigp_readvertise.go` call `buildRelayUpdate` too (design review r2, B-1)
  → Constraint: `buildRelayUpdate` resolves the source (`resolveRelaySource`, fail closed when it is not established) and hands it to `forwardUpdateCore`, so the egress filters see the true source address. Its `Meta` is nil, and its comment "RFC 9494 staleness is genuinely false for a replay" becomes false with D3; the comment is replaced in the same edit (stale-comments)
- [ ] `internal/component/bgp/plugins/filter_community/filter.go` - `applyIngressFilter`, `applyIngressOps`, `ingressStripCommunities` (design review r2, I-1); `config.go::parseCommunityDefinitions`, where the code-8 strip set is built without the preserved values once at config time (design review r3, answer 4)
  → Constraint: the strip builds its strip set from the configured values and rewrites the payload bytes; it never reaches `genericCommunityHandler`. It reads the same registered preserved set as the handler and keeps a preserved value whatever the strip list names
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - the raw-override branches of `runIngressPolicyChain` and `runEgressPolicyChainASN4`; `filter_chain.go::decodeFilterRawOverride` (design review r2, I-1)
  → Constraint: a raw response replaces the whole body and is terminal; the text-delta branch does not run, so no AttrMod reaches the code-8 handler
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `IngressFilterFunc`, `EgressFilterFunc`, `Filter`, `LastSetOrSuppress`, `RegisterAttrModHandler`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `sendCommunitySuppression`, `applySendCommunityMask`
- [ ] `internal/component/bgp/reactor/filter_delta.go` - the "Removed attributes" Suppress branch
- [ ] `internal/component/bgp/plugins/filter_community/handler.go`, `register.go` - `genericCommunityHandler` serves codes 8, 16, 32; Suppress drops the attribute, Remove drops matching values
  → Constraint: zero-copy retained runs (`p.Op` fragments) must survive the change; LLGR_STALE re-insertion must not allocate per destination

**Behavior to preserve:**
- TCP loss (no NOTIFICATION) of a GR peer still retains, marks stale and starts the Restart Time timer, and that now includes the routes it sent with LLGR_STALE (B-1).
- `mark-stale`, `purge-stale` and the EOR purge keep their meaning: they act on the retention mark only.
- `removed` teardown still releases GR state and metrics without retention.
- BMP Peer Down reason selection (it reads its NOTIFICATION cache, not the reason string).
- A family still in conventional GR at reconnect: RFC 4724 rule unchanged.
- A fresh route never touches the GR egress filter beyond the zero-level check.
- Self-attached LLGR_STALE (LLGR entry) keeps level 2 and today's readvertise rail.
- Section 4.6 IBGP partial deployment (NO_EXPORT plus LOCAL_PREF 0) unchanged.

**Behavior to change:** the rows D1 to D6 (Q-2 answered yes, owner 2026-10-14), and the Section 4.6 NO_EXPORT surviving a `send-community` Suppress (Q-7 folded into D5, AC-27).

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
| D3 | UPDATE received -> `reactor_notify.go` in-process ingress steps -> GR ingress filter: source in the "Ze sent LLGR" snapshot AND COMMUNITIES carries 0xFFFF0006 -> writes stale level 2 into `ingressMeta` -> `routeMeta` -> (a) `ReceivedUpdate.Meta` for the forward rails, (b) NEW: received `RawMessage.Meta` -> `se.Meta` -> RIB `handleReceivedStructured` inserts with `StaleLevel` 0 (fresh, as every insert does) and `IngressStaleLevel` 2 -> `extractCandidate` hands best path the effective level (the higher of the two) -> Step 0 makes it least preferred. TCP loss: `purge-stale` skips it (`StaleLevel` 0), retain keeps it, `mark-stale` sets `StaleLevel` 1, effective level stays 2. Reconnect: the peer's refresh re-inserts it at `StaleLevel` 0 with `IngressStaleLevel` from the new UPDATE, so the EOR `purge-stale` keeps it. A route held only by the GR or LLGR retention mark keeps today's behaviour |
| D4 | One decision, `LLGREgressFilter` per destination (withdraw EBGP non-LLGR, Section 4.6 IBGP, unchanged to LLGR), reached by every rail, each named (design review r2, B-1). (1) Live forward rails (`reactor_api_forward.go`, `forward_rs.go`): `update.Meta` stale 2 from the D3 ingress step. (2) RIB rails (`rib_replay.go::replayRoutesWithCursor` on a re-established destination's peer-up, `resendRoutesWithCursor`, `rib_commands.go::sendRoutes` on route refresh): `meta["stale"]` is the higher of `ribOutEntry.StaleLevel` and the source received entry's `IngressStaleLevel`, read by `sent_owner.go::sentSourceCurrent`'s one by-key lookup, the source's current entry by add-path mode and path id whatever its MsgID (Current Behavior, r3 I-1 and N-1) -> batch stale 2 -> `reactor_api_batch.go` readvertise filters. (3) Relay rails (`reactor_api_relay.go::RelayStoredRoute` for the `bgp-adj-rib-in` peer-up replay, `relay_recovery.go`, `aigp_readvertise.go`, all through `buildRelayUpdate`): `buildRelayUpdate` sets `Meta` to one package-level read-only map holding a declared key, `filterapi.MetaKeyRebuilt` ("rebuilt", documented in `docs/architecture/meta/README.md`), which says "this route was rebuilt from storage; no ingress step ran on it". `LLGREgressFilter`, finding no stale level and that key, recovers the fact with the D3 ingress predicate itself, the SAME function the ingress step calls, `gr_ingress.go::llgrStaleFromSource` (r4 N-1), not a second copy of its test (design review r3, answer 2): the source address is in the "Ze sent LLGR" snapshot AND the payload's COMMUNITIES holds 0xFFFF0006. The recovery is exact, not a guess, because the stored bytes are the post-ingress bytes and D5 keeps LLGR_STALE in them at every import site; it is the shape `role/otc.go::resolveSrcRole` already uses for `meta["src-role"]` on this rail. A live route never carries the key, so its fast path stays "stale level 0, no key: accept" |
| D5 | any COMMUNITIES op on egress (send-community Suppress, `community-remove`, policy delta Suppress or Set) -> code-8 handler -> when the source attribute holds a value registered as preserved, the emitted attribute keeps it (a Suppress emits an attribute holding only the preserved values). Three sites bypass the handler and read the same registered set (design review r2, I-1): `filter_community` `ingressStripCommunities` keeps a preserved value whatever the strip list names, on code 8 only (design review r3, answer 4, read at the producer): the function is generic over codes 8, 16 and 32 (`applyIngressOps` passes each definition's code), and the preserved set is code-8 values, so the exemption is computed once at config time, in `config.go::parseCommunityDefinitions`, as a code-8 definition's strip set without the registered preserved values, held apart from its tag set so `tag` still adds every configured value; the per-UPDATE scan does no extra work and codes 16 and 32 are untouched. The own-GA scrub (`scrub.go::scrubOwnGACommunities`) also calls the function on code 8, and cannot select 0xFFFF0006, because `ownGAStandardValues` refuses a local AS equal to `reservedCommunityHigh` (0xFFFF); a raw-override response in `runIngressPolicyChain` or `runEgressPolicyChainASN4`, when the step's input COMMUNITIES holds a preserved value the override lacks, has it added back through one `AttrModAdd` on code 8 and `buildModifiedPayload` with the registered handlers, the same machinery the text-delta branch uses, so the cost lands only on a raw override that dropped a preserved value. Q-7 (design round 6): the GR Section 4.6 branch records its NO_EXPORT as `AttrModAttach`, which the code-8 handler emits after whatever every Set, Suppress and Remove on code 8 leave, so a `send-community` Suppress toward an IBGP neighbour without LLGR emits COMMUNITIES holding the preserved source values (LLGR_STALE) then NO_EXPORT; every `AttrModAdd` is dropped by a Suppress or a Set as today (the sub-table below) |
| D6 | `LLGREgressFilter` early return extends from "level 0" to "level below `storage.DepreferenceThreshold`" (Q-2, owner 2026-10-14) |

#### Q-7: which Adds survive a Suppress (design round 6, 2026-10-14)

RFC 9494 Section 4.6: "The NO_EXPORT community [RFC1997] MUST be attached to the
stale routes." Read at HEAD: `gr_egress.go::LLGREgressFilter` records it as
`mods.Op(8, AttrModAdd, 0xFFFFFF01)` at `FilterStageAnnotation` (200). Both forward
rails then record `send-community` after every ordered egress step
(`reactor_api_forward.go` `applyFactsSendCommunity` after the step loop,
`forward_rs.go::reactorForwardRS` after its egress-filter loop; `applySendCommunityFilter`
has test callers only), as `AttrModSuppress` on code 8
(`peer_forward_facts.go::applySendCommunityMask`). The stale readvertise rail
(`reactor_api_batch.go::sendStaleReadvertiseUnit`, AC-16 rail 2) records no
`send-community` op at all: it runs only the Readvertise-opted filters, so there
`AttrModAttach` and `AttrModAdd` emit alike. Read in round 7: that rail is not
exposed. Its batch is rebuilt from the destination's Adj-RIB-Out
(`rib_replay.go`, `r.ribOut[dest]`, stored by `rib_structured.go::storeSentEntries`
from the UPDATE Ze sent), so its COMMUNITIES are already the post-`send-community`
bytes, and GR's NO_EXPORT is added with no Suppress after it. Not a defect.
`genericCommunityHandler` reads `filterapi.LastSetOrSuppress`, and a Suppress or
a Set as the last of the two drops every Add whatever its position, so the
NO_EXPORT is lost on the two forward rails. The `send-community` Suppress is the
only live producer of that loss (round 7, read at the producers): an export
policy never records a code-8 Set in the shared accumulator, because
`reactor/filter_ordered.go::runEgressPolicyChainASN4` applies its delta through
its own `buildModifiedPayload` and returns the result as `wireOverride`, and
`reactor_api_forward.go` then applies the shared mods (GR's NO_EXPORT, then
`applyFactsSendCommunity`) on top of that payload.

The rule the RFC states is about provenance, not position: a value a protocol
requires on this route stays, and an operator's value obeys the operator's
`send-community`. So the distinction is declared once, by the producer, as a
sixth action in the closed `filterapi` vocabulary:

`AttrModAttach`: append a value a protocol requires on this route. No Set,
Suppress or Remove on the same code removes it. The code-8 handler (the one
function serving codes 8, 16 and 32) emits attached values after whatever the
other operations leave: on a Suppress, the preserved source values (D5) then the
attached values, and the attribute is dropped only when both are empty; on a Set,
the Set buffer (with D5's preserved values) then the attached values; otherwise
the retained source values, the Adds, then the attached values. A Remove is
tested against source values only, as today, so it never reaches an attached
value. Like `AttrModPrepend` outside AS_PATH, the action means nothing to a
handler for another code; its one producer records it on code 8.

| Producer of an append on 8, 16 or 32 | Action after round 6 | Under a `send-community` Suppress or a later Set |
|---------------------------------------|----------------------|--------------------------------------------------|
| `gr/gr_egress.go::LLGREgressFilter`, Section 4.6 branch, NO_EXPORT on code 8 | `AttrModAttach` (changed) | survives (AC-27) |
| `filter_community/egress.go::applyEgressFilter`, `community { tag }` | `AttrModAdd` (unchanged) | dropped, as today: `send-community none` means none of the operator's values |
| `reactor/filter_delta.go` policy `community-add`, `large-community-add`, `extended-community-add` (per-peer chain text delta, `filter_modify`) | `AttrModAdd` (unchanged) | dropped, as today |
| D5 raw-override restore in `runIngressPolicyChain` / `runEgressPolicyChainASN4` | `AttrModAdd` (unchanged) | not exposed: it is applied by its own `buildModifiedPayload` inside the chain step, so the override wire then holds LLGR_STALE as a source value, which the later Suppress keeps as preserved (D5) |

Consumers that switch on the action: `genericCommunityHandler` (honours it),
`policy_dryrun.go::wireModVerb` (prints "attach", where the default arm would
print "modified"), `filter_delta.go::pendingAttrValue` (reads Set and Suppress on
AS_PATH and AS4_PATH only, unaffected). `AttrOp` keeps its layout: the action is a
value of the existing `Action` byte.

Rejected: (b) every Add survives a Suppress: an operator's `tag` and policy
`community-add` would leak past `send-community none` (Security Review, R-5);
(c) a provenance field on `AttrOp`: a second channel beside `Action` that every
handler would have to read, for the same one producer; (d) order the GR filter
after `send-community`: `send-community` is not an ordered step, and the handler's
Set/Suppress rule ignores position, so ordering alone changes nothing without
rewriting the rule for every producer; (e) register NO_EXPORT in D5's preserved
set: every received NO_EXPORT would then survive an operator's strip toward every
neighbour, which no RFC asks for.

New per-entry field for D3 (`storage/routeentry.go`, `RouteEntry`; design review r1, B-1):

| Field | Type | Description |
|-------|------|-------------|
| `IngressStaleLevel` | uint8 | the stale level the received UPDATE's ingress metadata assigned (`meta["stale"]`), 0 when absent; written on every insert and refresh, copied by the entry clone, never written by `mark-stale` and never read by `purge-stale` or `StaleCount`. The RIB learns no LLGR fact, only a level. Readers that start from a received entry use the effective level, the higher of `StaleLevel` and `IngressStaleLevel` |

Rejected alternative: keep one field and make `purge-stale` delete only retention marks. That needs `purge-stale` to tell a level-2 retention mark (LLGR entry, `mark-stale ... 2`) from a level-2 received mark in one byte, and `MarkStale` would still overwrite the received level at the GR down sequence.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ GR plugin | structured state event with a close reason | No |
| Reactor ↔ GR ingress filter | in-process `filterapi.IngressFilterFunc`, writes `meta["stale"]` (uint8) | No |
| Reactor ↔ RIB plugin | received UPDATE `StructuredEvent.Meta` (new for received events) | No |
| Reactor relay rail ↔ GR egress filter | `buildRelayUpdate` sets `Meta` to the read-only `filterapi.MetaKeyRebuilt` map; the filter recovers the stale fact from the source and the payload (B-1) | No |
| Reactor ↔ filter_community handler | `filterapi.AttrPlan` for code 8; preserved-value set read from `filterapi` | No |

### Integration Points
- `notifyPeerClosed`, `apiStateObserver.OnPeerClosed`
- `filterapi.Register` (GR adds a second entry, Name `bgp-gr-llgr-stale-ingress`, at `FilterStageProtocol` carrying `Ingress`), a new `filterapi` preserved-community registration that GR calls at `init()` and that `genericCommunityHandler`, `ingressStripCommunities` and the two raw-override branches read
- `filterapi.MetaKeyRebuilt`, set by `buildRelayUpdate`, read by `LLGREgressFilter`
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
| a destination that received the route re-establishes without LLGR, or sends ROUTE-REFRESH | → | RIB rail (`replayRoutesWithCursor`, `sendRoutes`) source-level read -> `LLGREgressFilter` | `TestPeerUpReplayReportsSourceIngressLevel` and `TestRIBReplayMetaReadsSourceIngressLevel` (rib) for the level each rail hands on, `TestLLGREgressFilter_ReceivedStaleMeta` (gr) for the decision on it (B-1). No `.ci` form: ze-peer drops no capability per connection and cannot order a close after a reload (r4, I-1; Functional Tests, `llgr-stale-received-egress`) |
| a NEW EBGP neighbour without LLGR comes up after the route is stored in `bgp-adj-rib-in` | → | `RelayStoredRoute` -> `buildRelayUpdate` rebuilt key -> `LLGREgressFilter` recovery | `test/plugin/llgr-stale-received-egress.ci` relay step (B-1) |
| `community { strip }` naming 0xFFFF0006 on the LLGR source's import | → | `ingressStripCommunities` preserved set | `test/plugin/llgr-stale-received-depreferenced.ci` strip step (I-1) |
| `send-community none` / `community-remove ffff0006` destination | → | code-8 handler preserved set | `test/plugin/llgr-stale-community-preserved.ci` (new) |
| real peer daemons | → | all of the above | interop scenarios in the Interop Tests table |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GR peer torn down by a NOTIFICATION sent or received | routes flushed, no stale retention |
| AC-2 | reconnect during LLGR with each Section 4.2 purge condition | that family's stale routes removed |
| AC-3 | route received with LLGR_STALE from an LLGR neighbour | least preferred; not advertised to a non-LLGR neighbour; community kept on further advertisement |
| AC-4 | the `weak` verdicts of RFC4724-4-2, RFC9494-4.2-6, RFC9494-4.2-8, RFC9494-4.3-1 and RFC9494-4.3-3, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from the "Blocked by" table of the BGP child spec-rfc-verdict-fix-bgp (P-3 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, 2026-09-28; RFC9494-4.3-3 transfer completed 2026-10-05) |
| AC-5 | GR peer whose TCP session is lost with no NOTIFICATION exchanged (D1 negative polarity) | routes retained and marked stale level 1, Restart Time timer armed, exactly as today |
| AC-6 | any Established -> non-Established transition after a NOTIFICATION was sent or received in that session | the state-down event's reason is `rpc.ReasonNotification` ("notification") on both the structured and the JSON plugin paths and in FSM history; a session without one keeps "session closed" / "connection lost"; peer removal keeps "removed" |
| AC-7 | reconnect, family F in its LLGR period, new GR lists F with F bit set, new LLGR lists F with F bit clear | F's stale routes purged at once (RFC9494-4.2-8 bullet 1) |
| AC-8 | reconnect, family F in its LLGR period, new GR lists F with F bit set, new LLGR omits F (or no LLGR capability at all) | F's stale routes purged at once (bullet 2) |
| AC-9 | reconnect during LLGR with neither GR nor LLGR capability | every stale family purged, peer state released (bullet 3; behaviour already correct, kept) |
| AC-10 | reconnect, family F in its LLGR period, both new capabilities list F with F bit set | F kept until its EOR (negative polarity) |
| AC-11 | (Q-1 AND, owner 2026-10-14) reconnect, family F in its LLGR period, new LLGR lists F with F set, new GR omits F or clears its F bit | F purged at once |
| AC-12 | reconnect, family G still in its conventional GR period (no LLGR timer), new GR lists G with F set, new LLGR omits G | G kept until EOR: the RFC 9494 bullets apply only "once the LLGR Period begins" |
| AC-13 | UPDATE carrying LLGR_STALE received from a peer Ze sent the LLGR capability to, competing with the same prefix from another peer at a LOWER LOCAL_PREF and a HIGHER peer address | the LLGR_STALE route is stored at stale level 2 and loses (`show bgp rib` best is the other route, best-path explain names the stale step) |
| AC-14 | the same UPDATE received from a peer Ze did NOT send LLGR to | stored at level 0; wins on LOCAL_PREF (negative polarity: the obligation binds only "A BGP speaker that has advertised" LLGR) |
| AC-15 | the LLGR_STALE route is refreshed by an UPDATE for the same NLRI without LLGR_STALE, for a CIDR family and for an opaque-storage family (VPN, EVPN), including a refresh whose attributes are otherwise equal (the no-op and equal branches, I-2); and the reverse, a fresh route refreshed WITH LLGR_STALE | entry returns to level 0 and competes normally; in the reverse case it moves to `IngressStaleLevel` 2 and loses Step 0 |
| AC-16 | a received LLGR_STALE route (AC-13 conditions) is forwarded | EBGP neighbour without LLGR: not advertised (withdrawn if previously sent); neighbour with LLGR: advertised with LLGR_STALE intact; IBGP neighbour without LLGR: advertised with NO_EXPORT and LOCAL_PREF 0. The same three outcomes on every rail into `LLGREgressFilter` (design review r2, B-1): (1) the live forward rails, `reactor_api_forward.go` and `forward_rs.go`; (2) the RIB rails, `replayRoutesWithCursor` when a destination that was sent the route while it had LLGR re-establishes without it, `resendRoutesWithCursor`, and `sendRoutes` on that destination's ROUTE-REFRESH, both with no retention mark on the source (red at HEAD, where the copy's `StaleLevel` is 0) and after `markStaleCommand` level 1 or `attachSentSourceCommunityLocked` has written the copy's `StaleLevel` (the outcome must not come from that field); (3) the relay rails through `buildRelayUpdate`: `RelayStoredRoute` when a NEW neighbour comes up after the route is stored in `bgp-adj-rib-in`, `relay_recovery.go`, `aigp_readvertise.go`. A relayed route WITHOUT LLGR_STALE, or one received from a source Ze sent no LLGR to, is advertised unchanged (negative polarity) |
| AC-17 | a route whose COMMUNITIES holds LLGR_STALE and other values, sent to a destination with `send-community none`, one with `community-remove ffff0006`, and one whose export policy removes the COMMUNITIES attribute | each destination receives COMMUNITIES containing LLGR_STALE; the other values are removed as configured (Q-3) |
| AC-18 | a route WITHOUT LLGR_STALE to the same three destinations | today's output byte for byte: no COMMUNITIES for `send-community none`, the configured removals otherwise (negative polarity) |
| AC-19 | (Q-2 yes, owner 2026-10-14) a route at stale level 1 (conventional GR period) replayed to a RE-ESTABLISHED EBGP neighbour without LLGR, or resent on its ROUTE-REFRESH (the RIB rail never reaches a first session, N-4) | advertised unchanged; a level-2 route to the same neighbour is still withheld |
| AC-20 | `./le rfc check` after the change | RFC9494-4.3-2 carries no `{gap}`, has positive and negative tagged tests with discrimination records; the `Support remaining` row of `rfc/short/rfc9494.md` no longer names 4.3-2; no new violation for any RFC, including every discrimination record whose producer this spec edits (the "Discrimination records this spec re-records" table) |
| AC-21 | the interop scenarios in the Interop Tests table | each passes against its named peer daemon, and each was seen red with its producer fix reverted and the Ze image rebuilt |
| AC-22 | (B-1) a GR and LLGR peer that sent a route with LLGR_STALE (AC-13 conditions) loses its TCP session with no NOTIFICATION | the route is retained with the peer's other routes (RFC 4724 Section 4.2 "MUST retain the routes received from the peer"), not deleted by the `purge-stale` that opens the down sequence, and stays least preferred |
| AC-23 | (B-1) the AC-22 peer reconnects, re-sends the route with LLGR_STALE, then sends EOR for the family | the route survives the EOR purge (it was replaced, so it is no longer retention-stale) and is still least preferred; a retained route the peer did not re-send is purged at that EOR |
| AC-24 | (B-1) the GR-period `mark-stale` (level 1) runs over a family holding a received LLGR_STALE route | the route keeps effective level 2: it still loses Step 0 to a fresh route at a lower LOCAL_PREF, and the GR egress filter still withholds it from an EBGP neighbour without LLGR |
| AC-25 | (N-6, I-1) an LLGR neighbour's route carrying LLGR_STALE meets an import policy that removes the value, by each of the three import sites: a `filter_modify` `community-remove ffff0006` (per-peer chain, code-8 handler), a `filter_community` `community { strip }` naming 0xFFFF0006 (`ingressStripCommunities`), and a raw-override import response that drops it (`runIngressPolicyChain`) | the route is stored least preferred (`IngressStaleLevel` 2, read before policy) and its stored COMMUNITIES still holds LLGR_STALE (D5); the other values the policy names are still removed. Negative polarity: a strip naming another value, on a route without LLGR_STALE, behaves byte for byte as today |
| AC-26 | (I-1) a route whose COMMUNITIES holds LLGR_STALE meets a raw-override EXPORT response that drops it (`runEgressPolicyChainASN4`) | the destination receives the override with LLGR_STALE added back to COMMUNITIES; an override that keeps it, or a route without it, is sent exactly as the override says |
| AC-27 | (Q-7, owner 2026-10-14) a route the GR egress filter sends to an IBGP neighbour without LLGR under RFC 9494 Section 4.6, where that neighbour's `send-community` suppresses standard communities (`none`, or a list without `standard`) | the neighbour receives the route with COMMUNITIES holding NO_EXPORT (RFC 9494 Section 4.6: "The NO_EXPORT community [RFC1997] MUST be attached to the stale routes.") and, per Q-3, LLGR_STALE; a route the 4.6 branch does not touch is still sent with COMMUNITIES suppressed as configured, and an operator's `community { tag }` or policy `community-add` toward that neighbour is still suppressed. Mechanism: `filterapi.AttrModAttach` (Data Flow, "Q-7: which Adds survive a Suppress", design round 6) |

### AC proofs

| AC | Proof (test that fails if the AC breaks) |
|----|------------------------------------------|
| AC-1 | `TestHandleStructuredState_NotificationReasonFlushes`, `TestHandleStateEvent_NotificationReasonFlushes` (gr, new); `test/plugin/gr-notification-flushes.ci`; interop `bgp-graceful-restart-frr` NOTIFICATION phase |
| AC-2 | AC-7 to AC-12 proofs |
| AC-3 | AC-13 to AC-17 and AC-22 to AC-26 proofs |
| AC-4 | five `./le rfc audit-stamp ... mode rejudge` records by an agent that wrote none of the tests, verdict `enforced` in `rfc/audit/*.json` |
| AC-5 | `TestHandleStructuredState_ConnectionLostRetains` (gr, new); `test/plugin/gr-mark-stale.ci` (existing) |
| AC-6 | `TestPeerClosedReasonNotificationWhenExchanged`, `TestPeerClosedReasonWithoutNotification` (reactor, new) |
| AC-7 | `TestOnSessionReestablished_LLGRFBitClearPurges` (gr, new) |
| AC-8 | `TestOnSessionReestablished_FamilyAbsentFromLLGRPurges` (gr, new); `TestOnSessionReestablished_DuringLLGR_NoLLGRCap` (untagged, assertion corrected) |
| AC-9 | `TestOnSessionReestablished_DuringLLGR_NoCaps` (existing tagged, unchanged body) |
| AC-10 | `TestOnSessionReestablished_DuringLLGR` (existing tagged; see the tagged-test table) |
| AC-11 | `TestOnSessionReestablished_LLGRPeriodGRFBitClearPurges` (gr, new); `test/plugin/llgr-reconnect-fbit-purge.ci` connections three and four (new GR omits the family; new GR clears its F bit) |
| AC-12 | `TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets` (gr, new); `test/plugin/llgr-reconnect-fbit-purge.ci` connection five (a family with nonzero Restart Time reconnects inside its GR period, new LLGR omits it: kept until EOR) |
| AC-13 | `TestLLGRIngressFilter_MarksReceivedStale` (gr, new); `TestRIBReceivedStaleMetaSetsLevel` (rib, new); `TestComparePair_LLGRStale` (strengthened); `test/plugin/llgr-stale-received-depreferenced.ci`; interop `bgp-llgr-stale-received-frr` |
| AC-14 | `TestLLGRIngressFilter_NoLLGRSentLeavesFresh` (gr, new); same `.ci`, second source peer |
| AC-15 | `TestRIBReceivedRefreshWithoutStaleResetsLevel` (rib, new); `TestFamilyRIBOpaqueEqualRefreshWritesIngressLevel` (rib storage, new: a VPN and an EVPN entry through `insertOpaqueNoOp` and the equal branch of `insertOpaque`, both directions, red at HEAD because neither branch writes the level); `test/plugin/llgr-stale-received-depreferenced.ci` refresh step (the LLGR source re-announces the prefix without LLGR_STALE: best path moves to it on LOCAL_PREF) |
| AC-16 | `TestLLGREgressFilter_ReceivedStaleMeta` (gr, new, all three destinations, live rail); `TestRIBReplayMetaReadsSourceIngressLevel` (rib, new: after `markStaleCommand` level 1 and `attachSentSourceCommunityLocked` over the source, `collectGroupedRibOutRoutesFiltered` and `collectRibOutRoutes` still report level 2 for the destination's copy, and a copy whose source entry is fresh reports 0; r3 N-1 case: the source re-announced while still LLGR_STALE, so its current entry carries a new MsgID the copy does not hold, then the refresh rail (`collectRibOutRoutes` feeding `sendRoutes`) to an EBGP neighbour without LLGR still reports level 2, the level on which `LLGREgressFilter` withholds it (`TestLLGREgressFilter_ReceivedStaleMeta` covers that decision), while `sentSourceCurrent`'s current answer for that copy stays false; red under a level read gated on MsgID); `TestPeerUpReplayReportsSourceIngressLevel` (rib, new, the peer-up RIB rail, design review r4 I-1: the destination has a copy of a route whose source entry holds `IngressStaleLevel` 2 and the copy's `StaleLevel` 0, and `collectPeerUpReplay(dest, seenBefore=true)` returns that copy's group at stale level 2, red at HEAD where the group carries the copy's 0; a copy whose source entry is fresh comes back at 0); `TestBuildRelayUpdateMarksRebuilt` (reactor, new: the update `buildRelayUpdate` returns carries `filterapi.MetaKeyRebuilt`); `TestLLGREgressFilter_RebuiltRecoversReceivedStale` (gr, new: rebuilt key plus a source in the sent-LLGR snapshot plus LLGR_STALE in COMMUNITIES withholds from EBGP non-LLGR; without the key, or with a source not in the snapshot, or without the community, it accepts); `test/plugin/llgr-stale-received-egress.ci` live and relay steps (no RIB-replay step, r4 I-1: ze-peer drops no capability per connection and cannot order a close after a reload); interop `bgp-llgr-stale-received-frr` |
| AC-17 | `TestCommunityHandler_PreservedSurvivesSuppress`, `_PreservedSurvivesRemove`, `_PreservedSurvivesSet` (filter_community, new); `test/plugin/llgr-stale-community-preserved.ci` |
| AC-18 | `TestCommunityHandler_NoPreservedValueUnchanged` (filter_community, new); `test/plugin/send-community-suppress.ci` (existing, unchanged) |
| AC-19 | `TestLLGREgressFilter_GRStaleLevelOnePassesThrough` (gr, new); `test/plugin/llgr-stale-received-egress.ci` GR-replay step (an EBGP neighbour without LLGR has received a GR peer's routes; that neighbour closes TCP and RECONNECTS while the GR peer's routes sit at level 1 after its own TCP loss, so `collectPeerUpReplay` runs with `seenBefore` true: the neighbour receives them, red at HEAD where the level-1 replay is a withdraw; a level-2 route is still withheld; N-4) |
| AC-20 | `./le rfc check` output pasted at closure, after every row of "Discrimination records this spec re-records" carries a fresh record |
| AC-22 | `TestRIBPurgeStaleKeepsIngressStale` (rib, new: `PurgeStale` over a family holding an `IngressStaleLevel` 2 entry at `StaleLevel` 0 keeps it); `test/plugin/llgr-stale-received-depreferenced.ci` TCP-loss step (`show bgp rib received` still holds the route, best path unchanged) |
| AC-23 | `TestRIBRefreshedIngressStaleSurvivesEORPurge` (rib, new); same `.ci`, reconnect step (re-send with LLGR_STALE, EOR, route present and not best; an un-resent route gone) |
| AC-24 | `TestRIBMarkStaleKeepsEffectiveIngressLevel` (rib, new: `MarkStale(1)` then `extractCandidate` reports level 2); same `.ci`, best path after the TCP-loss step; `TestLLGREgressFilter_ReceivedStaleMeta` covers the egress half |
| AC-25 | `TestLLGRIngressFilter_ReadsReceivedCommunity` (gr, new: the GR step is registered at `FilterStageProtocol`, so it sorts before every `FilterStagePolicy` step); `test/plugin/llgr-stale-received-depreferenced.ci` with an import `community-remove ffff0006` on the LLGR source, and a strip step with `filter_community` `community { strip }` naming 0xFFFF0006 (red at HEAD: the strip removes it before storage); `TestIngressStripKeepsPreservedCommunity` (filter_community, new, both polarities); `TestIngressRawOverrideRestoresPreservedCommunity` (reactor, new, `runIngressPolicyChain` with a raw response that drops LLGR_STALE, and one that keeps it) |
| AC-26 | `TestEgressRawOverrideRestoresPreservedCommunity` (reactor, new, `runEgressPolicyChainASN4`, both polarities). These reactor units through the real chain functions ARE the AC-26 proof (design review r3, N-3): the raw-override filters the harness can load (`filter_family/handler.go`, `filter_remove_private_as`, used by `test/plugin/remove-private-as-export.ci`) return a raw body, but none drops COMMUNITIES, and no `test/` script returns a raw body of its own, so no `.ci` step can drive the branch without writing a test plugin for it |
| AC-27 | `TestCommunityHandler_AttachSurvivesSuppress`, `_AddDroppedBySuppress` (filter_community, new, tagged RFC9494-4.6-2 positive and negative); `TestCommunityHandler_AttachSurvivesSet` (filter_community, new, untagged handler-contract unit: it pins the `AttrModAttach` contract under a Set, which no live producer records on code 8 in the shared accumulator, round 7, so it proves no RFC obligation); `TestLLGREgressFilter_NilStateDepreferencesIBGP` and `TestLLGREgressFilter_IBGPPartial` (gr, changed under Q-8: the op is `AttrModAttach`); `test/plugin/llgr-stale-community-preserved.ci` fourth destination (IBGP, no LLGR, `send-community none`: COMMUNITIES is exactly `FFFF0006 FFFFFF01`; a route without LLGR_STALE to it carries no COMMUNITIES; red with `gr_egress.go` back on `AttrModAdd`), plus a tag step on the same destination (a `filter_community` `community { tag }` naming a standard community toward it: the received-stale route still carries exactly `FFFF0006 FFFFFF01` and the route without LLGR_STALE still carries no COMMUNITIES, so the negative half, an operator's value still suppressed, is proven on the real path) |
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
| `TestRIBPurgeStaleKeepsIngressStale`, `TestRIBRefreshedIngressStaleSurvivesEORPurge`, `TestRIBMarkStaleKeepsEffectiveIngressLevel` | `internal/component/bgp/plugins/rib/rfc9494_test.go` | AC-22, AC-23, AC-24 (B-1); tagged RFC4724-4.2-3 positive (retain at TCP loss) and RFC9494-4.3-1 positive (still least preferred) | new |
| `TestLLGRIngressFilter_ReadsReceivedCommunity` | `internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go` | AC-25 (N-6): the ingress step sorts before `FilterStagePolicy` | new |
| `TestLLGREgressFilter_ReceivedStaleMeta`, `TestLLGREgressFilter_GRStaleLevelOnePassesThrough` | `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | AC-16, AC-19; tagged RFC9494-4.3-3 | new |
| `TestLLGREgressFilter_RebuiltRecoversReceivedStale` | `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | AC-16 relay rail (B-1); tagged RFC9494-4.3-3 positive and negative | new |
| `TestBuildRelayUpdateMarksRebuilt` | `internal/component/bgp/reactor/` (beside `relay_payload_test.go`) | AC-16 relay rail: the rebuilt key is set on every `buildRelayUpdate` result | new |
| `TestRIBReplayMetaReadsSourceIngressLevel` | `internal/component/bgp/plugins/rib/rfc9494_test.go` | AC-16 RIB rails (B-1); tagged RFC9494-4.3-3 positive | new |
| `TestPeerUpReplayReportsSourceIngressLevel` | `internal/component/bgp/plugins/rib/rfc9494_test.go` | AC-16 peer-up RIB rail through `collectPeerUpReplay(dest, seenBefore=true)` (r4 I-1, in place of a `.ci` step); the fixture's sent copy is current (its SourceOwner, SourceMessageID and path match the source entry), because `collectGroupedRibOutRoutesFiltered` drops a copy `sentSourceCurrent` calls not current and the test would never reach the level; tagged RFC9494-4.3-3 positive | new |
| `TestLLGRStaleFromSource` | `internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go` | D3/D4: the one predicate both callers use (r4 N-1): source in the sent-LLGR snapshot AND LLGR_STALE in COMMUNITIES is true; either half missing is false | new |
| `TestFamilyRIBOpaqueEqualRefreshWritesIngressLevel` | `internal/component/bgp/plugins/rib/storage/` | AC-15 opaque families (I-2) | new |
| `TestIngressStripKeepsPreservedCommunity` | `internal/component/bgp/plugins/filter_community/` | AC-25 strip site (I-1); tagged RFC9494-4.3-2 positive and negative | new |
| `TestIngressRawOverrideRestoresPreservedCommunity`, `TestEgressRawOverrideRestoresPreservedCommunity` | `internal/component/bgp/reactor/` | AC-25 raw import, AC-26 raw export (I-1); tagged RFC9494-4.3-2 | new |
| `TestCommunityHandler_PreservedSurvivesSuppress`, `_PreservedSurvivesRemove`, `_PreservedSurvivesSet`, `_NoPreservedValueUnchanged` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-17, AC-18; tagged RFC9494-4.3-2 positive and negative | new |
| `TestCommunityHandler_AttachSurvivesSuppress`, `_AttachSurvivesSet`, `_AddDroppedBySuppress` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-27 (Q-7): through `genericCommunityHandler`, an `AttrModAttach` NO_EXPORT under a code-8 Suppress emits LLGR_STALE (source) then NO_EXPORT, and under a Set emits the Set values then NO_EXPORT; the same value as `AttrModAdd` under a Suppress emits nothing (no preserved source value) or LLGR_STALE alone; `_AttachSurvivesSuppress` tagged RFC9494-4.6-2 positive, `_AddDroppedBySuppress` negative; `_AttachSurvivesSet` is an untagged handler-contract unit (no live producer records a code-8 Set in the shared accumulator, round 7) | new |
| `TestWireModVerbAttach` | `internal/component/bgp/reactor/` (beside the `policy_dryrun.go` tests) | dry-run prints "attach" for `AttrModAttach`, not the default "modified" | new |
| `TestLLGREgressFilter_NilStateDepreferencesIBGP`, `TestLLGREgressFilter_IBGPPartial` | `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | AC-27: the 4.6 NO_EXPORT op is `AttrModAttach`; tagged RFC9494-4.6-2 positive | change (Q-8) |
| `TestReceivedUpdateEventCarriesIngressMeta` | `internal/component/bgp/reactor/` | the plumb: a received UPDATE's plugin event carries `routeMeta` | new |
| `BenchmarkLLGRIngressFilter_NoLLGRPeer` | `internal/component/bgp/plugins/gr/` | the per-UPDATE cost for a peer Ze sent no LLGR is a snapshot load and a map miss, 0 allocs | new |

### RFC-tagged tests this spec changes (owner approval, Q-4)

| Tagged unit | Requirement | Change | Why |
|-------------|-------------|--------|-----|
| `internal/component/bgp/plugins/rib/bestpath_test.go::TestComparePair_LLGRStale` | RFC9494-4.3-1 positive | the stale candidate gets the higher LOCAL_PREF and the lower peer address, so only Step 0 can make it lose | the audit found it passes with Step 0 removed |
| `internal/component/bgp/plugins/gr/gr_state_test.go::TestOnSessionReestablished_DuringLLGR` | RFC9494-4.2-8 negative | body unchanged; tag prose rewritten (it cites `newForwarding` and gr_state.go line ranges the fix replaces) and a fresh discrimination record | a tagged unit whose producer and claim change owes a new record |
| `internal/component/bgp/plugins/gr/gr_state_test.go::TestOnSessionReestablished_DuringLLGR_NoCaps` | RFC9494-4.2-8 (bullet 3) | body unchanged; tag prose drops its `gr_state.go` and `gr.go` line citations, which have moved (design review r1, N-7), and the unit gets a discrimination record because D2 changes its producer `onSessionReestablished` | its producer changes. The 2026-10-13 Q-4 answer names the two units above; this third unit's prose change needs the owner's approval command too (below), and is raised here, not assumed |
| `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go::TestLLGREgressFilter_NilStateDepreferencesIBGP` | RFC9494-4.6-2 positive (4.6-3 positive unchanged) | the NO_EXPORT op is asserted as `AttrModAttach` in place of `AttrModAdd`; value and length assertions unchanged; fresh discrimination record | design round 6 (Q-7) moves the producer to `AttrModAttach`; not approved: Q-8 |
| `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go::TestLLGREgressFilter_IBGPPartial` | RFC9494-4.6-2 positive (4.6-1, 4.6-3 positive unchanged) | the same assertion change; the 4.6-2 tag prose names `mods.Op(attrCodeCommunity, AttrModAttach, communityNoExport)` and says the value survives a `send-community` Suppress; fresh discrimination record | as above; not approved: Q-8 |

Every other tagged unit named in the verdict table keeps its body and tag:
`TestGRStateManagerNotificationBypass`, `TestGRStateManagerRouteRetention`,
both `rib/rfc9494_test.go` units, `TestComparePair_GRStaleCompetesNormally`,
both 4.3-3 egress units and `test/plugin/llgr-egress-state-unloaded.ci`. New
tagged units are additions, each with a `./le rfc discriminate-record` record in
the same change. Keeping body and tag does not keep the record: a unit whose
producer this spec edits owes a fresh record, listed below.

#### Discrimination records this spec re-records (design review r1, I-1)

`./le rfc check` refuses a record whose `producer-sha` no longer matches the
committed producer (`docs/contributing/rfc-conformance-gates.md`), whatever RFC
the record belongs to. Searched: every `producer` field in
`rfc/discrimination/*.json` against every function this spec edits, 2026-10-14.
Each row is re-recorded with `./le rfc discriminate-record` in the phase that
edits its producer; a row marked "if edited" is owed only when the implementation
changes that function's body, and the phase names which way it went.

| Phase | Producer | Records (requirement, polarity, unit) | Owed |
|-------|----------|----------------------------------------|------|
| Wiring | `reactor/reactor_notify.go::notifyMessageReceiver` (received `Meta` plumb) | RFC4271-5.1.4-3 positive `reactor/rfc4271_med_ingress_test.go::TestRFC4271ReceivedMEDAlteredBeforeTheDecisionProcess`; RFC4271-9.2-4 positive `reactor/rfc4271_overlap_acceptance_test.go::TestRFC4271OverlappingRoutesAcceptedByPolicyBothReachTheDecisionProcess` | yes |
| D1 | `plugins/gr/gr.go::handleStateEvent` | RFC4724-4.2-4 negative `rfc4724_consecutive_test.go::TestRFC4724ConsecutiveRestartDeletesBeforeMarking`; RFC4724-4.2-4 positive `::TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes` | yes |
| D1 | `reactor/reactor_notify.go::notifyPeerClosed` | RFC4271-8.2.2-12 positive `reactor/rfc4271_established_teardown_peer_test.go::TestRFC4271EstablishedNotificationOrTCPFailureReleasesTheConnection`; RFC4760-7-1 positive `reactor/rfc4760_peer_down_link_test.go::TestRFC4760IncorrectMPReachRaisesThePeerDownThatClearsTheRIB`; RFC8654-5-4 positive `reactor/rfc8654_fatal_length_cleanup_test.go::TestRFC8654FatalLengthReleasesInstalledRoutes` | if edited (D1 changes the reason at `peer_run.go`; the rs spec's Phase 2b move may change this body) |
| D2 | `plugins/gr/gr_state.go::onSessionReestablished` | RFC4724-4.2-7 negative `rfc4724_dispatch_test.go::TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes`; RFC4724-4.2-8 negative `rfc4724_dispatch_test.go::TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes`; plus the two tagged units with no record today, `TestOnSessionReestablished_DuringLLGR` and `TestOnSessionReestablished_DuringLLGR_NoCaps` (first record) | yes |
| D3 | `plugins/rib/rib_structured.go::handleReceivedStructured` | RFC7705-3.3-2 positive `reactor/rfc7705_live_behavior_test.go::TestRFC7705NoPrependInstalledAndAdvertised` | yes |
| D3 | `plugins/rib/rib_structured.go::insertLabeledEntry` | RFC8277-2.5-3 negative `rfc8277_test.go::TestRFC8277AddPathSameIdentifierRebinds` | if edited (labelled inserts set `IngressStaleLevel` too) |
| D3 | `plugins/rib/rib_commands.go::extractCandidate` (effective level into `Candidate`) | RFC4271-9.1.2.2-4 negative `rfc4271_test.go::TestAbsentMedTiesAnExplicitMedOfZero`; positive `::TestAbsentMedStillComparesAsZeroInPhaseTwo` | yes |
| D3 | `plugins/gr/gr_llgr_exchange.go::exchangedLLGRLocked` | RFC9494-5-1 negative `rfc9494_helper_default_red_test.go::TestRFC9494HelperProceduresOffWithoutLocalConfig`; positive `::TestRFC9494HelperProceduresOnPerConfiguredFamily` | if edited (the snapshot is published from `recordSentLLGR`, which carries no record) |
| D4/D6 | `plugins/gr/gr_egress.go::LLGREgressFilter` | RFC9494-4.3-3 positive `rfc9494_gr_egress_test.go::TestLLGREgressFilter_NilStateWithdrawsEBGP` | yes |
| D4 | `plugins/rib/rib_commands.go::sendRoutes` (effective level into readvertise `meta`) | RFC2918-4-3 positive `rib_test.go::TestHandleRefresh_InternalState`; RFC7313-4-2 positive `rfc7313_sequence_test.go::TestRFC7313BoRRRoutesEoRRInOneSequence` | if edited |
| D4 | `plugins/rib/rib_replay.go::collectGroupedRibOutRoutesForFamily` | RFC2918-4-3 positive `rfc2918_config_static_test.go::TestRFC2918ConfigStaticRetainedForRefresh` | if edited |
| D5 | `plugins/filter_community/handler.go::genericCommunityHandler` | RFC9012-11-10 negative and positive `reactor/rfc9012_encapsulation_filter_test.go::TestRFC9012ConfiguredEncapsulationECEgress` | yes |
| D3 | `plugins/rib/storage/peerrib.go::InsertEntry` (design review r2, N-2) | RFC4271-9-2 negative `rib/rfc4271_receive_decision_test.go::TestRFC4271ReplacementWithdrawsTheOlderRouteEvenWhenItWasBetter`; RFC4271-9-2 positive `::TestRFC4271ReceivedRouteWithIdenticalNLRIReplacesTheOlder` | if edited (carrying the level into the insert may change its signature) |
| D4 | `reactor/reactor_api_relay.go::buildRelayUpdate` (rebuilt key, false comment replaced; design review r2, B-1) | RFC2545-3-1 positive and RFC2545-3-2 positive `test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci` | yes |
| D4 | `plugins/rib/rib_structured.go::handleSentStructured` | RFC2918-4-3 positive `test/plugin/refresh-config-static.ci` | if edited (the r2 design leaves it untouched: the RIB rails read the source entry at replay time, so the sent handler stores nothing new) |
| D5 | `plugins/filter_community/filter.go::ingressStripCommunities` (design review r2, I-1) | RFC9012-11-9 positive and negative `reactor/rfc9012_encapsulation_filter_test.go::TestRFC9012ConfiguredEncapsulationECIngress` | if edited (r3, answer 4: the exemption is computed in `config.go::parseCommunityDefinitions`, which holds no record, searched with this row's record as the control, so this function is expected unchanged) |
| D5 | `reactor/filter_ordered.go::runIngressPolicyChain` (raw-override branch) | RFC4271-5.1.5-5 positive `reactor/rfc4271_degree_of_preference_test.go::TestRFC4271ExternalRoutePreferenceFromPolicyReachesRIBAndInternalPeers`; RFC4271-9.1.1-2 positive `reactor/rfc4271_local_pref_readvertise_test.go::TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers`; RFC4271-9.2-4 negative `reactor/rfc4271_overlap_acceptance_test.go::TestRFC4271OverlappingRouteRejectedByPolicyIsNotConsidered` | yes |
| D5 | `reactor/filter_ordered.go::runEgressPolicyChainASN4` (raw-override branch) | RFC9012-11-7 negative and positive `reactor/rfc9012_session_filter_test.go::TestRFC9012TunnelFilterDoesNotAffectOtherSession` / `::TestRFC9012TunnelFilterSelectedSession`; RFC4271-5.1.2-2 negative `reactor/rfc4271_ibgp_aspath_export_test.go::TestRFC4271ExportASPathEditsFollowTheMigrationAwareInternalVerdict` and `::TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath`; RFC7947-x-4 positive and negative `reactor/rfc7947_consumer_test.go::TestRFC7947ClientPolicyControlsWire` | yes |
| D5 (Q-7, round 6) | `plugins/filter_community/handler.go::genericCommunityHandler` (`AttrModAttach`) | new records: RFC9494-4.6-2 positive `filter_community/handler_test.go::TestCommunityHandler_AttachSurvivesSuppress`, negative `::_AddDroppedBySuppress`, each by the break "treat `AttrModAttach` as `AttrModAdd`"; the D5 RFC9012-11-10 row above is re-recorded once, after both D5 edits | yes |
| D5 (Q-7, round 6) | `plugins/gr/gr_egress.go::LLGREgressFilter` (4.6 NO_EXPORT as `AttrModAttach`) | new records (claims change, Q-8): RFC9494-4.6-2 positive `rfc9494_gr_egress_test.go::TestLLGREgressFilter_NilStateDepreferencesIBGP` and `::TestLLGREgressFilter_IBGPPartial`, by the break "record `AttrModAdd`"; the D4/D6 RFC9494-4.3-3 row above is re-recorded once, after all three edits to this function | yes |

Each row added in round 2 was read from `rfc/discrimination/*.json` (the `producer`, `rid`, `polarity` and `unit` fields) on 2026-10-14. Searched and holding no record (round 2): `storage/familyrib_opaque.go::insertOpaqueNoOp` and `insertOpaque` (the file's one record is on `RouteKey`), `ribout_entry.go::reconstructRoute` and `collectRibOutRoutes`, `rib_replay.go::collectGroupedRibOutRoutesFiltered`, `filter_community/filter.go::applyIngressFilter` and `applyIngressOps`, `reactor_api_relay.go::RelayStoredRoute`. Searched and holding no record (round 3): `rib/sent_owner.go::sentSourceCurrent`, `filter_community/config.go::parseCommunityDefinitions`. Searched and holding no record (round 6): `reactor/policy_dryrun.go::wireModVerb` (the file's record is on `computeWireChanges`, which round 6 does not edit).

Searched and holding no record: `pkg/plugin/rpc/enums.go`, `bridge.go`,
`reactor/peer_run.go`, `gr.go::handleStructuredState`, `gr_llgr_exchange.go::recordSentLLGR`,
`filterapi/filterapi.go`, the `familyrib.go` insert functions and `MarkStale` /
`PurgeStale`, `pathset.go::refresh`, `rib_replay.go::replayRoutesWithCursor` /
`resendRoutesWithCursor`, `rib_sent_lifecycle.go`, `rib_attr_format.go`, and
`bmp/bmp_events.go::peerDownFor` (comment-only edit, N-4).

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

Not yet approved (raised by design review r1, N-7): the prose change on
`gr.TestOnSessionReestablished_DuringLLGR_NoCaps`. The Q-4 answer names two
units, so this unit needs its own `./le rfc approve unit` command from the owner,
with a reason naming this spec and "body unchanged, tag prose line citations
removed". Until the owner gives it, the implementer leaves that tag prose as it is
and records the unit against its unchanged claim.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stale level in route metadata | 0-255 | 1 passes the egress filter (Q-2), 2 is withheld | N/A | N/A |

### Functional Tests
- filled by the table below (2026-10-13)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-notification-flushes` | `test/plugin/gr-notification-flushes.ci` | peer with GR sends routes and EOR, then NOTIFICATION: `show bgp rib received` holds none of its routes before its second connection, no `gr-state` row exists; second connection ends with Ze's hold timer NOTIFICATION, same outcome | new |
| `llgr-reconnect-fbit-purge` | `test/plugin/llgr-reconnect-fbit-purge.ci` | `le test peer` with restart-time 0 and LLST 60 sends two families, closes TCP, reconnects with LLGR F clear for one family and absent for the other (one connection each): both purged before EOR. Connections three and four (AC-11, Q-1 = AND): new LLGR lists the family with F set, new GR omits it, then new GR clears its F bit: purged before EOR each time. Connection five (AC-12): restart-time 120, LLST 60, reconnect inside the GR period with the new LLGR omitting the family: kept until EOR | new; needs the `le test peer` OPEN option to clear the LLGR F bit per family (A-4) |
| `llgr-stale-received-depreferenced` | `test/plugin/llgr-stale-received-depreferenced.ci` | two sources announce one prefix; the LLGR one carries LLGR_STALE and the higher LOCAL_PREF; best path is the other source. Refresh step (AC-15): the LLGR source re-announces without LLGR_STALE, best path moves to it. TCP-loss step (AC-22, AC-24): with LLGR_STALE back on, the LLGR source closes TCP; the route is still held and still not best. Reconnect step (AC-23): it re-sends the route with LLGR_STALE and EOR; the route is present and not best, a retained route it did not re-send is gone. Import-policy step (AC-25): `community-remove ffff0006` on the LLGR source's import, route still not best, stored COMMUNITIES still holds LLGR_STALE. Strip step (AC-25, I-1): the same with `filter_community` `community { strip }` naming 0xFFFF0006 in place of `community-remove`, red at HEAD | new |
| `llgr-stale-received-egress` | `test/plugin/llgr-stale-received-egress.ci` | Live step: the AC-16 three destinations. No RIB-replay step (design review r4, I-1, main-thread decision): the RIB rails (AC-16 rail 2) need a destination that negotiated LLGR on one connection and not on the next, and ze-peer cannot give one, because it drops no capability per connection (it mirrors ze's OPEN, `internal/test/peer/open_capability.go::ownedCapabilities`; `option=open:value=drop-capability` is per peer block) and it cannot order a `close` after a config reload (`internal/test/peer/peer.go::runMessageLoop` checks the queued close before the rewrite and sighup actions of the same matched message, and the only wait after a signal is a fixed sleep, while an LLGR-only reload that keeps the session emits no BGP message to match), so such a step could pass on a lost race. The RIB rails are proven by the units instead: `TestPeerUpReplayReportsSourceIngressLevel` (peer-up), `TestRIBReplayMetaReadsSourceIngressLevel` (refresh and resend, and the overwrite case), with `TestLLGREgressFilter_ReceivedStaleMeta` for the withhold decision those levels reach. Relay step (AC-16 rail 3, B-1): with `bgp-adj-rib-in` carrying the replay, a NEW EBGP neighbour without LLGR comes up after the route is stored: it does not receive the route, red at HEAD where `buildRelayUpdate` sets `Meta` nil; a stored route without LLGR_STALE from the same source reaches it. Each step's config loads only the plugin whose rail it proves, so the other rail cannot answer for it. GR-replay step (AC-19, Q-2 = yes, N-4): an EBGP neighbour without LLGR that already received a GR peer's routes reconnects while those routes sit at level 1 after the GR peer's TCP loss: it receives them, while the received LLGR_STALE route is still withheld from it | new |
| `llgr-stale-community-preserved` | `test/plugin/llgr-stale-community-preserved.ci` | the AC-17 three destinations, exact COMMUNITIES hex; a fourth destination (AC-27, design round 6): an IBGP neighbour without LLGR at `send-community none` receives the received-stale route with COMMUNITIES exactly `FFFF0006 FFFFFF01` and LOCAL_PREF 0, and a route without LLGR_STALE with no COMMUNITIES. Tag step (AC-27 negative, round 7): with a `filter_community` `community { tag }` naming a standard community toward that destination, the received-stale route still carries exactly `FFFF0006 FFFFFF01` and the route without LLGR_STALE still carries no COMMUNITIES | new |

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
| `internal/component/bgp/plugins/gr/gr_ingress.go` (new) | `LLGRIngressFilter`, and `llgrStaleFromSource`, the one D3 predicate (source address in the "Ze sent LLGR" snapshot AND the payload's COMMUNITIES holds 0xFFFF0006), called by `LLGRIngressFilter` and by `LLGREgressFilter`'s rebuilt-route recovery and by nothing else (r4 N-1; the name is unused in `internal/` today) |
| `internal/component/bgp/plugins/gr/gr_egress.go` | D6 threshold (Q-2); the rebuilt-route recovery (B-1): no stale level plus `filterapi.MetaKeyRebuilt` -> `llgrStaleFromSource` over `src` and the payload |
| `internal/component/bgp/plugins/gr/register.go` | a second `filterapi.Filter` entry, Name `bgp-gr-llgr-stale-ingress`, `Stage: FilterStageProtocol`, `Ingress: LLGRIngressFilter` (N-6, r2 N-1); register LLGR_STALE as preserved |
| `internal/component/bgp/filterapi/filterapi.go` | preserved-community registration and read accessor; declare `MetaKeyRebuilt`; declare `AttrModAttach` with its contract (Q-7, round 6), and the `AttrOp.Action` comment lists it |
| `internal/component/bgp/plugins/filter_community/handler.go` | keep preserved values on Suppress, Remove and Set for code 8; emit `AttrModAttach` values after every Set, Suppress and Remove outcome (Q-7); the "Set and Suppress both override all prior Remove/Add ops" comment says attached values are the exception |
| `internal/component/bgp/plugins/gr/gr_egress.go` (Q-7) | the Section 4.6 branch records NO_EXPORT as `AttrModAttach`, with the Section 4.6 quote above the statement |
| `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | the two RFC9494-4.6-2 units assert `AttrModAttach` (Q-8) |
| `internal/component/bgp/reactor/policy_dryrun.go` | `wireModVerb` case for `AttrModAttach` ("attach") |
| `internal/component/bgp/reactor/reactor_stale_readvertise_test.go` | the two fakes modelling GR's Section 4.6 NO_EXPORT (`mods.Op(8, filterapi.AttrModAdd, ...)`, lines ~65 and ~184, untagged) record `AttrModAttach`, so the fakes model the producer they stand for (round 7) |
| `docs/architecture/core-design.md` | the `AttrOp` Action row lists all six actions (it omits `AttrModSuppress` at HEAD) and says what `AttrModAttach` survives |
| `internal/component/bgp/plugins/filter_community/filter.go`, `config.go` | a code-8 strip keeps a preserved value whatever the strip list names (I-1): `parseCommunityDefinitions` builds each code-8 definition's strip set once, without the registered preserved values and apart from its tag set, and `applyIngressOps` hands `ingressStripCommunities` that set; codes 16 and 32 unchanged (r3, answer 4) |
| `internal/component/bgp/reactor/filter_ordered.go` | the raw-override branches of `runIngressPolicyChain` and `runEgressPolicyChainASN4` add back a preserved value the override dropped (I-1) |
| `internal/component/bgp/reactor/reactor_api_relay.go` | `buildRelayUpdate` sets `Meta` to the read-only rebuilt map; its `Meta` comment is replaced, not amended: "RFC 9494 staleness is genuinely false for a replay" is deleted, and the OTC paragraph's "a nil Meta here" is rewritten for a non-nil map without `src-role` (stale-comments; B-1) |
| `internal/component/bgp/plugins/rib/rib_structured.go`, `storage/familyrib.go`, `storage/familyrib_opaque.go`, `storage/pathset.go`, `storage/routeentry.go` | `IngressStaleLevel` field (B-1), set from the received event's metadata on every insert and refresh, the opaque no-op and equal branches included (I-2); `StaleLevel` keeps resetting to fresh |
| `internal/component/bgp/plugins/rib/rib_commands.go`, `rib_attr_format.go` (and any other received-entry `StaleLevel` reader the implementer's grep finds) | read the effective level: `extractCandidate`, `show` `stale-level` |
| `internal/component/bgp/plugins/rib/rib_replay.go`, `ribout_entry.go`, `rib_commands.go`, `sent_owner.go` | `sent_owner.go::sentSourceCurrent` returns both the current answer (MsgID match, unchanged) and the source's ingress level (by key, MsgID ignored) from one lookup (r3 I-1, N-1); the RIB rails' `meta["stale"]` and replay group key read the higher of `ribOutEntry.StaleLevel` and that level: `collectGroupedRibOutRoutesFiltered` (feeding `replayRoutesWithCursor` and `resendRoutesWithCursor`), `reconstructRoute` / `collectRibOutRoutes` (feeding `sendRoutes`) (B-1) |
| `internal/component/bgp/plugins/bmp/bmp_events.go` | the comment on `fsmEventNone` (above the `peerDownFor` / `clearPeerState` group) says `Peer.Run` sends "session closed" or "connection lost"; after D1 it also sends "notification", which names no RFC 4271 Section 8.1 event either, so the comment's conclusion stands and only its list changes (stale-comments, N-4) |
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
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (filter pipeline: preserved communities; received event metadata; the `AttrModAttach` action) |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md` and the GR meta page: `stale` now set at ingress |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | GR registers a second filter entry (ingress, `FilterStageProtocol`) beside its existing egress entry |
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
| D3/D4: one predicate, owned by GR: "the source is a neighbour Ze sent LLGR to AND the route's COMMUNITIES holds LLGR_STALE". The GR in-process ingress filter (own entry, `FilterStageProtocol`) evaluates it once per UPDATE and writes stale level 2 into ingress metadata; the reactor plumbs that map to the received plugin event; the RIB stores it as `IngressStaleLevel`, separate from the retention mark `StaleLevel` (r1 B-1). The one egress decision, `LLGREgressFilter`, is reached by each rail with the fact (r2 B-1): the live forward rails carry the ingress metadata; the RIB rails write `meta["stale"]` from the higher of the copy's `StaleLevel` and the source entry's `IngressStaleLevel`; the relay rails mark the rebuilt route and the filter evaluates the same predicate over the source and the stored bytes; the ingress step and the relay re-check call one predicate function, `llgrStaleFromSource` (design review r3, answer 2; named r4 N-1) | (b) RIB scans COMMUNITIES for 0xFFFF0006 on every insert; (c) GR subscribes to UPDATEs and dispatches `mark-stale` per route; (d) storing the received mark in `StaleLevel` itself; (e) the egress filter reads LLGR_STALE from the route's bytes on every rail, with `meta["stale"]` only for Ze's own marks (the main thread's r2 preference); (f) a second byte on `ribOutEntry` set from the sent event's `meta["stale"]` | (b) cannot know whether Ze sent LLGR to the source, puts LLGR knowledge in the RIB, and leaves the forward rails without metadata; (c) is per-UPDATE plugin traffic and races the insert; (d), the round-0 design, lets `purge-stale` delete it at TCP loss and at the post-refresh EOR, and `mark-stale` reset it to 1. (e) was checked at the producers and fails three ways: the RIB rail runs the readvertise filters only when the batch stale level is above 0 and hands them a zero `src` (`reactor_api_batch.go`, `decideStaleReadvertise`), so a byte reading never runs there and could not test the source; without the source test it withholds routes Section 4.3 does not bind (AC-14's source, a policy-added value), a behaviour nobody decided; and it puts a per-destination COMMUNITIES scan on every fresh route of the live rails, which "a fresh route never touches the GR egress filter beyond the zero-level check" forbids. Locally marked routes do carry the community (`attach-community` writes it into the received entry and `attachSentSourceCommunityLocked` into the copies), so (e) was not refused on that ground. Its byte reading is kept where the source is known and no metadata exists: the relay rails. (f) has one producer, the sent event, and a relayed route's sent event carries no stale level, so a copy made from a relayed send would hold 0; reading the source entry keeps the fact in its one holder and puts nothing in reach of `markStaleCommand` or `attachSentSourceCommunityLocked` |
| D5: preserved-community registration in `filterapi`, consulted by the code-8 handler | (b) LLGR_STALE constant hardcoded in `genericCommunityHandler`; (c) exempt at each of the three producers | (b) is a central enumeration in a plugin that does not own LLGR; (c) is three copies and misses the next producer. One handler sees every removal |
| D5 / Q-7: a sixth action, `filterapi.AttrModAttach`, recorded by the producer the protocol obliges (GR's Section 4.6 branch), honoured by the code-8 handler over every Set, Suppress and Remove | (b) every Add survives a Suppress; (c) a provenance field on `AttrOp`; (d) order GR after `send-community`; (e) NO_EXPORT in the preserved set | the rule is provenance, so the producer declares it once in the op it already records; (b) leaks operator values past `send-community none`, (c) is a second channel beside `Action` for one producer, (d) changes nothing under the order-blind Set/Suppress rule, (e) keeps every received NO_EXPORT past an operator strip (Data Flow, "Q-7") |
| D6: threshold at `storage.DepreferenceThreshold` | keep any nonzero level | RFC 9494 Section 4.3 binds only routes received with LLGR_STALE or given it per Section 4.2, so withholding a level-1 route is a restriction no RFC asks for; a behaviour choice, made by the owner (Q-2, 2026-10-14). RFC 4724's "during forwarding" sentence is not the ground (it concerns forwarding state) |

## Implementation Steps

1. Failing tests; 2. fixes; 3. discrimination records and verdicts.

Detailed phases (each: failing test, fix, pass, page edit in the same phase):

1. **Phase: Wiring** -- declare `rpc.ReasonNotification`; register the empty GR ingress filter; register the preserved set; plumb received `Meta`. Tests: `TestReceivedUpdateEventCarriesIngressMeta`, wiring `.ci` files red. Records: the Wiring rows of "Discrimination records this spec re-records".
2. **Phase: D1** -- reason choice in `peer_run.go`, both GR handlers. Tests: AC-1, AC-5, AC-6 rows. Pages: api/architecture.md Close Reasons, graceful-restart.md Peer Restarts (one line each for the N-5 readings: a NOTIFICATION Ze built but could not write still retains; Ze's own hold-timer expiry, a written NOTIFICATION, now flushes); the `bmp_events.go` `fsmEventNone` comment. Records: the D1 rows of "Discrimination records this spec re-records".
3. **Phase: D2** -- `onSessionReestablished`. Tests: AC-7 to AC-12 rows. Page: graceful-restart.md LLGR step 5. Records: the D2 rows.
4. **Phase: D3** -- ingress filter (own entry at `FilterStageProtocol`), sent-LLGR snapshot, `IngressStaleLevel` and the effective-level readers. Tests: AC-13 to AC-15 and AC-22 to AC-25 rows, benchmark. Pages: meta docs, graceful-restart.md stale levels (the received level is distinct from the retention mark). Records: the D3 rows.
5. **Phase: D4/D6** -- egress threshold (Q-2), the RIB rails' source-level read, the relay rails' rebuilt key and the filter's recovery, each rail named in AC-16 proven on its own. Tests: AC-16, AC-19 rows. Pages: graceful-restart.md readvertise table (one line per rail), `docs/architecture/meta/README.md` (the `rebuilt` key). The `buildRelayUpdate` `Meta` comment is replaced in the same edit as the code. Records: the D4/D6 rows.
6. **Phase: D5** -- code-8 handler, `ingressStripCommunities`, and the two raw-override branches, plus the Q-7 fold-in (the Section 4.6 NO_EXPORT surviving a `send-community` Suppress: `filterapi.AttrModAttach`, recorded by `gr_egress.go`, honoured by `genericCommunityHandler`, printed by `wireModVerb`; design round 6). Needs Q-8 approved before the two GR units change. Tests: AC-17, AC-18, AC-25 strip and raw rows, AC-26, AC-27. Pages: core-design.md filter pipeline and `AttrOp` action table; `docs/guide/graceful-restart.md` (a Section 4.6 neighbour at `send-community none` still receives NO_EXPORT and LLGR_STALE); `rfc/short/rfc9494.md` gap and Support row. Records: the D5 rows, including the two round-6 rows.
7. **Phase: interop** -- extend `bgp-graceful-restart-frr`, add `bgp-llgr-stale-received-frr`, record red with each fix reverted and the image rebuilt. Page: testing/interop.md.
8. **Phase: verdicts** -- discrimination records for every new or changed tagged unit AND every row of "Discrimination records this spec re-records" whose producer changed (a phase that skipped one owes it here; the Wiring phase's rows land in Phase 1); `./le rfc check` shows no violation for any RFC (AC-20); independent re-judge of the five verdicts (AC-4).

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has a test in AC proofs and code at a named symbol |
| Correctness | the LLGR-period family set is captured before `stopLLSTTimersLocked`; a refresh without LLGR_STALE returns `IngressStaleLevel` to 0; `MarkStale`, `PurgeStale`, `StaleCount` and `purgeStaleCommand` never read or write `IngressStaleLevel` (B-1: AC-22 to AC-24); every received-entry `StaleLevel` reader in `plugins/rib/` reads the effective level |
| Data flow | the RIB reads a stale level, never the community; filter_community reads a registered set, never an LLGR constant; the GR ingress step reads the community as received, before any policy stage (N-6); every rail into `LLGREgressFilter` (live forward, RIB replay/resend/refresh, relay through `buildRelayUpdate`) has its own AC-16 proof, and the relay recovery and the ingress filter call ONE predicate function, `llgrStaleFromSource`, not two copies (r2 B-1, named r4 N-1); no `ribOutEntry` field holds the ingress fact; the retention-only readers named under Current Behavior (r2 N-3) still read `StaleLevel` alone |
| Rule: performance | the live egress fast path is unchanged (stale level 0 and no rebuilt key: accept, before any state read); the relay recovery runs only on rebuilt routes, one snapshot load and, for a source Ze sent LLGR, one zero-copy walk of the attribute block to code 8, no allocation; the rebuilt map is one package-level value, not one per route; ingress filter: no mutex, no allocation for a peer Ze sent no LLGR; community handler keeps zero-copy fragments; `RouteEntry` size checked before and after `IngressStaleLevel` (a byte that fits existing padding costs nothing per route, a growth is measured and reported) |
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
| Policy bypass | the preserved set lets one value through `send-community none`; it must hold LLGR_STALE only, registered by GR. `AttrModAttach` lets a second through, NO_EXPORT, and only on the Section 4.6 branch: GR is its one producer, and the operator's `tag` and policy `community-add` stay `AttrModAdd`, so `send-community none` still suppresses them (AC-27 negative) |

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
| A-6 | the RIB runs in-process, so `se.Meta` reaches it as a map | RIB is an internal plugin; `StructuredEvent.Meta` is a Go map on DirectBridge | an external RIB would need `meta` on its JSON event. The JSON received path that would then get no level is `rib.go::insertPoolNLRIs` (through `peerRIB.Insert` / `insertLabeled`), and its sent twin `rib.go::handleSent` (design review r2, N-5) | `TestReceivedUpdateEventCarriesIngressMeta` and the JSON event encoder check | unvalidated |
| A-7 | FRR can originate a route carrying `llgr-stale` via route-map toward a session where LLGR is negotiated | FRR community keyword list includes llgr-stale | the interop scenario needs a native injector instead | first interop dry run | unvalidated |
| A-8 | an import-side removal of LLGR_STALE, by `filter_community`'s in-process `FilterStagePolicy` ingress or by the per-peer chain's delta, ends in the code-8 handler, so D5 preserves the stored value (AC-25) | D5's "every removal of COMMUNITIES ends in the one code-8 attribute handler" (Key insights); the in-process ingress path not read in design round 1 | AC-25's stored-community half fails; D5 owes the ingress path too | read at the producers in design review r2 (I-1): the per-peer chain reaches the handler (`runIngressPolicyChain` -> `textDeltaToModOps` -> `buildModifiedPayload` with `r.attrModHandlers`); `filter_community`'s ingress does NOT (`ingressFilter` -> `applyIngressFilter` -> `applyIngressOps` -> `ingressStripCommunities` rewrites the bytes), and neither does a raw-override response. D5 now owns `ingressStripCommunities` and both raw-override branches (Data Flow D5, AC-25, AC-26) | refuted 2026-10-14 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | changing the reason string breaks a test or consumer that matches "session closed" after a NOTIFICATION | grep of tests for the two strings; reactor tests red | the change is limited to notified sessions; consumers that need the distinction already use BMP's cache |
| R-2 | ingress filter adds per-UPDATE cost on the receive hot path | benchmark; allocs/op above 0 | atomic snapshot lookup first; scan COMMUNITIES only for peers Ze sent LLGR |
| R-3 | plumbing `routeMeta` into received plugin events changes what other plugins see (`src-role` now visible) | plugin tests that assert `se.Meta == nil` on received events | no received-event reader of `se.Meta` exists in the RIB today (`handleReceivedStructured` reads none; the three named keys are read in `handleSentStructured`), so the audit is of the OTHER DirectBridge subscribers: grep every `se.Meta` read in `internal/component/bgp/plugins/` and `internal/plugins/` and confirm each one sits on a sent-event path or ignores unknown keys |
| R-4 | Q-1 AND reading purges LLGR-only families at every reconnect: a regression for a configuration RFC 9494 Section 4.2 contemplates ("only the procedures particular to LLGR would apply") and Ze supports today | AC-11 `.ci`; operator report | accepted by the owner (Q-1 AND, 2026-10-14, cost stated); the guide's LLGR "How It Works" step 5 says so in the D2 phase |
| R-5 | `send-community none` peers start receiving one community (Q-3) | `send-community-suppress.ci` stays green only for routes without LLGR_STALE | documented in the guide; only LLGR-capable or 4.6 IBGP destinations ever receive such routes after D4 |
| R-6 | Section 4.6 IBGP branch adds NO_EXPORT while `send-community none` suppresses code 8. Read from `genericCommunityHandler`: a Suppress as the last Set/Suppress drops the attribute and ignores every Add, so the NO_EXPORT the 4.6 branch adds is lost today for such a destination (RFC 9494 Section 4.6: "The NO_EXPORT community [RFC1997] MUST be attached to the stale routes.") | AC-16 IBGP destination with `send-community none` | Q-7 (owner, 2026-10-14): folded into D5 (AC-27). Round 6: the contract changes for no existing producer; GR's 4.6 NO_EXPORT moves to the new `AttrModAttach`, and every `AttrModAdd` keeps today's behaviour (Data Flow, "Q-7") |
| R-7 | concurrent reload changes which peers Ze sent LLGR to while UPDATEs arrive | race test on the snapshot | copy-on-write snapshot published under the GR mutex, read lock-free |
| R-8 | (B-1) a non-nil `Meta` on rebuilt routes changes what other egress filters and the sent event see; and the relay recovery reads the source's CURRENT "Ze sent LLGR" fact, which differs from the receive-time fact only if the source reconnected under a changed LLGR config and has not yet re-sent the route | a filter test asserting nil `Meta` on a relay; `TestOTCEgressSuppressProviderLearnedWithoutMeta` | grep every `Meta` / `meta[` read in egress filters and `handleSentStructured`: each reads named keys and ignores unknown ones (`resolveSrcRole` still recovers `src-role`, which the map lacks). The reconnect window closes at the source's refresh, which re-runs ingress; stated in `docs/guide/graceful-restart.md` |
| R-9 | (B-1, corrected r3 I-1) the RIB rails' source lookup is `sentSourceCurrent`'s, one per replayed copy: `ParseAddr`, one `bgpPeers` map read, and the retained by-key append into an inline four-path buffer with its release. On peer-up it is the lookup the rail already makes, now returning the level too, so the added cost there is zero; the refresh rails gain one per copy. It must run under `peerMu` | replay benchmark; `-race` on the replay tests | replay and refresh are not the live path; the lookup allocates nothing (inline buffer). Every caller already holds `peerMu` (Current Behavior); a new caller that does not moves the call into its existing locked section rather than taking a second lock |
| R-10 | (I-1) the raw-override guard adds back a value an operator's raw filter removed on purpose | the AC-25 / AC-26 units | intended: Section 4.3's "MUST NOT be removed when the route is further advertised" binds every removal, and Q-3 already applies it "whatever ... a policy Set/Suppress says"; the guide says so |
| R-11 | (r4 N-2, accepted) a source that held LLGR_STALE is re-announced FRESH, and a refresh rail (`collectRibOutRoutes` for `sendRoutes`, `outboundResend`) runs before the new send reaches the destination's copy: the level read by key is now 0, so the old copy, whose bytes still carry LLGR_STALE, can go to an EBGP neighbour without LLGR once | none a unit can order: it needs a refresh landing between the source's re-announcement and its forward | accepted, not mitigated: the window is transient and closes when the pending send replaces the copy with the fresh bytes; skipping a non-current copy instead is refused (Current Behavior, r4 N-2), because the false answer covers cases with no pending send and an omitted copy is purged at EoRR |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | routes kept or dropped wrongly across restarts; stale routes selected or leaked to neighbours; communities leaking past `send-community none` |
| How is it reverted? | single commit revert per phase; no config or wire format migration |
| Who else touches this path? | `plan/immediate/spec-bgp-llgr-per-family-config.md` (same GR plugin, skeleton); `plan/spec-gr-advanced.md` (N bit on top of D1); `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (AC-4 parent, through its BGP child spec-rfc-verdict-fix-bgp); `plan/immediate/spec-bgp-update-propagation-rfc-defects.md` (its D2 edits `filter_community/handler.go::genericCommunityHandler` and `filter_community/filter.go`, the handler D5 and Q-7 change) |
| Shared producer with the rs spec | `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` (Phase 2b, Data Flow 5, Blast Radius "Depends on GR D1") depends on D1 and shares two sites with it: D1's teardown reason producer in `reactor/peer_run.go`, and the no-retention check in `gr.go::handleStructuredState` / `handleStateEvent`. (a) It adds a second input to that producer, `receivePublication.failed`, reported as a second declared close reason `rpc.ReasonAdjRIBInInvalid` beside `rpc.ReasonNotification` (spelling is the rs spec's own Q-6, distinct from this spec's Q-6), and one declared predicate `rpc.ReasonForbidsRetention` in `pkg/plugin/rpc/enums.go`, true for `rpc.ReasonNotification`, `rpc.ReasonAdjRIBInInvalid` and `rpc.ReasonPeerRemoved`, which `gr.go` calls in place of a reason comparison. (b) It moves the down event, and the reason producer with it, from the FSM leave-Established callback to `Peer.runOnce` after `<-deliveryDone`, so a session's buffered UPDATEs reach every plugin before its down event. Whichever spec lands second carries the move: if D1 lands first, rs Phase 2b moves D1's producer; if rs Phase 2b lands first, D1 writes its producer at the `runOnce` site |

## Known Limitations

- RFC 8538 N-bit retention stays in `plan/spec-gr-advanced.md`.
- RFC9494-4.2-7, 4.2-9, 4.2-10 and 5-2 are not in this spec: 5-2 belongs to `plan/immediate/spec-bgp-llgr-per-family-config.md`; the other three belong to `plan/immediate/spec-bgp-llgr-reconnect-timers.md` (Q-5, owner 2026-10-14).

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC 4724
Section 4 at both GR down handlers' NOTIFICATION branch, RFC 9494 Section 4.2 at the
per-family reconnect rule, RFC 9494 Section 4.3 at the ingress filter, the RIB
insert level and the code-8 preservation, RFC 9494 Section 4.3's lead sentence at
the relay recovery in `LLGREgressFilter` (the same lead sentence the ingress filter quotes, since it evaluates the same predicate), a `// RFC 9494 Section 4.3` reference at each RIB-rail call that reads the source level and at `buildRelayUpdate`'s rebuilt key, the RFC 9494 Section 4.3 third bullet at `ingressStripCommunities` and each raw-override guard, the D6 threshold (the scope it binds; RFC 4724 Section 4.2 "during forwarding" is
NOT quoted there, design review r1 I-2), and RFC 4724 Section 4.2's retain sentence
where `PurgeStale` / `MarkStale` leave `IngressStaleLevel` untouched.

## Design Review Log

| Round | Date | Source | Result | Applied |
|-------|------|--------|--------|---------|
| 1 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-gr-design-r1.md` (independent, fresh context) | 1 BLOCKER, 4 ISSUE, 7 NIT | all applied 2026-10-14. B-1: received LLGR_STALE stored as `IngressStaleLevel`, separate from the retention mark; AC-22 to AC-24. I-1: "Discrimination records this spec re-records" table, AC-20 widened to every RFC. I-2: D6 rests on RFC 9494 Section 4.3's scope; the RFC 4724 "forwarding" citation dropped from D6, Q-2 and the RFC Documentation plan. I-3: `handleReceivedStructured` digest and R-3 corrected. I-4: `.ci` steps for AC-11, AC-12, AC-15, AC-19. N-1: Q intro, Q-6 / Q-7 order, state file. N-2: Q-1 counter-text. N-3: D5 producer. N-4: `bmp_events.go` `fsmEventNone` comment in Files to Modify. N-5: unsent NOTIFICATION and hold-timer readings in D1. N-6: ingress step at `FilterStageProtocol`, reads the received community; AC-25. N-7: `TestOnSessionReestablished_DuringLLGR_NoCaps` in the tagged-test and re-record tables, its prose approval raised |
| 2 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-gr-design-r2.md` (independent, fresh context, narrow: the `IngressStaleLevel` fact at its producers, and the N-6 stage choice) | 1 BLOCKER, 2 ISSUE, 5 NIT | all applied 2026-10-14. B-1: every rail into `LLGREgressFilter` named (D4 re-confirmation, Data Flow D4, AC-16): live forward rails by ingress metadata; RIB rails by the higher of the copy's `StaleLevel` and the source entry's `IngressStaleLevel`, no `ribOutEntry` field; relay rails (`RelayStoredRoute`, `relay_recovery.go`, `aigp_readvertise.go`) by a `filterapi.MetaKeyRebuilt` key from `buildRelayUpdate` and the filter's recovery with the ingress predicate. The main thread's byte-reading preference checked at the producers and kept for the relay rails only (Key Design Decision D3/D4, alternative (e)); the false `buildRelayUpdate` comment replaced; `.ci` RIB-replay and relay steps; re-record rows for `buildRelayUpdate` and (if edited) `handleSentStructured`. I-1: A-8 refuted; `ingressStripCommunities` and both raw-override branches exempt preserved values; AC-25 strip and raw rows, AC-26; Key insights, Required Reading and D5 re-confirmation corrected; Q-3 names the new sites; re-record rows for `ingressStripCommunities`, `runIngressPolicyChain`, `runEgressPolicyChainASN4`. I-2: `familyrib_opaque.go` insert branches in Current Behavior, Files to Modify and AC-15 with a VPN/EVPN unit. N-1: entry Name `bgp-gr-llgr-stale-ingress`. N-2: `peerrib.go::InsertEntry` re-record row. N-3: retention-only readers named. N-4: D6 and AC-19 say re-established or refreshed; the AC-19 `.ci` step reconnects the destination. N-5: `insertPoolNLRIs` under A-6 |
| 3 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-gr-design-r3.md` (independent, narrow: the r2 B-1 rail design and the I-1 sites) | 0 BLOCKER, 1 ISSUE, 4 NIT | all applied 2026-10-14. I-1: the RIB rails' source read is `sent_owner.go::sentSourceCurrent`'s by-key lookup, not `sentReceivedOwner` then `PeerRIB.Lookup` (Current Behavior, Data Flow D4, Files to Modify, R-9); one function returns the current answer and the ingress level, so peer-up does one lookup, and the refresh rails call it too; no discrimination record. N-1 (main-thread decision): the level is read from the source's current entry by key, MsgID ignored, MsgID kept only for the current answer; `TestRIBReplayMetaReadsSourceIngressLevel` gains the re-announced-stale refresh-rail case. N-2: the RIB-replay `.ci` step drops the destination's LLGR by `action=rewrite` plus `action=sighup`, then an explicit close, conn=2's OPEN asserted without code 71; the fallback to a RIB unit stated. N-3: the reactor units are the AC-26 proof. N-4: Q-3 widened text, owner confirms at approval. Answer 2: ingress step and relay re-check call one predicate function. Answer 4: the code-8-only exemption is computed in `parseCommunityDefinitions` |
| 4 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-gr-design-r4.md` (independent, narrow: the r3 edits) | 0 BLOCKER, 1 ISSUE, 2 NIT | all applied 2026-10-14 (main-thread decisions). I-1: the `.ci` RIB-replay step is dropped, with the reason (le test peer drops no capability per connection and cannot order a close after a reload); the peer-up rail's proof is `TestPeerUpReplayReportsSourceIngressLevel` driving `collectPeerUpReplay(dest, seenBefore=true)`, the withhold `TestLLGREgressFilter_ReceivedStaleMeta`; Wiring Test, AC-16 row, TDD plan updated, "implementer chooses" removed. N-1: the shared predicate is `gr_ingress.go::llgrStaleFromSource`, in Files to Modify and the TDD plan (`TestLLGRStaleFromSource`). N-2: read at `sentSourceCurrent` and the refresh rails: the current answer cannot gate a refresh (its false covers cases with no pending send, and RFC 7313 Section 4 purges an omitted copy at EoRR), so the window is accepted as R-11 |
| 5 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-gr-design-r5.md` (independent, narrow: the r4 edits) | 0 BLOCKER, 0 ISSUE, 1 NIT | NIT applied by the main thread: the `TestPeerUpReplayReportsSourceIngressLevel` fixture's copy must be current. Design converged; Q-1, Q-2, Q-3 (widened text), Q-5, Q-6, Q-7 stay with the owner |
| owner | 2026-10-14 | Thomas: "LDP, BFD, GR, Prefix-SID, accept" | Q-1, Q-2, Q-3, Q-5, Q-6, Q-7 answered: accept every recommendation | recorded in the Answer column. Q-1 AND, cost accepted (R-4). Q-2 yes, D6 in scope (Behavior to change, Data Flow D6, AC-19, Key Design Decisions). Q-3 unconditional on the widened text. Q-5 separate: skeleton `plan/immediate/spec-bgp-llgr-reconnect-timers.md` created, Known Limitations cites it. Q-6 "notification". Q-7 folded into D5: AC-27, R-6 and phase 6 updated. **Design work owed, round 6 (independent, narrow), before implementation:** the Q-7 mechanism only. It must decide which Adds survive a `send-community` Suppress in `genericCommunityHandler` (only the GR egress filter's Section 4.6 NO_EXPORT, or every egress filter's Add) and how the handler tells them apart, since R-6 says the change alters the handler contract for every producer; then add it to Data Flow D5, Files to Modify, the TDD plan (unit and `.ci` named in AC-27's proof row), the "Discrimination records this spec re-records" table if a tagged producer changes, and the guide. Every other answer only removes a branch; no other design changes |
| 6 | 2026-10-14 | design subagent, narrow: the Q-7 mechanism only, read at the producers (`gr_egress.go`, `filter_community/handler.go` and `egress.go`, `filter_delta.go`, `peer_forward_facts.go`, `reactor_api_forward.go`, `forward_rs.go`, `reactor_api_batch.go`, `policy_dryrun.go`, `forward_local_pref.go`) and RFC 9494 Section 4.6 in `rfc/full/rfc9494.txt` | mechanism decided; one owner question (Q-8); one found defect journalled | Data Flow "Q-7: which Adds survive a Suppress": a sixth action `filterapi.AttrModAttach`, recorded only by GR's Section 4.6 branch, honoured by `genericCommunityHandler` over every Set, Suppress and Remove; every `AttrModAdd` producer (`filter_community` tag, policy `community-add` family, D5's raw-override restore) keeps today's behaviour, listed producer by producer. Applied to D5's Data Flow row, AC-27 and its proof row, Unit Tests, the tagged-test table (two GR units, Q-8), the discrimination table (two new rows; `wireModVerb` holds no record), Functional Tests (fourth destination in `llgr-stale-community-preserved.ci`), Files to Modify, Documentation row 12, Key Design Decisions, phase 6, Security Review, R-6. Found, not caused: an export policy `local-preference` Set overrides the Section 4.6 LOCAL_PREF 0 (row in `plan/journal/conflicting-feature-contracts.md`, Known Limitations). Left for the review: whether the stale readvertise rail's batch attributes already carry `send-community` (Data Flow, Q-7). Owes an independent narrow review of these edits before implementation |
| 7 | 2026-10-14 | independent review, narrow: the round-6 edits; applied by a subagent after reading each finding at the producer (`reactor/filter_ordered.go::runEgressPolicyChainASN4`, `reactor_api_forward.go` step loop and mod application, `forward_rs.go`, `forward_local_pref.go::applyFactsLocalPref`, `reactor_api_batch.go::sendStaleReadvertiseUnit`, `rib/rib_replay.go`, `rib/rib_structured.go::storeSentEntries`) | 0 BLOCKER, 2 ISSUE, 3 NOTE, all applied | I-1: an export policy Set never reaches the shared accumulator (the chain bakes its delta into `wireOverride`, the shared mods apply on top), so the "policy Set loses NO_EXPORT the same way" sentence and Q-8's "or a policy Set" are removed; the `send-community` Suppress is the only live loser and still justifies `AttrModAttach`; `_AttachSurvivesSet` stays as an untagged handler-contract unit and leaves the discrimination table. I-2: the round-6 LOCAL_PREF finding is false for the same reason (GR's code-5 Set applies after the override and wins; `applyFactsLocalPref` only Suppresses toward EBGP): Known Limitations bullet removed, and the row in `plan/journal/conflicting-feature-contracts.md` deleted as a false finding; the stale "a policy chain can set any value" comment in `forward_local_pref.go` gets a row in `plan/journal/comment-describes-superseded-behaviour.md`. N-3: the stale readvertise rail reads the destination's Adj-RIB-Out bytes, already post-`send-community`, not a defect (Data Flow, Q-7). N-4: `spec-bgp-update-propagation-rfc-defects` added to Blast Radius. N-5: a tag step on the `.ci` fourth destination proves AC-27's negative half; `reactor_stale_readvertise_test.go` fakes move to `AttrModAttach` (Files to Modify). Design converged unless the next review finds a product BLOCKER; Q-8 stays with the owner |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
