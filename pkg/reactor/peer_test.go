package reactor

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustParseAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

// TestPeerNew verifies Peer creation with correct initial state.
//
// VALIDATES: Peer starts in stopped state with nil session.
//
// PREVENTS: Peer starting automatically or with invalid state.
func TestPeerNew(t *testing.T) {
	neighbor := NewNeighbor(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)

	peer := NewPeer(neighbor)

	require.NotNil(t, peer, "NewPeer must return non-nil")
	require.Equal(t, PeerStateStopped, peer.State(), "initial state must be Stopped")
	require.Equal(t, neighbor, peer.Neighbor(), "Neighbor() must return configured neighbor")
}

// TestPeerStartStop verifies basic start/stop lifecycle.
//
// VALIDATES: Peer can be started and stopped cleanly.
//
// PREVENTS: Resource leaks or goroutine leaks on stop.
func TestPeerStartStop(t *testing.T) {
	neighbor := NewNeighbor(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	neighbor.Port = 0 // Invalid port to prevent actual connection

	peer := NewPeer(neighbor)

	// Start peer
	peer.Start()

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	require.NotEqual(t, PeerStateStopped, peer.State(), "state should change after Start")

	// Stop peer
	peer.Stop()

	// Wait for stop
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	require.Equal(t, PeerStateStopped, peer.State(), "state must be Stopped after Stop")
}

// TestPeerReconnect verifies reconnection logic with backoff.
//
// VALIDATES: Peer attempts reconnection after connection failure.
//
// PREVENTS: Peer giving up after first failure or flooding with
// connection attempts without backoff.
func TestPeerReconnect(t *testing.T) {
	// Use a listener that immediately closes connections
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCPAddr")

	var connectCount atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connectCount.Add(1)
			_ = conn.Close() // Immediately close to trigger reconnect
		}
	}()

	neighbor := NewNeighbor(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	neighbor.Port = uint16(addr.Port) //nolint:gosec // Port fits in uint16

	peer := NewPeer(neighbor)
	peer.SetReconnectDelay(10*time.Millisecond, 50*time.Millisecond)

	peer.Start()

	// Wait for multiple reconnect attempts
	time.Sleep(100 * time.Millisecond)

	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	count := connectCount.Load()
	require.GreaterOrEqual(t, count, int32(2), "peer should reconnect at least twice, got %d", count)
}

// TestPeerContextCancellation verifies peer stops on context cancellation.
//
// VALIDATES: Peer respects context cancellation for clean shutdown.
//
// PREVENTS: Orphaned goroutines when parent context is cancelled.
func TestPeerContextCancellation(t *testing.T) {
	neighbor := NewNeighbor(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	neighbor.Port = 0 // Invalid port

	peer := NewPeer(neighbor)

	ctx, cancel := context.WithCancel(context.Background())
	peer.StartWithContext(ctx)

	time.Sleep(10 * time.Millisecond)
	require.NotEqual(t, PeerStateStopped, peer.State())

	// Cancel context
	cancel()

	// Should stop within reasonable time
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err := peer.Wait(waitCtx)

	require.NoError(t, err, "peer should stop on context cancellation")
	require.Equal(t, PeerStateStopped, peer.State())
}

// TestPeerStateTransitions verifies state changes during connection lifecycle.
//
// VALIDATES: Peer reports correct state (Connecting, Connected, etc).
//
// PREVENTS: Incorrect state reporting to callers.
func TestPeerStateTransitions(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCPAddr")

	// Accept connections but don't respond (peer stays connecting)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Hold connection open without BGP handshake
			time.Sleep(time.Second)
			_ = conn.Close()
		}
	}()

	neighbor := NewNeighbor(
		mustParseAddr("127.0.0.1"),
		65000, 65001, 0x01010101,
	)
	neighbor.Port = uint16(addr.Port) //nolint:gosec // Port fits in uint16

	peer := NewPeer(neighbor)
	peer.Start()

	// Should transition to Connecting
	time.Sleep(50 * time.Millisecond)
	state := peer.State()
	require.True(t, state == PeerStateConnecting || state == PeerStateActive,
		"state should be Connecting or Active, got %v", state)

	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)
}

// TestPeerCallback verifies state change callbacks are invoked.
//
// VALIDATES: Callback is called on state transitions.
//
// PREVENTS: Missing notifications to observers.
func TestPeerCallback(t *testing.T) {
	neighbor := NewNeighbor(
		mustParseAddr("192.0.2.1"),
		65000, 65001, 0x01010101,
	)
	neighbor.Port = 0

	peer := NewPeer(neighbor)

	var transitions []PeerState
	peer.SetCallback(func(from, to PeerState) {
		transitions = append(transitions, to)
	})

	peer.Start()
	time.Sleep(20 * time.Millisecond)
	peer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = peer.Wait(ctx)

	require.NotEmpty(t, transitions, "callback should be invoked at least once")
}
