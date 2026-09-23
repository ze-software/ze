// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 2205 Section 2 and 3
// obligations the signaling engine meets: the sender descriptor a PATH carries,
// the reverse path a RESV takes, and what a PathTear does to local state.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc2205Ingress, rfc2205Transit and rfc2205Egress are the three nodes of the
// LSP every test in this file signals: ingress -> transit -> egress.
var (
	rfc2205Ingress = netip.MustParseAddr("10.0.0.1")
	rfc2205Transit = netip.MustParseAddr("10.0.0.5")
	rfc2205Egress  = netip.MustParseAddr("10.0.0.9")
)

// rfc2205PSB returns the path state block the ingress signals, with an ERO
// through the transit node.
func rfc2205PSB() *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: rfc2205Egress, TunnelID: 7, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: rfc2205Ingress, LSPID: 1},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}},
		SenderTSpec:    FlowSpec{TokenRate: 4e8, TokenBucket: 4e8, PeakRate: 4e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
}

// rfc2205TransitWithPath returns a transit engine that holds path state for the
// LSP, installed by a PATH received from the ingress.
func rfc2205TransitWithPath(t *testing.T) (*engine, *fakeTransport, *pathStateBlock) {
	t.Helper()
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	psb := rfc2205PSB()
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	_, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok, "transit installed path state")
	return e, ft, psb
}

func keyFromPSB(psb *pathStateBlock) lspKey {
	return lspKey{
		TunnelEndpoint: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID,
		ExtTunnelID: psb.Session.ExtTunnelID, SenderAddr: psb.SenderTemplate.SenderAddr, LSPID: psb.SenderTemplate.LSPID,
	}
}

// sentCount returns how many messages the fake transport has sent.
func sentCount(ft *fakeTransport) int {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return len(ft.sent)
}

// RFC requirement: RFC2205-2-3 positive — the PATH buildPath encodes carries a SENDER_TEMPLATE naming the sender address and LSP-ID of its path state block.
func TestRFC2205PathCarriesSenderTemplate(t *testing.T) {
	psb := rfc2205PSB()
	msg, err := DecodeMessage(buildPath(psb, rfc2205Ingress, defaultIPTTL))
	require.NoError(t, err)
	require.True(t, msg.HasSenderTemplate, "PATH carries SENDER_TEMPLATE")
	assert.Equal(t, rfc2205Ingress, msg.SenderTemplate.SenderAddr)
	assert.Equal(t, uint16(1), msg.SenderTemplate.LSPID)
}

// RFC requirement: RFC2205-2-4 positive — the PATH buildPath encodes carries a SENDER_TSPEC whose token rate is the path state block's requested bandwidth.
func TestRFC2205PathCarriesSenderTSpec(t *testing.T) {
	psb := rfc2205PSB()
	msg, err := DecodeMessage(buildPath(psb, rfc2205Ingress, defaultIPTTL))
	require.NoError(t, err)
	require.True(t, msg.HasSenderTSpec, "PATH carries SENDER_TSPEC")
	assert.InDelta(t, 4e8, msg.SenderTSpec.TokenRate, 1)
}

// RFC requirement: RFC2205-2-3 negative — an egress that receives a PATH without a SENDER_TEMPLATE installs no path state and answers no RESV.
func TestRFC2205PathWithoutSenderTemplateDropped(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Egress.String(), nil)
	raw := encodeWithout(MsgTypePath, conformantMessages()[MsgTypePath], "SENDER_TEMPLATE")
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: raw})

	assert.Empty(t, e.table.All(), "no path state for a PATH without SENDER_TEMPLATE")
	_, _, sent := ft.lastByType(MsgTypeResv)
	assert.False(t, sent, "no RESV answers a PATH without SENDER_TEMPLATE")
}

// RFC requirement: RFC2205-2-1 positive — the egress sends its RESV to the node the PATH arrived from, the reverse of the path the PATH took.
func TestRFC2205ResvSentToPreviousHop(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Egress.String(), nil)
	_, transitTransport, psb := rfc2205TransitWithPath(t)
	path, sent := transitTransport.lastSentPayload(MsgTypePath)
	require.True(t, sent, "transit emitted the advanced PATH")
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: path})

	resv, dst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "egress answers the PATH with a RESV")
	assert.Equal(t, rfc2205Transit, dst, "RESV goes back to the PATH's previous hop")
	assert.Equal(t, psb.Session, resv.Session)
}

// RFC requirement: RFC2205-2-1 negative — a transit node that holds no path state for a RESV's session relays nothing upstream, because it has no reverse path to follow.
func TestRFC2205ResvWithoutPathStateNotRelayed(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	psb := rfc2205PSB()
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)})

	assert.Zero(t, ft.countByType(MsgTypeResv), "no RESV is relayed upstream without path state")
	resvErr, dst, ok := ft.lastByType(MsgTypeResvErr)
	require.True(t, ok, "missing path state is reported downstream")
	assert.Equal(t, rfc2205Egress, dst)
	assert.Equal(t, uint8(3), resvErr.ErrorSpec.ErrorCode)
	assert.Zero(t, resvErr.ErrorSpec.ErrorValue)
	assert.Equal(t, 1, sentCount(ft), "only the downstream ResvErr is sent")
	assert.Empty(t, e.table.All(), "a RESV creates no state of its own")
}

// RFC requirement: RFC2205-2-2 positive — the ingress that originated the PATH receives the RESV and brings its LSP up with the label the RESV carries.
func TestRFC2205ResvDeliveredToSender(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Ingress.String(), nil)
	psb := rfc2205PSB()
	key := keyFromPSB(psb)
	setupTunnel(e.log, e.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, e.cfg(), e)
	require.Equal(t, 1, ft.countByType(MsgTypePath), "ingress signaled the PATH")

	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Transit)})

	got, ok := e.table.Get(key)
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, got.State, "the sender's LSP is up once the RESV reaches it")
	assert.Equal(t, uint32(16050), got.OutLabel)
}

// RFC requirement: RFC2205-2-2 negative — a RESV naming an unknown sender in a signaled session brings no LSP up and is not relayed further.
func TestRFC2205ResvForUnknownSenderIgnored(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Ingress.String(), nil)
	psb := rfc2205PSB()
	setupTunnel(e.log, e.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, e.cfg(), e)
	require.Equal(t, 1, ft.countByType(MsgTypePath), "session has real PATH state")
	signaledKey := keyFromPSB(psb)
	psb.SenderTemplate.LSPID = 2
	before := sentCount(ft)
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Transit)})

	_, created := e.table.Get(keyFromPSB(psb))
	assert.False(t, created, "no LSP comes up for a RESV nobody asked for")
	require.Len(t, e.table.All(), 1, "unmatched RESV creates no additional state")
	signaled, ok := e.table.Get(signaledKey)
	require.True(t, ok)
	assert.Equal(t, LSPStatePathSent, signaled.State, "the known sender remains unreserved")
	assert.Zero(t, ft.countByType(MsgTypeResv), "an unmatched RESV is not relayed")
	resvErr, dst, ok := ft.lastByType(MsgTypeResvErr)
	require.True(t, ok, "missing sender state is reported downstream")
	assert.Equal(t, rfc2205Transit, dst)
	assert.Equal(t, uint8(4), resvErr.ErrorSpec.ErrorCode)
	assert.Zero(t, resvErr.ErrorSpec.ErrorValue)
	assert.Equal(t, before+1, sentCount(ft), "only the downstream ResvErr is sent")
}

// RFC requirement: RFC2205-2-7 positive — a transit node relays a received PathTear to its next hop inside the same handlePacket call, before any timer fires.
func TestRFC2205TransitPathTearRelayedWithoutDelay(t *testing.T) {
	e, ft, psb := rfc2205TransitWithPath(t)
	before := sentCount(ft)

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPathTear(psb, rfc2205Ingress)})

	tear, dst, ok := ft.lastByType(MsgTypePathTear)
	require.True(t, ok, "transit relays the PathTear downstream")
	assert.Equal(t, rfc2205Egress, dst, "PathTear relayed to the next hop of the path")
	assert.Equal(t, psb.Session, tear.Session)
	assert.Equal(t, before+1, sentCount(ft), "exactly one message left: the relayed PathTear")
	assert.Empty(t, e.table.All(), "transit path state torn down")
}

// RFC requirement: RFC2205-2-7 negative — a PathTear for a session the transit node holds no path state for is not relayed.
func TestRFC2205PathTearWithoutStateNotRelayed(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPathTear(rfc2205PSB(), rfc2205Ingress)})

	assert.Zero(t, sentCount(ft), "no PathTear leaves a node that holds no state for it")
}

// rfc2205EgressWithReservation returns an egress engine holding one admitted LSP
// of 3.2e9 bits/s on eth0, from a 4e8 bytes/s wire TSpec.
func rfc2205EgressWithReservation(t *testing.T) (*engine, *pathStateBlock) {
	t.Helper()
	e, _, _ := testEngine(t, rfc2205Egress.String(), func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 10e9}}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)
	_, transitTransport, psb := rfc2205TransitWithPath(t)
	path, sent := transitTransport.lastSentPayload(MsgTypePath)
	require.True(t, sent, "transit emitted the advanced PATH")
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: path})
	ib, _ := e.admission.GetInterface("eth0")
	require.InDelta(t, 3.2e9, ib.ReservedBandwidth, 1, "reservation held after PATH")
	return e, psb
}

// RFC requirement: RFC2205-3-14 positive — deleting path state on a PathTear releases the bandwidth the reservation held on the admission interface.
func TestRFC2205PathTearReleasesReservation(t *testing.T) {
	e, psb := rfc2205EgressWithReservation(t)

	e.handlePacket(Packet{Src: rfc2205Transit, Payload: buildPathTear(psb, rfc2205Transit)})

	ib, _ := e.admission.GetInterface("eth0")
	assert.InDelta(t, 0, ib.ReservedBandwidth, 1, "reservation released with the path state")
	assert.Empty(t, e.table.All(), "path state deleted")
}

// RFC requirement: RFC2205-3-14 negative — a PathTear for another LSP-ID of the same session deletes nothing and leaves the reservation in place.
func TestRFC2205PathTearForOtherLSPLeavesReservation(t *testing.T) {
	e, psb := rfc2205EgressWithReservation(t)

	other := *psb
	other.SenderTemplate.LSPID = 2
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: buildPathTear(&other, rfc2205Transit)})

	ib, _ := e.admission.GetInterface("eth0")
	assert.InDelta(t, 3.2e9, ib.ReservedBandwidth, 1, "reservation untouched by a PathTear for another LSP")
	_, ok := e.table.Get(keyFromPSB(psb))
	assert.True(t, ok, "path state of the untorn LSP remains")
}
