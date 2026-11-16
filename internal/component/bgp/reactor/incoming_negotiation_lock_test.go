// Design: docs/architecture/behavior/peer-lifecycle.md -- negotiation callback lock ordering.
// Related: peer_stats.go -- the factory's selected-session timer callback.
package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestNegotiatedCallbackDoesNotHoldSessionOwner installs a proxy before TCP,
// blocks it on the Peer lock, then queries the Session in the same Peer-to-
// Session direction as collision resolution. The query must complete before
// the Peer lock is released, and that same connection must still establish.
func TestNegotiatedCallbackDoesNotHoldSessionOwner(t *testing.T) {
	peer := NewPeer(NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101))
	session := peer.newSession()
	t.Cleanup(func() { session.closeConn(); session.stopSessionTimers() })
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	entered := make(chan struct{})
	continueCallback := make(chan struct{})
	releaseCallback := sync.OnceFunc(func() { close(continueCallback) })
	t.Cleanup(releaseCallback)
	callback := session.onNegotiated
	session.onNegotiated = func(hold, keepalive uint32) {
		close(entered)
		<-continueCallback
		// This lock probe must release before the saved callback takes
		// the Peer lock for its actual publication.
		func() {
			peer.mu.RLock()
			defer peer.mu.RUnlock()
		}()
		callback(hold, keepalive)
	}
	remote, local := collisionProofTCPPair(t)
	if err := session.Start(); err != nil {
		t.Fatal(err)
	}
	if err := session.Accept(local); err != nil {
		t.Fatal(err)
	}
	delayOpenReadType(t, remote, 1)
	processed := make(chan error, 1)
	go func() { processed <- session.ReadAndProcess() }()
	delayOpenWrite(t, remote, delayOpenRemote(0x02020202))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		releaseCallback()
		t.Fatal("OPEN did not reach its installed negotiation callback")
	}
	peer.mu.Lock()
	releaseCallback()
	queried := make(chan struct{})
	go func() { _ = session.Conn(); _ = session.Negotiated(); close(queried) }()
	blocked := false
	select {
	case <-queried:
	case <-time.After(5 * time.Second):
		blocked = true
	}
	// Always release before an assertion or cleanup joins the callback/reader.
	peer.mu.Unlock()
	delayOpenReadType(t, remote, 4)
	select {
	case err := <-processed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OPEN reader did not finish after callback release")
	}
	select {
	case <-queried:
	case <-time.After(5 * time.Second):
		t.Fatal("Session query did not finish after callback release")
	}
	if blocked {
		t.Fatal("negotiation callback held Session.mu while waiting for Peer.mu")
	}
	go func() { processed <- session.ReadAndProcess() }()
	delayOpenWrite(t, remote, &message.Keepalive{})
	select {
	case err := <-processed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("useful confirmation did not finish")
	}
	if session.State() != fsm.StateEstablished {
		t.Fatalf("same socket did not establish: %s", session.State())
	}
}
