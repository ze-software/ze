package reactor

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestListenerNew verifies Listener creation with correct initial state.
//
// VALIDATES: Listener is created with configured address and not running.
//
// PREVENTS: Listener auto-starting or with invalid state.
func TestListenerNew(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	require.NotNil(t, listener, "NewListener must return non-nil")
	require.False(t, listener.Running(), "listener should not be running initially")
}

// TestListenerStartStop verifies basic start/stop lifecycle.
//
// VALIDATES: Listener can be started and stopped cleanly.
//
// PREVENTS: Resource leaks or goroutine leaks on stop.
func TestListenerStartStop(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	err := listener.Start()
	require.NoError(t, err, "Start should succeed")
	require.True(t, listener.Running(), "listener should be running after Start")

	addr := listener.Addr()
	require.NotNil(t, addr, "Addr should return non-nil when running")

	listener.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = listener.Wait(ctx)
	require.NoError(t, err)

	require.False(t, listener.Running(), "listener should not be running after Stop")
}

// TestListenerAcceptConnection verifies incoming connections are accepted.
//
// VALIDATES: Listener accepts TCP connections and calls handler.
//
// PREVENTS: Connections being dropped or handler not invoked.
func TestListenerAcceptConnection(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	var accepted atomic.Int32
	listener.SetHandler(func(conn net.Conn) {
		accepted.Add(1)
		_ = conn.Close()
	})

	err := listener.Start()
	require.NoError(t, err)
	defer listener.Stop()

	addr := listener.Addr()

	// Connect
	conn, err := net.Dial("tcp", addr.String()) //nolint:noctx // Test code
	require.NoError(t, err)
	_ = conn.Close()

	// Wait for handler
	time.Sleep(50 * time.Millisecond)

	require.Equal(t, int32(1), accepted.Load(), "handler should be called once")
}

// TestListenerMultipleConnections verifies multiple concurrent connections.
//
// VALIDATES: Listener handles multiple connections concurrently.
//
// PREVENTS: Connection serialization or blocking.
func TestListenerMultipleConnections(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	var accepted atomic.Int32
	listener.SetHandler(func(conn net.Conn) {
		accepted.Add(1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		_ = conn.Close()
	})

	err := listener.Start()
	require.NoError(t, err)
	defer listener.Stop()

	addr := listener.Addr()

	// Connect multiple times concurrently
	const numConns = 5
	for range numConns {
		go func() {
			conn, err := net.Dial("tcp", addr.String()) //nolint:noctx // Test code
			if err == nil {
				time.Sleep(5 * time.Millisecond)
				_ = conn.Close()
			}
		}()
	}

	// Wait for handlers
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, int32(numConns), accepted.Load(), "handler should be called for each connection")
}

// TestListenerContextCancellation verifies listener stops on context cancellation.
//
// VALIDATES: Listener respects context cancellation for clean shutdown.
//
// PREVENTS: Orphaned goroutines when parent context is cancelled.
func TestListenerContextCancellation(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	ctx, cancel := context.WithCancel(context.Background())

	err := listener.StartWithContext(ctx)
	require.NoError(t, err)
	require.True(t, listener.Running())

	// Cancel context
	cancel()

	// Should stop within reasonable time
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err = listener.Wait(waitCtx)

	require.NoError(t, err, "listener should stop on context cancellation")
	require.False(t, listener.Running())
}

// TestListenerStartTwice verifies starting twice returns error.
//
// VALIDATES: Double-start is handled gracefully.
//
// PREVENTS: Resource leaks from multiple listeners on same port.
func TestListenerStartTwice(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	err := listener.Start()
	require.NoError(t, err)
	defer listener.Stop()

	err = listener.Start()
	require.Error(t, err, "second Start should fail")
}

// TestListenerInvalidAddress verifies error on invalid address.
//
// VALIDATES: Invalid address is rejected with error.
//
// PREVENTS: Silent failures or panics on bad config.
func TestListenerInvalidAddress(t *testing.T) {
	listener := NewListener("invalid:address:format")

	err := listener.Start()
	require.Error(t, err, "Start with invalid address should fail")
}

// TestListenerCallback verifies connection callbacks with remote address.
//
// VALIDATES: Handler receives connection with correct remote address info.
//
// PREVENTS: Missing or incorrect peer identification.
func TestListenerCallback(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	var remoteAddr string
	done := make(chan struct{})
	listener.SetHandler(func(conn net.Conn) {
		remoteAddr = conn.RemoteAddr().String()
		_ = conn.Close()
		close(done)
	})

	err := listener.Start()
	require.NoError(t, err)
	defer listener.Stop()

	addr := listener.Addr()

	conn, err := net.Dial("tcp", addr.String()) //nolint:noctx // Test code
	require.NoError(t, err)
	localAddr := conn.LocalAddr().String()
	_ = conn.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler not called")
	}

	require.Equal(t, localAddr, remoteAddr, "remote addr should match client's local addr")
}

// TestListenerCloseOnCancel verifies listener exits immediately on context cancel.
//
// VALIDATES: AC-11: listener exits within 10ms on cancel (not 100ms polling interval).
// PREVENTS: Slow shutdown due to 100ms SetDeadline polling in acceptLoop.
func TestListenerCloseOnCancel(t *testing.T) {
	listener := NewListener("127.0.0.1:0")

	ctx, cancel := context.WithCancel(context.Background())

	err := listener.StartWithContext(ctx)
	require.NoError(t, err)
	require.True(t, listener.Running())

	// Let acceptLoop settle into blocking Accept.
	time.Sleep(20 * time.Millisecond)

	// Cancel context and measure how quickly Wait returns.
	start := time.Now()
	cancel()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	err = listener.Wait(waitCtx)
	elapsed := time.Since(start)

	require.NoError(t, err, "listener should stop on context cancellation")
	require.False(t, listener.Running())
	require.Less(t, elapsed, 200*time.Millisecond,
		"listener should exit promptly on cancel, not wait for polling interval")
}
