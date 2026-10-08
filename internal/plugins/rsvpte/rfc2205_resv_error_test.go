// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 2205 Section 2.5,
// 3.1.5, 3.1.6 and 3.1.8 obligations of the reservation error and teardown
// paths: ResvErr origination and relay, ResvTear routing, independent FF
// descriptor processing, the InPlace flag, and PathTear objects that are ignored.
// Related: rfc2205_engine_test.go -- the ingress, transit and egress fixture.
//
// VALIDATES: ResvErr reaches the responsible receiver, ResvTear follows the
// Resv, FF descriptors fail one by one, a failed increase stays InPlace.
// PREVENTS: a PathTear refused or mis-relayed over objects it must ignore.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resvErrFlowSpec is the FLOWSPEC every reservation in this file requests:
// 1e8 bytes/s, 8e8 bits/s of admitted bandwidth.
var resvErrFlowSpec = FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8}

// decodedSent returns every message of msgType the transport sent, decoded,
// with the address each one was sent to.
func decodedSent(t *testing.T, ft *fakeTransport, msgType uint8) ([]*ParsedMessage, []netip.Addr) {
	t.Helper()
	ft.mu.Lock()
	defer ft.mu.Unlock()
	var messages []*ParsedMessage
	var targets []netip.Addr
	for index := range ft.sent {
		msg, err := DecodeMessage(ft.sent[index].payload)
		if err != nil || msg.Header.MsgType != msgType {
			continue
		}
		messages = append(messages, msg)
		targets = append(targets, ft.sent[index].dst)
	}
	return messages, targets
}

// admittingTransit returns a transit engine whose one interface, eth0, can
// reserve maxReservable bits/s, holding path state for the rfc2205PSB LSP.
func admittingTransit(t *testing.T, maxReservable float64) (*engine, *fakeTransport, *pathStateBlock) {
	t.Helper()
	e, ft, _ := testEngine(t, rfc2205Transit.String(), func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 10e9, MaxReservableBW: float32(maxReservable)}}
	})
	e.admission.setInterface("eth0", 10e9, maxReservable)
	psb := rfc2205PSB()
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	_, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok, "transit installed path state")
	return e, ft, psb
}

// resvFrom returns the RESV the egress sends the transit for psb's sender,
// with the given style and FLOWSPEC.
func resvFrom(psb *pathStateBlock, style uint32, flowSpec FlowSpec) []byte {
	rsb := &resvStateBlock{Session: psb.Session, FlowSpec: flowSpec, Label: labelObject{Label: 16050}, Style: style}
	return buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)
}

// reservationError returns a ResvErr the ingress sends the transit, naming
// sender with the given style and ERROR_SPEC.
func reservationError(psb *pathStateBlock, sender senderTemplateIPv4, style uint32, es errorSpec) []byte {
	descriptor := flowDescriptor{FlowSpec: resvErrFlowSpec, Filters: []reservationFilter{{Filter: sender}}}
	return buildReservationControl(MsgTypeResvErr, psb.Session, rsvpHop{NextHop: rfc2205Ingress}, style, &descriptor, es, nil)
}

// reservationTear returns a ResvTear the egress sends the transit for psb's
// sender, carrying the given RSVP_HOP.
func reservationTear(psb *pathStateBlock, hop rsvpHop) []byte {
	descriptor := flowDescriptor{Filters: []reservationFilter{{Filter: psb.SenderTemplate}}}
	return buildReservationControl(MsgTypeResvTear, psb.Session, hop, StyleSharedExplicit, &descriptor, errorSpec{}, nil)
}

// fixedFilterResv returns an FF-style RESV from the egress carrying one flow
// descriptor per sender, each with its own FLOWSPEC, FILTER_SPEC and LABEL.
func fixedFilterResv(psb *pathStateBlock, senders []senderTemplateIPv4) []byte {
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: rfc2205Egress}) },
		func(b []byte) int {
			return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(DefaultRefreshPeriod)})
		},
		func(b []byte) int { return encodeStyle(b, StyleFixedFilter) },
	}
	for index, sender := range senders {
		label := labelObject{Label: 16050 + uint32(index)}
		encoders = append(encoders,
			func(b []byte) int { return encodeFlowSpec(b, ClassFlowSpec, resvErrFlowSpec) },
			func(b []byte) int { return encodeFilterSpec(b, sender) },
			func(b []byte) int { return encodeLabelObject(b, label) },
		)
	}
	return encodeMessage(MsgTypeResv, defaultIPTTL, encoders)
}

// TestRFC2205ResvErrRelayedToReceiver sends a transit holding a reservation a
// ResvErr from its previous hop, and checks the error reaches the receiver
// side: the next hop the reservation came from.
//
// RFC requirement: RFC2205-2-8 positive -- a transit holding the reservation a ResvErr names relays that ResvErr, received from its previous hop, to the next hop its Resv came from (the egress receiver), with the ERROR_SPEC node and code unchanged and the reservation's own FILTER_SPEC; handleResvErr in reservation.go.
// RFC 2205 Section 2.5: "Since a request that fails may be the result of merging a number of requests, a reservation error must be reported to all of the responsible receivers."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205ResvErrRelayedToReceiver(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)
	es := errorSpec{ErrorNode: rfc2205Ingress, ErrorCode: ErrCodePolicyControlFailure}

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: reservationError(psb, psb.SenderTemplate, StyleSharedExplicit, es)})

	errors, targets := decodedSent(t, ft, MsgTypeResvErr)
	require.Len(t, errors, 1, "one ResvErr is relayed")
	assert.Equal(t, rfc2205Egress, targets[0], "the ResvErr goes to the next hop the Resv came from")
	assert.Equal(t, psb.Session, errors[0].Session)
	assert.Equal(t, es, errors[0].ErrorSpec, "the ERROR_SPEC travels unchanged")
	require.Len(t, errors[0].FlowDescriptors, 1)
	require.Len(t, errors[0].FlowDescriptors[0].Filters, 1)
	assert.Equal(t, psb.SenderTemplate, errors[0].FlowDescriptors[0].Filters[0].Filter)
}

// TestRFC2205ResvErrForOtherSenderNotRelayed sends a transit holding one
// reservation a ResvErr naming a sender it holds no reservation for, and
// checks the receiver of the held reservation is told nothing.
//
// RFC requirement: RFC2205-2-8 negative -- a ResvErr naming a sender (LSP-ID 2) for which the transit holds no reservation is relayed to nobody: the receiver of the LSP-ID 1 reservation is not responsible for that request and receives no ResvErr; handleResvErr in reservation.go.
// RFC 2205 Section 2.5: "Since a request that fails may be the result of merging a number of requests, a reservation error must be reported to all of the responsible receivers."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205ResvErrForOtherSenderNotRelayed(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)
	other := psb.SenderTemplate
	other.LSPID = 2
	es := errorSpec{ErrorNode: rfc2205Ingress, ErrorCode: ErrCodePolicyControlFailure}

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: reservationError(psb, other, StyleSharedExplicit, es)})

	errors, _ := decodedSent(t, ft, MsgTypeResvErr)
	assert.Empty(t, errors, "no receiver is told of an error in a request it did not make")
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	assert.NotNil(t, lsp.RSB, "the held reservation is untouched")
	lsp.mu.Unlock()
}

// TestRFC2205PathTearObjectsIgnored sends a transit holding path state a
// PathTear whose SENDER_TSPEC and ADSPEC are both malformed, objects a PATH
// would be refused for, and checks the tear is still acted on.
//
// RFC requirement: RFC2205-3-13 positive -- a PathTear whose sender descriptor carries a SENDER_TSPEC and an ADSPEC whose IntServ bodies are both invalid (C-Type 2, body 0xdeadbeef) still deletes the transit's path state and is relayed to the next hop, and no PathErr answers it, because both objects are skipped unread; DecodeMessage in wire.go and handlePathTear in engine.go.
// RFC 2205 Section 3.1.5: "A PathTear message may include a SENDER_TSPEC or ADSPEC object in its sender descriptor, but these must be ignored."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205PathTearObjectsIgnored(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	tspec := []byte{0, 8, ClassSenderTSpec, 2, 0xde, 0xad, 0xbe, 0xef}
	adspec := []byte{0, 8, ClassAdspec, 2, 0xde, 0xad, 0xbe, 0xef}
	tear := encodeMessage(MsgTypePathTear, defaultIPTTL, []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: rfc2205Ingress}) },
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
		func(b []byte) int { return encodeOpaqueObject(b, tspec) },
		func(b []byte) int { return encodeOpaqueObject(b, adspec) },
	})
	require.NotEmpty(t, tear)

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: tear})

	assert.Empty(t, e.table.All(), "the PathTear deleted the path state")
	_, dst, relayed := ft.lastByType(MsgTypePathTear)
	require.True(t, relayed, "the PathTear is relayed downstream")
	assert.Equal(t, rfc2205Egress, dst)
	assert.Zero(t, ft.countByType(MsgTypePathErr), "the ignored objects draw no error")
}

// TestRFC2205PathTearTSpecNotUsed sends a transit a PathTear whose
// SENDER_TSPEC disagrees with the PATH's, and checks that value reaches
// neither the decoded message nor the relayed PathTear.
//
// RFC requirement: RFC2205-3-13 negative -- the SENDER_TSPEC of a received PathTear is not used: DecodeMessage records no SENDER_TSPEC for it, and the PathTear the transit relays carries the TSPEC of its stored path state (4e8 bytes/s), not the received 9e8; DecodeMessage in wire.go and pathTearLocked in reroute.go.
// RFC 2205 Section 3.1.5: "A PathTear message may include a SENDER_TSPEC or ADSPEC object in its sender descriptor, but these must be ignored."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205PathTearTSpecNotUsed(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	received := *psb
	received.SenderTSpec = FlowSpec{TokenRate: 9e8, TokenBucket: 9e8, PeakRate: 9e8}
	tear := buildPathTear(&received, rfc2205Ingress)
	decoded, err := DecodeMessage(tear)
	require.NoError(t, err)
	assert.False(t, decoded.HasSenderTSpec, "no SENDER_TSPEC is recorded from a PathTear")

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: tear})

	relayed, sent := ft.lastSentPayload(MsgTypePathTear)
	require.True(t, sent, "the PathTear is relayed downstream")
	object, carried := objectBytes(relayed, ClassSenderTSpec)
	require.True(t, carried, "the relayed PathTear carries the stored SENDER_TSPEC")
	tspec, err := decodeFlowSpec(object[objHdrLen:])
	require.NoError(t, err)
	assert.InDelta(t, 4e8, tspec.TokenRate, 1, "the stored TSPEC, not the received one, is relayed")
}

// TestRFC2205ResvTearRoutedLikeResv tears a transit's reservation from the
// egress, and checks the ResvTear goes where the Resv went: the unicast
// address of the previous hop.
//
// RFC requirement: RFC2205-3-17 positive -- a ResvTear matching the transit's reservation is sent on to the same destination the transit relayed its Resv to, the unicast address of the previous hop (the ingress), and the reservation state is removed; handleResvTear and removeReservation in reservation.go.
// RFC 2205 Section 3.1.6: "A ResvTear message must be routed like the corresponding Resv message, and its IP destination address will be the unicast address of a previous hop."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205ResvTearRoutedLikeResv(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)
	_, resvTarget, relayed := ft.lastByType(MsgTypeResv)
	require.True(t, relayed, "the transit relayed its Resv")

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: reservationTear(psb, rsvpHop{NextHop: rfc2205Egress})})

	tears, targets := decodedSent(t, ft, MsgTypeResvTear)
	require.Len(t, tears, 1, "one ResvTear goes upstream")
	assert.Equal(t, resvTarget, targets[0], "the ResvTear follows the Resv")
	assert.Equal(t, rfc2205Ingress, targets[0], "addressed to the previous hop")
	assert.Equal(t, psb.Session, tears[0].Session)
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	assert.Nil(t, lsp.RSB, "the reservation is removed")
	lsp.mu.Unlock()
}

// TestRFC2205ResvTearForOtherHopNotRouted sends a transit a ResvTear whose
// RSVP_HOP names a logical interface handle the reservation was not made on,
// and checks it is routed nowhere.
//
// RFC requirement: RFC2205-3-17 negative -- a ResvTear whose RSVP_HOP (LIH 7) does not match the reservation's is not routed upstream: no ResvTear reaches the previous hop and the reservation stays; handleResvTear in reservation.go.
// RFC 2205 Section 3.1.6: "A ResvTear message must be routed like the corresponding Resv message, and its IP destination address will be the unicast address of a previous hop."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205ResvTearForOtherHopNotRouted(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: reservationTear(psb, rsvpHop{NextHop: rfc2205Egress, LIH: 7})})

	assert.Zero(t, ft.countByType(MsgTypeResvTear), "a tear matching no reservation goes nowhere")
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	assert.NotNil(t, lsp.RSB, "the reservation stays")
	lsp.mu.Unlock()
}

// TestRFC2205FixedFilterErrorPerDescriptor sends a transit holding path state
// for LSP-ID 1 an FF RESV with three descriptors, two of them for senders it
// does not know, and counts the ResvErr messages it answers with.
//
// RFC requirement: RFC2205-3-18 positive -- an FF-style Resv carrying two descriptors in error (LSP-ID 5 and 6, no path state) draws two separate ResvErr messages to the egress, each carrying exactly one FILTER_SPEC, one for each failed descriptor; handleResv in engine.go and acceptReservation in reservation.go.
// RFC 2205 Section 3.1.8: "Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205FixedFilterErrorPerDescriptor(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	five, six := psb.SenderTemplate, psb.SenderTemplate
	five.LSPID, six.LSPID = 5, 6

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: fixedFilterResv(psb, []senderTemplateIPv4{psb.SenderTemplate, five, six})})

	errors, targets := decodedSent(t, ft, MsgTypeResvErr)
	require.Len(t, errors, 2, "one ResvErr per descriptor in error")
	var named []uint16
	for index, msg := range errors {
		assert.Equal(t, rfc2205Egress, targets[index])
		assert.Equal(t, StyleFixedFilter, msg.Style)
		require.Len(t, msg.FlowDescriptors, 1)
		require.Len(t, msg.FlowDescriptors[0].Filters, 1, "each ResvErr names one descriptor")
		named = append(named, msg.FlowDescriptors[0].Filters[0].Filter.LSPID)
	}
	assert.ElementsMatch(t, []uint16{5, 6}, named)
}

// TestRFC2205FixedFilterGoodDescriptorKept sends the same FF RESV and checks
// that the descriptor which is not in error is reserved as if it came alone.
//
// RFC requirement: RFC2205-3-18 negative -- the failure of sibling descriptors in an FF-style Resv does not reach the descriptor that is not in error: no ResvErr names LSP-ID 1, its reservation is installed with its own label, and its Resv is relayed to the ingress; handleResv in engine.go and acceptReservation in reservation.go.
// RFC 2205 Section 3.1.8: "Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205FixedFilterGoodDescriptorKept(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	five, six := psb.SenderTemplate, psb.SenderTemplate
	five.LSPID, six.LSPID = 5, 6

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: fixedFilterResv(psb, []senderTemplateIPv4{five, psb.SenderTemplate, six})})

	errors, _ := decodedSent(t, ft, MsgTypeResvErr)
	for _, msg := range errors {
		for _, filter := range msg.FlowDescriptors[0].Filters {
			assert.NotEqual(t, psb.SenderTemplate, filter.Filter, "no ResvErr names the good descriptor")
		}
	}
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	require.NotNil(t, lsp.RSB, "the good descriptor is reserved")
	assert.Equal(t, uint32(16051), lsp.OutLabel, "with its own label")
	assert.Equal(t, LSPStateUp, lsp.State)
	lsp.mu.Unlock()
	_, dst, relayed := ft.lastByType(MsgTypeResv)
	require.True(t, relayed, "its Resv goes upstream")
	assert.Equal(t, rfc2205Ingress, dst)
}

// TestRFC2205ResvErrCarriesErrorAndRoute rejects a RESV for an unknown sender
// and decodes the ResvErr the transit originates.
//
// RFC requirement: RFC2205-3-19 positive -- the ResvErr a transit originates for a rejected SE Resv carries the SESSION, an RSVP_HOP naming the transit, an ERROR_SPEC naming the transit as error node with code 4 (No sender information), a copy of the Resv's STYLE, and the failing flow descriptor's FLOWSPEC and FILTER_SPEC, and is sent to the node the Resv came from; rejectReservation, sendResvError and buildReservationControl in reservation_build.go.
// RFC 2205 Section 3.1.8: "This ResvErr message must contain the information required to define the error and to route the error message in later hops."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205ResvErrCarriesErrorAndRoute(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	unknown := *psb
	unknown.SenderTemplate.LSPID = 9

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvFrom(&unknown, StyleSharedExplicit, resvErrFlowSpec)})

	errors, targets := decodedSent(t, ft, MsgTypeResvErr)
	require.Len(t, errors, 1)
	got := errors[0]
	assert.Equal(t, rfc2205Egress, targets[0], "sent to the node the erroneous Resv came from")
	assert.Equal(t, psb.Session, got.Session)
	assert.Equal(t, rfc2205Transit, got.Hop.NextHop, "RSVP_HOP names the reporting node")
	assert.Equal(t, rfc2205Transit, got.ErrorSpec.ErrorNode)
	assert.Equal(t, ErrCodeNoSender, got.ErrorSpec.ErrorCode)
	assert.Equal(t, StyleSharedExplicit, got.Style, "a copy of the STYLE object")
	require.Len(t, got.FlowDescriptors, 1)
	assert.InDelta(t, resvErrFlowSpec.TokenRate, got.FlowDescriptors[0].FlowSpec.TokenRate, 1, "the failing FLOWSPEC")
	require.Len(t, got.FlowDescriptors[0].Filters, 1)
	assert.Equal(t, unknown.SenderTemplate, got.FlowDescriptors[0].Filters[0].Filter, "the failing FILTER_SPEC")
}

// TestRFC2205ResvErrWrongStyleNotRouted sends a transit holding an SE
// reservation a ResvErr whose STYLE copy says FF, and checks it is routed
// nowhere: the STYLE is part of what routes the error in later hops.
//
// RFC requirement: RFC2205-3-19 negative -- a ResvErr whose STYLE (FF) does not match the transit's SE reservation for that sender is not routed: the transit sends no ResvErr toward the receiver; handleResvErr in reservation.go.
// RFC 2205 Section 3.1.8: "This ResvErr message must contain the information required to define the error and to route the error message in later hops."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205ResvErrWrongStyleNotRouted(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)
	es := errorSpec{ErrorNode: rfc2205Ingress, ErrorCode: ErrCodePolicyControlFailure}

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: reservationError(psb, psb.SenderTemplate, StyleFixedFilter, es)})

	errors, _ := decodedSent(t, ft, MsgTypeResvErr)
	assert.Empty(t, errors, "a ResvErr matching no reservation style is not routed")
}

// TestRFC2205FailedIncreaseLeavesReservationInPlace admits a reservation of
// 8e8 bits/s on a 1e9 bits/s interface, then asks to raise it to 2.4e9.
//
// RFC requirement: RFC2205-3-20 positive -- an admission control failure on a Resv raising an existing reservation leaves that reservation in place (its FLOWSPEC, label, Up state and the 8e8 bits/s charged on eth0 unchanged, no Resv relayed upstream) and the ResvErr sent to the egress carries ERROR_SPEC code 1, value 2, with the InPlace flag on; acceptReservation in reservation.go and rejectReservation in reservation_build.go.
// RFC 2205 Section 3.1.8: "If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205FailedIncreaseLeavesReservationInPlace(t *testing.T) {
	e, ft, psb := admittingTransit(t, 1e9)
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvFrom(psb, StyleSharedExplicit, resvErrFlowSpec)})
	before, _ := e.admission.GetInterface("eth0")
	require.InDelta(t, 8e8, before.ReservedBandwidth, 1, "the first reservation is admitted")
	relayedBefore := ft.countByType(MsgTypeResv)

	larger := FlowSpec{TokenRate: 3e8, TokenBucket: 3e8, PeakRate: 3e8}
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvFrom(psb, StyleSharedExplicit, larger)})

	errors, targets := decodedSent(t, ft, MsgTypeResvErr)
	require.Len(t, errors, 1, "the failed increase is reported")
	assert.Equal(t, rfc2205Egress, targets[0])
	assert.Equal(t, ErrCodeAdmissionControlFailure, errors[0].ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueRequestedBandwidth, errors[0].ErrorSpec.ErrorValue)
	assert.Equal(t, ErrFlagInPlace, errors[0].ErrorSpec.Flags&ErrFlagInPlace, "InPlace is on")
	after, _ := e.admission.GetInterface("eth0")
	assert.InDelta(t, 8e8, after.ReservedBandwidth, 1, "the admitted charge stays")
	assert.Equal(t, relayedBefore, ft.countByType(MsgTypeResv), "the refused increase is not relayed")
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	defer lsp.mu.Unlock()
	require.NotNil(t, lsp.RSB, "the existing reservation is left in place")
	assert.InDelta(t, 1e8, lsp.RSB.FlowSpec.TokenRate, 1, "with its admitted FLOWSPEC")
	assert.True(t, lsp.Reserved)
	assert.Equal(t, uint32(16050), lsp.OutLabel)
	assert.Equal(t, LSPStateUp, lsp.State)
}

// TestRFC2205FailedFirstReservationNotInPlace asks a 1e8 bits/s interface for
// a first reservation of 8e8 bits/s, where nothing exists to leave in place.
//
// RFC requirement: RFC2205-3-20 negative -- an admission control failure on a Resv for which no reservation exists sends a ResvErr with code 1, value 2 and the InPlace flag off, and installs no reservation state; acceptReservation in reservation.go and rejectReservation in reservation_build.go.
// RFC 2205 Section 3.1.8: "If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message."
// The quote is read from rfc/full/rfc2205.txt.
func TestRFC2205FailedFirstReservationNotInPlace(t *testing.T) {
	e, ft, psb := admittingTransit(t, 1e8)

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvFrom(psb, StyleSharedExplicit, resvErrFlowSpec)})

	errors, _ := decodedSent(t, ft, MsgTypeResvErr)
	require.Len(t, errors, 1, "the failed reservation is reported")
	assert.Equal(t, ErrCodeAdmissionControlFailure, errors[0].ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueRequestedBandwidth, errors[0].ErrorSpec.ErrorValue)
	assert.Zero(t, errors[0].ErrorSpec.Flags&ErrFlagInPlace, "InPlace is off: nothing was in place")
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	assert.Nil(t, lsp.RSB, "no reservation state is installed")
	lsp.mu.Unlock()
}
