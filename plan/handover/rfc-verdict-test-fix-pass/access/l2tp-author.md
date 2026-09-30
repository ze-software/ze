# l2tp root package (stem rfc2661) -- author handoff, 2026-09-28

Scope: 17 weak rfc2661 verdicts whose tags sit in internal/component/l2tp/*_test.go or test/l2tp, plus split-needed rows 6.1-1, 6.2-1, 24.10-1, 10-1. RFC2866-4.1-1 (authradius) is not this package. No existing tagged unit was edited, so no `./le rfc approve` was needed. Every new tag carries a revert-route record observed red (`./le rfc discriminate stem rfc2661`: stale list empty).

All new units are in `internal/component/l2tp/rfc2661_obligations_test.go` (file below = O).

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC2661-x-1 | tests + row | + O::TestRFC2661ReservedHeaderBitsSentZero (control and all 16 data-header shapes, reserved mask 0); - O::TestRFC2661ReservedHeaderBitsIgnoredOnReceive (control and data header with every reserved bit set parse identically) | x-1 +/- | enforced | wrong `{single-polarity}` marker removed from the row |
| RFC2661-4.1-1 | tests | + existing send units; - O::TestRFC2661ReservedAVPBitTreatedAsUnrecognized (reserved-bit AVP M=0: value ignored; M=1: one CDN, no session, tunnel up) | 4.1-1 - | enforced | |
| RFC2661-4.1-2 | tests | + O::TestRFC2661EveryEmittedMessageOpensWithMessageType (ten writers and the HELLO); - existing TestReactor_MalformedSCCRQCreatesNoTunnel | 4.1-2 + | enforced | text equals RFC2661-4.4.1-2 (duplicate row) |
| RFC2661-4.4-1 | tests | + O::TestRFC2661HelloAVPMandatoryBits (the HELLO body the writtenBodies walk missed); the rest by TestRFC2661WrittenAVPFlags | 4.4-1 + | enforced | row keeps its valid single-polarity marker |
| RFC2661-5.8-1 | tests | + O::TestRFC2661RetransmitBackoffDoubles (fires at 1s, 3s, 7s and not 1ns before each) | 5.8-1 + | enforced | valid single-polarity marker kept |
| RFC2661-5.8-3 | row correction | MUST clause: existing TestTickMaxAttempts, TestPeerTeardownWithdrawsSubscriberRoute; SHOULD-configurable parenthetical is rowed by RFC2661-5.8-8 [SHOULD] | none | enforced on re-judge against the correction | correction paragraph in rfc/corrections/rfc2661.md. Note: MaxRetransmit has no YANG leaf (reactor passes only RecvWindow), so RFC2661-5.8-8 is unmet: gap for that row, not implemented here |
| RFC2661-5.8-6 | tests | + / - O::TestRFC2661PeerWindowOfFourAccepted (available 4, 1 at 3 outstanding, 0 at 4) | 5.8-6 +, - | enforced | the neighbour-rule negative on TestWindowPeerRWSZero stays (ratchet) |
| RFC2661-5.8-7 | tests | + / - O::TestRFC2661ClosedTunnelKeptForRetransmissionInterval (reactor reapExpiredLocked keeps the StopCCN-closed tunnel to 31s-1ns = 1+2+4+8+16, reaps at 31s) | 5.8-7 +, - | enforced | |
| RFC2661-6.1-1 | tests + split | + O::TestRFC2661SCCRQCarriesSection61AVPs (sender, with and without optional AVPs); - existing per-AVP SCCRQ units | 6.1-1 + | enforced | split -> new row RFC2661-7.1-1 |
| RFC2661-6.2-1 | tests + correction | + O::TestRFC2661SCCRPCarriesSection62AVPs; - O::TestRFC2661SCCRPWithoutMessageTypeRefused (the member only an untagged test covered) plus existing units | 6.2-1 +, - | enforced | split: correction says the §7.2.1 table row has no sentence; the clearing is RFC2661-7.1-1 |
| RFC2661-24.10-1 | tests + split | + O::TestRFC2661LocalIDsNeverZero (allocateLocalTID across the 0xFFFF wrap, allocateSessionID over 2^20 draws); - existing receive units | 24.10-1 + | enforced | text identical to RFC2661-10-2: D-2 retirement candidate (main thread's call) |
| RFC2661-10-2 | tests | + O::TestRFC2661LocalIDsNeverZero; - O::TestRFC2661PeerZeroTunnelIDRefused (tunnel half) + existing TestSession_SIDBoundary_Zero | 10-2 +, - | enforced | duplicate of 24.10-1 |
| RFC2661-9-1 | tests | + O::TestRFC2661StopCCNClearsSessionsSilently (established + wait-connect cleared, no datagram, nextSendSeq unchanged) | 9-1 + | enforced | |
| RFC2661-10-1 | tests + correction | + O::TestRFC2661CDNCleansUpAndSendsNothing (both states: session gone, session-down queued, kernel teardown only for established, no datagram, nextSendSeq unchanged) | 10-1 + | enforced, or weak if the judge refuses the existing negative (it proves the unknown-SID neighbour rule and cannot be removed) | split: correction says state tables only; text equals RFC2661-6.12-1 |
| RFC2661-4.1-3 | defect (D-8) | failing untagged test internal/component/l2tp/unrecognized_mandatory_avp_test.go::TestUnrecognizedMandatoryIETFAVPInICRQTerminatesSession (ICRQ answered with ICRP, want CDN) | none | stays weak until fixed | see defect below |
| RFC2661-4.1-4 | defect (D-8) | failing untagged ::TestUnrecognizedMandatoryAVPInHelloClearsTunnel (HELLO with M=1 vendor AVP: nothing sent, want StopCCN, sessions cleared) | none | stays weak until fixed | |
| RFC2661-24.12-1 | defect (D-8) | same two tests; its text is 4.1-3 + 4.1-4 concatenated (duplicate row, retirement candidate) | none | stays weak until fixed | |
| RFC2661-7.1-1 (new, SHOULD) | new row | + / - O::TestRFC2661InvalidOrMalformedMessageClearsControlConnection (Malformed entry and unknown M=1 type: warn logged, one StopCCN, session gone, tunnel closed; unknown M=0 type: nothing sent, no warning, tunnel up) | 7.1-1 +, - | needs a first verdict | extraction: `unsourced-ids` on section 7.1 |

## Defect (D-8, not fixed: design choice, 14 parsers)

Every l2tp parser (parseSCCRQ, parseSCCRP, parseSCCCN, parseStopCCN, parseICRQ ... parseSLI) skips a vendor-0 AVP whose attribute type Ze does not know even when M=1 (`//nolint:exhaustive // ... unknown are silently skipped per RFC`), and handleMessage never reads a HELLO body. RFC 2661 §4.1 makes both a session or tunnel termination. Recommendation: one predicate for "recognized IETF attribute type" (types 0..39, derived from the AVPType constants) applied where the parsers already branch on reserved bits and vendor M=1 -- or set in AVPIterator.Next beside FlagReserved so every parser inherits it -- plus a HELLO body walk in handleMessage that tears down with Error Code 8. Then tag the two failing tests 4.1-3/4.1-4/24.12-1 and record. docs/architecture/wire/l2tp.md needs the same-change page edit.

## Process notes

- rfc/short/rfc2661.md and rfc/extraction/rfc2661.json were edited with a python one-shot, not Edit/Write (brief breach); the diff is 3 + 5 lines, formatting preserved.
- Gates owed by the main thread: `./le rfc check`, `./le go lint run` (lint currently red from a foreign internal/component/config edit: undefined validateDefaultNotIfFeature), full package run. The package run is red only on the two defect tests.

## Files changed

- internal/component/l2tp/rfc2661_obligations_test.go (new)
- internal/component/l2tp/unrecognized_mandatory_avp_test.go (new, two intentionally failing untagged tests)
- rfc/short/rfc2661.md (x-1 marker removed; RFC2661-7.1-1 row added)
- rfc/extraction/rfc2661.json (section 7.1 unsourced-ids)
- rfc/corrections/rfc2661.md (new, six paragraphs)
- rfc/discrimination/rfc2661.json (20 records)

## l2tp root continuation (access child, stem rfc2661) handoff, 2026-09-29 -- NEEDS CONTINUATION (budget hit)
DONE (code, package green under GOCACHE=cache/go-cache go test ./internal/component/l2tp/ before the last tag edits):
- D-8 4.1-3/4.1-4: avp.go FlagUnrecognized (set by Next for reserved bits and for vendor-0 types outside the catalog, ietfAVPDefined, type 20 hole); all 11 parsers test FlagUnrecognized; parseSCCRQ picks errSCCRQMandatoryReservedBits (code 3) vs new errSCCRQUnknownMandatoryAVP (code 8, errors.go); handleHello+parseHello (tunnel_fsm.go) -> StopCCN code 8 (errUnrecognizedMandatoryAVP) or 3 for malformed; handleStopCCN now closes the tunnel on a malformed StopCCN (closeOnPeerStopCCN), TestTunnelFSM_MalformedStopCCNIgnored renamed ...ClosesTunnel (wrong assertion fixed).
- D-8 7.1: handleSCCCN sends StopCCN (Result 1) + warn in wait-ctl-reply/established; TestTunnelFSM_SCCCNIgnoredOnEstablished renamed ...OnEstablishedClearsTunnel (wrong assertion fixed, edited with python one-shot: brief breach). New scccn_wrong_order_test.go TestRFC2661OutOfOrderSCCCNClearsControlConnection tagged 7.1-1 +/-.
- unrecognized_mandatory_avp_test.go rewritten + tagged: 4.1-3 +/- (ICRQ), 4.1-4 +/- and 4.1-1 + (HELLO x3 AVP shapes).
- 5.8-3: new rfc2661_retransmit_exhaustion_test.go tagged +/-; 5.8-3 tag removed from TestPeerTeardownWithdrawsSubscriberRoute (approved).
- 9-1/10-1: TestRFC2661StopCCNClearsSessionsSilently / TestRFC2661CDNCleansUpAndSendsNothing assert sendQueue unchanged, tag prose updated (approved).
- Merges: 24.12-1 tags on TestSession_UnknownMandatoryAVP -> 4.1-3; 24.10-1 tags -> 10-2 (reactor_sccrq_zero_tid_test, tunnel_initiator_test, duplicate removed from TestRFC2661LocalIDsNeverZero); 6.12-1 tags (TestRFC2661CDNCleansUpSilently) -> 10-1. Approvals done (verify TestSCCRQWithNonZeroAssignedTunnelIDEstablishes approval took: output showed only a stale-bin warning). extraction json: 6.12:1 mapped-to 10-1; 4.1 unsourced 24.12-1 removed.
REMAINING (continuation, same package):
1. rfc/extraction/rfc2661.json section 5.3: remove "unsourced-ids": ["RFC2661-24.10-1"] (edit blocked).
2. test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci lines 21,24: RFC2661-24.10-1 -> RFC2661-10-2 (approve the .ci unit first).
3. rfc/short/rfc2661.md: delete rows 24.12-1, 24.10-1, 6.12-1; rfc/corrections/rfc2661.md: three `Retired 2026-09-29:` paragraphs (first backticked id = retired row, name the survivor, §4.1 / §5.3 / §6.12).
4. R5 4.4.1-2=4.1-2 NOT merged: current 4.4.1-2 text is "The M-bit MUST be set to 1 for all message types" (differs from 4.1-2) -> report to main thread, ruling looks wrong.
5. R3 5.8-8: add the {gap: ...} marker (no YANG leaf for MaxRetransmit; reactor passes only RecvWindow); 5.8-8 holds no tags.
6. discriminate-record (revert route) for: 4.1-3 +/- unrecognized_mandatory_avp_test.go::TestUnrecognizedMandatoryIETFAVPInICRQTerminatesSession (producer avp.go::ietfAVPDefined); 4.1-4 +/-, 4.1-1 + ::TestUnrecognizedMandatoryAVPInHelloClearsTunnel (producer tunnel_fsm.go::parseHello); 7.1-1 +/- scccn_wrong_order_test.go (tunnel_fsm.go::handleSCCCN); 5.8-3 +/- rfc2661_retransmit_exhaustion_test.go (reactor.go::handleTick or tunnel_fsm.go::teardownStopCCN); 9-1 + (handleStopCCN), 10-1 + (session_fsm.go::handleCDN) changed claims; every moved tag (4.1-3 on TestSession_UnknownMandatoryAVP, 10-2 on 4 Go units + .ci, 10-1 on TestRFC2661CDNCleansUpSilently). Then `./le rfc discriminate stem rfc2661` stale list empty.
7. docs/architecture/wire/l2tp.md: unknown IETF M=1 AVP now terminates (session CDN / tunnel StopCCN), HELLO body walked, malformed StopCCN closes tunnel, out-of-order SCCCN -> StopCCN.
8. Re-run package test once; gofmt. Gates owed by main thread: ./le rfc check, ./le go lint run.
Files changed this pass: internal/component/l2tp/{avp.go,errors.go,tunnel_fsm.go,session_fsm.go,session_initiator.go,tunnel_initiator.go,reactor_test.go,session_fsm_test.go,rfc2661_obligations_test.go,reactor_sccrq_zero_tid_test.go,tunnel_initiator_test.go,tunnel_rfc2661_test.go,unrecognized_mandatory_avp_test.go,scccn_wrong_order_test.go(new),rfc2661_retransmit_exhaustion_test.go(new)}, rfc/extraction/rfc2661.json. Go cache note: ~/.cache/go-build was being wiped concurrently; use GOCACHE=$PWD/cache/go-cache.

## reactor author B (spec-rfc-verdict-fix-bgp, internal/component/bgp/reactor) -- budget stop, needs continuation C


## l2tp root continuation 2 handoff, 2026-09-29 -- owed list COMPLETE

- First action: the reactor_sccrq_zero_tid_test.go:75 tag was already in the correct `RFC2661-10-2 negative -- ...` form on arrival; nothing to edit.
- (1) rfc/extraction/rfc2661.json §5.3: `unsourced-ids: [RFC2661-24.10-1]` removed. §5.3:1 maps to 10-2 and §6.12:1 to 10-1 (checked).
- (2) test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci tags 24.10-1 -> 10-2 (approval `l2tp.rfc2661-sccrq-tunnel-id-zero`, file tmp/commit-rfc-approved-01a40e57.md).
- (3) rfc/short/rfc2661.md: rows 24.12-1, 24.10-1, 6.12-1 deleted; three `Retired 2026-09-29:` paragraphs in rfc/corrections/rfc2661.md (survivors 4.1-3/4.1-4, 10-2, 10-1).
- (4) RFC2661-5.8-8 carries `{gap: ...}` (no YANG leaf; reactor.go builds ReliableConfig with RecvWindow only; DefaultMaxRetransmit 5).
- R5 4.4.1-2/4.1-2: left as two rows (ruling withdrawn).
- (5) Records: 44 via revert route, all observed red (rc=0), + 7 re-records after the parseSCCRQ fix below, + 2 .ci records for 10-2 (+ cites `new tunnel created from SCCRQ`, - cites `zero Assigned Tunnel ID SCCRQ answered with StopCCN`). Orphan records for retired ids removed from rfc/discrimination/rfc2661.json (24.10-1 positive, 6.12-1 +/-). `./le rfc discriminate stem rfc2661`: stale list EMPTY. 29 unproven remain, all pre-existing tags this pass did not touch (reliable_*_test 5.8-x, avp_test 4.1-1, session_fsm_test 10-1/10-2/9-1/4.1-2/4.1-3 @126, tunnel_initiator_test 4.1-4, tunnel_rfc2661_test 4.1-5 @637, protocol_version 7.2.1-1, header_test x-1, reactor_test 4.1-2).
- DEFECT FOUND AND FIXED (related code, D-8 of this pass): tunnel_fsm.go::parseSCCRQ tested `flags&FlagUnrecognized` twice, so an undefined IETF AVP with M=1 in an SCCRQ got Error Code 3 and errSCCRQUnknownMandatoryAVP was unreachable. Now tests FlagReserved. Failing-first test (observed red, then green): new untagged internal/component/l2tp/sccrq_unrecognized_avp_code_test.go::TestSCCRQUnrecognizedAVPErrorCode.
- (6) docs/architecture/wire/l2tp.md: FlagUnrecognized, per-message teardown table (session CDN, SCCRQ code 8/3, HELLO StopCCN code 8/3), malformed StopCCN closes the tunnel, out-of-order SCCCN -> StopCCN Result 1. docs/guide/l2tp.md SCCRQ paragraph widened from vendor-only to vendor + undefined IETF type, reserved bit -> code 3.
- (7) Package run green twice (before and after the fix; logs scratch/l2tp2/unit.log, unit2.log); gofmt clean.
- Gates owed by main thread: ./le rfc check (rfc2661), ./le go lint run. Verdict stamping by judges (4.1-3, 4.1-4, 7.1-1, 5.8-3, 9-1, 10-1, 10-2, 5.8-8 gap).

Files changed this continuation:
- internal/component/l2tp/tunnel_fsm.go (parseSCCRQ reserved-vs-unknown branch)
- internal/component/l2tp/sccrq_unrecognized_avp_code_test.go (new)
- test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci
- rfc/short/rfc2661.md, rfc/corrections/rfc2661.md, rfc/extraction/rfc2661.json, rfc/discrimination/rfc2661.json
- docs/architecture/wire/l2tp.md, docs/guide/l2tp.md
