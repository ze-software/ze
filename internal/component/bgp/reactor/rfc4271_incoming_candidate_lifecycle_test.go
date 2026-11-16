// Design: docs/architecture/behavior/peer-lifecycle.md -- incoming connection failure isolation.
// Related: rfc4271_incoming_listener_test.go -- actual listener and wire helpers.
// RFC 4271 Sections 8.2.1 and 8.2.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestIncomingCandidateFailureKeepsPeerRetryHistory exercises a real second
// TCP connection held in Active by DelayOpen. Its EOF must retire
// its exchange and update the peer-owned retry history, while the original useful
// connection remains able to finish its own handshake.
// RFC requirement: RFC4271-8.2.1-3 negative -- an admitted incoming connection held in Active by DelayOpen reaches EOF, clears the pending exchange and increments the peer's ConnectRetryCounter once; the original selected Session stays held and then establishes on its original TCP connection.
func TestIncomingCandidateFailureKeepsPeerRetryHistory(t *testing.T) {
	r, peer, original, _, _ := incomingListenerPeer(t, false, 240*time.Second)
	outgoing := incomingListenerDial(t, r)
	delayOpenAwait(t, original.holdsOpen)
	before := peer.Stats().ConnectRetryCounter
	incoming := incomingListenerDial(t, r)
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		candidate, ok := peer.pending.(*incomingCandidate)
		return ok && candidate.session.State() == fsm.StateActive && candidate.session.holdsOpen()
	})
	if err := incoming.Close(); err != nil {
		t.Fatal(err)
	}
	delayOpenAwait(t, func() bool {
		return !peer.hasPendingConnection() && peer.Stats().ConnectRetryCounter == before+1
	})
	if peer.currentSession() != original || !original.holdsOpen() {
		t.Fatal("incoming EOF retired the original connection")
	}
	delayOpenWrite(t, outgoing, delayOpenRemote(0x02020202))
	delayOpenReadType(t, outgoing, 1)
	delayOpenReadType(t, outgoing, 4)
	delayOpenWrite(t, outgoing, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return original.State() == fsm.StateEstablished })
}
