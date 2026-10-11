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
