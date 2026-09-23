// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- a PATH names a sender and its
// traffic together (RFC 2205 Section 2).
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC2205-2-4 negative — a PATH that carries a SENDER_TEMPLATE and no SENDER_TSPEC is refused: no reservation state is installed and no RESV is sent.
func TestRFC2205PathWithoutSenderTSpecRefused(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(nil)
	psb.ERO = psb.ERO[len(psb.ERO)-1:]
	encoders := []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: ingress}) },
		func(b []byte) int {
			return encodeTimeValues(b, timeValues{RefreshPeriod: refreshMillis(DefaultRefreshPeriod)})
		},
		func(b []byte) int { return encodeERO(b, psb.ERO) },
		func(b []byte) int { return encodeLabelRequest(b, psb.LabelRequest) },
		func(b []byte) int { return encodeSenderTemplate(b, psb.SenderTemplate) },
	}
	e.handlePacket(Packet{Src: ingress, Payload: encodeMessage(MsgTypePath, 64, encoders)})

	_, _, sent := ft.lastByType(MsgTypeResv)
	assert.False(t, sent, "no RESV for a PATH without SENDER_TSPEC")
	_, held := e.table.Get(protectedKey())
	require.False(t, held, "no reservation state is installed")
	assert.Zero(t, sentCount(ft), "a malformed PATH is dropped without a protocol error")

	// The same path with its TSpec must reach reservation processing, so an
	// unrelated ERO failure cannot satisfy the missing-TSpec assertions.
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	resv, dst, accepted := ft.lastByType(MsgTypeResv)
	require.True(t, accepted, "restoring only SENDER_TSPEC establishes the reservation")
	assert.Equal(t, ingress, dst)
	assert.Equal(t, psb.Session, resv.Session)
	lsp, held := e.table.Get(protectedKey())
	require.True(t, held)
	assert.Equal(t, LSPStateUp, lsp.State)
}
