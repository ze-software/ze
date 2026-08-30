// Design: docs/guide/bmp.md -- Peer Up carries two OPEN PDUs, or is not sent
// Detail: RFC 7854 Section 4.10 requires both OPEN PDUs, so a Peer Up that
// cannot carry them is not sent at all. The proof is what the collector reads
// off the socket, never the absence of a panic.
// Related: bmp_events.go -- handleSenderState, the producer under test.
// Related: event_test.go -- TestBMPPeerUpSkippedOnCacheMiss, the earlier form.
package bmp

import (
	"net"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPeerUpOnCacheMissNeverReachesTheCollector proves that a peer reaching
// Established with no cached OPEN PDUs produces NO Peer Up on the wire, so the
// collector never receives one missing the two OPEN messages the message format
// requires.
//
// RFC 7854 Section 4.10 defines the Peer Up body as Local Address, Local Port,
// Remote Port, then: "Sent OPEN Message: The full OPEN message transmitted by
// the monitored router to its peer." and "Received OPEN Message: The full OPEN
// message received by the monitored router from its peer." Neither is marked
// OPTIONAL; the Information field that follows them is. A Peer Up without them
// is not a shorter Peer Up, it is an unparseable one: a collector locates the
// Information TLVs by walking past both OPENs.
//
// METHOD: one collector session on a pipe, then two events for the same peer in
// order -- Established with an empty OPEN cache, then Down. Peer Down needs no
// cached OPEN, so it is always emitted. Both writes go through the session's
// ordered queue (sender_drain.go enqueueLocked), so the FIRST message the
// collector reads settles the question: a suppressed Peer Up makes it the Peer
// Down, and an emitted one makes it the Peer Up.
//
// VALIDATES: handleSenderState suppresses the Peer Up when recordPeerUp cached
// no state for the peer.
// PREVENTS: an OPEN-less Peer Up desynchronizing every collector that parses the
// Information TLVs by offset.
//
// TestBMPPeerUpSkippedOnCacheMiss carries this same tag and asserts nothing at
// all -- it calls handleStructuredEvent on a session whose conn is nil and
// checks only that nothing panics. Replacing the `if st == nil { return }` guard
// with a Peer Up carrying nil OPENs leaves it green; it turns this test red
// (mutation-tested, 2026-08-30).
//
// RFC requirement: RFC7854-x-8 negative -- when the sent and received OPEN PDUs
// are not available, the sender emits no Peer Up at all rather than one missing
// the OPEN messages Section 4.10 requires it to carry.
func TestPeerUpOnCacheMissNeverReachesTheCollector(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	bp := &BMPPlugin{
		state:     newBMPState(),
		openCache: make(map[string]*openPair), // deliberately empty
		stopCh:    make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	first := asyncRead(server)

	up := &rpc.StructuredEvent{
		PeerAddress:  "10.0.0.1",
		PeerAS:       65001,
		LocalAS:      65000,
		LocalAddress: "10.0.0.100",
		EventType:    rpc.EventKindState,
		State:        rpc.SessionStateUp,
	}
	bp.handleStructuredEvent(up)

	down := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindState,
		State:       rpc.SessionStateDown,
		Reason:      "remote-close",
	}
	bp.handleStructuredEvent(down)

	r := <-first
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	if pu, ok := r.msg.(*PeerUp); ok {
		t.Fatalf("collector received a Peer Up with %d-octet sent OPEN and %d-octet received OPEN; "+
			"RFC 7854 Section 4.10 requires both, so no Peer Up may be sent when neither is cached",
			len(pu.SentOpenMsg), len(pu.ReceivedOpenMsg))
	}
	if _, ok := r.msg.(*PeerDown); !ok {
		t.Fatalf("first message = %T, want *PeerDown (the Peer Up must have been suppressed)", r.msg)
	}
}
