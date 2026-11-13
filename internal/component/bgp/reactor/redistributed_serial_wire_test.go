// Design: docs/guide/redistribution.md -- independent redistributed announcements.
// Related: default_next_hop_controls_test.go -- wire assertions and peer fixtures.
package reactor

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	updatecmd "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/require"
)

// TestRedistributedSerialAnnouncementsKeepBothPrefixes observes the same two
// independent consumer calls as the OSPF-to-FRR scenario, after the session is
// established. Both routes must survive on TCP even though their attributes and
// gateway are identical and all build buffers can be reused between the calls.
// API acknowledgments alone cannot establish that lifetime contract.
func TestRedistributedSerialAnnouncementsKeepBothPrefixes(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65001, 65002, 0x01010101)
	peer := NewPeer(settings)
	session, client := newConnectedLocalSession(t, settings, "127.0.0.1")
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	peer.session = session
	peer.state.Store(int32(PeerStateEstablished))
	peer.upSession = session
	peer.upPhase = sessionUpDelivered

	localCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: settings.LocalAS},
	}
	remoteCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: settings.PeerAS},
	}
	// RFC 6793 Section 3: both endpoints advertise four-octet ASN support.
	negotiated := capability.Negotiate(localCaps, remoteCaps, capability.PeerIdentity{
		LocalASN: settings.LocalAS, PeerASN: settings.PeerAS,
	})
	session.negotiated = negotiated
	peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
	peer.setEncodingContexts(negotiated)

	boundary := &serialRedistributionWireBoundary{t: t, adapter: defaultNextHopControlAdapter(peer)}
	consumer := bgpredist.NewBGPConsumer(boundary)
	for _, prefix := range []string{"10.82.0.3/32", "172.30.0.0/24"} {
		consumer.InjectRoute(t.Context(), family.IPv4Unicast, configredist.RouteEntry{
			Prefix: prefix, NextHop: "172.30.0.3", Source: "ospf",
		})
	}
	require.Equal(t, 2, boundary.added, "both actual consumer calls must reach the writer")
	session.closeConn()
	var bodies [][]byte
	for {
		packet, err := core4271ReadMessage(client)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		require.Equal(t, byte(msgtype.TypeUPDATE), packet[18])
		bodies = append(bodies, defaultNextHopControlBodies(t, packet)...)
	}
	require.Len(t, bodies, 2, "each independent announcement must reach the TCP receiver")
	assertDefaultNextHopWire(t, bodies[0], netip.MustParseAddr("127.0.0.1"), []byte{32, 10, 82, 0, 3})
	assertDefaultNextHopWire(t, bodies[1], netip.MustParseAddr("127.0.0.1"), []byte{24, 172, 30, 0})
	for _, body := range bodies {
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		_, _, pathWire, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrASPath)
		require.True(t, found, "each independently received route needs its AS_PATH")
		path, err := attribute.ParseASPath(pathWire, true)
		require.NoError(t, err)
		require.Len(t, path.Segments, 1)
		require.Equal(t, []uint32{65001}, path.Segments[0].ASNs)
	}
}

// serialRedistributionWireBoundary uses the parser and final announcement API;
// it does not substitute an acknowledgment for an actual send.
type serialRedistributionWireBoundary struct {
	t       *testing.T
	adapter *reactorAPIAdapter
	added   int
}

func (b *serialRedistributionWireBoundary) UpdateRoute(ctx context.Context, selected, command string) (uint32, uint32, error) {
	b.t.Helper()
	require.Equal(b.t, "*", selected)
	text, found := strings.CutPrefix(command, "update text ")
	require.True(b.t, found)
	parsed, err := updatecmd.ParseSendableUpdateText(strings.Fields(text))
	if err != nil {
		return 0, 0, err
	}
	require.Len(b.t, parsed.Groups, 1)
	group := parsed.Groups[0]
	require.Len(b.t, group.Announce, 1)
	require.Empty(b.t, group.Withdraw)
	batch := bgptypes.NLRIBatch{
		Family: group.Family, NLRIs: group.Announce, NextHop: group.NextHop,
		Wire: group.Wire, OriginAS: group.OriginAS,
	}
	// RFC 4271 Section 4.3: exercise the final announcement writer.
	if err := b.adapter.AnnounceNLRIBatch(ctx, selector.All(), batch, plugin.OperatorSender()); err != nil {
		return 0, 0, err
	}
	b.added++
	return 1, 0, nil
}
