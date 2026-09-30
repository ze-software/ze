# bmp author handoff (spec-rfc-verdict-fix-bgp, package internal/component/bgp/plugins/bmp)

Author: bmp author agent, 2026-09-28. Nothing stamped, nothing committed. Package test green under -race (`go test -race` over the package, job-unit-pkg-59a9773b.log).

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC7854-x-1 | tests | + TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength (octet 0 == 3 on all 7 writers' output); - held: TestBMPCommonHeaderDecode#2, TestBMPMalformedHeaderDrops | x-1 + (revert WriteCommonHeader) | enforced | |
| RFC7854-x-2 | tests | + same unit: Version 3 (never 0), Message Length == whole message for all 7 types, Type octet; - held: TestBMPCommonHeaderDecode (short buffer) | x-2 + (revert WriteCommonHeader) | enforced | split-table "row correction" (octet widths from figures only): no sentence states them, so nothing to split (R-9) |
| RFC7854-x-3 | tests | + TestRFC7854PerPeerHeaderFollowsTheCommonHeader (RM, Stats, PeerDown, PeerUp, Mirroring: per-peer header decoded at octet 6); - same unit (Initiation, Termination: TLV at octet 6, no room for a per-peer header) | x-3 + (writePeerHeader), x-3 - (writeInitiation) | enforced | split-table rows (§4.3, 4.5, 4.7, 4.8, 4.10 per-type layouts) NOT created: outstanding for a continuation. §4.6 already rowed as RFC7854-x-9. Old TestHasPeerHeader tags kept (hasPeerHeader has no non-test caller) |
| RFC7854-x-4 | tests | + TestRFC7854PeerASIsFourOctetsWithTheShortASZeroPadded (65001 -> 00 00 FD E9, 4200000001 -> FA 56 EA 01, raw octets) | x-4 + (writePeerHeader) | enforced | row keeps {single-polarity: positive}; split-table width note same as x-2 |
| RFC7854-x-16 | tests (tag moved) | + TestRFC7854SessionPeerUpOnlyForEstablishedPeers (one Peer Up for the Established peer, first message); - same unit (OPEN-only peer gets none) | x-16 + (primeSender), x-16 - (recordPeerUp) | enforced | wrong tag removed from TestConcurrentDumpsStayAddressedToTheirOwnCollector (approval recorded) |
| RFC7854-x-17 | tests (tag moved) | + TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp (Adj-RIB-In route, L clear, after the Peer Up); - same unit (non-Established peer's UPDATE not dumped) | x-17 +/- (replayPeerLocked) | enforced | wrong tag removed as above |
| RFC7854-x-18 | tests | + TestRFC7854InitiationCarriesSysName (new), TestRFC7854InitiationCarriesSysDescr; - TestBMPInitiationMissingSysNameEndsSession (tag added to an untagged unit), TestRFC7854InitiationMissingSysDescrEndsSession | x-18 + (sendInitiation), x-18 - (decodeInitiation) | enforced | |
| RFC7854-5-3 | unresolved (design call) | first clause held by existing units | none | weak | ze never emits post-policy Adj-RIB-In (L set, O clear): L set is only the sent direction with O set (peerHeaderFromEvent). The "announced with L both clear and set" state of an Adj-RIB-In route is unreachable, so the "withdraw twice" clause has no input. Options: owner ruling that the clause is out of reach (annotation refused over a tagged row today), or implement post-policy Adj-RIB-In. Needs main thread / owner |
| RFC7854-4.9-1 | defect, not fixed (cross-boundary) | reasons 1, 3, 5 held | none | weak | verified at peerDownFor (bmp_events.go): every close without a NOTIFICATION and not peer-removed becomes reason 2 with fsmEventNone. rpc.StructuredEvent carries only a Reason string, no FSM event number, so the reason-2 event code needs the reactor to publish the event across pkg/plugin/rpc: design choice, crosses components. Reason 4 (the subtest's "connection lost") is plan/immediate/spec-bmp-sflow-export-rfc-defects.md D1; code left alone. No failing test written: no input at the plugin boundary can express the event yet |
| RFC7854-x-10 | blocked | TestHandleSenderStatePeerDown, TestBMPSenderPeerDown | none | weak | the missing cause-to-reason tie is the same peerDownFor producer: reason 4 is spec-bmp-sflow-export-rfc-defects D1 (not listed under "Blocked by"; add it there, P-3). Split-table "1-octet Reason" width: nothing to split (R-9) |
| RFC8671-x-3 | tests | + TestRFC8671PeerUpDoesNotDependOnTheMonitoredRIB (same Peer Up under pre-policy, post-policy, all) | x-3 + (primeSender) | enforced | row keeps {single-polarity: positive}; annotation now names the new unit |
| RFC9069-4.2-1 | tests + row prose | + TestLocallySourcedRouteIsConveyedWithTheLocRIBPeerType (empty AS_PATH route, type 3), TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType; - TestMonitoredPeerRouteMonitoringIsNotTheLocRIBPeerType | 4.2-1 + (locRIBPeerHeader) | enforced | row prose named TestPeerHeaderFromEvent as the negative; corrected to the tagged unit. Judge may still read the negative as a neighbouring rule |
| RFC9069-5.2-1 | tests + split | + TestFabricatedLocRIBOpenCarriesNoCapabilityBeyondASN4AndTheDumpFamilies (any non-ASN4, non-MP capability fails; exactly one ASN4; MP set == dumpFamilies) | 5.2-1 + (fabricateLocRIBOpen) | enforced | split: new row RFC9069-6.1.1-2 (the §6.1.1 sender sentence), extraction site 6.1.1:2 remapped to it |
| RFC9069-6.1.1-2 (new row) | row split | + TestLocRIBPeerUpOpenIndicatesTheAddressFamilies (both OPENs of the wire Peer Up indicate dumpFamilies) | 6.1.1-2 + (fabricateLocRIBOpen) | needs first verdict | {single-polarity: positive}, no negative existed |
| RFC9069-6.1.1-1 | tests | + TestReceiverKeepsEachPeersAddressFamiliesApart (two peers, one router, IPv4 vs IPv6 kept apart), TestReceiverRecordsThePeerUpAddressFamilies; - TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone | 6.1.1-1 + (state.go peerUp) | enforced | |
| RFC9069-6.1.3-1 | tests + row prose | + TestBehaviorChangeBouncesTheLocRIBEmulatedPeer (Peer Type 3 Peer Down then Peer Up), TestBehaviorChangeBouncesThePeersOfALocRIBSession; - TestAConfigurationThatAltersNoBehaviorBouncesNothing | 6.1.3-1 + (sendLocRIBPeerDown) | enforced | row prose named untagged TestRFC8671* units and a stale "reload rails" story; corrected to the tagged units and applySenderConfig |
| RFC9069-x-4 | tests | + TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring (stand-in RIB answers the replay-request; collector reads Peer Up then a type-3 Route Monitoring carrying the prefix) | x-4 + (startLocRIB) | enforced | row keeps {single-polarity: positive}; prose still describes only the trigger test, could name the new unit |

Counts: tests 13 (x-1, x-2, x-3, x-4, x-16, x-17, x-18, 8671-x-3, 9069-4.2-1, 5.2-1, 6.1.1-1, 6.1.3-1, x-4), row split 1 (RFC9069-6.1.1-2), unresolved 1 (5-3), defect not fixed 1 (4.9-1), blocked 1 (x-10).

All records are the `revert` route; every one observed red naming the unit (18 new records).

Approvals recorded (D-15): bmp.TestRFC7854PerPeerHeaderFollowsTheCommonHeader, bmp.TestRFC8671PeerUpDoesNotDependOnTheMonitoredRIB, bmp.TestFabricatedLocRIBOpenCarriesNoCapabilityBeyondASN4AndTheDumpFamilies, bmp.TestLocRIBPeerUpOpenIndicatesTheAddressFamilies, bmp.TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring, bmp.TestBehaviorChangeBouncesTheLocRIBEmulatedPeer, bmp.TestReceiverKeepsEachPeersAddressFamiliesApart, bmp.TestRFC7854SessionPeerUpOnlyForEstablishedPeers, bmp.TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp, bmp.TestConcurrentDumpsStayAddressedToTheirOwnCollector (tag removal).

Gates owed (not run here): `./le rfc reseal` (bmp_reconnect_test.go and rfc7854_codec_test.go carry other verdicts: file sha shifted), `./le rfc check` (new row RFC9069-6.1.1-2: quote check, extraction mapping, count tests in internal/le/rfc may redden), `./le go lint run`, the independent re-judge and audit-stamp mode rejudge for every row above.

Outstanding for a continuation: RFC7854-x-3 split rows for §4.3, §4.5, §4.7, §4.8, §4.10 (tags could go on TestRFC7854PerPeerHeaderFollowsTheCommonHeader); RFC9069-x-4 row prose; 5-3 owner call; 4.9-1/x-10 routing to the bmp-sflow spec.

## Files changed

- internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go (new)
- internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go (new)
- internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go (new)
- internal/component/bgp/plugins/bmp/rfc7854_codec_test.go (x-18 negative tag added)
- internal/component/bgp/plugins/bmp/bmp_reconnect_test.go (wrong x-16/x-17 tags removed)
- rfc/short/rfc9069.md (new row RFC9069-6.1.1-2; prose of 4.2-1, 6.1.1-1, 6.1.3-1)
- rfc/short/rfc8671.md (RFC8671-x-3 annotation names the new unit)
- rfc/extraction/rfc9069.json (site 6.1.1:2 mapped to RFC9069-6.1.1-2)
- rfc/discrimination/rfc7854.json, rfc/discrimination/rfc8671.json, rfc/discrimination/rfc9069.json (records, written by discriminate-record)

# Continuation 2026-09-29 (bmp author, second pass)

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC9069-5.2-1 | defect fixed (D-8) + tests | + TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop (new: encodes IPv4 /24 via 2001:db8::1, reads MP_REACH AFI 1 NH len 16, requires ENH (1,1,2) in the OPEN; observed red before the fix), TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses (renamed from ...NoCapabilityBeyondASN4AndTheDumpFamilies; now also requires exactly one ENH naming only (1,1,2)) | 5.2-1 + x2 (revert fabricateLocRIBOpen); 6.1.1-2 + re-recorded (producer changed); stale record of the old unit name removed | enforced | fix: locRIBExtendedNextHop declaration + ENH capability in fabricateLocRIBOpen with RFC 9069 5.2 and RFC 8950 4 quotes; docs/guide/bmp.md updated. Verified producer: rib entryNextHopAddr answers an IPv4 unicast next hop from MP_REACH when no legacy NEXT_HOP, so an IPv6 next hop reaches buildLocRIBUpdateBody |
| RFC8671-x-3 | tests | + TestRFC8671PeerDownDoesNotDependOnTheMonitoredRIB (new: same down event under pre-policy, post-policy, all; same Peer Down header, reason, data), TestRFC8671PeerUpDoesNotDependOnTheMonitoredRIB, TestBMPPeerUpRoundTrip | x-3 + (revert handleSenderState) | enforced | row prose names the Peer Down unit |
| RFC9069-4.2-1 | tests (R1 genuine negative) | + TestLocallySourcedRouteIsConveyedWithTheLocRIBPeerType, TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType; - TestLocallySourcedRouteIsNotConveyedUnderTheMonitoredPeer (new, R1(b): locally sourced route whose next hop is the monitored peer's address; Peer Up and RM still type 3, zero address, own AS/BGP ID) | 4.2-1 - (revert locRIBPeerHeader); stale record of the old negative removed | enforced | old negative TestMonitoredPeerRouteMonitoringIsNotTheLocRIBPeerType: tag REMOVED (approval), not moved: no row in rfc7854/8671/9069 states "a monitored peer's route is not type 3" (grep of rfc7854.md rows found none). Test kept, comment says so |
| RFC7854-x-10 | blocked (moved) | - | none | weak | AC-4 appended to plan/immediate/spec-bmp-sflow-export-rfc-defects.md (continuation 2); Blocked-by row added to plan/pre-release/spec-rfc-verdict-fix-bgp.md (continuation 3) |

Continuation 2 files: internal/component/bgp/plugins/bmp/{bmp_locrib.go,rfc9069_verdict_test.go,rfc9069_test.go}, docs/guide/bmp.md, rfc/short/{rfc9069.md,rfc8671.md}, rfc/discrimination/{rfc9069.json,rfc8671.json}, plan/immediate/spec-bmp-sflow-export-rfc-defects.md. Continuation 2 owed: full bmp package test -race, reseal + independent rejudge of 5.2-1, 6.1.1-2, 8671-x-3, 9069-4.2-1.

# Continuation 3 2026-09-29 (bmp author, third pass)

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC7854-5-3 | row retired (R2) | - | stale 5-3 records removed by hand | retired | Retired 2026-09-29 paragraph in rfc/corrections/rfc7854.md: binds a router monitoring both pre- and post-policy Adj-RIB-In for one peer; peerHeaderFromEvent (bmp_events.go) sets L only with O on the sent direction, so the twice-sent withdraw has no input. The rfc/audit/rfc7854.json 5-3 entry was NOT touched (P-2): the judge decides how the retired id leaves the audit |
| RFC7854-5-4 (new row) | row split + tags moved | + TestRFC7854LFlagFollowsPolicy; - TestRFC7854LFlagNotDecidedByBody (both approved D-15) | 5-4 + and 5-4 - (revert peerHeaderFromEvent), both observed red | needs first verdict | quote: first clause of the §5 sentence, verbatim. Extraction site 5:4 remapped to 5-4 (its quote still carries the whole sentence) |
| RFC7854-x-3 | split (R5) | x-3 keeps its §4.2 row and tags; five per-type layout rows added below | - | unchanged | |
| RFC7854-4.3-2 (new row) | row split + tests | + TestRFC7854InitiationIsTheCommonHeaderThenTwoOrMoreTLVs (raw octets from sendInitiation, TLVs hand-walked from octet 6, >= 2); - TestRFC7854InitiationWithFewerThanTwoTLVsIsRefused (receiver refuses a one-TLV Initiation, R1(a)) | + (revert sender.go::sendInitiation), - (revert decodeInitiation), both observed red | needs first verdict | [MUST] under D-3/R5 (no keyword, format sentence); extraction 4.3 unsourced-ids |
| RFC7854-4.5-4 (new row) | row split + tests | + TestRFC7854TerminationIsTheCommonHeaderThenTLVs (raw octets from sendTermination, TLVs from octet 6, exactly one Reason); - TestRFC7854TerminationWithNoTLVIsRefused | + (writeTerminationLocked), - (decodeTermination) | needs first verdict | quote stops before "as follows:"; extraction 4.5 unsourced-ids |
| RFC7854-4.7-3 (new row) | row split + tests | + TestRFC7854RouteMirroringIsThePerPeerHeaderThenTLVs (writer octets: per-peer header at 6, TLVs from 48 to the end, in order); - TestRFC7854RouteMirroringWhoseBodyIsNotTLVsIsRefused (3-octet non-TLV tail refused) | + (writeRouteMirroring), - (decodeRouteMirroring) | needs first verdict | extraction 4.7 unsourced-ids |
| RFC7854-4.8-3 (new row) | row split + tests | + TestRFC7854StatsReportCountNamesTheCountersThatFollow (count at 48 == 3, three TLVs to the end); - TestRFC7854StatsReportCountDisagreeingWithItsCountersIsRefused (count +1 and -1 both refused) | + (writeStatisticsReport), - (decodeStatisticsReport) | needs first verdict | extraction 4.8 unsourced-ids |
| RFC7854-4.10-1 (new row) | row split + tests | + TestRFC7854PeerUpIsThePerPeerHeaderThenTheFixedFieldsAndBothOPENs (per-peer header, Local Address, ports, Sent OPEN, Received OPEN at their offsets, exact length); - TestRFC7854PeerUpShorterThanItsFixedFieldsIsRefused | + (writePeerUp), - (decodePeerUp) | needs first verdict | quote "Following the common BMP header and per-peer header is the following" points at the §4.10 figure: the only sentence §4.10 has for the layout; a judge may call it a list pointer. Positives use the writers (the sender's own writers), except Initiation and Termination which go through the sender |
| RFC9069-x-4 | row prose | + TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring, TestStartLocRIBTriggersInitialDump | none (claims unchanged) | enforced | marker prose rewritten with no parentheses, names both units; cite now (§5.4, Route Monitoring) |

All 12 records of this pass: route revert, every one observed red. Approvals (D-15): bmp.TestRFC7854LFlagFollowsPolicy, bmp.TestRFC7854LFlagNotDecidedByBody, bmp.TestRFC7854TerminationIsTheCommonHeaderThenTLVs (edit of a new unit).
Tests: the ten new units pass (`go test -run 'TestRFC7854(Initiation|Termination|RouteMirroring|StatsReport|PeerUp)'`, 27 PASS lines incl. neighbours). Full package run under -race still owed.

Files changed (continuation 3):
- plan/pre-release/spec-rfc-verdict-fix-bgp.md (Blocked-by row RFC7854-x-10)
- internal/component/bgp/plugins/bmp/rfc7854_test.go (5-3 tags -> 5-4)
- internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go (new)
- rfc/short/rfc7854.md (5-3 -> 5-4; new rows 4.3-2, 4.5-4, 4.7-3, 4.8-3, 4.10-1)
- rfc/short/rfc9069.md (x-4 prose)
- rfc/corrections/rfc7854.md (Retired 2026-09-29 RFC7854-5-3)
- rfc/extraction/rfc7854.json (site 5:4 -> 5-4; unsourced-ids for the five new rows)
- rfc/discrimination/rfc7854.json (12 records; two stale 5-3 records removed by hand)

RFC7854-4.9-1 (R4): NOT STARTED, deliberately. The path is wider than one continuation can finish while keeping the tree compiling: fsm.StateCallback func(from, to State) (fsm/fsm.go) carries no event and has 6 callers (peer_run.go plus 5 tests in fsm and reactor); the close reason crosses reactor.notifyPeerClosed -> the OnPeerClosed(peer, reason string) observer interface, which internal/component/plugin/registry/interfaces.go, internal/plugins/mrt/register.go and 3 test files also implement; then server EventDispatcher.OnPeerStateChange -> rpc.StructuredEvent (pkg/plugin/rpc/bridge.go, only Reason string), whose JSON transport to an external bmp plugin must carry the new field too, or the field is a cross-boundary no-op. peer.go and reactor_api_batch.go are NOT on the path (callback lives in peer_run.go). Recommended design for the next author: (1) fsm: map Event to its RFC 4271 Section 8.1 number with a method `RFC4271Number() uint16` on fsm.Event (the iota order is not the RFC order) and record the event that caused the last transition, read through `(*FSM).LastEvent()` inside the callback, which avoids changing StateCallback; (2) peer_run.go: pass that number to notifyPeerClosed alongside the reason; (3) replace `reason string` in OnPeerClosed/OnPeerStateChange by a small value struct {Reason string; FSMEvent uint16} and add `FSMEvent uint16` to rpc.StructuredEvent plus its JSON encoding; (4) bmp peerDownFor: reason 2 writes the 2-octet event. Reason 2 vs 4 (D1 of spec-bmp-sflow-export-rfc-defects): Event 2 ManualStop and Event 8 AutomaticStop without NOTIFICATION are reason 2 with that event; Event 18 TcpConnectionFails, and a peer close without NOTIFICATION, are reason 4 with no data. Implementing (4) therefore lands D1 in the same change: the two specs must agree which one commits it. Failing test to write first: bmp peerDownFor for a ManualStop close expects Data 00 02 (today 00 00).

Gates owed (main thread): full bmp package test -race (RUN at the end of continuation 3: ok, 16.5 s), `./le rfc check` (new rows: quote check, extraction, retired 5-3, audit entry for 5-3 left in rfc/audit/rfc7854.json), `./le go lint run`, reseal + independent rejudge.

# rfc7854 extraction re-walk 2026-09-29 (bmp author, fourth pass)

The five keyword-less layout rows raised the gated count to 37 against 33 capitalized sites, so the register derives `prose`. Re-walked every site of the prose skeleton against rfc/full/rfc7854.txt and moved it to rfc/extraction/rfc7854.json (register prose, signed-off 2026-09-29, resign-reason set: exclusions 10 -> 14). `./le rfc check` after the walk: no extraction violation for rfc7854 and no quote refusal for the new rows.

| site | decision |
|------|----------|
| front:1 | excluded not-a-requirement (IETF Trust license boilerplate) |
| 4.5:1 | mapped RFC7854-4.5-5 (new row) |
| 4.5:2, 4.5:3, 4.5:4 | mapped 4.5-1, 4.5-2, 4.5-3 (ids shifted by one under prose) |
| 4.8:2 | excluded cross-document (RFC 2856 Section 4 64-bit Gauge definition) |
| 4.8:3 | mapped 4.8-2 |
| 5:4 | mapped RFC7854-5-6 (new row) |
| 5:5 | mapped 5-4 |
| 5:6 | excluded not-a-requirement (ordinary-English "required"; lowercase "should attempt promptly" relaxed by the same paragraph) |
| 8.2:1 | excluded not-a-requirement ("Some consideration is required") |
| 8.2:2 | mapped 8.2-1 |

| id | resolution | what proves it (+/-) | records written | expected verdict | notes |
|----|-----------|----------------------|-----------------|------------------|-------|
| RFC7854-4.5-5 (new) | row + tests | + TestBMPReceiverEndsTheSessionWhenTheRouterLeavesWithoutTermination (new: Initiation + Peer Up, router closes with no Termination; session ends, router and peer dropped); + TestSenderStopSendsTerminationToCollector (RECOMMENDED clause, tag added, approval D-15) | + (revert state.go::removeRouter), + (revert sender.go::writeTerminationLocked), both observed red | needs first verdict | {single-polarity: positive}: the sentence permits the no-message close, so no non-compliant input exists. Side effect: RFC7854-x-11 audit verdict is now STALE (its unit's comment changed); owes a rejudge |
| RFC7854-5-6 (new) | row + test | + TestRFC7854ChangedAttributesCanReturnToEarlierValue (was untagged; now asserts every one of the three RMs carries its ORIGIN, not only the last) | + (revert bmp_events.go::handleSenderUpdate), observed red | needs first verdict | {single-polarity: positive} |
| RFC7854-5-5 {gap} (R3) | NOT added, by rule | - | none | - | The withdraw-twice clause is conditional on a route announced with L both clear and set, which only the optional post-policy Adj-RIB-In view produces (§5 "A BMP speaker may send pre-policy routes, post-policy routes, or both"); Ze's post-policy choice is Adj-RIB-Out (ze-bmp-conf.yang). The condition never holds, so a {gap} would claim a failure Ze does not have; ai/rules/rfc-compliance.md excludes an obligation conditional on an absent optional feature from the count and discloses the feature on the Support rows. Done: Support remaining names the absent view; the Retired 5-3 paragraph says why no gap row. If the main thread still wants the row: id RFC7854-5-5, quote "if the route in question was previously announced with L flag both clear and set, the withdraw MUST similarly be sent twice, with L flag clear and set.", section 5 unsourced-ids |
| RFC8671-x-3 (D-15 cite) | tag prose fixed | TestBMPPeerUpRoundTrip: "bmp.go:757-772" -> "recordPeerUp (bmp_events.go)" (approval D-15) | + (revert msg.go::writePeerUp), observed red (first record for this unit) | rejudge | audit verdict STALE (claim prose changed) |

Tests: the four touched units pass under -race (job-unit-pkg-a3e0ab71.log).
rfc check leftovers on these stems (judge actions, not run here): STALE x-11 and RFC8671-x-3 (rejudge), SHIFTED 3.3-1 and 5-2 (reseal).

Files changed (fourth pass):
- rfc/extraction/rfc7854.json (re-walk under prose)
- rfc/short/rfc7854.md (rows 4.5-5, 5-6; Support remaining)
- rfc/corrections/rfc7854.md (Retired 5-3 paragraph: site id 5:5, why no gap row)
- internal/component/bgp/plugins/bmp/termination_absent_test.go (new unit)
- internal/component/bgp/plugins/bmp/rfc7854_replay_test.go (tag + per-message assertion)
- internal/component/bgp/plugins/bmp/sender_queue_test.go (4.5-5 tag)
- internal/component/bgp/plugins/bmp/msg_test.go (cite fix)
- rfc/discrimination/rfc7854.json, rfc/discrimination/rfc8671.json (4 records)
- rfc approvals for bmp.TestSenderStopSendsTerminationToCollector and bmp.TestBMPPeerUpRoundTrip (written by ./le rfc approve)
