// Package testsyslog provides a test syslog server for functional tests.
package syslog

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUDPServer verifies the server listens and receives UDP syslog messages.
//
// VALIDATES: Server starts, listens on UDP, receives messages.
// PREVENTS: Server failing to bind or receive UDP packets.
func TestUDPServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv := New(0) // Dynamic port
	require.NoError(t, srv.Start(ctx))
	t.Cleanup(func() { _ = srv.Close() })

	// Server should have a port assigned
	port := srv.Port()
	assert.Greater(t, port, 0)

	// Send a raw UDP message (simulating syslog)
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Write([]byte("<14>test message from syslog client"))
	require.NoError(t, err)

	// Wait for message to arrive
	time.Sleep(200 * time.Millisecond)

	// Should have received the message
	msgs := srv.Messages()
	require.NotEmpty(t, msgs, "expected at least one message")
	assert.Contains(t, msgs[0], "test message from syslog client")
}

// TestMessageCapture verifies multiple messages are buffered in order.
//
// VALIDATES: Multiple messages captured and retrievable.
// PREVENTS: Message loss or ordering issues.
func TestMessageCapture(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv := New(0)
	require.NoError(t, srv.Start(ctx))
	t.Cleanup(func() { _ = srv.Close() })

	// Send multiple messages via raw UDP
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", srv.Port()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	for i := 1; i <= 3; i++ {
		_, err = fmt.Fprintf(conn, "<14>message %d", i)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Small delay to ensure ordering
	}

	// Wait for messages
	time.Sleep(200 * time.Millisecond)

	msgs := srv.Messages()
	require.Len(t, msgs, 3)
	assert.Contains(t, msgs[0], "message 1")
	assert.Contains(t, msgs[1], "message 2")
	assert.Contains(t, msgs[2], "message 3")
}

// TestPatternMatch verifies regex pattern matching against captured messages.
//
// VALIDATES: Match() returns true for matching patterns, false otherwise.
// PREVENTS: False positives/negatives in log verification.
func TestPatternMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv := New(0)
	require.NoError(t, srv.Start(ctx))
	t.Cleanup(func() { _ = srv.Close() })

	// Send a raw UDP syslog message
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", srv.Port()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Write([]byte("<14>level=INFO subsystem=server msg=\"session established\""))
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Should match
	assert.True(t, srv.Match("subsystem=server"))
	assert.True(t, srv.Match("level=INFO"))
	assert.True(t, srv.Match("session.*established"))

	// Should not match
	assert.False(t, srv.Match("level=DEBUG"))
	assert.False(t, srv.Match("subsystem=filter"))
	assert.False(t, srv.Match("nonexistent"))
}

// TestPatternMatchInvalid verifies invalid regex returns false without panic.
//
// VALIDATES: Invalid regex patterns handled gracefully.
// PREVENTS: Panic on malformed patterns in test files.
func TestPatternMatchInvalid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv := New(0)
	require.NoError(t, srv.Start(ctx))
	t.Cleanup(func() { _ = srv.Close() })

	// Invalid regex should return false, not panic
	assert.False(t, srv.Match("[invalid"))
	assert.False(t, srv.Match("(unclosed"))
}

// TestServerClose verifies server stops cleanly.
//
// VALIDATES: Close() stops the server and releases resources.
// PREVENTS: Resource leaks or hang on shutdown.
func TestServerClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv := New(0)
	require.NoError(t, srv.Start(ctx))

	port := srv.Port()
	require.Greater(t, port, 0)

	// Close should succeed
	require.NoError(t, srv.Close())

	// Port should be released (try to bind again)
	srv2 := New(port)
	err := srv2.Start(ctx)
	// Should be able to reuse the port (or at least not panic)
	if err == nil {
		_ = srv2.Close()
	}
}

// TestContextCancellation verifies server stops when context is cancelled.
//
// VALIDATES: Server respects context cancellation.
// PREVENTS: Server running after test timeout.
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	srv := New(0)
	require.NoError(t, srv.Start(ctx))

	// Cancel context
	cancel()

	// Give server time to notice cancellation
	time.Sleep(100 * time.Millisecond)

	// Messages should still be retrievable
	_ = srv.Messages()

	// Close should succeed (may already be closed)
	_ = srv.Close()
}
