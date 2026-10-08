# Extended Message Support (RFC 8654)

## TL;DR

Capability 6 is directional. Advertising it permits the other speaker to send
messages up to 65,535 octets; receiving it permits Ze to send such messages.
The advertisements are independent, not a bilateral intersection. OPEN remains
limited to 4,096 octets and KEEPALIVE remains exactly 19 octets.

<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate, ExtendedMessageRecv, ExtendedMessageSend -->
<!-- source: internal/component/bgp/message/header.go -- Header.ValidateLengthWithMax -->

## Capability and limits

The Extended Message capability has code 6 and length 0. In an OPEN optional
parameter containing only this capability, the bytes are `02 02 06 00`:
parameter type 2, parameter length 2, capability code 6, capability length 0.
The BGP header remains 19 octets: a 16-octet marker, two-octet total Length,
and one-octet Type.

<!-- source: internal/core/bgp/capability/capability.go -- ExtendedMessage -->
<!-- source: internal/component/bgp/message/header.go -- HeaderLen, MarkerLen, MaxMsgLen, ExtMsgLen -->

[RFC 8654 Section 4](https://www.rfc-editor.org/rfc/rfc8654#section-4) says:

> A BGP speaker MAY send BGP Extended Messages to a peer only if the BGP
> Extended Message Capability was received from that peer.

[Section 6](https://www.rfc-editor.org/rfc/rfc8654#section-6) sets the receive
limit from the receiver's advertisement:

> For all messages except for OPEN and KEEPALIVE messages, if the receiver
> has advertised the BGP Extended Message Capability, this document raises
> that limit to 65,535.

| Local OPEN advertises 6 | Peer OPEN advertises 6 | Ze receive maximum | Ze send maximum |
|---|---|---|---|
| No | No | 4,096 | 4,096 |
| Yes | No | 65,535 | 4,096 |
| No | Yes | 4,096 | 65,535 |
| Yes | Yes | 65,535 | 65,535 |

These maxima do not replace message-specific minima or the OPEN and KEEPALIVE
exceptions. A 4,097-octet OPEN or a 20-octet KEEPALIVE is invalid in every row.

<!-- source: internal/component/bgp/message/header.go -- Header.ValidateLengthWithMax -->

## Ze implementation

`capability.Negotiated` and its `EncodingCaps` component retain
`ExtendedMessageRecv` from the local advertisement and `ExtendedMessageSend`
from the peer advertisement. `EncodingContext.ExtendedMessage()` selects the
field matching that context's direction. The reactor's outbound-only
`NegotiatedCapabilities.ExtendedMessage` projects the send permission.

<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate, buildSubComponents -->
<!-- source: internal/core/bgp/capability/encoding.go -- EncodingCaps -->
<!-- source: internal/core/bgp/context/context.go -- EncodingContext.ExtendedMessage -->
<!-- source: internal/component/bgp/reactor/negotiated.go -- NewNegotiatedCapabilities -->

Session negotiation selects the receive buffer pool from the local
advertisement and sizes the reusable write buffer from the peer advertisement.
Both the ordinary and coalesced readers validate the header before attempting
to read its declared body. Outbound rewrite scratch buffers are sized from the
body being rewritten, not from the session's unrelated receive permission.

Capability negotiation computes its result under the session lock, before taking
the writer lock. Publishing that result and resizing the write buffer remain one
writer-locked operation, so an internal negotiation panic cannot strand that lock
and block connection cleanup.

<!-- source: internal/component/bgp/reactor/session_negotiate.go -- negotiateWith -->
<!-- source: internal/component/bgp/reactor/session.go -- getReadBuffer -->
<!-- source: internal/component/bgp/reactor/session_read.go -- readAndProcessMessage -->
<!-- source: internal/component/bgp/reactor/session_coalesce.go -- readAndProcessCoalesced -->
<!-- source: internal/component/bgp/reactor/session_write.go -- writeRawUpdateBody -->
<!-- source: internal/component/bgp/reactor/session_aigp.go -- writeUpdateWithoutAIGP -->

When an UPDATE exceeds the destination's send limit, `sendUpdateWithSplit`
partitions its NLRI into independently encoded UPDATEs. That cannot make an
indivisible attribute set fit: a payload the splitter cannot represent within
the destination's limit is rejected rather than sent oversized.

<!-- source: internal/component/bgp/reactor/peer_send.go -- sendUpdateWithSplit -->
<!-- source: internal/component/bgp/message/update_split.go -- Splitter.Split -->

## Invalid lengths and cleanup

A peer that has not advertised capability 6 must not accept an extended
message ([RFC 8654 Section 5](https://www.rfc-editor.org/rfc/rfc8654#section-5)).
Ze sends Message Header Error / Bad Message Length (NOTIFICATION 1/2), with
the erroneous two-octet Length as Data, and closes the connection. For example,
an UPDATE Length of 4,097 produces Data `10 01`; the reader does not wait for
the oversized body.

Fatal closure also invokes the peer lifecycle: timers and session resources
are released and the peer's Adj-RIB-In is cleared. The selecting RIB re-elects
surviving paths or removes routes with no survivor from the Loc-RIB. The route
server separately submits withdrawals from its failed-source inventory; that
does not by itself prove replacement-best delivery to recipients.
See [the Established FSM](../behavior/fsm-established.md) and
[peer lifecycle](../behavior/peer-lifecycle.md).

<!-- source: internal/component/bgp/reactor/session_read.go -- readAndProcessMessage -->
<!-- source: internal/component/bgp/reactor/session_coalesce.go -- readAndProcessCoalesced -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce -->

## Regression boundaries

- `TestRFC8654ExtendedMessageDirections` asserts all four capability
  combinations, including the two asymmetric encoding components.
- `TestRFC8654ReceiveLimitFollowsLocalOPEN` exchanges OPENs and checks both
  readers at Length 4,096, 4,097 and 65,535, including exact accepted payloads
  and rejected-length NOTIFICATION data.
- `TestRFC8654AnnouncementAccumulation` feeds standard-sized announcements
  through the accumulating reader, compares exact combined consumer payloads
  beyond 4,096 through 65,535 octets, and checks overflow/KEEPALIVE flushes.
- `TestRFC8654SendLimitFollowsPeerOPEN` observes actual emitted UPDATEs after
  each OPEN combination and checks that splitting preserves all withdrawals.
- `TestRFC8654AdvertisementDoesNotExtendControlBounds` asserts that OPEN and
  KEEPALIVE bounds do not expand.
- `TestRFC8654FatalLengthReleasesInstalledRoutes` seeds the real RIB and a
  recipient session before the fatal header, then asserts notification, EOF,
  storage cleanup, resource release and downstream withdrawals.
  `TestRFC8654ValidLengthRetainsInstalledRoutes` supplies the adjacent valid
  4,096-octet case and observes its changed MED in storage and recipient TCP.
  These assertions cover final cleanup, not the ordering of final route-owner
  release versus outbound advertisement admission.
- `TestRFC8654ExtendedAttributeDiscard` isolates malformed ATOMIC_AGGREGATE
  and AGGREGATOR against their valid counterparts through both readers.
  `TestRFC8654TreatAsWithdrawRemovesInstalledRoutes` observes actual RIB
  removal, downstream withdrawal and recovery after a malformed extended UPDATE.
- `TestRFC8654ExtendedDuplicateAttributes` compares the exact consumer attribute
  section after removing later recognized and unrecognized duplicates.
  `TestRFC8654ExtendedDuplicateMPResets` checks the MP exception: one occurrence
  is accepted, a second causes NOTIFICATION 3/1 with no additional dispatch.
- `TestRFC8654ExtendedTwoFamilyWithdrawalRecovery` isolates the extra synthesized
  MP-family dispatch with actual RIB and Loc-RIB removal and recipient TCP
  withdrawals. An unrelated IPv6 route survives, and both withdrawn routes
  recover on the same session. These receive-path cases compose with the
  clause-specific RFC 7606 tests; they are not a claim that selected malformed
  UPDATEs alone prove every referenced RFC 7606 obligation.
- `TestRFC8654FatalLengthReelectsAlternateBest` requires replacement attributes
  on recipient TCP after the fatal event and a single Loc-RIB replacement,
  without transient removal. Source-DOWN recovery queries the selecting RIB
  through the strict applied-delivery fence and one destination writer operation.
- `TestRFC7606CoalescedMalformedNLRIBoundaries` checks each original NLRI
  boundary through both readers, with standard and extended messages and
  ADD-PATH. A truncated /24 cannot consume the next UPDATE's default route.
  The reader dispatches a preceding valid announcement before NOTIFICATION 3/10.
- `TestRFC7606DiagnosticWireAndMPNLRI` requires separate original-message error
  records, exact header-inclusive wire hex, and legacy plus MP NLRI lists.
  IPv4, IPv6 and typed MP inputs compose with valid-input silence and the
  disabled-debug allocation guard. First-AS cases retain original bytes before
  Partial stamping or in-place discard, and use reconstructed OLD-speaker paths
  with both AGGREGATOR gate outcomes. A separate NEXT_HOP policy control requires
  an explicitly processed representation rather than claiming original wire.
  Empty MP_REACH plus a late first-AS or tunnel-endpoint error requires session
  reset with no consumer UPDATE or false End-of-RIB. Matching-AS, valid-endpoint
  and real-NLRI controls separate that requirement from valid empty input and
  ordinary withdrawal handling.
- `TestReceiveAdmissionSurvivesIdleTransition` stages a timer transition while
  an unexpected UPDATE is blocked at the existing session mutex. It requires
  FSM rejection and connection cleanup without an unclassified dispatch or EOR.

<!-- source: internal/component/bgp/reactor/rfc7606_receive_boundary_test.go -- TestRFC7606CoalescedMalformedNLRIBoundaries, TestRFC7606DiagnosticWireAndMPNLRI, TestReceiveAdmissionSurvivesIdleTransition -->

Fixture presence, native execution and fresh discrimination records are separate
proof obligations.
RFC 4271 Section 6 does not require a socket write to precede an unrelated
plugin's Adj-RIB-In cleanup. The route server retains compact withdrawal records
for its asynchronous sender, and the forwarding queue retains its own work.
The tests do not yet establish the exact last-owner release/admission ordering;
see the [RFC summary's proof boundaries](../../../rfc/short/rfc8654.md#proof-boundaries).

<!-- source: internal/core/bgp/capability/rfc8654_extended_message_test.go -->
<!-- source: internal/component/bgp/reactor/rfc8654_directional_session_test.go -->
<!-- source: internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go -->
<!-- source: internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go -->
<!-- source: internal/component/bgp/reactor/rfc8654_rib_recovery_test.go -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- emitPurgedWithdraws -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleStateDown, sendBatchedWithdrawals -->

The named FRR interop scenarios exercise a real producer and consumer. An
OPEN-only fixture relay makes asymmetric advertisements visible to Ze because
FRR 10.3.1 applies a bilateral gate itself. The relay does not rewrite UPDATEs.
Its results establish Ze's directional behavior, not native asymmetric
conformance by FRR. See [interop testing](../testing/interop.md).

