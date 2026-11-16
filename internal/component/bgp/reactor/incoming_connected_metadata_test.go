// Design: docs/architecture/api/json-format.md -- TCP events name their own connection.
// Related: delay_open_connected_event_test.go -- real IPC connected-event subscriber.
package reactor

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestIncomingConnectedMetadataUsesCandidateSocket contrasts an incumbent's
// auto-address pipe with an incoming candidate's actual TCP endpoint. A cached
// dynamic AS from an earlier epoch must not identify this new connection.
func TestIncomingConnectedMetadataUsesCandidateSocket(t *testing.T) {
	var deliveries *stateLog
	peer, original, outgoing, _ := startDelayOpenPeer(t, true, 240*time.Second, func(peer *Peer) {
		peer.settings.IsDynamic = true
		deliveries = delayOpenConnectedSubscriber(t, peer)
	})
	const connected = `\"type\":\"connected\"`
	delayOpenAwait(t, func() bool {
		return strings.Count(strings.Join(deliveries.recorded(), "\n"), connected) == 1
	})
	_, near := collisionProofTCPPair(t)
	candidate := peer.newSession()
	t.Cleanup(func() {
		candidate.closeConn()
		candidate.stopSessionTimers()
		peer.releaseRouterIDClaim()
		peer.stopCapture(candidate.captureWriter)
	})
	if err := candidate.startDamped(); err != nil {
		t.Fatal(err)
	}
	if err := candidate.Accept(near); err != nil {
		t.Fatal(err)
	}
	var candidateEvent string
	delayOpenAwait(t, func() bool {
		var connectedEvents []string
		for _, delivery := range deliveries.recorded() {
			if strings.Contains(delivery, connected) {
				connectedEvents = append(connectedEvents, delivery)
			}
		}
		if len(connectedEvents) != 2 {
			return false
		}
		candidateEvent = connectedEvents[1]
		return true
	})
	if !strings.Contains(candidateEvent, `\"local\":{\"address\":\"127.0.0.1\"`) {
		t.Fatalf("connected borrowed the incumbent's endpoint: %s", candidateEvent)
	}
	if strings.Contains(candidateEvent, `\"remote\":{\"address\":\"127.0.0.1\",\"as\":`) {
		t.Fatalf("connected borrowed the incumbent's dynamic AS: %s", candidateEvent)
	}
	// The independent candidate notification leaves the original useful.
	delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
	delayOpenReadType(t, outgoing, 1)
	delayOpenReadType(t, outgoing, 4)
	delayOpenWrite(t, outgoing, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return original.State() == fsm.StateEstablished })
}
