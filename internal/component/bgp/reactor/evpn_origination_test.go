package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

func TestEVPNAPIOriginChecksRouteAfterPathIdentifier(t *testing.T) {
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	encoded := []byte{3, 17, 0, 0, 0xfd, 0xe8, 0, 0, 0, 7, 0, 0, 0, 0, 32, 192, 0, 2, 1}
	withPath := append([]byte{3, 0, 0, 1}, encoded...)
	route, err := nlri.NewWireNLRI(fam, withPath, true)
	require.NoError(t, err)
	attrs := attribute.NewBuilder()
	attrs.AddExtendedCommunity(attribute.ExtendedCommunity{0, 2, 0xfd, 0xe8, 0, 0, 0, 100})
	batch := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{route}, Attrs: attrs}
	facts := announceFacts{nextHop: netip.MustParseAddr("192.0.2.1"), isIBGP: true, asn4: true, addPath: true}
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	update, err := adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen), batch, facts)
	require.NoError(t, err)
	_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, found)
	require.Equal(t, withPath, reach[5+int(reach[3]):])

	batch.Attrs = attribute.NewBuilder()
	update, err = adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen), batch, facts)
	require.Error(t, err)
	require.Nil(t, update)
}

func TestQueuedEVPNOriginRefusesInvalidRouteBeforeDraining(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "false")
	peer.state.Store(int32(PeerStateActive))
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{fam: true}})
	adapter := groupUpdatesReactor([]*Peer{peer}, false)
	encoded := []byte{3, 17, 0, 0, 0xfd, 0xe8, 0, 0, 0, 7, 0, 0, 0, 0, 32, 192, 0, 2, 1}
	route, err := nlri.NewWireNLRI(fam, encoded, false)
	require.NoError(t, err)
	batch := bgptypes.NLRIBatch{
		Family: fam, NLRIs: []nlri.NLRI{route}, Attrs: attribute.NewBuilder(),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}
	require.ErrorIs(t, adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()), message.ErrEVPNOrigination)

	batch.Attrs.AddExtendedCommunity(attribute.ExtendedCommunity{0, 2, 0xfd, 0xe8, 0, 0, 0, 100})
	require.NoError(t, adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))
	peer.state.Store(int32(PeerStateEstablished))
	peer.drainAndCloseQueueGate("10.0.0.2", message.MaxMsgLen)
	frames := updateBodies(t, conn.written())
	require.Len(t, frames, 1, "only the valid announcement may leave the queue")
	announced, found := mpReachNLRIOf(t, frames[0])
	require.True(t, found)
	require.Equal(t, encoded, announced)
}

// VALIDATES: typed Type 4 announcements retain ES-Import authorization while
// queued and preserve both the route and its community when the peer connects.
// PREVENTS: the queued origin gate dropping the dedicated ES-Import community.
func TestQueuedEVPNType4PreservesESImportThroughDrain(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "false")
	peer.state.Store(int32(PeerStateActive))
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{fam: true}})
	adapter := groupUpdatesReactor([]*Peer{peer}, false)
	encoded := []byte{
		4, 23,
		0, 1, 192, 0, 2, 1, 0, 7,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		32, 198, 51, 100, 1,
	}
	route, err := nlri.NewWireNLRI(fam, encoded, false)
	require.NoError(t, err)
	esImport := attribute.ExtendedCommunity{6, 2, 0, 1, 2, 3, 4, 5}
	attrs := attribute.NewBuilder()
	attrs.AddExtendedCommunity(esImport)
	batch := bgptypes.NLRIBatch{
		Family: fam, NLRIs: []nlri.NLRI{route}, Attrs: attrs,
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}
	require.NoError(t, adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))
	require.Empty(t, conn.written(), "a disconnected peer must retain the route until drain")
	peer.state.Store(int32(PeerStateEstablished))
	peer.drainAndCloseQueueGate("10.0.0.2", message.MaxMsgLen)
	frames := updateBodies(t, conn.written())
	require.Len(t, frames, 1)
	announced, found := mpReachNLRIOf(t, frames[0])
	require.True(t, found)
	require.Equal(t, encoded, announced)
	sections, err := wire.ParseUpdateSections(frames[0])
	require.NoError(t, err)
	_, _, communities, found := attribute.AttrFind(sections.Attrs(frames[0]), attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, esImport[:], communities)
}
