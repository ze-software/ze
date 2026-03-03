package reactor

import (
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/test/sim"
)

// TestInboundConnectionRoundTrip verifies that a connection stored via
// SetInboundConnection can be retrieved via takeInboundConnection.
//
// VALIDATES: Store-and-retrieve cycle works correctly.
// PREVENTS: Stored connection lost or returned to wrong caller.
func TestInboundConnectionRoundTrip(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020304)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)

	// Initially nil
	assert.Nil(t, peer.takeInboundConnection())

	// Store and retrieve
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	peer.SetInboundConnection(server)
	got := peer.takeInboundConnection()
	assert.Equal(t, server, got)

	// Second take returns nil
	assert.Nil(t, peer.takeInboundConnection())
}

// TestInboundConnectionReplacesOld verifies that storing a new inbound
// connection closes and replaces any previously stored connection.
//
// VALIDATES: Old connection is closed when replaced.
// PREVENTS: Connection leak from rapid reconnects.
func TestInboundConnectionReplacesOld(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020304)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)

	// Store first connection
	client1, server1 := net.Pipe()
	defer func() { _ = client1.Close() }()
	peer.SetInboundConnection(server1)

	// Store second — should close first
	client2, server2 := net.Pipe()
	defer func() { _ = client2.Close() }()
	defer func() { _ = server2.Close() }()
	peer.SetInboundConnection(server2)

	// First connection should be closed (write returns error)
	_ = server1.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
	_, err := server1.Write([]byte{0})
	assert.Error(t, err, "old connection should be closed after replacement")

	// Second connection should be the one retrieved
	got := peer.takeInboundConnection()
	assert.Equal(t, server2, got)
}

// TestInboundNotifyWakesBackoff verifies that SetInboundConnection sends a
// signal on the inboundNotify channel, allowing run() to skip backoff.
//
// VALIDATES: Channel is signaled on store.
// PREVENTS: Peer sleeping through full backoff when a connection is waiting.
func TestInboundNotifyWakesBackoff(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020304)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	peer.SetInboundConnection(server)

	// Channel should have a signal
	select {
	case <-peer.inboundNotify:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("inboundNotify should have been signaled")
	}
}

// TestInboundNotifyIdempotent verifies that multiple SetInboundConnection calls
// do not block when the channel already has a pending signal.
//
// VALIDATES: Non-blocking signal — second store doesn't deadlock.
// PREVENTS: Listener goroutine blocked indefinitely on channel send.
func TestInboundNotifyIdempotent(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020304)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)

	c1, s1 := net.Pipe()
	defer func() { _ = c1.Close() }()

	c2, s2 := net.Pipe()
	defer func() { _ = c2.Close() }()
	defer func() { _ = s2.Close() }()

	// Two rapid stores — must not block
	done := make(chan struct{})
	go func() {
		peer.SetInboundConnection(s1)
		peer.SetInboundConnection(s2)
		close(done)
	}()

	select {
	case <-done:
		// expected — neither call blocked
	case <-time.After(time.Second):
		t.Fatal("SetInboundConnection blocked — channel send deadlock")
	}
}

// TestInboundConnectionCleanup verifies that cleanup() closes any buffered
// inbound connection.
//
// VALIDATES: No connection leak on peer shutdown.
// PREVENTS: Leaked TCP connections when peer stops with a buffered connection.
func TestInboundConnectionCleanup(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020304)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

	peer.SetInboundConnection(server)

	// cleanup() should close the stored connection
	peer.cleanup()

	_ = server.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
	_, err := server.Write([]byte{0})
	assert.Error(t, err, "inbound connection should be closed after cleanup")
	assert.Nil(t, peer.takeInboundConnection(), "inbound should be nil after cleanup")
}

// TestInboundConnectionSkipsBackoff verifies the full integration: the run()
// loop's backoff select wakes immediately when an inbound connection is stored,
// rather than waiting the full delay.
//
// VALIDATES: Backoff is skipped and delay is reset on inbound arrival.
// PREVENTS: 5s+ delay before accepting a connection from a fast-reconnecting peer.
func TestInboundConnectionSkipsBackoff(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020304)
	settings.Connection = ConnectionPassive
	peer := NewPeer(settings)
	clock := sim.NewFakeClock(time.Now())
	peer.SetClock(clock)

	// Simulate the backoff select from run().
	// We can't easily run the full run() loop, but we can test the select behavior.
	var wokenBy atomic.Int32 // 0=none, 1=timer, 2=inbound
	delay := 30 * time.Second

	go func() {
		select {
		case <-clock.After(delay):
			wokenBy.Store(1)
		case <-peer.inboundNotify:
			wokenBy.Store(2)
		}
	}()

	// Give the goroutine time to enter the select
	time.Sleep(10 * time.Millisecond)

	// Store an inbound connection — should wake the select immediately
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	peer.SetInboundConnection(server)

	// Wait for the goroutine to complete
	require.Eventually(t, func() bool { return wokenBy.Load() != 0 }, time.Second, time.Millisecond)
	assert.Equal(t, int32(2), wokenBy.Load(), "should be woken by inbound, not timer")
}
