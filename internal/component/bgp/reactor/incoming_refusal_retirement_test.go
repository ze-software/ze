// Design: docs/architecture/behavior/peer-lifecycle.md -- bounded incoming ownership.
// Related: pending_established_survival_test.go -- actual Established wire exchange.
package reactor

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
)

// TestIncomingRefusalRetainsAdmissionUntilRetired blocks the losing candidate's
// actual NOTIFICATION writer. Its decision is no longer pending, but its socket,
// Session and workers must retain the sole admission until retirement finishes.
// A third connection must receive Connection Rejected, never its own local OPEN.
func TestIncomingRefusalRetainsAdmissionUntilRetired(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session, original := firstASSession(t, settings, nil, nil)
	peer := NewPeer(settings)
	peer.session = session
	peer.setState(PeerStateEstablished)
	cleanupPendingFixturePeer(t, peer)
	r := &Reactor{clock: clock.RealClock{}}
	client, server := net.Pipe()
	gate := &incomingRefusalGate{Conn: server, entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { closeConnQuietly(client); closeConnQuietly(gate) })
	release := sync.OnceFunc(func() { close(gate.release) })
	t.Cleanup(release)
	r.acceptOrReject(gate, peer, nil)
	delayOpenReadType(t, client, 1)
	delayOpenWrite(t, client, &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0xffffffff})
	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the candidate did not reach its actual rejection writer")
	}
	if peer.hasPendingConnection() {
		t.Fatal("a decided refusal still reports an active pending exchange")
	}
	before := peer.Stats().OpensSent
	third, thirdServer := net.Pipe()
	t.Cleanup(func() { closeConnQuietly(third); closeConnQuietly(thirdServer) })
	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		r.acceptOrReject(thirdServer, peer, nil)
	}()
	refusal := delayOpenReadType(t, third, 3)
	if !bytes.Equal(refusal[message.HeaderLen:], []byte{6, 5}) {
		t.Fatalf("third arrival did not receive Connection Rejected: %x", refusal)
	}
	var octet [1]byte
	if n, err := third.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("third arrival was not closed after refusal: bytes=%d error=%v", n, err)
	}
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("the third arrival's refusal did not complete")
	}
	if got := peer.Stats().OpensSent; got != before {
		t.Fatalf("a closing candidate admitted another Session: OPENs=%d, want %d", got, before)
	}
	release()
	delayOpenReadType(t, client, 3)
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		return peer.pending == nil
	})
	// Retirement releases the same admission gate. A later arrival gets its
	// own real OPEN exchange; permanently refusing later sockets cannot pass.
	fourth, fourthServer := net.Pipe()
	t.Cleanup(func() { closeConnQuietly(fourth); closeConnQuietly(fourthServer) })
	r.acceptOrReject(fourthServer, peer, nil)
	delayOpenReadType(t, fourth, 1)
	delayOpenWrite(t, fourth, &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0xffffffff})
	refusal = delayOpenReadType(t, fourth, 3)
	if !bytes.Equal(refusal[message.HeaderLen:], []byte{6, 7}) {
		t.Fatalf("later admitted arrival did not complete its collision exchange: %x", refusal)
	}
	if got := peer.Stats().OpensSent; got != before+1 {
		t.Fatalf("retirement did not reopen admission exactly once: OPENs=%d, want %d", got, before+1)
	}
	if peer.currentSession() != session || session.State() != fsm.StateEstablished {
		t.Fatal("retiring the rejected candidate replaced the incumbent")
	}
	pendingSurvivalRoute(t, session, original, []byte{24, 198, 51, 100})
}

// incomingRefusalGate owns one bounded test connection. The test MUST release
// its NOTIFICATION writer before fixture Peer cleanup joins that connection.
type incomingRefusalGate struct {
	net.Conn
	entered chan struct{}
	release chan struct{}
}

func (c *incomingRefusalGate) Write(data []byte) (int, error) {
	if len(data) >= message.HeaderLen && data[18] == 3 {
		close(c.entered)
		<-c.release
	}
	return c.Conn.Write(data)
}
