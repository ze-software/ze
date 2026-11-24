// Design: docs/architecture/api/process-protocol.md -- exact marker destination.
package reactor

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A same-address configured sibling cannot borrow the original peer's update
// grant or peer owner. Each permitted session nevertheless produces its marker.
func TestSelectedRefreshSameAddressSiblingKeepsOwnAuthority(t *testing.T) {
	fixture, original := selectedRefreshFixture(t)
	sibling, wire := newGroupUpdatesPeer(t, original.Destination, "absent")
	sibling.settings.Port = 1179
	sibling.settings.ExportPolicy = fixture.peer.settings.ExportPolicy
	sibling.settings.Bus = testBusNone()
	sibling.reactor = fixture.api.r
	sibling.session.initialReplay.Store(72)
	sibling.negotiated.Load().EnhancedRouteRefresh = true
	sibling.refreshForwardFacts()
	fixture.api.r.peers[sibling.Settings().PeerKey()] = sibling
	input := original
	input.PeerOwner, input.DestinationSession = sibling.sourceOwner, 72
	request, err := plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.Error(t, err, "the original's grant must not authorize its sibling")
	require.Empty(t, wire.written())
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, original).Outcome)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, fixture.conn.written()))
	sibling.settings.Bus = fixture.peer.settings.Bus.Clone()
	sibling.refreshForwardFacts()
	borrowed := input
	borrowed.PeerOwner = original.PeerOwner
	request, err = plugin.NewSelectedRefresh(borrowed)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.Error(t, err, "a correct sibling session must not borrow the original peer owner")
	require.Empty(t, wire.written())
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, wire.written()))
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, fixture.conn.written()))
}

// An old successful EOR and marker do not authorize a replaced connection.
// The replacement must send its own EOR before its own useful marker completes.
func TestSelectedRefreshReplacementNeedsOwnEOR(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	selectedRefreshNegotiateGR(t, fixture, true, false)
	require.NoError(t, fixture.peer.session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	replacement, wire := newGroupUpdatesPeer(t, input.Destination, "absent")
	session := replacement.session
	session.initialReplay.Store(72)
	session.sendCtxID = fixture.peer.session.sendCtxID
	writer := &observedBGPWriter{session: session, writer: wire}
	writer.context.Store(uint32(session.sendCtxID))
	session.wireWriter, session.bufWriter = writer, bufio.NewWriter(writer)
	fixture.peer.session = session
	fixture.peer.refreshForwardFacts()
	selectedRefreshNegotiateGR(t, fixture, true, false)
	request, err := plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.Error(t, err, "the retired session token must not follow a reconnect")
	input.DestinationSession = 72
	request, err = plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.ErrorContains(t, err, "EOR has not reached")
	require.Empty(t, wire.written())
	require.NoError(t, session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	require.Equal(t, []string{"eor", "borr"}, refreshWireLabels(t, wire.written()))
	require.Equal(t, []string{"eor", "borr"}, refreshWireLabels(t, fixture.conn.written()))
}
