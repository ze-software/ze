// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 2205 soft state,
// reverse routing and error handling, driven through the engine's own entry
// points: handlePacket, refreshPaths, cleanupTick and teardownLSP.
//
// VALIDATES: RFC 2205 Sections 2, 2.3, 3.1.1, 3.1.5, 3.1.6, 3.10, 3.11.5 and Appendix B at the
// engine, not at a helper: what the node sends, where, and what state it keeps.
package rsvpte

import (
	"bytes"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// rfc2205Multiplier is the refresh multiplier every soft-state test here runs
// with, so the cleanup timeout is three refresh periods.
const rfc2205Multiplier = 3

// softStateEgress returns an egress holding one admitted LSP of 3.2e9 bits/s
// on eth0, with a cleanup timeout of rfc2205Multiplier refresh periods.
func softStateEgress(t *testing.T) (*engine, *pathStateBlock) {
	t.Helper()
	e, _, _ := testEngine(t, rfc2205Egress.String(), func(c *rsvpteConfig) {
		c.RefreshMultiplier = rfc2205Multiplier
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

// ageLSP moves the last refresh of the LSP's path state back by age.
func ageLSP(t *testing.T, e *engine, key lspKey, age time.Duration) {
	t.Helper()
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	lsp.PSB.LastRefresh = time.Now().Add(-age)
	lsp.mu.Unlock()
}

// RFC requirement: RFC2205-2.3-1 positive -- refresh is periodic: each refreshPaths tick re-sends the ingress LSP's PATH, three ticks send three more PATHs.
func TestRFC2205RefreshRecursEachTick(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Ingress.String(), nil)
	psb := rfc2205PSB()
	setupTunnel(e.log, e.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, e.cfg(), e)
	before := ft.countByType(MsgTypePath)
	require.Equal(t, 1, before, "ingress signaled the PATH")

	for tick := 1; tick <= 3; tick++ {
		refreshPaths(slogutil.DiscardLogger(), e.table, e)
		assert.Equal(t, before+tick, ft.countByType(MsgTypePath), "tick %d re-sends the PATH", tick)
	}
}

// RFC requirement: RFC2205-2.3-1 positive -- path state that no refresh reached within the cleanup timeout (three refresh periods) is deleted by cleanupTick, and its reservation with it.
// RFC requirement: RFC2205-3-14 positive -- deleting path state by timeout releases the bandwidth the reservation held on the admission interface.
func TestRFC2205PathStateDeletedAtCleanupTimeout(t *testing.T) {
	e, psb := softStateEgress(t)
	ageLSP(t, e, keyFromPSB(psb), (rfc2205Multiplier+1)*DefaultRefreshPeriod)

	cleanupTick(slogutil.DiscardLogger(), e.table, e.cfg(), e, time.Now(), DefaultRefreshPeriod)

	assert.Empty(t, e.table.All(), "unrefreshed path state deleted")
	ib, _ := e.admission.GetInterface("eth0")
	assert.InDelta(t, 0, ib.ReservedBandwidth, 1, "reservation released with the path state")
}

// RFC requirement: RFC2205-2.3-1 negative -- path state whose matching PATH refresh arrived is not deleted: an LSP aged past the cleanup timeout and then refreshed by the same PATH survives cleanupTick.
// RFC requirement: RFC2205-3-14 negative -- path state that does not time out keeps its reservation: the refreshed LSP still holds its bandwidth after cleanupTick.
func TestRFC2205RefreshedPathStateKept(t *testing.T) {
	e, psb := softStateEgress(t)
	key := keyFromPSB(psb)
	ageLSP(t, e, key, (rfc2205Multiplier+1)*DefaultRefreshPeriod)
	refresh := psb
	refresh.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.9/32")}}
	e.handlePacket(Packet{Src: rfc2205Transit, Payload: buildPath(refresh, rfc2205Transit, defaultIPTTL)})

	cleanupTick(slogutil.DiscardLogger(), e.table, e.cfg(), e, time.Now(), DefaultRefreshPeriod)

	_, ok := e.table.Get(key)
	assert.True(t, ok, "refreshed path state kept")
	ib, _ := e.admission.GetInterface("eth0")
	assert.InDelta(t, 3.2e9, ib.ReservedBandwidth, 1, "reservation kept")
}

// transitWithResv returns a transit holding path state from the ingress and
// reservation state from the egress, with a cleanup timeout of
// rfc2205Multiplier refresh periods, and the RESV the egress sent.
func transitWithResv(t *testing.T) (*engine, *fakeTransport, *pathStateBlock, []byte) {
	t.Helper()
	e, ft, _ := testEngine(t, rfc2205Transit.String(), func(c *rsvpteConfig) { c.RefreshMultiplier = rfc2205Multiplier })
	psb := rfc2205PSB()
	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPath(psb, rfc2205Ingress, defaultIPTTL)})
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	resv := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resv})
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok, "transit installed path state")
	lsp.mu.Lock()
	require.NotNil(t, lsp.RSB, "the RESV installed reservation state")
	lsp.mu.Unlock()
	return e, ft, psb, resv
}

// ageRSB moves the last refresh of the LSP's reservation state back by age.
func ageRSB(t *testing.T, e *engine, key lspKey, age time.Duration) {
	t.Helper()
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	lsp.RSB.LastRefresh = time.Now().Add(-age)
	lsp.mu.Unlock()
}

// RFC requirement: RFC2205-2.3-1 positive -- reservation state that no RESV refreshed within the cleanup timeout is deleted by cleanupTick while its path state lives: a transit drops the RSB, keeps the PSB and sends a ResvTear to the previous hop; an ingress drops the RSB and falls back from Up to PathSent.
func TestRFC2205ResvStateDeletedAtCleanupTimeout(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)
	key := keyFromPSB(psb)
	ageRSB(t, e, key, (rfc2205Multiplier+1)*DefaultRefreshPeriod)

	cleanupTick(slogutil.DiscardLogger(), e.table, e.cfg(), e, time.Now(), DefaultRefreshPeriod)

	lsp, ok := e.table.Get(key)
	require.True(t, ok, "path state outlives the reservation")
	lsp.mu.Lock()
	assert.Nil(t, lsp.RSB, "unrefreshed reservation state deleted")
	assert.NotNil(t, lsp.PSB, "refreshed path state kept")
	lsp.mu.Unlock()
	tear, dst, sent := ft.lastByType(MsgTypeResvTear)
	require.True(t, sent, "the timed-out reservation is torn upstream")
	assert.Equal(t, rfc2205Ingress, dst)
	assert.Equal(t, psb.Session, tear.Session)

	ingress, _, _ := testEngine(t, rfc2205Ingress.String(), func(c *rsvpteConfig) { c.RefreshMultiplier = rfc2205Multiplier })
	setupTunnel(ingress.log, ingress.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, ingress.cfg(), ingress)
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	ingress.handlePacket(Packet{Src: rfc2205Transit, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Transit)})
	ingressLSP, ok := ingress.table.Get(key)
	require.True(t, ok)
	ingressLSP.mu.Lock()
	require.Equal(t, LSPStateUp, ingressLSP.State, "the RESV brought the LSP up")
	ingressLSP.mu.Unlock()
	ageRSB(t, ingress, key, (rfc2205Multiplier+1)*DefaultRefreshPeriod)

	refreshPaths(slogutil.DiscardLogger(), ingress.table, ingress)
	cleanupTick(slogutil.DiscardLogger(), ingress.table, ingress.cfg(), ingress, time.Now(), DefaultRefreshPeriod)

	ingressLSP.mu.Lock()
	defer ingressLSP.mu.Unlock()
	assert.Nil(t, ingressLSP.RSB, "unrefreshed reservation state deleted at the ingress")
	assert.Equal(t, LSPStatePathSent, ingressLSP.State, "the LSP is not up on a reservation that timed out")
}

// RFC requirement: RFC2205-2.3-1 negative -- reservation state whose matching RESV refresh arrived is not deleted: an RSB aged past the cleanup timeout and then refreshed by the same RESV survives cleanupTick, and no ResvTear is sent.
func TestRFC2205RefreshedResvStateKept(t *testing.T) {
	e, ft, psb, resv := transitWithResv(t)
	key := keyFromPSB(psb)
	ageRSB(t, e, key, (rfc2205Multiplier+1)*DefaultRefreshPeriod)
	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resv})

	cleanupTick(slogutil.DiscardLogger(), e.table, e.cfg(), e, time.Now(), DefaultRefreshPeriod)

	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	assert.NotNil(t, lsp.RSB, "refreshed reservation state kept")
	lsp.mu.Unlock()
	_, _, torn := ft.lastByType(MsgTypeResvTear)
	assert.False(t, torn, "no ResvTear for a live reservation")
}

// logEngine returns an egress engine whose log records land in the returned buffer.
func logEngine(t *testing.T) (*engine, *fakeTransport, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	ft := newFakeTransport()
	ft.localAddresses = []netip.Addr{rfc2205Egress}
	c := rsvpteConfig{RouterID: rfc2205Egress, RefreshPeriod: DefaultRefreshPeriod}
	e := newEngine(ft, newLSPTable(), newAdmissionController(), &fakeFIB{}, c, slog.New(slog.NewTextHandler(&logs, nil)))
	return e, ft, &logs
}

// RFC requirement: RFC2205-3.1.3-1 negative -- a malformed PATH (no TIME_VALUES) is logged locally: handlePacket writes a WARN record naming the source and the decode error, sends nothing, and keeps no state.
func TestRFC2205MalformedMessageLoggedLocally(t *testing.T) {
	e, ft, logs := logEngine(t)
	raw := encodeWithout(MsgTypePath, conformantMessages()[MsgTypePath], "TIME_VALUES")

	e.handlePacket(Packet{Src: rfc2205Transit, Payload: raw})

	record := logs.String()
	assert.Contains(t, record, "level=WARN")
	assert.Contains(t, record, "decode failed")
	assert.Contains(t, record, "src="+rfc2205Transit.String())
	assert.Contains(t, record, "TIME_VALUES")
	assert.Zero(t, sentCount(ft), "the error is not reported on the wire")
	assert.Empty(t, e.table.All())
}

// RFC requirement: RFC2205-3.1.3-1 positive -- a well-formed PATH is processed with no WARN record: the egress answers it with a RESV.
func TestRFC2205WellFormedMessageNotLogged(t *testing.T) {
	e, ft, logs := logEngine(t)

	e.handlePacket(Packet{Src: rfc2205Transit, Payload: encodeWithout(MsgTypePath, conformantMessages()[MsgTypePath], "")})

	assert.NotContains(t, logs.String(), "level=WARN")
	_, _, ok := ft.lastByType(MsgTypeResv)
	assert.True(t, ok, "egress answers the PATH")
}

// unknownClassObject is an object of Class-Num 100 (0b01100100), a class of
// the form 0bbbbbbb that ze does not implement.
func unknownClassObject(b []byte) int {
	copy(b, []byte{0, 8, 100, 1, 0, 0, 0, 0})
	return 8
}

// RFC requirement: RFC2205-3.10-1 negative -- a PATH carrying an object of unknown Class-Num 0bbbbbbb is rejected whole: the egress keeps no path state, sends no RESV, and returns a PathErr to the previous hop with Error Code 13 (Unknown object class) and Error Value Class-Num<<8|C-Type.
func TestRFC2205UnknownClassPathRejectedWithError(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Egress.String(), nil)
	encoders := []objEncoder{}
	for _, obj := range conformantMessages()[MsgTypePath] {
		encoders = append(encoders, obj.encode)
	}
	encoders = append(encoders, unknownClassObject)

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: encodeMessage(MsgTypePath, defaultIPTTL, encoders)})

	assert.Empty(t, e.table.All(), "no path state")
	assert.Zero(t, ft.countByType(MsgTypeResv), "no RESV")
	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "a PathErr is returned")
	assert.Equal(t, rfc2205Ingress, dst)
	assert.Equal(t, uint8(13), perr.ErrorSpec.ErrorCode, "Unknown Object Class")
	assert.Equal(t, uint16(100<<8|1), perr.ErrorSpec.ErrorValue)
}

// RFC requirement: RFC2205-3.10-1 negative -- a RESV carrying an object of unknown Class-Num 0bbbbbbb is rejected whole: the transit relays no RESV upstream, keeps its LSP unreserved, and returns a ResvErr to the next hop with Error Code 13.
func TestRFC2205UnknownClassResvRejectedWithError(t *testing.T) {
	e, ft := constructionTransit(t)
	encoders := []objEncoder{}
	// The unknown object goes before STYLE: after the flow descriptor list it
	// would be a placement error, which is a different refusal.
	for _, obj := range conformantMessages()[MsgTypeResv] {
		if obj.name == "STYLE" {
			encoders = append(encoders, unknownClassObject)
		}
		encoders = append(encoders, obj.encode)
	}

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: encodeMessage(MsgTypeResv, defaultIPTTL, encoders)})

	assert.Zero(t, ft.countByType(MsgTypeResv), "no RESV relayed upstream")
	for _, lsp := range e.table.All() {
		lsp.mu.Lock()
		assert.Nil(t, lsp.RSB, "no reservation state")
		lsp.mu.Unlock()
	}
	resvErr, dst, ok := ft.lastByType(MsgTypeResvErr)
	require.True(t, ok, "a ResvErr is returned")
	assert.Equal(t, rfc2205Egress, dst)
	assert.Equal(t, uint8(13), resvErr.ErrorSpec.ErrorCode, "Unknown Object Class")
}

// RFC requirement: RFC2205-3-12 positive -- a PathTear leaves on exactly the route of its PATH: the whole PathRoute (source, destination, next hop, lookup, interface, table) of the PathTear a transit relays and of the PathTear an ingress originates at teardown equals the PathRoute of the PATH the same node sent.
func TestRFC2205PathTearRouteEqualsPathRoute(t *testing.T) {
	transit, transitTransport, psb := rfc2205TransitWithPath(t)
	transit.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPathTear(psb, rfc2205Ingress)})
	paths, tears := sentRoutes(t, transitTransport, MsgTypePath), sentRoutes(t, transitTransport, MsgTypePathTear)
	require.Len(t, paths, 1)
	require.Len(t, tears, 1)
	assert.Equal(t, paths[0], tears[0], "relayed PathTear route")

	ingress, ingressTransport, _ := testEngine(t, rfc2205Ingress.String(), nil)
	setupTunnel(ingress.log, ingress.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, ingress.cfg(), ingress)
	ingress.teardownLSP(keyFromPSB(psb))
	paths, tears = sentRoutes(t, ingressTransport, MsgTypePath), sentRoutes(t, ingressTransport, MsgTypePathTear)
	require.Len(t, paths, 1)
	require.Len(t, tears, 1, "the ingress originates a PathTear at teardown")
	assert.Equal(t, paths[0], tears[0], "originated PathTear route")
}

// TestRFC2205PathTearAddressedFromSenderToSession checks the two addresses of
// a PathTear: the transit that relays one and the ingress that originates one
// both send it to the session DestAddress from the sender address of the path
// state being torn down.
//
// RFC requirement: RFC2205-3.1.5-1 positive -- the PathTear a transit relays and the PathTear an ingress originates each carry IP destination = the SESSION tunnel endpoint and IP source = the SENDER_TEMPLATE sender address of the torn-down path state.
func TestRFC2205PathTearAddressedFromSenderToSession(t *testing.T) {
	transit, transitTransport, psb := rfc2205TransitWithPath(t)
	transit.handlePacket(Packet{Src: rfc2205Ingress, Payload: buildPathTear(psb, rfc2205Ingress)})
	tears := sentRoutes(t, transitTransport, MsgTypePathTear)
	require.Len(t, tears, 1, "the transit relays the PathTear")
	assert.Equal(t, psb.Session.TunnelEndpoint, tears[0].Destination, "relayed: session DestAddress")
	assert.Equal(t, psb.SenderTemplate.SenderAddr, tears[0].Source, "relayed: sender address")

	ingress, ingressTransport, _ := testEngine(t, rfc2205Ingress.String(), nil)
	setupTunnel(ingress.log, ingress.table, tunnelConfig{Destination: psb.Session.TunnelEndpoint, TunnelID: psb.Session.TunnelID, ERO: psb.ERO, Bandwidth: 3.2e9}, ingress.cfg(), ingress)
	ingress.teardownLSP(keyFromPSB(psb))
	tears = sentRoutes(t, ingressTransport, MsgTypePathTear)
	require.Len(t, tears, 1, "the ingress originates a PathTear at teardown")
	assert.Equal(t, psb.Session.TunnelEndpoint, tears[0].Destination, "originated: session DestAddress")
	assert.Equal(t, psb.SenderTemplate.SenderAddr, tears[0].Source, "originated: sender address")
}

// TestRFC2205PathTearSourceNotTakenFromCarrier pushes the relay toward the
// wrong source: the PathTear reaches the transit from another interface
// address than the sender's. The relayed PathTear still leaves from the sender
// address of the path state, never from the carrier's source or the transit's
// own address.
//
// RFC requirement: RFC2205-3.1.5-1 negative -- a PathTear that arrives with an IP source other than the sender address is relayed with IP source = the path state's sender address (not the arriving source, not the transit's router ID) and IP destination = the session DestAddress.
func TestRFC2205PathTearSourceNotTakenFromCarrier(t *testing.T) {
	transit, transitTransport, psb := rfc2205TransitWithPath(t)
	carrier := netip.MustParseAddr("10.0.0.77")
	transit.handlePacket(Packet{Src: carrier, Payload: buildPathTear(psb, rfc2205Ingress)})

	tears := sentRoutes(t, transitTransport, MsgTypePathTear)
	require.Len(t, tears, 1, "the transit relays the PathTear")
	assert.Equal(t, psb.SenderTemplate.SenderAddr, tears[0].Source, "sender address, not the carrier source")
	assert.NotEqual(t, rfc2205Transit, tears[0].Source, "never the transit's own address")
	assert.Equal(t, psb.Session.TunnelEndpoint, tears[0].Destination, "session DestAddress")
}

// twoSenderEgress returns an egress that received a PATH of one session from
// two senders, LSP-ID 1 through previous hop 10.0.0.5 and LSP-ID 2 through
// previous hop 10.0.0.6.
func twoSenderEgress(t *testing.T) (*fakeTransport, [2]netip.Addr) {
	t.Helper()
	e, ft, _ := testEngine(t, rfc2205Egress.String(), nil)
	hops := [2]netip.Addr{rfc2205Transit, netip.MustParseAddr("10.0.0.6")}
	for i, hop := range hops {
		psb := rfc2205PSB()
		psb.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.9/32")}}
		psb.SenderTemplate.LSPID = uint16(i + 1)
		e.handlePacket(Packet{Src: hop, Payload: buildPath(psb, hop, defaultIPTTL)})
	}
	require.Len(t, e.table.All(), 2, "one path state per sender")
	return ft, hops
}

// resvByHop returns, for each RESV the fake transport sent, its destination
// and the LSP-ID of every FILTER_SPEC it carries.
func resvByHop(t *testing.T, ft *fakeTransport) map[netip.Addr][]uint16 {
	t.Helper()
	ft.mu.Lock()
	defer ft.mu.Unlock()
	out := map[netip.Addr][]uint16{}
	for i := range ft.sent {
		msg, err := DecodeMessage(ft.sent[i].payload)
		require.NoError(t, err)
		if msg.Header.MsgType != MsgTypeResv {
			continue
		}
		for _, fd := range msg.FlowDescriptors {
			for _, f := range fd.Filters {
				out[ft.sent[i].dst] = append(out[ft.sent[i].dst], f.Filter.LSPID)
			}
		}
	}
	return out
}

// RFC requirement: RFC2205-2-1 positive -- a RESV goes upstream to every sender in the selection by the reverse of each data path: with two senders of one session arriving through two previous hops, the egress sends each sender's reservation to that sender's own previous hop; and a transit relays the egress RESV on to the ingress that sent the PATH.
func TestRFC2205ResvFollowsEachSendersReversePath(t *testing.T) {
	ft, hops := twoSenderEgress(t)
	byHop := resvByHop(t, ft)
	assert.Contains(t, byHop[hops[0]], uint16(1), "sender 1's reservation goes to its previous hop")
	assert.Contains(t, byHop[hops[1]], uint16(2), "sender 2's reservation goes to its previous hop")

	transit, transitTransport, psb := rfc2205TransitWithPath(t)
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 16050}, Style: StyleSharedExplicit}
	transit.handlePacket(Packet{Src: rfc2205Egress, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, rfc2205Egress)})
	relayed, dst, ok := transitTransport.lastByType(MsgTypeResv)
	require.True(t, ok, "transit relays the RESV upstream")
	assert.Equal(t, rfc2205Ingress, dst, "the next hop upstream is the sender")
	assert.Equal(t, psb.Session, relayed.Session)
}

// RFC requirement: RFC2205-2-1 negative -- a sender's reservation never leaves on another sender's reverse path: the egress sends no RESV for sender 1 to sender 2's previous hop, nor for sender 2 to sender 1's.
func TestRFC2205ResvNeverTakesAnotherSendersPath(t *testing.T) {
	ft, hops := twoSenderEgress(t)
	byHop := resvByHop(t, ft)
	assert.NotContains(t, byHop[hops[1]], uint16(1), "sender 1's reservation does not go to hop 2")
	assert.NotContains(t, byHop[hops[0]], uint16(2), "sender 2's reservation does not go to hop 1")
}

// RFC requirement: RFC2205-x-1 negative -- no ResvConf the engine emits leaves through Transport.Send, the carrier that writes no Router Alert option: the ResvConf a node originates for a satisfied confirmation request and the ResvConf a transit relays both go through SendPath, addressed to the RESV_CONFIRM receiver.
func TestRFC2205ResvConfNeverLeavesWithoutRouterAlertCarrier(t *testing.T) {
	e, ft, _ := plrEngine(t)
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr, Payload: buildPath(psb, psb.SenderTemplate.SenderAddr, 64)})
	mp := e.cfg().Bypasses[0].MergePoint
	rsb := &resvStateBlock{Session: psb.Session, FlowSpec: psb.SenderTSpec, Label: labelObject{Label: 18000},
		Style: StyleSharedExplicit, ResvConfirm: psb.Session.TunnelEndpoint}
	request := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)
	e.handlePacket(Packet{Src: mp, Payload: request})
	e.handlePacket(Packet{Src: mp, Payload: request})
	originated := sentRoutes(t, ft, MsgTypeResvConf)
	require.Len(t, originated, 1, "the node originated one ResvConf")
	assert.Equal(t, rsb.ResvConfirm, originated[0].Destination, "originated ResvConf went through SendPath")

	transit, transitTransport := constructionTransit(t)
	transit.handlePacket(Packet{Src: rfc2205Egress, Payload: encodeWithout(MsgTypeResvConf, conformantMessages()[MsgTypeResvConf], "")})
	relayed := sentRoutes(t, transitTransport, MsgTypeResvConf)
	require.Len(t, relayed, 1, "the transit relayed one ResvConf")
	assert.Equal(t, rfc2205Egress, relayed[0].Destination, "relayed ResvConf went through SendPath")
}

// resvTearWithScope returns the ResvTear the egress sends for the transit's
// reservation, carrying a SCOPE object (RFC 2205 Section A.6, IPv4 C-Type)
// that lists scoped as its one sender address. The object sits after
// RSVP_HOP, where the Section 3.1.6 grammar places it.
func resvTearWithScope(t *testing.T, e *engine, psb *pathStateBlock, scoped netip.Addr) []byte {
	t.Helper()
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	hop, style := lsp.RSB.Hop, lsp.RSB.Style
	lsp.mu.Unlock()
	scope := func(b []byte) int {
		b[0], b[1], b[2], b[3] = 0, 8, ClassScope, CTypeIPv4
		address := scoped.As4()
		copy(b[4:8], address[:])
		return 8
	}
	return encodeMessage(MsgTypeResvTear, defaultIPTTL, []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, psb.Session) },
		func(b []byte) int { return encodeRSVPHop(b, hop) },
		scope,
		func(b []byte) int { return encodeStyle(b, style) },
		func(b []byte) int {
			return encodeFlowSpec(b, ClassFlowSpec, FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8})
		},
		func(b []byte) int { return encodeFilterSpec(b, psb.SenderTemplate) },
	})
}

// TestRFC2205ResvTearWithScopeTearsDown sends a transit a ResvTear that
// carries a SCOPE object naming the torn-down sender, and checks the tear is
// processed as if the object were absent.
//
// RFC requirement: RFC2205-3.1.6-1 positive -- a ResvTear carrying a SCOPE object is accepted and acted on as a plain ResvTear: the transit answers no ResvErr, deletes the matching reservation state, keeps the path state, and sends the ResvTear on to the previous hop.
func TestRFC2205ResvTearWithScopeTearsDown(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvTearWithScope(t, e, psb, psb.SenderTemplate.SenderAddr)})

	_, _, refused := ft.lastByType(MsgTypeResvErr)
	assert.False(t, refused, "SCOPE in a ResvTear draws no ResvErr")
	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok, "path state outlives the reservation")
	lsp.mu.Lock()
	assert.Nil(t, lsp.RSB, "reservation state torn down")
	assert.NotNil(t, lsp.PSB, "path state kept")
	lsp.mu.Unlock()
	tear, dst, sent := ft.lastByType(MsgTypeResvTear)
	require.True(t, sent, "the ResvTear goes on upstream")
	assert.Equal(t, rfc2205Ingress, dst)
	assert.Equal(t, psb.Session, tear.Session)
}

// TestRFC2205ResvTearScopeNotObeyed pushes the transit toward obeying the
// SCOPE object: the object lists only 192.0.2.99, a sender the transit holds
// no state for. A node that honored SCOPE would narrow the tear to the listed
// sender and keep the reservation of the real one. Ignoring it, the transit
// tears the reservation the FILTER_SPEC names.
//
// RFC requirement: RFC2205-3.1.6-1 negative -- a SCOPE object in a ResvTear that excludes the sender the FILTER_SPEC names does not narrow the tear: that sender's reservation state is still deleted and the ResvTear still goes to its previous hop.
func TestRFC2205ResvTearScopeNotObeyed(t *testing.T) {
	e, ft, psb, _ := transitWithResv(t)
	other := netip.MustParseAddr("192.0.2.99")
	require.NotEqual(t, other, psb.SenderTemplate.SenderAddr)

	e.handlePacket(Packet{Src: rfc2205Egress, Payload: resvTearWithScope(t, e, psb, other)})

	lsp, ok := e.table.Get(keyFromPSB(psb))
	require.True(t, ok)
	lsp.mu.Lock()
	assert.Nil(t, lsp.RSB, "the scope list does not shield the named sender's reservation")
	lsp.mu.Unlock()
	_, dst, sent := ft.lastByType(MsgTypeResvTear)
	require.True(t, sent, "the tear is forwarded despite the scope list")
	assert.Equal(t, rfc2205Ingress, dst)
}

// pathChecksummed returns the PATH the ingress sends the transit, with its
// common header checksum field set to checksum, and requires that the
// message does not verify with that field, so only the zero rule can admit it.
func pathChecksummed(t *testing.T, checksum uint16) []byte {
	t.Helper()
	raw := buildPath(rfc2205PSB(), rfc2205Ingress, defaultIPTTL)
	raw[2], raw[3] = byte(checksum>>8), byte(checksum)
	require.NotZero(t, internetChecksum(raw), "the message does not verify with this checksum field")
	return raw
}

// TestRFC2205ZeroChecksumMeansNoneTransmitted sends a transit a PATH whose
// checksum field is all zero over a body whose real checksum is not, and
// checks the transit reads the zero as "no checksum" rather than as a
// checksum that fails.
//
// RFC requirement: RFC2205-3.1.1-1 positive -- a PATH whose common header checksum is all zero is accepted without verification at the engine entry point: the transit installs path state and relays the PATH toward the egress.
func TestRFC2205ZeroChecksumMeansNoneTransmitted(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: pathChecksummed(t, 0)})

	_, ok := e.table.Get(keyFromPSB(rfc2205PSB()))
	assert.True(t, ok, "path state installed from the unchecksummed PATH")
	_, dst, sent := ft.lastByType(MsgTypePath)
	require.True(t, sent, "the PATH is relayed")
	assert.Equal(t, rfc2205Egress, dst)
}

// TestRFC2205NonzeroChecksumStillVerified pushes the zero rule past its
// bound: the same PATH carries a nonzero checksum that does not verify. Only
// the all-zero value means no checksum was transmitted, so this one is
// checked and the message is dropped.
//
// RFC requirement: RFC2205-3.1.1-1 negative -- a PATH whose checksum field is nonzero is verified, and one that fails verification is dropped at the engine entry point: no path state, and no PATH relayed.
func TestRFC2205NonzeroChecksumStillVerified(t *testing.T) {
	e, ft, _ := testEngine(t, rfc2205Transit.String(), nil)

	e.handlePacket(Packet{Src: rfc2205Ingress, Payload: pathChecksummed(t, 0x1234)})

	assert.Empty(t, e.table.All(), "no path state from a PATH that fails its checksum")
	_, _, sent := ft.lastByType(MsgTypePath)
	assert.False(t, sent, "nothing relayed")
}
