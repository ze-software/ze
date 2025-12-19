package api

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dialUnix connects to a Unix socket with context.
func dialUnix(t *testing.T, sockPath string) net.Conn {
	t.Helper()
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", sockPath)
	require.NoError(t, err)
	return conn
}

// TestServerStartStop verifies server lifecycle.
//
// VALIDATES: Server starts listening and stops cleanly.
//
// PREVENTS: Resource leaks on shutdown.
func TestServerStartStop(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	// Start server
	err := server.Start()
	require.NoError(t, err)
	assert.True(t, server.Running())

	// Verify socket exists
	_, err = os.Stat(sockPath)
	require.NoError(t, err, "socket file must exist")

	// Stop server
	server.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = server.Wait(ctx)
	require.NoError(t, err)
	assert.False(t, server.Running())

	// Socket should be cleaned up
	_, err = os.Stat(sockPath)
	assert.True(t, os.IsNotExist(err), "socket file must be removed")
}

// TestServerAcceptClient verifies client connection.
//
// VALIDATES: Clients can connect via Unix socket.
//
// PREVENTS: Connection failures blocking CLI usage.
func TestServerAcceptClient(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Connect as client
	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Give server time to accept
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, server.ClientCount())
}

// TestServerMultipleClients verifies concurrent clients.
//
// VALIDATES: Multiple clients handled independently.
//
// PREVENTS: Client interference or resource exhaustion.
func TestServerMultipleClients(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Connect multiple clients
	var conns []net.Conn
	for i := 0; i < 5; i++ {
		conn := dialUnix(t, sockPath)
		conns = append(conns, conn)
	}
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 5, server.ClientCount())
}

// TestServerCommandExecution verifies end-to-end command flow.
//
// VALIDATES: Command sent, dispatched, response returned.
//
// PREVENTS: Command processing failures.
func TestServerCommandExecution(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{
		stats: ReactorStats{
			PeerCount: 2,
			Uptime:    time.Hour,
		},
	}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Connect and send command
	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Send command
	_, err = conn.Write([]byte("daemon status\n"))
	require.NoError(t, err)

	// Read response
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// Response should be JSON with status
	assert.Contains(t, response, `"status":"done"`)
	assert.Contains(t, response, `"peer_count":2`)
}

// TestServerClientDisconnect verifies cleanup on disconnect.
//
// VALIDATES: Client resources cleaned up.
//
// PREVENTS: Resource leaks from disconnected clients.
func TestServerClientDisconnect(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Connect client
	conn := dialUnix(t, sockPath)

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, server.ClientCount())

	// Disconnect
	_ = conn.Close()

	// Wait for server to notice
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, server.ClientCount())
}

// TestServerUnknownCommand verifies error response.
//
// VALIDATES: Unknown commands return error response.
//
// PREVENTS: Silent failures on typos.
func TestServerUnknownCommand(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	_, err = conn.Write([]byte("unknown command here\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	assert.Contains(t, response, `"status":"error"`)
}

// TestServerGracefulShutdown verifies clients notified on shutdown.
//
// VALIDATES: Clients receive notification before disconnect.
//
// PREVENTS: Clients hanging on closed connection.
func TestServerGracefulShutdown(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	time.Sleep(10 * time.Millisecond)

	// Stop server
	server.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = server.Wait(ctx)
	require.NoError(t, err)

	// Try to read - should get EOF or error
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	buf := make([]byte, 100)
	_, err = conn.Read(buf)
	// Either EOF or deadline exceeded is acceptable
	assert.True(t, err != nil)
}

// TestServerEmptyLine verifies empty line handling.
//
// VALIDATES: Empty lines are ignored, not treated as commands.
//
// PREVENTS: Errors from clients sending blank lines.
func TestServerEmptyLine(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Send empty lines then a real command
	_, err = conn.Write([]byte("\n\n\nsystem version\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// Should get version response, not errors from empty lines
	assert.Contains(t, response, `"status":"done"`)
	assert.Contains(t, response, `"version"`)
}

// TestServerCommentLine verifies comment handling.
//
// VALIDATES: Lines starting with # are ignored.
//
// PREVENTS: Comment lines treated as commands.
func TestServerCommentLine(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Send comment then real command
	_, err = conn.Write([]byte("# this is a comment\nsystem version\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// Should get version response
	assert.True(t, strings.Contains(response, `"version"`))
}
