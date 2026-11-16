// Design: docs/architecture/behavior/peer-lifecycle.md -- bounded incoming admission.
// Related: incoming_refusal_retirement_test.go -- admission during joined retirement.
// RFC 4486 Section 4 distinguishes admission refusal from RFC 4271 Section 6.8 collision.
// This focused capacity regression is not enrolled as whole-requirement RFC evidence.
package reactor

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestIncomingCapacityRefusalPreservesConnections follows the actual listener:
// an excess TCP connection is refused before OPEN, while the admitted candidate
// still reaches real collision arbitration and the incumbent still receives traffic.
func TestIncomingCapacityRefusalPreservesConnections(t *testing.T) {
	r, peer, original, _, _ := incomingListenerPeer(t, false, 240*time.Second)
	incumbent := incomingListenerDial(t, r)
	delayOpenAwait(t, original.holdsOpen)
	delayOpenWrite(t, incumbent, delayOpenRemote(0x02020202))
	delayOpenReadType(t, incumbent, 1)
	delayOpenReadType(t, incumbent, 4)
	delayOpenWrite(t, incumbent, &message.Keepalive{})
	delayOpenAwait(t, func() bool {
		return original.State() == fsm.StateEstablished && peer.Stats().KeepalivesReceived == 1
	})
	originalConn := original.Conn()

	incoming := incomingListenerDial(t, r)
	var admitted *incomingCandidate
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		candidate, ok := peer.pending.(*incomingCandidate)
		if !ok || !candidate.session.holdsOpen() {
			return false
		}
		admitted = candidate
		return true
	})
	delayOpenNoBytes(t, incoming)
	excess := incomingListenerDial(t, r)
	incomingReadCapacityRefusalClose(t, excess)
	peer.mu.RLock()
	samePending := peer.pending == admitted
	peer.mu.RUnlock()
	if !samePending || peer.currentSession() != original || original.Conn() != originalConn || original.State() != fsm.StateEstablished {
		t.Fatal("capacity refusal displaced an owned connection")
	}

	// A real OPEN on the same admitted connection must take the collision
	// path, not the capacity path. Refusing every connection with 6/5 fails.
	delayOpenWrite(t, incoming, delayOpenRemote(0x02020202))
	delayOpenReadCollisionClose(t, incoming)
	delayOpenAwait(t, func() bool {
		peer.mu.RLock()
		defer peer.mu.RUnlock()
		return peer.pending == nil
	})
	if peer.currentSession() != original || original.Conn() != originalConn || original.State() != fsm.StateEstablished {
		t.Fatal("collision refusal retired the incumbent")
	}
	before := peer.Stats().KeepalivesReceived
	delayOpenWrite(t, incumbent, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return peer.Stats().KeepalivesReceived == before+1 })
}

func incomingReadCapacityRefusalClose(t *testing.T, conn net.Conn) {
	t.Helper()
	packet := delayOpenReadType(t, conn, 3)
	if !bytes.Equal(packet[message.HeaderLen:], []byte{6, 5}) {
		t.Fatalf("capacity refusal is not Cease/Connection Rejected: %x", packet)
	}
	var octet [1]byte
	if n, err := conn.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("capacity refusal did not close after Cease: bytes=%d error=%v", n, err)
	}
}
