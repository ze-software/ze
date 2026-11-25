// Design: docs/architecture/chaos-web-dashboard.md -- route propagation validation

package validation

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/peer"
)

// VALIDATES: A zero receipt count fails only when a route has a destination.
// PREVENTS: Flagging deliberate iBGP-to-eBGP isolation as a forwarding failure.
func TestObservationRequiresReceiptsForReachableRoutes(t *testing.T) {
	observation := NewEventProcessor(2, time.Second, ScenarioReachability([]bool{true, false}, nil))
	at := time.Now()
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	observation.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix, Time: at})
	assert.False(t, observation.MissingAllReceipts())
	observation.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 1, Prefix: prefix, Time: at})
	assert.True(t, observation.MissingAllReceipts())
	observation.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 0, Prefix: prefix, Time: at})
	assert.False(t, observation.MissingAllReceipts())
}

// VALIDATES: Only the affected peer can finish in a later planned outage.
// PREVENTS: One chaos action waiving unrelated session and recovery failures.
func TestObservationRequiresEvidenceOfRecovery(t *testing.T) {
	observation := NewEventProcessor(2, time.Second, nil)
	for index := range 2 {
		observation.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: index})
		observation.Process(peer.Event{Type: peer.EventEORSent, PeerIndex: index})
	}
	disconnect := peer.Event{Type: peer.EventDisconnected, PeerIndex: 0}
	fault := peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, ChaosAction: engine.NameTCPDisconnect}
	observation.Process(fault)
	observation.Process(disconnect)
	assert.Equal(t, []int{0}, observation.UnsynchronizedPeers())
	observation.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0})
	assert.Equal(t, []int{0}, observation.UnsynchronizedPeers())
	observation.Process(peer.Event{Type: peer.EventEORSent, PeerIndex: 0})

	observation.Process(fault)
	observation.Process(disconnect)
	assert.Empty(t, observation.UnsynchronizedPeers())
	observation.Process(peer.Event{Type: peer.EventDisconnected, PeerIndex: 1})
	assert.Equal(t, []int{1}, observation.UnsynchronizedPeers())
}
