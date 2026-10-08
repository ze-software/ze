// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- the optional objects Ze
// writes into a PathErr sender descriptor and a ResvTear flow descriptor.
// Related: build.go -- buildPathErr; reservation_build.go -- buildReservationControl.
//
// VALIDATES: every PathErr Ze originates or relays carries the ADSPEC of the
// path state it reports on, and a relayed ResvTear carries the FLOWSPEC of the
// reservation it tears down.
// PREVENTS: a PathErr or ResvTear that a peer requiring those optional objects
// (freeRouter) discards, so the head-end never learns the error or teardown.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// descriptorAdspec returns the ADSPEC object an upstream node advertises: path
// MTU 1500 for the Controlled Load service.
func descriptorAdspec(t *testing.T) []byte {
	t.Helper()
	adspec := make([]byte, adspecSize)
	n := encodeAdspec(adspec, 1500, serviceControlledLoad)
	require.Positive(t, n, "the ADSPEC encodes")
	return adspec[:n]
}

// sentObject returns the first object of class classNum in the last message of
// msgType the transport sent, header included. It walks the raw payload
// because Ze's own decoder skips a FLOWSPEC in a ResvTear.
func sentObject(t *testing.T, ft *fakeTransport, msgType, classNum uint8) ([]byte, bool) {
	t.Helper()
	payload, sent := ft.lastSentPayload(msgType)
	require.True(t, sent, "a message of type %d was sent", msgType)
	for off := rsvpHdrLen; off+objHdrLen <= len(payload); {
		header, err := decodeObjectHeader(payload[off:])
		require.NoError(t, err)
		require.GreaterOrEqual(t, int(header.Length), objHdrLen)
		require.LessOrEqual(t, off+int(header.Length), len(payload))
		if header.ClassNum == classNum {
			return payload[off : off+int(header.Length)], true
		}
		off += int(header.Length)
	}
	return nil, false
}

// TestPathErrOriginatedCopiesAdspec sends a transit a PATH carrying an ADSPEC
// whose strict next hop is reached through an outside node, and checks the
// PathErr it originates copies that ADSPEC into its sender descriptor.
//
// Method: the refusal path of TestRFC3209StrictHopThroughOutsideNodeRefused,
// with an ADSPEC on the PATH; the decoded PathErr's ADSPEC is compared byte for
// byte with the one the ingress sent.
func TestPathErrOriginatedCopiesAdspec(t *testing.T) {
	strict := netip.MustParsePrefix("10.0.0.9/32")
	e, ft := explicitHopTransit(t, "10.0.0.5", strict, netip.MustParseAddr("10.0.0.7"))
	adspec := descriptorAdspec(t)
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: strict.Addr(), TunnelID: 11, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: explicitHopIngress, LSPID: 1},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Address: strict}},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		Adspec:         adspec,
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}

	e.handlePacket(Packet{Src: explicitHopIngress, Payload: buildPath(psb, explicitHopIngress, defaultIPTTL)})

	requireBadStrictNode(t, ft)
	pathErr, _, _ := ft.lastByType(MsgTypePathErr)
	require.True(t, pathErr.HasAdspec, "the PathErr sender descriptor carries the ADSPEC")
	assert.Equal(t, adspec, pathErr.AdspecRaw, "copied from the PATH in error")
}

// TestPathErrRelayedCarriesReceivedAdspec has the egress send a transit a
// PathErr with no ADSPEC, and checks the PathErr the transit relays upstream
// carries the ADSPEC the transit received from the ingress.
func TestPathErrRelayedCarriesReceivedAdspec(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	adspec := descriptorAdspec(t)
	psb := rfc2205PSB()
	psb.Adspec = adspec
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	_, installed := e.table.Get(keyFromPSB(psb))
	require.True(t, installed, "transit installed path state")
	es := errorSpec{ErrorNode: rfc2205Egress, ErrorCode: ErrCodeRoutingProblem, ErrorValue: ErrValueNoRouteAvailable}

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: buildPathErr(psb.Session, psb.SenderTemplate, psb.SenderTSpec, nil, es)})

	pathErr, dst, sent := ft.lastByType(MsgTypePathErr)
	require.True(t, sent, "the transit relays the PathErr")
	assert.Equal(t, rfc2205Ingress, dst, "upstream toward the sender")
	require.True(t, pathErr.HasAdspec, "the relayed PathErr carries the ADSPEC")
	assert.Equal(t, adspec, pathErr.AdspecRaw, "the ADSPEC the ingress sent")
}

// TestPathErrLinkDownCarriesReceivedAdspec fails a transit's downstream link
// and checks the PathErr it sends upstream carries the received ADSPEC.
func TestPathErrLinkDownCarriesReceivedAdspec(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.5", oneIface("eth0"))
	key := transitLSP(e)
	adspec := descriptorAdspec(t)
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	lsp.PSB.ReceivedAdspec = adspec
	lsp.mu.Unlock()

	e.handleLinkDown("eth0")

	pathErr, _, sent := ft.lastByType(MsgTypePathErr)
	require.True(t, sent, "the transit reports the failure upstream")
	require.True(t, pathErr.HasAdspec, "the PathErr carries the ADSPEC")
	assert.Equal(t, adspec, pathErr.AdspecRaw)
}

// TestResvTearRelayedCarriesFlowSpec installs a reservation on a transit, has
// the egress tear it down with a ResvTear that omits FLOWSPEC, and checks the
// ResvTear the transit relays upstream carries the reservation's FLOWSPEC.
//
// Method: the raw relayed payload is walked for the FLOWSPEC object, whose body
// must decode to the Controlled Load request the egress reserved.
func TestResvTearRelayedCarriesFlowSpec(t *testing.T) {
	e, ft, psb := admittingTransit(t, 1e9)
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvFrom(psb, StyleSharedExplicit, resvErrFlowSpec)})
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	require.NotNil(t, lsp.RSB, "the RESV installed reservation state")
	lsp.mu.Unlock()

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: reservationTear(psb, rsvpHop{NextHop: rfc2205Egress})})

	flowSpec, found := sentObject(t, ft, MsgTypeResvTear, ClassFlowSpec)
	require.True(t, found, "the relayed ResvTear carries a FLOWSPEC")
	assert.Equal(t, uint8(2), flowSpec[3], "IntServ C-Type")
	decoded, err := decodeFlowSpec(flowSpec[objHdrLen:])
	require.NoError(t, err)
	assert.Equal(t, serviceControlledLoad, decoded.Service)
	assert.InDelta(t, resvErrFlowSpec.TokenRate, decoded.TokenRate, 0)
	assert.InDelta(t, resvErrFlowSpec.TokenBucket, decoded.TokenBucket, 0)
	assert.InDelta(t, resvErrFlowSpec.PeakRate, decoded.PeakRate, 0)
}
