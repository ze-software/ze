// Design: docs/architecture/bgp/structural-forwarding.md -- coherent original handback targets.
// Related: peer_run.go -- encoding publication precedes the Established admission.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestEstablishedPublishesCurrentForwardTarget reproduces the real callback's
// encoding-then-establishment order. The first Established observer must get
// the same captured destination tuple as that physical Session's UP event.
func TestEstablishedPublishesCurrentForwardTarget(t *testing.T) {
	peer := NewPeer(NewPeerSettings(netip.MustParseAddr("192.0.2.82"), 65000, 65001, 0x0a000001))
	session := peer.newSession()
	t.Cleanup(session.timers.StopAll)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	negotiated := &capability.Negotiated{ASN4: true}
	peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
	peer.setEncodingContexts(negotiated)
	originalFacts := peer.forwardFacts()
	require.Equal(t, session.connectionID, originalFacts.forwardTarget().DestinationSession)
	var observed rpc.ForwardTarget
	peer.SetCallback(func(_, state PeerState) {
		if state == PeerStateEstablished {
			observed = originalFacts.forwardTarget()
		}
	})
	peer.enterEstablished(session)
	require.NotEqual(t, session.connectionID, session.initialReplay.Load(), "Established issues its own UP receipt")
	require.Equal(t, capturedForwardTarget(peer, session), observed,
		"the first Established observer must not inherit the pre-UP connection token")
	require.Equal(t, capturedForwardTarget(peer, session), originalFacts.forwardTarget(),
		"a saved pre-UP snapshot still names its own physical Session")
	originalTarget := originalFacts.forwardTarget()
	next := peer.newSession()
	t.Cleanup(next.timers.StopAll)
	peer.mu.Lock()
	peer.session = next
	peer.mu.Unlock()
	peer.setEncodingContexts(negotiated)
	peer.enterEstablished(next)
	require.Equal(t, originalTarget, originalFacts.forwardTarget(), "a saved snapshot cannot borrow the successor's token")
	require.Equal(t, capturedForwardTarget(peer, next), peer.forwardFacts().forwardTarget())
	require.NotEqual(t, originalTarget.DestinationSession, peer.forwardFacts().forwardTarget().DestinationSession)
}
