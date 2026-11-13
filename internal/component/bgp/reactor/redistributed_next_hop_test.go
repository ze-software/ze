package reactor

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	updatecmd "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/require"
)

// redistributionNextHopBoundary reaches the same command parser, queue and
// destination resolver as a consumer's update-route request. It keeps the
// resolved result observable without a BGP neighbor or a second route producer.
type redistributionNextHopBoundary struct {
	t      *testing.T
	peer   *Peer
	queued bool
	got    netip.Addr
	count  int
}

func (b *redistributionNextHopBoundary) UpdateRoute(ctx context.Context, sel, command string) (uint32, uint32, error) {
	b.t.Helper()
	require.Equal(b.t, "*", sel)
	words := strings.Fields(command)
	require.GreaterOrEqual(b.t, len(words), 2)
	require.Equal(b.t, []string{"update", "text"}, words[:2])
	parsed, err := updatecmd.ParseSendableUpdateText(words[2:])
	if err != nil {
		return 0, 0, err
	}
	require.Len(b.t, parsed.Groups, 1)
	group := parsed.Groups[0]
	require.Len(b.t, group.Announce, 1, "a correct next hop must still announce the route")
	nextHop := group.NextHop
	if b.queued {
		api := &reactorAPIAdapter{}
		batch := bgptypes.NLRIBatch{Family: group.Family, NLRIs: group.Announce, NextHop: nextHop}
		_, err = api.queueAnnounceForPeer(ctx, b.peer, batch, nil, nil, b.peer.IsIBGP())
		if err != nil {
			return 0, 0, err
		}
		require.Len(b.t, b.peer.opQueue, 1)
		nextHop = queuedNextHopPolicy(b.peer.opQueue[0])
	}
	b.got, err = b.peer.resolveNextHop(b.peer.session, nextHop, group.Family)
	if err != nil {
		return 0, 0, err
	}
	b.count++
	return 1, 0, nil
}

// TestRedistributedGatewayUsesDestinationNextHopPolicy sends an actual BGP
// consumer command through the announcement parser and destination resolver,
// including the real queue when selected. It distinguishes a route's IGP
// gateway from an explicitly chosen BGP next hop.
//
// VALIDATES: an external peer gets this session's endpoint, an internal peer
// gets the eligible gateway, and a gateway equal to the internal peer is
// replaced with self. Configured self and explicit overrides remain effective.
// PREVENTS: the direct or queued rail advertising an unreachable IGP gateway
// to an external neighbor, or advertising an internal peer as its own next hop.
func TestRedistributedGatewayUsesDestinationNextHopPolicy(t *testing.T) {
	for _, queued := range []bool{false, true} {
		rail := "direct"
		if queued {
			rail = "queued"
		}
		t.Run(rail, func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				peerAS   uint32
				gateway  string
				mode     uint8
				explicit string
				want     string
			}{
				{name: "external", peerAS: 65002, gateway: "10.1.0.2", want: "127.0.0.1"},
				{name: "internal", peerAS: 65001, gateway: "10.1.0.2", want: "10.1.0.2"},
				{name: "internal gateway is peer", peerAS: 65001, gateway: "192.0.2.2", want: "127.0.0.1"},
				{name: "configured self", peerAS: 65001, gateway: "10.1.0.2", mode: NextHopSelf, want: "127.0.0.1"},
				{name: "configured explicit", peerAS: 65002, gateway: "10.1.0.2", mode: NextHopExplicit, explicit: "192.0.2.9", want: "192.0.2.9"},
				{name: "configured unchanged", peerAS: 65002, gateway: "10.1.0.2", mode: NextHopUnchanged, want: "10.1.0.2"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, tc.peerAS, 0x01010101)
					settings.NextHopMode = tc.mode
					if tc.explicit != "" {
						settings.NextHopAddress = netip.MustParseAddr(tc.explicit)
					}
					peer := NewPeer(settings)
					peer.session, _ = newConnectedLocalSession(t, settings, "127.0.0.1")
					boundary := &redistributionNextHopBoundary{t: t, peer: peer, queued: queued}
					bgpredist.NewBGPConsumer(boundary).InjectRoute(t.Context(), family.IPv4Unicast, configredist.RouteEntry{
						Prefix: "10.0.0.9/32", NextHop: tc.gateway, Source: "ospf",
					})
					require.Equal(t, 1, boundary.count, "the route must reach the announcement resolver")
					require.Equal(t, netip.MustParseAddr(tc.want), boundary.got)
				})
			}
		})
	}
}

// VALIDATES: an explicit API next hop remains explicit even when a session
// would choose self for a redistributed route's default policy.
// PREVENTS: fixing redistribution by changing existing API/ExaBGP announcements.
func TestExplicitOriginationNextHopKeepsItsAddress(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, 65002, 0x01010101)
	settings.NextHopMode = NextHopSelf
	peer := NewPeer(settings)
	peer.session, _ = newConnectedLocalSession(t, settings, "127.0.0.1")
	boundary := &redistributionNextHopBoundary{t: t, peer: peer}
	_, _, err := boundary.UpdateRoute(t.Context(), "*", "update text origin incomplete nhop 10.1.0.2 nlri add 10.0.0.9/32")
	require.NoError(t, err)
	require.Equal(t, 1, boundary.count)
	require.Equal(t, netip.MustParseAddr("10.1.0.2"), boundary.got)
}
