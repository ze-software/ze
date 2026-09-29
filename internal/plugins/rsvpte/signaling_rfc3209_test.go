// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 3209 obligations
// driven through the engine's entry points: the PATH an ingress emits, the
// explicit route a transit evaluates, the RRO subobject a node records, the
// make-before-break PATH, and the STYLE of the egress RESV.
//
// VALIDATES: RFC 3209 Sections 4.2.4, 4.3.4.1, 4.4.3 and 4.6.4 at handlePacket,
// setupTunnel, refreshPaths and reroute, never at a helper alone.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// rfc3209Ingress returns an ingress that signaled the rfc2205PSB tunnel.
func rfc3209Ingress(t *testing.T) (*engine, *fakeTransport, *pathStateBlock) {
	t.Helper()
	e, ft, _ := testEngine(t, rfc2205Ingress.String(), nil)
	psb := rfc2205PSB()
	setupTunnel(e.log, e.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, e.cfg(), e)
	require.Equal(t, 1, ft.countByType(MsgTypePath), "ingress signaled the PATH")
	return e, ft, psb
}

// sentPaths decodes every PATH the fake transport sent, in send order.
func sentPaths(t *testing.T, ft *fakeTransport) []*ParsedMessage {
	t.Helper()
	ft.mu.Lock()
	defer ft.mu.Unlock()
	var paths []*ParsedMessage
	for i := range ft.sent {
		msg, err := DecodeMessage(ft.sent[i].payload)
		require.NoError(t, err)
		if msg.Header.MsgType == MsgTypePath {
			paths = append(paths, msg)
		}
	}
	return paths
}

// RFC requirement: RFC3209-4.2-1 positive -- the PATH an ingress sends to establish its LSP tunnel carries a LABEL_REQUEST naming L3PID 0x0800 (IPv4).
func TestRFC3209IngressPathCarriesLabelRequest(t *testing.T) {
	_, ft, _ := rfc3209Ingress(t)
	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	require.True(t, path.HasLabelRequest, "PATH carries LABEL_REQUEST")
	assert.Equal(t, uint16(0x0800), path.LabelRequest.L3PID)
}

// RFC requirement: RFC3209-4.2-1 negative -- no PATH the ingress sends for its LSP tunnel lacks the LABEL_REQUEST: the originated PATH, two refreshes and the make-before-break PATH each carry it with L3PID 0x0800.
func TestRFC3209IngressPathNeverWithoutLabelRequest(t *testing.T) {
	e, ft, psb := rfc3209Ingress(t)
	refreshPaths(slogutil.DiscardLogger(), e.table, e)
	refreshPaths(slogutil.DiscardLogger(), e.table, e)
	_, ok := e.reroute(keyFromPSB(psb), psb.ERO)
	require.True(t, ok, "reroute signaled a replacement")

	paths := sentPaths(t, ft)
	require.GreaterOrEqual(t, len(paths), 4)
	for i, path := range paths {
		require.True(t, path.HasLabelRequest, "PATH %d carries LABEL_REQUEST", i)
		assert.Equal(t, uint16(0x0800), path.LabelRequest.L3PID, "PATH %d L3PID", i)
	}
}

// RFC requirement: RFC3209-4.3.4.1-1 negative -- a transit evaluates the first ERO subobject: a PATH whose first subobject 10.0.0.77/32 does not contain this node is not forwarded, installs no state, and is answered with a PathErr Routing Problem (24) / Bad initial subobject (4) to the previous hop.
func TestRFC3209TransitRefusesForeignFirstSubobject(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	psb := rfc2205PSB()
	psb.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.77/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}}

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})

	assert.Zero(t, ft.countByType(MsgTypePath), "PATH not forwarded")
	assert.Empty(t, e.table.All(), "no path state")
	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok)
	assert.Equal(t, rfc2205Ingress, dst)
	assert.Equal(t, uint8(24), perr.ErrorSpec.ErrorCode)
	assert.Equal(t, uint16(4), perr.ErrorSpec.ErrorValue)
}

// RFC requirement: RFC3209-4.3.4-1 negative -- a transit that is not adjacent to the strict abstract node of the second subobject (no route to 10.0.0.9/32) selects no next hop: it forwards no PATH, keeps no state, and answers with PathErr Routing Problem (24) / Bad strict node (2).
func TestRFC3209TransitNotAdjacentToSecondSubobject(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	ft.routes = map[netip.Prefix]RouteInfo{}

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(rfc2205PSB(), rfc2205Ingress, defaultIPTTL)})

	assert.Zero(t, ft.countByType(MsgTypePath), "PATH not forwarded")
	assert.Empty(t, e.table.All(), "no path state")
	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok)
	assert.Equal(t, rfc2205Ingress, dst)
	assert.Equal(t, uint8(24), perr.ErrorSpec.ErrorCode)
	assert.Equal(t, uint16(2), perr.ErrorSpec.ErrorValue)
}

// recordingPath returns the rfc2205PSB PATH with route recording on and an
// RRO of hops IPv4 subobjects, the first being the ingress.
func recordingPath(hops int) *pathStateBlock {
	psb := rfc2205PSB()
	psb.RecordRoute = true
	psb.RRO = make([]rroEntry, hops)
	for i := range psb.RRO {
		psb.RRO[i] = rroEntry{Type: 1, Address: netip.AddrFrom4([4]byte{10, 1, byte(i >> 8), byte(i)})}
	}
	psb.RRO[0].Address = rfc2205Ingress
	return psb
}

// RFC requirement: RFC3209-4.4.3-1 positive -- the subobject a node adds to an RRO is this router's own address: the transit prepends 10.0.0.5 to the RRO of the PATH it relays downstream and of the RESV it relays upstream, and keeps the rest in order.
func TestRFC3209EngineRecordsOwnAddress(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	psb := recordingPath(1)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	require.Len(t, path.RRO, 2)
	assert.Equal(t, rfc2205Transit, path.RRO[0].Address, "PATH: added subobject is this router")
	assert.Equal(t, rfc2205Ingress, path.RRO[1].Address)

	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit,
		RRO: []rroEntry{{Type: 1, Address: rfc2205Egress}}}
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)})
	rro := relayedResvRRO(t, ft)
	require.Len(t, rro, 2)
	assert.Equal(t, rfc2205Transit, rro[0].Address, "RESV: added subobject is this router")
	assert.Equal(t, rfc2205Egress, rro[1].Address)
}

// RFC requirement: RFC3209-4.4.3-1 negative -- the subobject a node adds is never a neighbor's address: the RRO head of the relayed PATH is not the next hop (egress), and the RRO head of the relayed RESV is not the previous hop (ingress).
func TestRFC3209EngineNeverRecordsNeighborAddress(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	psb := recordingPath(1)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	path, dst, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	require.NotEmpty(t, path.RRO)
	assert.NotEqual(t, dst, path.RRO[0].Address, "PATH RRO head is not the next hop")
	assert.NotEqual(t, rfc2205Egress, path.RRO[0].Address)

	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit,
		RRO: []rroEntry{{Type: 1, Address: rfc2205Egress}}}
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)})
	_, resvDst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	rro := relayedResvRRO(t, ft)
	require.NotEmpty(t, rro)
	assert.NotEqual(t, resvDst, rro[0].Address, "RESV RRO head is not the previous hop")
	assert.NotEqual(t, rfc2205Ingress, rro[0].Address)
}

// relayedResvRRO returns the RRO of the one filter of the last RESV sent.
func relayedResvRRO(t *testing.T, ft *fakeTransport) []rroEntry {
	t.Helper()
	resv, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.Len(t, resv.FlowDescriptors, 1)
	require.Len(t, resv.FlowDescriptors[0].Filters, 1)
	filter := resv.FlowDescriptors[0].Filters[0]
	require.True(t, filter.HasRRO, "the relayed RESV records the route")
	return filter.RRO
}

// RFC requirement: RFC3209-4.4.3-3 positive -- a PATH whose RRO would grow past maxRecordRouteHops (32) with this node's subobject is relayed without the RRO, and processing continues: the transit installs path state and forwards the PATH to the egress.
func TestRFC3209PathRRODroppedWhenTooBig(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(recordingPath(32), rfc2205Ingress, defaultIPTTL)})

	path, dst, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "the PATH is still forwarded")
	assert.Equal(t, rfc2205Egress, dst)
	assert.False(t, path.HasRRO, "the RRO is dropped from the PATH")
	assert.Len(t, e.table.All(), 1, "path state installed")
}

// RFC requirement: RFC3209-4.4.3-3 negative -- a PATH whose RRO still fits after this node's subobject keeps it: from 31 subobjects the relayed PATH carries 32, headed by this router.
func TestRFC3209PathRROKeptWhenItFits(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(recordingPath(31), rfc2205Ingress, defaultIPTTL)})

	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	require.True(t, path.HasRRO, "the RRO is kept")
	require.Len(t, path.RRO, 32)
	assert.Equal(t, rfc2205Transit, path.RRO[0].Address)
}

// RFC requirement: RFC3209-6-1 positive -- the make-before-break PATH the ingress emits carries the existing SESSION object unchanged (end point, Tunnel_ID, Extended_Tunnel_ID) and a SENDER_TEMPLATE with a new LSP_ID from the same sender.
func TestRFC3209ReroutePathKeepsSessionNewLSPID(t *testing.T) {
	e, ft, psb := rfc3209Ingress(t)
	original, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)

	_, ok = e.reroute(keyFromPSB(psb), psb.ERO)
	require.True(t, ok)
	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	assert.Equal(t, original.Session, path.Session, "SESSION object unchanged")
	assert.Equal(t, psb.Session, path.Session)
	assert.Equal(t, original.SenderTemplate.SenderAddr, path.SenderTemplate.SenderAddr)
	assert.NotEqual(t, original.SenderTemplate.LSPID, path.SenderTemplate.LSPID, "new LSP_ID")
}

// RFC requirement: RFC3209-6-1 negative -- a reroute never reuses an LSP_ID in use: two successive make-before-break PATHs carry LSP_IDs distinct from each other and from the original, all under the same SESSION.
func TestRFC3209RerouteNeverReusesLSPID(t *testing.T) {
	e, ft, psb := rfc3209Ingress(t)
	first, ok := e.reroute(keyFromPSB(psb), psb.ERO)
	require.True(t, ok)
	_, ok = e.reroute(first, psb.ERO)
	require.True(t, ok)

	seen := map[uint16]bool{}
	for _, path := range sentPaths(t, ft) {
		assert.Equal(t, psb.Session, path.Session, "same SESSION")
		seen[path.SenderTemplate.LSPID] = true
	}
	assert.Len(t, seen, 3, "three distinct LSP_IDs")
}

// RFC requirement: RFC3209-6-2 positive -- an egress that receives a PATH answers with a RESV whose STYLE is Shared Explicit, option vector 0x12.
func TestRFC3209EgressResvStyleIsSharedExplicit(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Egress.String(), nil)
	psb := rfc2205PSB()
	psb.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.9/32")}}
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: buildPath(psb, rfc2205Transit, defaultIPTTL)})

	resv, dst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	assert.Equal(t, rfc2205Transit, dst)
	require.True(t, resv.HasStyle)
	assert.Equal(t, uint32(0x12), resv.Style, "Shared Explicit")
}
