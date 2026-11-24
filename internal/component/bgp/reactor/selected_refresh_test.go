// Design: docs/architecture/update-building.md -- exact selected refresh authority.
package reactor

import (
	"bufio"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func selectedRefreshFixture(t *testing.T) (*selectedWriterFixture, rpc.SelectRefreshInput) {
	t.Helper()
	fixture := newSelectedWriterFixture(t, false)
	original := fixture.owner
	lifetime, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	owner, err := plugin.NewSelectedOwner(original.Sender(), done, nil)
	require.NoError(t, err)
	fixture.owner, err = owner.WithConnection(original.CurrentSelection, lifetime)
	require.NoError(t, err)
	fixture.peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv4Unicast: true},
		EnhancedRouteRefresh: true, RouteRefresh: true})
	session := fixture.peer.session
	writer := &observedBGPWriter{session: session, writer: fixture.conn}
	writer.context.Store(uint32(session.sendCtxID))
	session.wireWriter = writer
	session.bufWriter = bufio.NewWriter(writer)
	input := rpc.SelectRefreshInput{Destination: fixture.peer.settings.Address.String(),
		PeerOwner: fixture.peer.sourceOwner, DestinationSession: 71, Family: "ipv4/unicast",
		Action: rpc.SelectedRefreshBegin, DeadlineMS: time.Now().Add(5 * time.Second).UnixMilli()}
	return fixture, input
}

func selectedRefreshCall(t *testing.T, fixture *selectedWriterFixture, input rpc.SelectRefreshInput) *rpc.SelectRefreshOutput {
	t.Helper()
	request, err := plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	output, err := fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.NoError(t, err)
	require.NoError(t, output.Validate(input))
	return output
}

func selectedRefreshNegotiateGR(t *testing.T, fixture *selectedWriterFixture, local, remote bool) {
	t.Helper()
	session := fixture.peer.session
	session.proposal = preparedOpenForTest(session, &message.Open{Version: 4, MyAS: 65000, HoldTime: 90, BGPIdentifier: 1})
	session.peerOpen = &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 2}
	var localCaps, remoteCaps []capability.Capability
	if local {
		localCaps = append(localCaps, &capability.GracefulRestart{})
	}
	if remote {
		remoteCaps = append(remoteCaps, &capability.GracefulRestart{})
	}
	session.negotiateWith(localCaps, remoteCaps)
	require.NotNil(t, session.negotiated, "actual OPEN publication must run")
	fixture.peer.negotiated.Load().GracefulRestart = session.negotiated.GracefulRestart
}

// Peer GR alone does not impose the sender's EOR precondition. Local OPEN
// publication resets the fact, so mutable remote capabilities cannot supply it.
func TestSelectedRefreshGRUsesLocalOpen(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	selectedRefreshNegotiateGR(t, fixture, false, true)
	require.NotNil(t, fixture.peer.negotiated.Load().GracefulRestart)
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, fixture.conn.written()))
}

// A selected owner with only send[update] can bracket its own output; a
// similarly addressed or replacement identity cannot supply its authority.
func TestSelectedRefreshUsesExactCapturedDestination(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	for _, change := range []func(*rpc.SelectRefreshInput){
		func(value *rpc.SelectRefreshInput) { value.PeerOwner++ },
		func(value *rpc.SelectRefreshInput) { value.DestinationSession++ },
	} {
		refused := input
		change(&refused)
		request, err := plugin.NewSelectedRefresh(refused)
		require.NoError(t, err)
		_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
		require.Error(t, err)
		require.Empty(t, fixture.conn.written())
	}
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	input.Action = rpc.SelectedRefreshEnd
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	require.Equal(t, []string{"borr", "eorr"}, refreshWireLabels(t, fixture.conn.written()))
}

// Lack of enhanced negotiation is an explicit no-marker completion, while the
// same authenticated destination remains useful when negotiation permits it.
func TestSelectedRefreshRequiresEnhancedNegotiation(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	fixture.peer.negotiated.Load().EnhancedRouteRefresh = false
	require.Equal(t, rpc.SelectedRefreshNotNegotiated, selectedRefreshCall(t, fixture, input).Outcome)
	require.Empty(t, fixture.conn.written())
	fixture.peer.negotiated.Load().EnhancedRouteRefresh = true
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, fixture.conn.written()))
}

// An update grant alone cannot impersonate this destination's deciding role.
func TestSelectedRefreshRequiresEffectiveSelectingOwner(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	const otherProcess = "selected-refresh-other"
	fixture.peer.settings.Bus = testBusBlockForGrants(
		testPeerEndpointGrant{Name: selectedWriterProcess, Receive: []string{"state", "refresh", "update-sent"}, Send: []string{"update"}},
		testPeerEndpointGrant{Name: otherProcess, Send: []string{"update"}},
	)
	originRevocationPublishGraph(t, fixture)
	fixture.peer.refreshForwardFacts()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	other, err := plugin.NewSelectedOwner(plugin.ProcessSender(otherProcess), done, nil)
	require.NoError(t, err)
	other, err = other.WithConnection(fixture.owner.CurrentSelection, t.Context())
	require.NoError(t, err)
	request, err := plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, other)
	require.ErrorContains(t, err, "belongs to process")
	require.Empty(t, fixture.conn.written())
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	require.Equal(t, []string{"borr"}, refreshWireLabels(t, fixture.conn.written()))
}

// A cleared owed flag is not a sent EOR: manual-EOR and failed writes must not
// permit BoRR. Actual complete EOR bytes from this connection open the gate.
func TestSelectedRefreshGRWaitsForActualEOR(t *testing.T) {
	fixture, input := selectedRefreshFixture(t)
	selectedRefreshNegotiateGR(t, fixture, true, false)
	require.Nil(t, fixture.peer.negotiated.Load().GracefulRestart, "the peer did not advertise GR")
	fixture.peer.session.initialSyncEOROwed.Store(false)
	request, err := plugin.NewSelectedRefresh(input)
	require.NoError(t, err)
	_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
	require.ErrorContains(t, err, "EOR has not reached")
	require.Empty(t, fixture.conn.written())
	require.NoError(t, fixture.peer.session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, input).Outcome)
	require.Equal(t, []string{"eor", "borr"}, refreshWireLabels(t, fixture.conn.written()))
}
