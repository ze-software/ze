// Design: docs/architecture/update-building.md -- Session family authority.
package reactor

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An encoding ID groups wire-compatible writers and deliberately omits the
// negotiated family set. Each Session must observe its own useful EOR while a
// complete EOR of the other writer's family leaves its BoRR gate closed.
func TestSelectedRefreshSharedEncodingKeepsSessionFamilies(t *testing.T) {
	ipv6, six := selectedRefreshIPv6(t)
	sibling, wire := newGroupUpdatesPeer(t, six.Destination, "absent")
	sibling.settings.Port = 1179
	sibling.settings.ExportPolicy = ipv6.peer.settings.ExportPolicy
	sibling.settings.ProcessBindings = ipv6.peer.settings.ProcessBindings
	sibling.reactor = ipv6.api.r
	sibling.session.initialReplay.Store(72)
	writer := &observedBGPWriter{session: sibling.session, writer: wire}
	sibling.session.wireWriter = writer
	sibling.session.bufWriter = bufio.NewWriter(writer)
	ipv6.api.r.peers[sibling.Settings().PeerKey()] = sibling
	ipv4 := &selectedWriterFixture{api: ipv6.api, peer: sibling, conn: wire, owner: ipv6.owner}
	selectedRefreshNegotiateFamily(t, ipv4, family.IPv4Unicast)
	four := six
	four.PeerOwner, four.DestinationSession, four.Family = sibling.sourceOwner, 72, "ipv4/unicast"
	require.Equal(t, ipv6.peer.session.sendCtxID, ipv4.peer.session.sendCtxID,
		"the useful sessions must really share one encoding ID")
	require.NotEqual(t, ipv6.peer.session.negotiated.Families(), ipv4.peer.session.negotiated.Families())

	for _, tc := range []struct {
		name    string
		fixture *selectedWriterFixture
		input   rpc.SelectRefreshInput
		own     family.Family
		other   family.Family
	}{
		{"ipv6", ipv6, six, family.IPv6Unicast, family.IPv4Unicast},
		{"ipv4", ipv4, four, family.IPv4Unicast, family.IPv6Unicast},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.fixture.peer.session.SendUpdate(message.BuildEOR(tc.other)))
			request, err := plugin.NewSelectedRefresh(tc.input)
			require.NoError(t, err)
			_, err = tc.fixture.api.SelectRefresh(t.Context(), request, tc.fixture.owner)
			require.ErrorContains(t, err, "EOR has not reached")
			require.Len(t, splitWireMessages(t, tc.fixture.conn.written()), 1)
			require.NoError(t, tc.fixture.peer.session.SendUpdate(message.BuildEOR(tc.own)))
			require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, tc.fixture, tc.input).Outcome)
			frames := splitWireMessages(t, tc.fixture.conn.written())
			require.Len(t, frames, 3)
			require.Equal(t, message.PackTo(&message.RouteRefresh{AFI: tc.own.AFI,
				SAFI: tc.own.SAFI, Subtype: message.RouteRefreshBoRR}, nil), frames[2])
		})
	}
}

// No-MP OPENs already negotiate classic IPv4. An empty explicit MP
// intersection instead grants no family, even when diagnostic bytes are sent.
func TestSelectedRefreshEmptyIntersectionCannotRecordEOR(t *testing.T) {
	fixture, _ := selectedRefreshIPv6(t)
	session := fixture.peer.session
	session.negotiateWith([]capability.Capability{
		&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
	}, []capability.Capability{
		&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
	})
	require.Empty(t, session.negotiated.Families())
	require.NoError(t, session.SendRawMessage(2, []byte{0, 0, 0, 0}))
	frames := splitWireMessages(t, fixture.conn.written())
	require.Len(t, frames, 1)
	require.Equal(t, []byte{0, 0, 0, 0}, frames[0][19:])
	require.Empty(t, session.wireWriter.sentEOR)

	selectedRefreshNegotiateFamily(t, fixture, family.IPv4Unicast)
	require.NoError(t, session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
	require.True(t, session.wireWriter.sentEOR[family.IPv4Unicast], "a real common family must still be useful")
}
