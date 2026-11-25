// Design: docs/architecture/behavior/peer-lifecycle.md -- coherent captured targeted UP.
package reactor

import (
	"encoding/json"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// boundConnectionCapture observes the actual bind producer's typed metadata
// and negotiated value before either downstream event transport projects it.
// No production hook or alternate event builder participates.
type boundConnectionCapture struct {
	info            []plugin.PeerInfo
	negotiated      []format.DecodedNegotiated
	stateNames      [][]string
	negotiatedNames [][]string
}

func (capture *boundConnectionCapture) OnPeerBound(info *plugin.PeerInfo, negotiated any, stateNames, negotiatedNames []string) {
	capture.info = append(capture.info, *info)
	decoded, ok := negotiated.(format.DecodedNegotiated)
	if !ok {
		panic("BUG: OnPeerBound negotiated payload is not format.DecodedNegotiated")
	}
	capture.negotiated = append(capture.negotiated, decoded)
	capture.stateNames = append(capture.stateNames, append([]string(nil), stateNames...))
	capture.negotiatedNames = append(capture.negotiatedNames, append([]string(nil), negotiatedNames...))
}

func (*boundConnectionCapture) OnPeerUnbound(*plugin.PeerInfo, rpc.SessionState, string, []string) {}

// A hot reload captures original before it can block in unbind. Publish a
// replacement at that exact pre-bind boundary, then call the real bind producer
// with original. Its UP and negotiated caps must stay one coherent tuple, and
// an independent subsequent bind must retain the replacement's useful facts.
// MUTATION: bindProcesses builds info from establishedPeerInfo(p).
func TestBoundPhysicalConnectionKeepsCapturedSession(t *testing.T) {
	fixture := newSelectedWriterFixture(t, false)
	peer := fixture.peer
	original := peer.session
	original.openMu.Lock()
	err := original.prepareOpen()
	original.openMu.Unlock()
	require.NoError(t, err)
	original.negotiated = &capability.Negotiated{PeerASN: 65001, RouteRefresh: true, HoldTime: 90}
	original.peerOpen = &message.Open{BGPIdentifier: 0xc0000251}
	original.transport.Store(&sessionTransport{local: netip.MustParseAddr("192.0.2.251"), localString: "192.0.2.251", localPort: 1179, remotePort: 40001})
	original.initialReplay.Store(1<<53 + 101)
	replacementSettings := peer.settingsSnapshot()
	replacementSettings.LocalAS = 65010
	replacementSettings.RouterID = 0xc00002fe
	replacement := NewSession(replacementSettings)
	t.Cleanup(replacement.timers.StopAll)
	replacement.openMu.Lock()
	err = replacement.prepareOpen()
	replacement.openMu.Unlock()
	require.NoError(t, err)
	replacement.negotiated = &capability.Negotiated{PeerASN: 65002, EnhancedRouteRefresh: true, HoldTime: 180}
	replacement.peerOpen = &message.Open{BGPIdentifier: 0xc0000252}
	replacement.transport.Store(&sessionTransport{local: netip.MustParseAddr("192.0.2.252"), localString: "192.0.2.252", localPort: 2179, remotePort: 40002})
	replacement.initialReplay.Store(1<<53 + 102)
	peer.mu.Lock()
	peer.session = replacement
	peer.mu.Unlock()
	capture := &boundConnectionCapture{}
	reactor := &Reactor{bindEvents: capture}
	bus := testBusBlockForGrants(operatorGrant("monitor", "state negotiated", ""))
	peer.bindProcesses(reactor, original, bus, []string{"monitor"}, nil)
	require.Len(t, capture.info, 1)
	first := &capture.info[0]
	require.Equal(t, original.connectionID, first.ConnectionID)
	require.Equal(t, original.initialReplay.Load(), first.DestinationSession)
	require.Empty(t, first.SelectedOwner, "the ended Session has no current selected-output authority")
	require.Zero(t, first.InitialReplay, "an attach never restarts initial routing replay")
	require.Equal(t, peer.sourceOwner, first.PeerOwner)
	require.Equal(t, rpc.RemoteASOf(65001), first.PeerAS)
	require.Equal(t, uint32(65000), first.LocalAS)
	require.Equal(t, original.proposal.message.BGPIdentifier, first.RouterID)
	require.Equal(t, uint32(0xc0000251), first.RemoteRouterID)
	require.Equal(t, "192.0.2.251", first.LocalAddrStr())
	require.Equal(t, uint16(1179), first.LocalPort)
	require.Equal(t, uint16(40001), first.RemotePort)
	require.Equal(t, "normal", capture.negotiated[0].RouteRefresh)
	require.Equal(t, uint16(90), capture.negotiated[0].HoldTime)
	require.Equal(t, []string{"monitor"}, capture.stateNames[0])
	require.Equal(t, []string{"monitor"}, capture.negotiatedNames[0])
	var encoded struct {
		BGP struct {
			ConnectionID string `json:"connection-id"`
		} `json:"bgp"`
	}
	require.NoError(t, json.Unmarshal(format.AppendStateChange(nil, first, rpc.SessionStateUp, rpc.ReasonAttached, nil, plugin.EncodingJSON), &encoded))
	connection, err := strconv.ParseUint(encoded.BGP.ConnectionID, 10, 64)
	require.NoError(t, err)
	require.Equal(t, original.connectionID, connection)
	// Re-publish the real successor's own forwarding snapshot. An old bound
	// event must no longer borrow its ordinary owner or replay session.
	peer.refreshForwardFacts()
	oldAgain := boundPeerInfo(peer, original)
	require.Equal(t, original.connectionID, oldAgain.ConnectionID)
	require.Equal(t, original.initialReplay.Load(), oldAgain.DestinationSession)
	require.Empty(t, oldAgain.SelectedOwner, "a replacement's selected owner is not the old Session's")
	peer.bindProcesses(reactor, replacement, bus, []string{"monitor"}, nil)
	require.Len(t, capture.info, 2)
	second := &capture.info[1]
	require.Equal(t, replacement.connectionID, second.ConnectionID)
	require.Equal(t, replacement.initialReplay.Load(), second.DestinationSession)
	require.Equal(t, selectedWriterProcess, second.SelectedOwner)
	require.Zero(t, second.InitialReplay)
	require.Equal(t, rpc.RemoteASOf(65002), second.PeerAS)
	require.Equal(t, uint32(65010), second.LocalAS)
	require.Equal(t, uint32(0xc00002fe), second.RouterID)
	require.Equal(t, uint32(0xc0000252), second.RemoteRouterID)
	require.Equal(t, "192.0.2.252", second.LocalAddrStr())
	require.Equal(t, uint16(2179), second.LocalPort)
	require.Equal(t, uint16(40002), second.RemotePort)
	require.Equal(t, "enhanced", capture.negotiated[1].RouteRefresh)
	require.Equal(t, uint16(180), capture.negotiated[1].HoldTime)
	require.Same(t, replacement, peer.currentSession())
}
