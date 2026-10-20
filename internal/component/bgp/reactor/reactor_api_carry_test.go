// Design: docs/architecture/api/update-syntax.md -- which peers a route is for
// Related: reactor_api_carry.go -- SelectedPeersCarry

package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// carryAdapter holds one peer per configured capability list, each down.
func carryAdapter(peerCaps ...[]capability.Capability) *reactorAPIAdapter {
	peers := make(map[netip.AddrPort]*Peer, len(peerCaps))
	for i, caps := range peerCaps {
		settings := &PeerSettings{
			Connection:   ConnectionBoth,
			Address:      netip.AddrFrom4([4]byte{10, 0, 0, byte(2 + i)}),
			LocalAS:      65000,
			PeerAS:       65001,
			RouterID:     0x01020301,
			Capabilities: caps,
		}
		peers[settings.PeerKey()] = NewPeer(settings)
	}
	return &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}, peers: peers}}
}

// TestSelectedPeersCarryReadsTheConfiguredFamily pins that the answer is the
// families ze's OPEN offers each selected peer, whatever the session state.
//
// VALIDATES: a down peer configured for IPv4 unicast only refuses IPv6 unicast
// with route.ErrNoPeerConfiguredFamily and admits IPv4 unicast; a second peer
// configured for IPv6 unicast admits it; a peer with no family block and no
// plugin family admits IPv4 unicast alone.
// PREVENTS: a route queued, and answered accepted, for a down session whose
// OPEN never offers its family, so no session can ever carry it.
func TestSelectedPeersCarryReadsTheConfiguredFamily(t *testing.T) {
	ipv4 := []capability.Capability{&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}}
	ipv6 := []capability.Capability{&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}}
	op := plugin.OperatorSender()
	v4, v6 := uint16(family.AFIIPv4), uint16(family.AFIIPv6)
	unicast := uint8(family.SAFIUnicast)

	one := carryAdapter(ipv4)
	err := one.SelectedPeersCarry(selector.All(), v6, unicast, op)
	require.ErrorIs(t, err, route.ErrNoPeerConfiguredFamily)
	assert.Contains(t, err.Error(), "ipv6/unicast")
	require.NoError(t, one.SelectedPeersCarry(selector.All(), v4, unicast, op))

	two := carryAdapter(ipv4, ipv6)
	require.NoError(t, two.SelectedPeersCarry(selector.All(), v6, unicast, op))

	bare := carryAdapter(nil)
	require.NoError(t, bare.SelectedPeersCarry(selector.All(), v4, unicast, op))
	require.ErrorIs(t, bare.SelectedPeersCarry(selector.All(), v6, unicast, op), route.ErrNoPeerConfiguredFamily)

	none := carryAdapter()
	require.ErrorIs(t, none.SelectedPeersCarry(selector.All(), v4, unicast, op), route.ErrNoPeersMatch)
}
