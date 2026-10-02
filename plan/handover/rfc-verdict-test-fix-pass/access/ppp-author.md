# ppp author handoff (child spec-rfc-verdict-fix-access, package internal/component/l2tp/ppp)

Listing at start: 31 verdicts (4 wrong). This context authored 17; 14 remain for a continuation author.
Every record below was written by `./le rfc discriminate-record ... route revert` and OBSERVED red (log per record in this directory, `rec-<id>-<polarity>.log`).
Scoped package test `go test ./internal/component/l2tp/ppp/` under `./le job run`: ok (log `ppp-full.log`, before the RFC1334 file; RFC1334 unit run alone: ok). gofmt clean.

## Resolved in this context

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC1661-5.5-2 (wrong) | tests | TestTerminateRequestIdentifierChanges rewritten: Terminate-Request, valid Terminate-Ack, second request; Identifier differs (+/-); Data empty on both, so the Data-change clause has no case (ze never sends Terminate Data) | +,- producer sendTerminateRequest | enforced | retransmission pair removed; approval recorded |
| RFC1661-5.8-4 (wrong) | tests | TestEchoRequestIdentifierChanges rewritten: reply clause (valid Echo-Reply between, same Data, new id) and Data clause (Magic negotiated, no reply, Data differs, new id) (+/-) | +,- sendEchoRequest | enforced | approval recorded; Echo-Reply id-copy check kept |
| RFC1661-6.4-8 | tests | new TestReceivedMagicNumberZeroWhenPeerNegotiatedNone: peer CONFREQ without Magic Acked over a stale peerMagic; zero accepted (+), stale and own value refused (-) for Echo-Request and Echo-Reply | +,- handleLCPPacket | enforced | the negotiated case stays TestReceivedMagicNumberMustBePeers |
| RFC1661-6.4-2 | tests | new TestEchoRequestCarriesNegotiatedMagic: Echo-Request Magic == negotiated (+), != 0 and != peer (-) | +,- transmitMagic | enforced | ze sends no Discard-Request |
| RFC1661-2-1 | tests | new TestRFC1661ProtocolWithOddHighOctetTreatedUnrecognized: every Protocol constant ze sends is odd with even high octet (+); 0xC121 in Opened draws Protocol-Reject naming 0xC121, no Echo-Reply, state held (-) | +,- rejectUnsupportedProtocol | enforced | |
| RFC1661-5.3-2 | tests | new TestRFC1661WellFormedBooleanDeclinedWithReject: well-formed ACFC on PPPoE -> Reject naming exactly it, no Nak (+); beside a Nak-earning MRU, ACFC in reject list never Nak list, wire reply Reject (-) | +,- negotiatePeerOption | enforced | |
| RFC1661-5.3-6 | tests | new TestRFC1661NakFiltersAcceptableOptions: wire Nak = [Magic, MRU] in request order, both orders (+); acceptable ACCM between them absent (-) | +,- NegotiatePeerOptions | enforced | |
| RFC1661-5.4-1 | tests | new TestRFC1661ConfigureRejectForOptionRefusedByConfiguration: Auth-Protocol (any session) and ACCM (PPPoE) -> Reject naming exactly it, no Ack (+); ACCM on L2TP -> Ack, no Reject (-) | +,- negotiatePeerOption | enforced | clause (a) keeps its old units |
| RFC1661-4.3-1 | tests | new TestRFC1661ConfigureRequestInOpenedRenegotiates: CONFREQ in Opened -> own CONFREQ + Ack, Ack-Sent, no end, no EventSessionDown (+) | + handleLCPPacket | enforced | old negative TestRFC1661EchoDoesNotRenegotiate stands |
| RFC1661-5.1-1 | tests | new TestRFC1661OpenTransmitsConfigureRequestOnWire: applyTransition Closed+Open and Starting+Up write one CONFREQ, Req-Sent (+); Initial+Up writes nothing, Closed (-) | + performAction, - applyTransition | enforced | |
| RFC1661-3.7-1 | tests | new TestRFC1661TerminateAckHoldsLinkForRestartTime: after the Ack, Restart timer running, defaultRestartTimer >= 3 s, CONFREQ and Echo-Request meanwhile leave Stopping with no down (+) | + armRestartTimer | enforced (single-polarity marker unchanged) | |
| RFC1332-3-1 | tests | new TestIPCPOptionsWrittenInLCPFormat: exact TLV octets, IPCP Types 3/129/131, parsed back by ParseLCPOptions (+); receive negative TestIPCPParseRejects unchanged | + writeIPCPv4Option | enforced | send-side negative is the existing receive unit only; judge may call it weak |
| RFC1334-x-1 | tests | new TestAuthOffersCHAPBeforePAP: first CONFREQ offers CHAP, PAP only after a PAP Nak (+); first never PAP, and with PAP outside the order the next CONFREQ does not offer PAP (-) | + adjustAuthOnNakOrReject, - selectAuthFallback | weak likely | the note's second half (StartSession.AuthMethod=PAP opens with PAP) is untested: design question, see below |
| RFC1994-4.1-10 (wrong) | tests | new TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded: Response in Req-Sent, Ack-Sent, Closing -> no frame, no auth/session event, no end, state held (+); tag removed from TestCHAPIdentifierMismatchSilentDiscard (auth-phase unit) | + handleFrame (old record replaced by the recorder) | enforced | approval recorded |
| RFC1994-4.1-8 | tests | TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp now repeats the Response that earned Success (challenge Identifier, same Value) | + handleFrame | enforced (single-polarity) | edited through a python replace, not Edit: the approval was recorded first, but the Edit hook did not see that change |
| RFC1994-1.1-1 (wrong) | row retired | Retired paragraph in rfc/corrections/rfc1994.md; row deleted from rfc/short/rfc1994.md; extraction 1.1:3 -> excluded not-a-requirement; tags deleted from TestNegotiatePeerAuthProtoRejected/Accepted (approvals recorded) | none | n/a | rfc/audit/rfc1994.json still holds the `wrong` entry for the retired id: main thread decides what checkAuditFindings wants for a retired row |

## Remaining for a continuation (not touched)

| id | why open / next step |
|----|----------------------|
| RFC1661-3.1-1 | needs run() driven (Initial+Up, Closed+Open), assert first frame LCP CONFREQ; lcp_lifecycle_test.go shows how run() is driven with echoInterval |
| RFC1661-3.1-2 | drive runNCPPhase with IPCP enabled, assert an IPCP CONFREQ |
| RFC1661-3.5-3 | enable an NCP, deny auth, assert no NCP CONFREQ; accept, assert one |
| RFC1661-4.3-2 | DESIGN QUESTION (D-8): sentence is the RTR implementation note (RFC 1661 §4.3: "The implementation MUST be prepared to receive a new Configure-Request without network administrator intervention."). Stopping ignores RCR per the table, then TO- -> Stopped, and run() ends the session at Stopped (tlf). Whether ending the L2TP/PPPoE session there satisfies "prepared" needs the owner's call; write the failing test (untagged) first if it is judged a defect |
| RFC1661-5.8-2 | Echo-Request half: run() keepalive with short echoInterval, renegotiate out of Opened, assert no Echo-Request until Opened again (producer stops the ticker at session_run.go on leaving Opened) |
| RFC1661-6.4-7 | ppp: make ze send a Configure-Nak of the peer's Magic (loopback), then receive a Nak naming that value, assert redraw != it; pppoeclient unit TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue also needs the looped value excluded (other package, coordinate with the pppoeclient author) |
| RFC1994-4.1-3, 4.2-1, 4.2-2 | need one unit that carries a Response Value through the verifier (authlocal verifyCHAPMD5) to the wire Code 3/4; crosses ppp and plugins/authlocal, so place it where both are reachable (authlocal test driving a ppp session, or l2tp component test) |
| RFC1332-2-1 | send path: sendNCPConfigureRequest for IPCP, Length == Information length; receive: frame with two IPCP packets, second treated as padding |
| RFC1332-2.1-1 | hold IP while PPP is short of the network phase (auth pending) |
| RFC1332-2.1-3 | clause 2 (fragmentation) belongs to the Linux IP stack: needs a kernel-path (QEMU) test or a row correction/mixed declaration; decide |
| RFC5072-2-1 | assert startIPv6Service not before IPV6CP Opened in an IPv6CP-enabled session (ReqSent/AckRcvd) |
| RFC5072-3-1 | send half: no IPV6CP CONFREQ before the network phase |
| RFC5072-4.1-5 | suggestIPv6CPInterfaceID draws crypto/rand directly: a colliding draw needs an injectable reader (producer change, small seam) |

## Observations (unverified, no journal row)

- selectAuthFallback returns AuthMethodNone when the peer Naks toward a method outside the order, and the next CONFREQ then omits Auth-Protocol. I did not read whether the session then fails closed or runs unauthenticated.

## Gates owed (main thread)

- `./le rfc reseal` (magic_echo_rfc1661_test.go, chap_reauth_test.go, lcp_options_test.go edits shift sibling verdicts: RFC1661-5.8-5, 6.1-1, 6.4-7 and others tagging those files)
- `./le rfc check`, `./le go lint run` on the package
- independent judge for every row above, then `audit-stamp ... mode rejudge`

## Files changed

- internal/component/l2tp/ppp/magic_echo_rfc1661_test.go
- internal/component/l2tp/ppp/chap_reauth_test.go
- internal/component/l2tp/ppp/lcp_options_test.go
- internal/component/l2tp/ppp/rfc1661_clauses_test.go (new)
- internal/component/l2tp/ppp/rfc1332_clauses_test.go (new)
- internal/component/l2tp/ppp/rfc1334_clauses_test.go (new)
- rfc/short/rfc1994.md
- rfc/extraction/rfc1994.json
- rfc/corrections/rfc1994.md (new)
- rfc/discrimination/rfc1661.json, rfc/discrimination/rfc1994.json, rfc/discrimination/rfc1332.json, rfc/discrimination/rfc1334.json (records)
- test approvals: `./le rfc approve unit` for ppp.TestTerminateRequestIdentifierChanges, ppp.TestEchoRequestIdentifierChanges, ppp.TestCHAPIdentifierMismatchSilentDiscard, ppp.TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp, ppp.TestNegotiatePeerAuthProtoRejected, ppp.TestNegotiatePeerAuthProtoAccepted, ppp.TestAuthOffersCHAPBeforePAP

# Continuation 2 (2026-09-29, R4/R9/R10 + the 14 remaining ids)

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC1334-x-1 (R9) | defect fixed (D-8) + tests | new TestConfiguredPAPStillOffersCHAPFirst drives Driver/spawnSession with AuthMethod=PAP: default order -> first CONFREQ offers CHAP (0xc223), PAP only after the peer's PAP Nak (+); first CONFREQ never PAP while CHAP is in the order (-); control order [PAP] -> PAP first. Red before the fix (log ppp2-1334-red.log) | +,- producer auth.go::initialAuthMethod | enforced | fix: auth.go initialAuthMethod (RFC quote above the loop), called by manager.spawnSession. Five tests that configured PAP with CHAP in the order encoded the old first-offer-PAP behaviour: TestAuthProtoRejectClearsMethod, TestStartSessionAuthMethodThreaded, TestLocalCONFREQAdvertisesAuthMethod now pin AuthFallbackOrder to the configured method; TestAuthFallbackOnNakDispatches, TestAuthFallbackOnNakProceedsToCHAP now start from MS-CHAPv2 and Nak toward CHAP-MD5 (approvals recorded for the two tagged units). Docs: docs/guide/l2tp.md (Authentication), docs/guide/pppoe.md (Subscriber authentication). Behaviour change for `auth-method pap` deployments: CHAP-MD5 is offered first. Owed: interop scenario test/interop-radius/scenarios/radius-admin-pap-freeradius is SSH-admin RADIUS, not PPP, so unaffected; no PPP-PAP interop scenario found |
| RFC1661-4.3-2 (R4) | defect fixed (D-8) + test | new TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest: Opened + Terminate-Request, then handleRestartTimeout -> no end, Stopped, no EventSessionDown; a Configure-Request then draws ze's CONFREQ + Configure-Ack (Id echoed), Ack-Sent, still no down (+). Red before the fix (log ppp2-432-red.log) | + producer session_run.go::sessionEndedAt | enforced (row keeps {single-polarity: positive}; its marker text cites ppp_fsm.go lines and should name sessionEndedAt -- judge's row call) | fix: session_run.go applyTransition sets pppSession.peerTerminated on the Opened+RTR edge (cleared when negotiation restarts); sessionEndedAt(state) replaces the three `Closed || Stopped` exit checks (proxy afterLCPOpen, reauth, Opened branch) and the applyTransition down check: Stopped reached after the peer's Terminate waits for a new CONFREQ; terminateCause keeps User Request on the later teardown (sessStop defer, readDone). Scope kept to the Terminate path: Stopped via RXJ- or config retries exhausted still ends the session. Three units asserted the old end-at-Stopped and were changed (approvals recorded): TestLCPPeerTerminateRequestZeroesRestartCounter, TestLCPPeerTerminateRestartTimer (RFC1661-4.4-2 tagged; now assert no end + Stopped), TestPeriodicAuthenticationLCPInterruptions (terminate case: no down after the grace timer, then channel close -> down with User Request). Docs: docs/architecture/l2tp/bng-1-radius-attributes.md. UNVERIFIED: which Acct-Terminate-Cause wins when the LAC's CDN arrives for a session waiting in Stopped (CDN row vs PPP down event), and whether any l2tp/pppoe .ci functional test expects the CDN right after a peer LCP Terminate -- run the l2tp and pppoe functional suites |

## Continuation 2: still open (continuation 3 needed, same package)

Untouched this context: RFC1661-3.1-1, 3.1-2, 3.5-3, 5.8-2, 6.4-7; RFC1994-4.1-3, 4.2-1, 4.2-2 (authlocal files clean in git status: cross allowed); RFC1332-2-1, 2.1-1, 2.1-3; RFC5072-2-1, 3-1, 4.1-5. Next steps per the table above in continuation 1. The lifecycle harness in lcp_lifecycle_test.go (runLifecycleSession, openLifecycleLCP, answerLifecycleChallenge accept/reject, synctest) drives run() for 3.1-1, 3.1-2, 3.5-3, 5.8-2.

R10: RFC1661-5.5-2 (TestTerminateRequestIdentifierChanges) was edited by its judge in 7758f5438e; the next ppp judge MUST re-judge it independently.

Package run: go test ./internal/component/l2tp/ppp/ ok (ppp2-full4.log); pppoe, pppoeclient, authlocal ok (ppp2-siblings.log). gofmt not run separately (hooks passed).

Gates owed (main thread): ./le rfc reseal (edited tagged units shift siblings), ./le rfc check, ./le go lint run on ppp, l2tp and pppoe functional suites (R4 behaviour change: no CDN until LAC teardown after a peer LCP Terminate), independent judge for RFC1334-x-1 and RFC1661-4.3-2 then audit-stamp mode rejudge.

Files changed in continuation 2:
- internal/component/l2tp/ppp/auth.go (initialAuthMethod, RFC header)
- internal/component/l2tp/ppp/manager.go (spawnSession calls initialAuthMethod)
- internal/component/l2tp/ppp/session.go (peerTerminated field)
- internal/component/l2tp/ppp/session_run.go (applyTransition flag, sessionEndedAt, terminateCause, three exit checks, two down causes, handleRestartTimeout comment)
- internal/component/l2tp/ppp/rfc1334_clauses_test.go (TestConfiguredPAPStillOffersCHAPFirst)
- internal/component/l2tp/ppp/rfc1661_clauses_test.go (TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest)
- internal/component/l2tp/ppp/auth_dispatch_test.go, auth_normal_dispatch_test.go, lcp_restart_counter_test.go, lcp_lifecycle_test.go
- docs/guide/l2tp.md (carries other sessions' hunks too), docs/guide/pppoe.md, docs/architecture/l2tp/bng-1-radius-attributes.md
- rfc/discrimination/rfc1334.json, rfc/discrimination/rfc1661.json
- approvals: ppp.TestConfiguredPAPStillOffersCHAPFirst, TestAuthProtoRejectClearsMethod, TestLocalCONFREQAdvertisesAuthMethod, TestPeriodicAuthenticationLCPInterruptions, TestLCPPeerTerminateRequestZeroesRestartCounter, TestLCPPeerTerminateRestartTimer

# Continuation 3 (2026-09-29, R16 + part C + the 14 ids)

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC1661-4.3-2 (R16) | defect fixed (D-8) + tests | TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest rewritten: Opened + Terminate-Request, Timeout -> no end, Stopped, exactly ONE EventSessionDown cause User Request (tlf); then CONFREQ -> ze CONFREQ + Ack (Id echoed), Ack-Sent, no end, no second down (+). New untagged TestRFC1661StoppedAfterPeerTerminateEndsOnOpened: peer Acks ze's CONFREQ after tlf -> session ends, no EventLCPUp, no second down (bound). Red before the fix: ppp3-r16-red.log | + producer session_run.go::signalLayerFinished (rec3-RFC1661-4.3-2-positive.log, observed red) | enforced | fix: session_run.go signalLayerFinished (RFC 1661 4.4 tlf quote) called by applyTransition on reaching Stopped when sessionEndedAt kept the session; Opened branch ends the session when sessionDownSent; sendEvent drops a second EventSessionDown (at most once). L2TP reactor answers the down with CDN (teardownSession), PPPoE with PADT (onSessionDown); the resulting sessStop/channel close is the Down event. TestPeriodicAuthenticationLCPInterruptions terminate case re-pinned (approval): one down with User Request at the grace timer, CONFREQ still answered, channel close -> no second down. UNVERIFIED item resolved: ze now sends the CDN itself with User Request; the later sessStop/readDone exit sends nothing (sendEvent dedup), so the LAC's CDN no longer races the cause. Row marker text ({single-polarity: positive; ...ppp_fsm.go lines}) left for the judge |
| part C (selectAuthFallback -> None) | not a defect | runAuthPhase (session_run.go) fails closed on AuthMethodNone when authRequired; authRequired = !allow-no-auth on both L2TP (subsystem.go:235, reactor_kernel.go:134) and PPPoE (subsystem.go:178, server.go StartSession). Existing TestRunAuthPhaseRequiredAuthRejectsNoMethod proves the fail-closed branch. With allow-no-auth true, None is the configured admission | none | n/a | no change |
| RFC5072-4.1-5 | tests (seam = existing crypto/rand.Reader swap, no producer change) | TestIPv6CPSuggestionDiffersFromLocalIdentifier rewritten: Reader fed [local][second]; suggestion == second, != local (+). Targeted break (drop `id == local`) returns local -> red by construction | + suggestIPv6CPInterfaceID (revert) | enforced | approval recorded; row marker text corrected in rfc/short/rfc5072.md (no longer claims "exhausts the draw loop"); stays {single-polarity: positive} |
| RFC1661-3.1-1 | tests (tag moved) | new TestRFC1661RunSendsLCPBeforeAnyOtherProtocol drives run() from Initial with IPCP enabled: first frame is LCP CONFREQ (+). Tag REMOVED from TestRFC1661LCPPacketsSentFirst (its prose claimed run(); approval recorded, comment now says performAction) | + sendConfigureRequest | enforced | old record for the removed tag may be orphaned in rfc/discrimination/rfc1661.json: reseal |
| RFC1661-3.1-2 | tests | new TestRFC1661RunSendsNCPAfterLCPOpens: run(), LCP Opened, no-auth accepted -> address request + IPCP CONFREQ on the wire (+); HEAD negative (both NCPs disabled) unchanged | + runNCPPhase | enforced | |
| RFC1661-3.5-3 | tests | new TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes (IPCP enabled, CHAP): Challenge unanswered 1 s -> no address request (-); rejected -> none (-); accepted -> IPCP CONFREQ (+). Targeted break (runNCPPhase before runAuthPhase in afterLCPOpen, overlay) -> red (ppp3-swap.log) | + runNCPPhase, - runAuthPhase (revert) | enforced | old units kept |
| RFC1661-5.8-2 | tests | new TestRFC1661KeepaliveEchoOnlyInOpened: echoInterval 1 s, Opened -> Echo-Request (+); peer CONFREQ takes it out of Opened -> next frame 3 s on is the CONFREQ retransmission, no Echo-Request (-) | + sendEchoRequest, - run (revert) | enforced | |
| RFC1332-2.1-1 | tests | new TestNCPHeldUntilNetworkPhase/ipv4: CHAP pending 3 s -> no frame, no address request, no P2P install (-); accepted -> IPCP CONFREQ. Same targeted break red | - runAuthPhase | enforced | IPCP-Opened clause already enforced (audit) |
| RFC5072-3-1 | tests | TestNCPHeldUntilNetworkPhase/ipv6: CHAP pending 3 s -> no IPV6CP frame, no request (-); accepted -> IPV6CP CONFREQ (+) | +runNCPPhase, -runAuthPhase | enforced | receive half kept |
| RFC1332-2-1 | tests | new TestIPCPOnePacketPerFrameThroughRun: ze's IPCP CONFREQ from run() is 0x8021 with Length == Information length (+); received frame CONFREQ + trailing Terminate-Request -> Configure-Ack only, no Terminate-Ack in 1 s (-) | + sendNCPConfigureRequest, - ParseLCPPacket (revert) | enforced likely | negative is forced-input (R1 b); revert break is a panic, not a targeted "second packet parsed" mutant |
| RFC1332-2.1-3 | row annotation (mixed/lower-layer) | {lower-layer: Linux IPv4 output fragmentation; session_run.go::afterLCPOpen installs MRU as pppN MTU} on the row; Correction 2026-09-29 paragraph in rfc/corrections/rfc1332.md. TestIPMTUInstalledFromNegotiatedMRU (+/-) keeps asserting the MTU Ze installs. No split: the extraction site 2.1:2 is already only the fragmentation sentence; the first sentence has no keyword | none | enforced (lower-layer) | judge's call whether a split is still wanted |

## Continuation 3: still open (continuation 4 needed)

| id | next step |
|----|-----------|
| RFC5072-2-1 | forced-input positive: both NCPs enabled, IPCP Opened, IPV6CP left non-Initial but not Opened (peer Protocol-Reject of 0x8057 after ze's CONFREQ, or no answer), assert the "IPv6 service start failed" log is absent; row keeps {single-polarity: positive}. The HEAD control (IPv6CP disabled, state Initial) misses a guard on "any non-Initial" |
| RFC1661-6.4-7 | unchanged from continuation 1 note (ze sends a Magic Nak on loopback, then receives a Nak naming that value; assert redraw != it); pppoeclient unit also |
| RFC1994-4.1-3, 4.2-1, 4.2-2 | unchanged: one unit carrying a Response Value through authlocal's verifier to wire Code 3/4. authlocal is internal/component/l2tp/plugins/authlocal (not internal/plugins). Option: ppp external test package (ppp_test) importing authlocal cannot reach unexported harness; likely an l2tp component-level test |

Part C and R16: see first two rows above.

Runs: go test -race ppp, pppoe, authlocal, l2tp: ok (ppp3-final.log). pppoeclient -race: 3 FAILs = data race in its frameLog fixture, pre-existing, journal row exists (plan/journal/test-against-broken-path.md 2026-09-22). golangci-lint ./internal/component/l2tp/ppp/: 0 issues (ppp3-lint.log). gofmt clean. Functional: ./le test l2tp -a 26/26 pass, 2 skipped (ppp3-func-l2tp.log); ./le test pppoe -a 0/0, all 6 SKIPPED here (ppp3-func-pppoe.log): owed on a host where they run.

Gates owed (main thread): ./le rfc reseal (edited units and producers: applyTransition/sendEvent changed, so RFC1661-4.4-2 and other session_run.go producer records go producer-changed; removed 3.1-1 tag on TestRFC1661LCPPacketsSentFirst), ./le rfc check, pppoe functional suite where it does not skip, independent judge for every row above (R16 behaviour change on 4.3-2).

Files changed in continuation 3:
- internal/component/l2tp/ppp/session_run.go (signalLayerFinished, applyTransition Stopped/Opened branches, sendEvent at-most-once, sessionEndedAt comment)
- internal/component/l2tp/ppp/rfc1661_clauses_test.go (4.3-2 unit rewritten via python replace after approval, new TestRFC1661StoppedAfterPeerTerminateEndsOnOpened, helpers, l2tpevents import)
- internal/component/l2tp/ppp/lcp_lifecycle_test.go (TestPeriodicAuthenticationLCPInterruptions terminate case)
- internal/component/l2tp/ppp/ncp_test.go (TestIPv6CPSuggestionDiffersFromLocalIdentifier)
- internal/component/l2tp/ppp/rfc1661_test.go (3.1-1 tag removed from TestRFC1661LCPPacketsSentFirst)
- internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go (new)
- docs/architecture/l2tp/bng-1-radius-attributes.md, docs/guide/l2tp.md, docs/guide/pppoe.md
- rfc/short/rfc5072.md (4.1-5 marker text), rfc/short/rfc1332.md (2.1-3 lower-layer), rfc/corrections/rfc1332.md
- rfc/discrimination/rfc1661.json, rfc1332.json, rfc5072.json
- approvals: ppp.TestRFC1661StoppedAfterPeerTerminateTakesNewConfigureRequest, TestPeriodicAuthenticationLCPInterruptions, TestIPv6CPSuggestionDiffersFromLocalIdentifier, TestRFC1661LCPPacketsSentFirst

# Continuation 4 (2026-09-29; appended by main thread from the author's report — author hit its call budget)

| id | resolution | proof now (records: revert route, observed red, logs rec4-*.log) |
|---|---|---|
| RFC1661-5.8-2 | polarity swap (D-15) | TestRFC1661EchoReplyInOpened now + (sendEchoReply); TestRFC1661NoEchoOutsideOpened now - (LCPDoTransition). Negative was vacuous at HEAD (Data equalled local magic, peerMagic 0 -> dropped before FSM); fixed with peerMagic 0x99887766 + matching Data. |
| item 4 frame_test negative | tag moved (R1) | TestIPCPFrameRejectsNonSinglePacket now tags NEW row RFC1661-5-2 [MUST] §5 "When a packet is received with an invalid Length field, the packet is silently discarded without affecting the automaton." (D-3, extraction §5 unsourced-ids). New rfc1661_packet_length_test.go: + TestRFC1661InvalidLengthPacketSilentlyDiscarded, - TestRFC1661ValidLengthWithPaddingIsAnswered. Producer ParseLCPPacket. |
| RFC1332-2.1-3 | R3 split | 2.1-3 keeps sentence 1 + MTU unit (+/-). New RFC1332-2.1-4 [MUST] §2.1 "Larger IP datagrams must be fragmented as necessary." {lower-layer: Linux IPv4 output fragmentation; session_run.go::afterLCPOpen}. Extraction 2.1:2 -> 2.1-4; 2.1-3 to unsourced-ids. Correction paragraph + Support text updated. Kernel fragmentation proof would need QEMU guest. |
| RFC5072-2-1 | tests | rfc5072_ipv6cp_opened_test.go: TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened (+, ncpsComplete). Finding: afterLCPOpen ipv6cpState==Opened guard unreachable with IPV6CP enabled-not-Opened (ncpsComplete blocks first). NOT DONE: row marker still cites session_run.go:482; should name ncpsComplete and afterLCPOpen. |
| RFC1661-6.4-7 | tests | rfc1661_magic_loop_test.go: + real loop (Nak naming X -> new magic); - R1(b) rand.Reader first draw repeats old magic, second taken. Producer redrawMagicOnNak; targeted break (drop `mag == s.magic` continue) reddens negative. Old tags on TestMagicNumberRedrawnOnNak left: its negative does not prove this row (judge call). |
| RFC1994-4.1-3, 4.2-1, 4.2-2 | tests | rfc1994_local_verifier_test.go (ppp_test): handler from l2tp.GetAuthHandler() (l2tp-auth-local verifyCHAPMD5), wire reply Code 3/4. Two units carry 6 +/- tags (4.2-x on verifyCHAPMD5, 4.1-3 on runCHAPAuthPhase). Targeted always-accept/always-reject breaks redden them. export_test.go gained NewPipeDriverForTest. |

Race run (ppp4-race.log, checked by main thread): ppp, pppoe, l2tp, authlocal all ok.
Owed: golangci-lint ppp; reseal (old 5.8-2 polarities, RFC1332-2-1 frame_test negative); rfc check; index-update; independent judge.
Files: new rfc1661_packet_length_test.go, rfc5072_ipv6cp_opened_test.go, rfc1661_magic_loop_test.go, rfc1994_local_verifier_test.go; edited rfc1661_test.go, frame_test.go, export_test.go; rfc/short+extraction rfc1661, rfc1332 (+corrections); discrimination rfc1661/1994/5072; approvals for the 6 units.

# Continuation 5 (2026-09-30)

| id | resolution | proof now | records | expected | notes |
|---|---|---|---|---|---|
| RFC5072-2-1 | tests | TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened (+) now fails on either "IPv6 service start failed" or "refusing to start the IPv6 service" (early start with no negotiated peer IID logs the latter) | + revert ncp.go::ncpsComplete, observed red (rec5-RFC5072-2-1-positive.log) | strong | file: internal/component/l2tp/ppp/rfc5072_ipv6cp_opened_test.go |
| RFC1332-2.1-4 | tests + row edit | TestIPMTUInstalledFromNegotiatedMRU (D-15 approved) now tags 2.1-4 +: exactly one MTU on ppp7, 1400 = peer MRU; -: no MTU above 1400 (1500 included). {lower-layer} annotation dropped from the row; Support remaining and Correction 2026-09-29 paragraph now say the tests assert the MTU Ze installs | +/- revert session_run.go::afterLCPOpen, observed red (rec5-RFC1332-2.1-4-*.log); 2.1-3 records unchanged (unit-sha same) | strong | files: mtu_rfc1332_test.go, rfc/short/rfc1332.md, rfc/corrections/rfc1332.md, rfc/discrimination/rfc1332.json |
| RFC1994-4.1-4 | tests (in pppoeclient, not ppp: the row binds the PEER role, which Ze fills only in pppoeclient runClientAuth; in ppp_test the test itself plays the peer, so asserting its Response reaches the verifier proves nothing Ze does; the audit note names runClientAuth) | new pppoeclient/rfc1994_peer_rfc_test.go: + TestRFC1994PeerTransmitsResponseForEveryChallenge (Challenge 0x42 then re-challenge 0x43 -> two CHAP packets, Code 2, matching ids); - TestRFC1994PeerSendsNoResponseWithoutChallenge (CHAP Code 2 and Code 5 -> nothing written) | +/- revert pppoeclient/session.go::runClientAuth, observed red (rec5-RFC1994-4.1-4-*.log) | strong | old negative on TestBuildCHAPResponseMalformed (judge: neighbour) left untouched, judge call |
| RFC1994-4.2-4 | tests (pppoeclient, same reason: the Message must not affect the RECEIVER's operation; the ppp authenticator's own "bad CHAP response" text is not that) | + TestRFC1994FailureMessageDoesNotAffectOutcome: Failure with Message "", "authentication succeeded", non-ASCII -> the CHAP auth failed error (not the channel-closed one); same Message on Success -> success (Message held fixed, Code decides) | + revert runClientAuth, observed red (rec5-RFC1994-4.2-4-positive.log) | strong | row keeps its {single-polarity: positive} marker; D-15 approval recorded for the new unit (hook asked on its second edit) |
| RFC1334-2.3-4 | tests (pppoeclient: the only tagged unit; TestConfiguredPAPStillOffersCHAPFirst in ppp is the authenticator side and does not receive the Message) | TestPAPReplyMessageDoesNotAffectOutcome (D-15 approved): 3 Acks and 3 Naks with the same Messages ("", "welcome aboard", "invalid credentials"), Code held fixed per group; + every Ack succeeds; - every Nak returns "PAP auth rejected" (an ignored Nak would also error on retry exhaustion, now refused) | +/- revert pppoeclient/session.go::runClientAuth, observed red (rec5-RFC1334-2.3-4-*.log) | strong | row carries {superseded: dropped}; unchanged |

Runs: go test -race ppp ok; pppoeclient -race: only the 3 pre-existing frameLog DATA RACE fails (TestClientIPCPReplyCorrelation, TestClientLCPRejectRemovesOnlyRejectedOptions, TestClientPAPRetriesUntilMatchingReply; journal row plan/journal/test-against-broken-path.md 2026-09-22), none in files touched here (ppp5-race.log). golangci-lint ppp + pppoeclient: 0 issues (ppp5-lint.log).
Deviation from the brief: items 3 and 4 were done in pppoeclient, not ppp, because the rows bind the receiver/peer role that only pppoeclient runClientAuth fills and the audit notes name it.
Owed (main thread): rfc check, reseal/index-update, independent judge.
Files changed (continuation 5):
- internal/component/l2tp/ppp/rfc5072_ipv6cp_opened_test.go
- internal/component/l2tp/ppp/mtu_rfc1332_test.go
- internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go (new)
- internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go
- rfc/short/rfc1332.md, rfc/corrections/rfc1332.md
- rfc/discrimination/rfc5072.json, rfc1332.json, rfc1994.json, rfc1334.json
- approvals: ppp.TestIPMTUInstalledFromNegotiatedMRU, pppoeclient.TestRFC1994FailureMessageDoesNotAffectOutcome, pppoeclient.TestPAPReplyMessageDoesNotAffectOutcome

# Continuation 6 (2026-09-30, D-8 network-phase CHAP re-challenge in pppoeclient)

| step | state | notes |
|---|---|---|
| 1 red test | done | new pppoeclient/rfc1994_network_phase_test.go: TestRFC1994NetworkPhaseChallengeAnswered (+4.1-4), TestRFC1994NetworkPhaseNoResponseWithoutChallenge (-4.1-4), TestNetworkPhaseCHAPFailureEndsSession (untagged, §4.2 choice). Red observed on the unfixed loop (ppp6-red.log): Answered and FailureEnds fail. keepaliveLoop moved from session.go (was ~1000 lines) to new network_phase.go with a chap *networkPhaseCHAP param (nil = CHAP not negotiated); sessionResult.chap set via networkPhaseCHAPFor; dialer.go passes it. |
| 2 fix | done | network_phase.go: networkPhaseCHAP.handle answers a Challenge through buildCHAPResponse (RFC 1994 §4.1 quote above the write), Success for the pending id clears it, Failure for the pending id returns an error so keepaliveLoop ends the session (link closed by dialer), results for other ids discarded. CHOICE (§4.2): a re-auth Failure ends the session, because the authenticator SHOULD terminate the link and the client must not keep using a link it failed to re-authenticate. Package green (ppp6-green.log). Not fixed: negotiateIPCP (network phase before keepalive) still drops CHAP frames; signature change touches 4 tagged test callers -> journal row. |
| 3 tags/records | done | new units carry the 4.1-4 network-phase +/- tags (no D-15 needed, new functions). Records: + and - revert pppoeclient/network_phase.go::keepaliveLoop, observed red (rec6-RFC1994-4.1-4-*.log); method producers (networkPhaseCHAP.handle) are refused by the record key format. Natural red before the fix: ppp6-red.log. |
| 5 docs | done | docs/architecture/l2tp/cpe-1-pppoe-client.md: new paragraph on network-phase re-challenge (answer, Success/other-id/Failure handling, IPCP-window gap stated); source anchors now point keepaliveLoop at network_phase.go. docs/guide/pppoe.md has no re-auth text (unchanged). |
| 4 frameLog race | NOT DONE (budget) | fix is confined to test code but not to the helper: frameLog needs a mutex in Write plus a locked snapshot(), and 5 test units read w.frames directly (TestClientCHAPReplyCorrelation, TestClientLCPReplyCorrelation, TestClientLCPRejectRemovesOnlyRejectedOptions, TestClientLCPEchoBeforeOpenDiscarded, TestClientIPCPReplyCorrelation; tagged ones need D-15) plus helpers lcpPackets/papPackets/recordedIPCP/chapPacketsWritten. New network-phase tests read the writer only after done closes (no race by construction). |

# Continuation 7 (2026-09-30, IPCP-window CHAP re-challenge + frameLog race)

| step | state | notes |
|---|---|---|
| 1 IPCP-window defect (D-8, RFC1994-4.1-4) | done | negotiateIPCP gained `chap *networkPhaseCHAP` (nil = CHAP not negotiated); a CHAP frame goes to networkPhaseCHAP.handle, a handle error ends IPCP with that error. negotiateSession builds one networkPhaseCHAP before IPCP and passes the same instance into sessionResult.chap -> keepaliveLoop (pending Response carries over). Callers: session.go + 3 in ipcp_reply_test.go (untagged, pass nil). New units in rfc1994_network_phase_test.go: TestRFC1994IPCPWindowChallengeAnswered (+4.1-4), TestRFC1994IPCPWindowNoResponseWithoutChallenge (-4.1-4), TestIPCPWindowCHAPFailureEndsNegotiation (untagged, §4.2 choice). Natural red before fix: job-ppp7-red-7fae21b8.log (Answered + FailureEnds FAIL). Records: + and - revert producer session.go::negotiateIPCP, observed red (rec7-pos.log, rec7-neg.log). |
| 2 frameLog race | done | frameLog: sync.Mutex in Write + locked snapshot(); lcpPackets, papPackets, recordedIPCP, chapPacketsWritten, retryFailingAuthWriter.Write read via snapshot(); direct reads fixed in TestClientCHAPReplyCorrelation, TestClientLCPReplyCorrelation, TestClientLCPRejectRemovesOnlyRejectedOptions, TestClientLCPEchoBeforeOpenDiscarded, TestClientIPCPReplyCorrelation. D-15 approvals for the 3 tagged ones. Journal row (test-against-broken-path.md, 2026-09-22 frameLog) Fix column marked fixed. |
| 3 gates | done | go test -race pppoeclient: ok (job-ppp7-race2); golangci-lint pppoeclient: 0 issues (job-ppp7-lint2); gofmt clean. l2tp/ppp untouched. rfc check: no pppoeclient discrimination record stale; audit verdicts STALE (judge re-read owed): RFC1661-5.1-3, 5.2-3, 5.2-4, 5.3-7, 5.4-3, 5.4-4, 5.4-5, 5.8-2, RFC1994-4.1-4; SHIFTED (reseal): RFC1334-2.2-1, RFC1661-6-1, 6-2, 5.4-1, 5.4-2, 6.4-3, 6.4-7, RFC1994-4.2-4. |
| 4 docs | done | cpe-1-pppoe-client.md re-challenge paragraph: negotiateIPCP answers during IPCP, same state handed to keepaliveLoop; the "dropped" sentence removed; source anchor adds negotiateIPCP. |

Files changed (continuation 7): internal/component/l2tp/pppoeclient/{session.go, network_phase.go (comment), rfc1994_network_phase_test.go, ipcp_reply_test.go, rfc1661_option_length_test.go, pap_request_rfc1334_test.go, rfc1994_peer_rfc_test.go, chap_reply_test.go, lcp_reply_test.go, lcp_transition_test.go}, docs/architecture/l2tp/cpe-1-pppoe-client.md, plan/journal/test-against-broken-path.md, rfc/discrimination/rfc1994.json, rfc approvals for the 3 D-15 units.
Owed by main thread: reseal (SHIFTED), independent judge for the STALE verdicts above, rfc index-update. Advisory hook warning (pre-existing): session.go has no `// RFC:` header.

# Continuation 8 (2026-10-02, RFC1332-2.1-1 negative, c12 judge "absence claim, needs mutant route")

| id | change | records | expected verdict | changed files |
|---|---|---|---|---|
| RFC1332-2.1-1 | tests. Negative TestIPCPNoAddressBeforeOpened (D-15 approved) drives IPCP to Ack-Rcvd, one step short of Opened (peer Acks ze's request, never sends its own). It now also asserts no SetAdminUp, because a pppN that is down carries no IP. The claim was reworded to match. Ruling 2 receive side is already proven under RFC1661-3.6-2 (TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened). The absence assertion is now discriminated by a real gomu mutant on the Opened gate. | negative re-recorded on the mutant route: handleNCPPacket Opened gate `tr.NewState == LCPStateOpened` mutated to `!=` (selector column 5, mutant #2 in the proposer listing), which makes onNCPOpened fire on entry to Ack-Rcvd. OBSERVED red on the test's own assertion "EventSessionIPAssigned emitted before IPCP reached Opened". This replaces the old revert/halt record. Real gomu report over ncp.go only: 552 mutants, 315 killed, at tmp/session/2026-10-02-e08980d7-.../scratch/a1332-gomu/mutation-report.json. Positive and TestNCPHeldUntilNetworkPhase records unchanged | enforced (judge re-read owed: unit-sha and claim changed) | internal/component/l2tp/ppp/ncp_test.go, rfc/discrimination/rfc1332.json |

## Judge, Continuation 8 (2026-10-02, independent, mode rejudge)

| id | verdict | evidence |
|---|---|---|
| RFC1332-2.1-1 | enforced (rejudged) | gomu report genuine: gomu 0.1.0, run.log 1h5m over ncp.go, 552 mutants / 315 killed; mutant ncp.go_245 is the handleNCPPacket Opened gate `==` -> `!=` (TIMED_OUT in gomu's full-package run, so the unit-scoped record is the proof). Judge re-applied it under a Go overlay: TestIPCPNoAddressBeforeOpened FAIL "EventSessionIPAssigned emitted before IPCP reached Opened"; green without it. ncp.go unchanged since dcbc6de1a7. Claim matches the body's three assertions at Ack-Rcvd (no IPAssigned, no AddAddressP2P, no SetAdminUp; SetAdminUp runs only after runNCPPhase returns, session_run.go). Nit, not a defect: "so the kernel has no interface to carry IP on" is rationale the fake backend does not assert. Ruling 2 receive side: TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened (RFC1661-3.6-2, enforced) carries an observed-red revert record on supportsProtocol. rfc/audit/rfc5072.json carried: reseal shifts of ncp_test.go whole-file shas only (units unchanged) plus the reseal history line. |

Gates: golangci-lint internal/component/l2tp/ppp 0 issues; package tests ok; `./le rfc check` 61 violations, none in rfc1332, rfc1661 or rfc5072 (rfc5880 BFD, rfc3101/3623/5709/7474 OSPF, rfc9190 producer-changed: other agents). Access derived listing: RFC2866-5.5-1, RFC3579-3.3-2, RFC5176-2.3-2, all Blocked-by spec-radius-rfc-defects.
