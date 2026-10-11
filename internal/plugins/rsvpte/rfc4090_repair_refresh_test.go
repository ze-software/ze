// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- control traffic during local repair
package rsvpte

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepairedLSPKeepsUpstreamRefresh reproduces the freeRouter lab failure in
// plr-backup-path-to-egress-merge-point. After the protected link fails, the
// ingress keeps refreshing the PATH it always sent, whose strict hop is the
// merge point across the failed link. Native routing now reaches that node only
// through the bypass side, so resolving the ERO again would refuse the refresh
// as a Bad strict node. The PLR MUST instead keep the LSP on the bypass: no
// PathErr upstream, the refresh relayed through the bypass under the PLR's
// sender, and the merge point's RESV still accepted.
//
// RFC requirement: RFC4090-6.4.3-2 positive — under local repair the PLR keeps sending the protected LSP's control traffic onto the bypass tunnel: an upstream PATH refresh naming the accepted route draws no PathErr, is relayed once through the bypass under the PLR's sender, and the merge point's RESV is still accepted.
func TestRepairedLSPKeepsUpstreamRefresh(t *testing.T) {
	e, ft, fib, selected, _ := twoBypassPLR(t)
	protected, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	protected.mu.Lock()
	inUse := protected.ProtectionInUse
	protected.mu.Unlock()
	require.True(t, inUse, "fixture: the link failure repaired the protected LSP")

	// The failed link took the connected route with it: the merge point is now
	// reached through the bypass next hop, which is not the strict hop's node.
	bypassHop := netip.MustParseAddr("10.0.1.3")
	ft.mu.Lock()
	ft.routes = map[netip.Prefix]RouteInfo{
		netip.MustParsePrefix("10.0.0.3/32"): {NextHop: bypassHop, Lookup: bypassHop, MTU: 1500},
	}
	ft.mu.Unlock()

	before := sentCount(ft)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	ft.mu.Lock()
	sent := slices.Clone(ft.sent[before:])
	ft.mu.Unlock()
	relayed := 0
	for _, message := range sent {
		decoded := mustDecode(t, message.payload)
		require.NotEqual(t, MsgTypePathErr, decoded.Header.MsgType, "the refresh of a repaired LSP is not a routing error")
		if decoded.Header.MsgType != MsgTypePath {
			continue
		}
		relayed++
		assert.Equal(t, bypassTableID(selected), message.route.TableID, "the refresh travels the bypass")
		assert.Equal(t, e.cfg().RouterID, decoded.SenderTemplate.SenderAddr, "the refresh carries the PLR's backup sender")
	}
	assert.Equal(t, 1, relayed, "the refresh is relayed once, through the bypass")

	protected.mu.Lock()
	inUse = protected.ProtectionInUse
	protected.mu.Unlock()
	assert.True(t, inUse, "the refresh leaves the repair in place")

	filter := psb.SenderTemplate
	filter.SenderAddr = e.cfg().RouterID
	rsb := &resvStateBlock{Session: psb.Session, Label: labelObject{Label: 24000}, Style: StyleSharedExplicit}
	errorsBefore := ft.countByType(MsgTypeResvErr)
	e.handlePacket(Packet{Src: selected.TunnelEndpoint, Payload: buildResv(rsb, filter, DefaultRefreshPeriod, selected.TunnelEndpoint)})
	assert.Equal(t, errorsBefore, ft.countByType(MsgTypeResvErr), "the merge point's RESV still names a known sender")
	assert.Equal(t, []uint32{5000, 24000}, fib.backups[len(fib.backups)-1].out)
}

// TestRepairedLSPChangedEROResolvedAgain checks the other side of the same
// rule: only a refresh repeating the accepted route keeps the pre-failure
// selection. Each case repairs the protected LSP, moves native routing off the
// failed link, then refreshes the PATH with a different ERO: one that drops the
// first downstream hop, and one that names another strict hop. Each is a new
// route request, so it is resolved again; the strict hop is not adjacent, so
// the PLR answers a Routing Problem / Bad strict node PathErr and relays no
// PATH. A PLR that matched the refresh to the stored route would relay it
// through the bypass instead.
//
// RFC requirement: RFC4090-6.4.3-2 negative — the control traffic carried onto the bypass is the protected LSP's own: a refresh under repair whose ERO differs from the accepted one, a shorter tail or another strict hop, is not relayed on the old selection but resolved again and refused as a Bad strict node.
func TestRepairedLSPChangedEROResolvedAgain(t *testing.T) {
	plr := netip.MustParsePrefix("10.0.0.2/32")
	egress := netip.MustParsePrefix("10.0.0.9/32")
	cases := []struct {
		name string
		ero  []eroHop
	}{
		{name: "first downstream hop dropped", ero: []eroHop{{Address: plr}, {Address: egress}}},
		{name: "another strict hop", ero: []eroHop{{Address: plr}, {Address: netip.MustParsePrefix("10.0.0.4/32")}, {Address: egress}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, ft, _, _, _ := twoBypassPLR(t)
			protected, ok := e.table.Get(protectedKey())
			require.True(t, ok)
			protected.mu.Lock()
			inUse := protected.ProtectionInUse
			protected.mu.Unlock()
			require.True(t, inUse, "fixture: the link failure repaired the protected LSP")

			bypassHop := netip.MustParseAddr("10.0.1.3")
			ft.mu.Lock()
			ft.routes = map[netip.Prefix]RouteInfo{
				netip.MustParsePrefix("10.0.0.3/32"): {NextHop: bypassHop, Lookup: bypassHop, MTU: 1500},
				egress:                               {NextHop: bypassHop, Lookup: bypassHop, MTU: 1500},
			}
			ft.mu.Unlock()

			pathsBefore := ft.countByType(MsgTypePath)
			ingress := netip.MustParseAddr("10.0.0.1")
			psb := protectedTransitPSB(&protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8, SetupPrio: 7, HoldPrio: 7})
			psb.ERO = tc.ero
			e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

			assert.Equal(t, pathsBefore, ft.countByType(MsgTypePath), "a changed route is not relayed on the old selection")
			pathErr, dst, sent := ft.lastByType(MsgTypePathErr)
			require.True(t, sent, "the changed route is resolved again and refused")
			assert.Equal(t, ingress, dst)
			assert.Equal(t, ErrCodeRoutingProblem, pathErr.ErrorSpec.ErrorCode)
			assert.Equal(t, ErrValueBadStrictNode, pathErr.ErrorSpec.ErrorValue)
		})
	}
}
