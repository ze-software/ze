package reactor

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestHandlePendingCollisionOversizedLength drives the collision OPEN read with a
// header whose Length exceeds the 4096-byte read buffer.
//
// message.ParseHeader enforces only the LOWER length bound (header.go:80, and its
// own doc at header.go:102: "For upper bound validation, use ValidateLengthWithMax"),
// so handlePendingCollision read the wire's 16-bit Length straight into
// buf[HeaderLen:hdr.Length] over a buffer of message.MaxMsgLen (4096). Any host able
// to open a colliding TCP connection to a configured peer address could therefore
// panic the reactor before any capability negotiation. The two sibling read paths
// (session_read.go, session_coalesce.go) already called ValidateLengthWithMax; this
// one did not.
//
// Lives in its own file rather than collision_test.go because that file carries an
// RFC requirement tag and the edit gate cannot distinguish an added test from a
// rewritten one.
//
// VALIDATES: an OPEN header claiming more than 4096 octets is rejected, not indexed.
// PREVENTS: "slice bounds out of range [:65535] with capacity 4096" in the reactor,
// reachable from a remote pre-Established connection.
func TestHandlePendingCollisionOversizedLength(t *testing.T) {
	r := newTestReactor(t)
	peer := NewPeer(NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020304,
	))

	client, server := net.Pipe()
	t.Cleanup(func() {
		if err := client.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			t.Logf("client close: %v", err)
		}
	})

	// A syntactically valid BGP header: all-ones marker, Length past MaxMsgLen,
	// Type OPEN. Only the length is abusive, so every earlier guard passes and the
	// body read is reached.
	var hdr [message.HeaderLen]byte
	for i := range message.MarkerLen {
		hdr[i] = 0xFF
	}
	hdr[16] = 0xFF // Length = 65535, well past MaxMsgLen
	hdr[17] = 0xFF
	hdr[18] = byte(msgtype.TypeOPEN)

	var wg sync.WaitGroup
	wg.Go(func() {
		if _, err := client.Write(hdr[:]); err != nil {
			return
		}
		// Drain whatever the reactor writes back (the rejection NOTIFICATION) so
		// its Write over net.Pipe cannot block.
		drain := make([]byte, 256)
		for {
			if _, err := client.Read(drain); err != nil {
				return
			}
		}
	})

	// Before the guard was added this panicked inside handlePendingCollision.
	r.handlePendingCollision(peer, server)

	assert.False(t, peer.HasPendingConnection(),
		"an over-long OPEN header must clear the pending connection, not panic")

	wg.Wait()
}
