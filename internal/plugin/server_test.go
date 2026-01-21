package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/bgp/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustParseAddr parses an IP address or fails the test.
func mustParseAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	require.NoError(t, err)
	return addr
}

// parseJSON unmarshals JSON string into a map.
func parseJSON(t *testing.T, s string, result *map[string]any) error {
	t.Helper()
	return json.Unmarshal([]byte(s), result)
}

// TypeNOTIFICATION is message.TypeNOTIFICATION for test convenience.
const TypeNOTIFICATION = message.TypeNOTIFICATION

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

// TestParseSerial verifies serial parsing from command lines.
//
// VALIDATES: #N prefix is correctly parsed as serial.
//
// PREVENTS: Incorrect serial extraction, comments confused with serials.
func TestParseSerial(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantSerial string
		wantCmd    string
	}{
		{"no prefix", "system version", "", "system version"},
		{"numeric serial", "#1 system version", "1", "system version"},
		{"multi-digit serial", "#123 update text", "123", "update text"},
		{"comment not serial", "# this is a comment", "", "# this is a comment"},
		{"alpha not serial", "#abc command", "", "#abc command"},
		{"hash only", "#", "", "#"},
		{"hash space", "# ", "", "# "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serial, cmd := parseSerial(tt.line)
			assert.Equal(t, tt.wantSerial, serial, "serial mismatch")
			assert.Equal(t, tt.wantCmd, cmd, "command mismatch")
		})
	}
}

// TestIsComment verifies comment detection.
//
// VALIDATES: Lines starting with "# " are detected as comments.
//
// PREVENTS: Commands mistakenly treated as comments.
func TestIsComment(t *testing.T) {
	tests := []struct {
		line   string
		isComm bool
	}{
		{"# this is a comment", true},
		{"#  indented comment", true},
		{"#1 command with serial", false},
		{"system version", false},
		{"#", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			assert.Equal(t, tt.isComm, isComment(tt.line))
		})
	}
}

// TestEncodeAlphaSerial verifies alpha serial encoding.
//
// VALIDATES: Numbers are encoded as shifted digits (0->a, 1->b, ..., 9->j).
//
// PREVENTS: Collision with numeric serials from processes.
func TestEncodeAlphaSerial(t *testing.T) {
	tests := []struct {
		n    uint64
		want string
	}{
		{0, "a"},
		{1, "b"},
		{9, "j"},
		{10, "ba"},
		{99, "jj"},
		{123, "bcd"},
		{1000, "baaa"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := encodeAlphaSerial(tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsAlphaSerial verifies alpha serial detection.
//
// VALIDATES: Only a-j characters are recognized as alpha serials.
//
// PREVENTS: Numeric or invalid serials being mistaken for alpha.
func TestIsAlphaSerial(t *testing.T) {
	tests := []struct {
		serial string
		want   bool
	}{
		{"a", true},
		{"abc", true},
		{"bcd", true},
		{"j", true},
		{"jjj", true},
		{"k", false},    // k is out of range
		{"abc1", false}, // contains digit
		{"123", false},  // all digits
		{"", false},     // empty
		{"ABC", false},  // uppercase
	}

	for _, tt := range tests {
		t.Run(tt.serial, func(t *testing.T) {
			got := isAlphaSerial(tt.serial)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestServerNoSerialCommand verifies commands without serial get response without serial field.
//
// VALIDATES: Response is sent, serial field omitted from JSON.
//
// PREVENTS: Missing responses or unwanted empty serial field.
func TestServerNoSerialCommand(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Send command without serial
	_, err = conn.Write([]byte("system version\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// Should NOT have serial field (omitempty)
	assert.NotContains(t, response, `"serial"`)
	assert.Contains(t, response, `"status":"done"`)
}

// TestServerSerialCommand verifies serial prefix handling via socket.
//
// VALIDATES: Command with #N prefix returns response with serial field.
//
// PREVENTS: Serial not being echoed back in response.
func TestServerSerialCommand(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Send command with serial
	_, err = conn.Write([]byte("#42 system version\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// Should have serial in JSON body (no @prefix for JSON)
	assert.Contains(t, response, `"serial":"42"`)
	assert.Contains(t, response, `"status":"done"`)
}

// TestServerFormatMessageNotificationJSON verifies Server.formatMessage with NOTIFICATION+JSON.
//
// VALIDATES: Notification JSON output has fields at top level (flat format).
// PREVENTS: Plugin receiving malformed JSON or missing fields for NOTIFICATION events.
func TestServerFormatMessageNotificationJSON(t *testing.T) {
	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: "/tmp/unused.sock"}, reactor)

	peer := PeerInfo{
		Address:      mustParseAddr(t, "10.0.0.1"),
		LocalAddress: mustParseAddr(t, "10.0.0.2"),
		LocalAS:      65001,
		PeerAS:       65002,
	}

	// NOTIFICATION: Cease/Administrative Shutdown with message "goodbye"
	rawBytes := []byte{
		0x06, // Error code: Cease (6)
		0x02, // Subcode: Administrative Shutdown (2)
		0x07, // Message length: 7
		'g', 'o', 'o', 'd', 'b', 'y', 'e',
	}

	msg := RawMessage{
		Type:      TypeNOTIFICATION,
		RawBytes:  rawBytes,
		MessageID: 42,
		Direction: "received",
	}

	binding := PeerProcessBinding{
		Encoding:            EncodingJSON,
		Format:              FormatParsed,
		ReceiveNotification: true,
	}

	output := server.formatMessage(peer, msg, binding, "")

	// Parse JSON
	var result map[string]any
	err := parseJSON(t, output, &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// Check message wrapper (includes type, id, direction)
	msgWrapper, ok := result["message"].(map[string]any)
	require.True(t, ok, "message wrapper must exist")
	assert.Equal(t, "notification", msgWrapper["type"])
	assert.Equal(t, float64(42), msgWrapper["id"])
	assert.Equal(t, "received", msgWrapper["direction"])

	// Check peer structure (flat format)
	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "peer must be object")
	assert.Equal(t, "10.0.0.1", peerMap["address"])
	assert.Equal(t, float64(65002), peerMap["asn"])

	// Notification fields at top level (flat format, hyphenated names)
	assert.Equal(t, float64(6), result["code"])
	assert.Equal(t, float64(2), result["subcode"])
	assert.Equal(t, "Cease", result["code-name"])
	assert.Equal(t, "Administrative Shutdown", result["subcode-name"])
}

// TestServerFormatMessageNotificationText verifies Server.formatMessage with NOTIFICATION+text.
//
// VALIDATES: Notification text output is parseable.
// PREVENTS: Plugin receiving malformed text for NOTIFICATION events.
func TestServerFormatMessageNotificationText(t *testing.T) {
	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: "/tmp/unused.sock"}, reactor)

	peer := PeerInfo{
		Address: mustParseAddr(t, "10.0.0.1"),
		PeerAS:  65002,
	}

	// NOTIFICATION: Hold Timer Expired
	rawBytes := []byte{0x04, 0x00}

	msg := RawMessage{
		Type:      TypeNOTIFICATION,
		RawBytes:  rawBytes,
		MessageID: 99,
		Direction: "received",
	}

	binding := PeerProcessBinding{
		Encoding:            EncodingText,
		Format:              FormatParsed,
		ReceiveNotification: true,
	}

	output := server.formatMessage(peer, msg, binding, "")

	// Verify text format
	assert.Contains(t, output, "peer 10.0.0.1")
	assert.Contains(t, output, "received")
	assert.Contains(t, output, "notification")
	assert.Contains(t, output, "99")     // msg-id
	assert.Contains(t, output, "code 4") // error code
	assert.Contains(t, output, "subcode 0")
}

// TestServerFormatMessageOverrideDir verifies overrideDir parameter works.
//
// VALIDATES: overrideDir overrides msg.Direction in formatted output.
// PREVENTS: Incorrect direction in forwarded/echoed messages.
func TestServerFormatMessageOverrideDir(t *testing.T) {
	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: "/tmp/unused.sock"}, reactor)

	peer := PeerInfo{
		Address: mustParseAddr(t, "10.0.0.1"),
		PeerAS:  65002,
	}

	msg := RawMessage{
		Type:      TypeNOTIFICATION,
		RawBytes:  []byte{0x04, 0x00},
		MessageID: 1,
		Direction: "received", // Original direction
	}

	binding := PeerProcessBinding{
		Encoding:            EncodingText,
		Format:              FormatParsed,
		ReceiveNotification: true,
	}

	// Without override - uses msg.Direction
	output1 := server.formatMessage(peer, msg, binding, "")
	assert.Contains(t, output1, "received")
	assert.NotContains(t, output1, "sent")

	// With override - uses overrideDir
	output2 := server.formatMessage(peer, msg, binding, "sent")
	assert.Contains(t, output2, "sent")
	assert.NotContains(t, output2, "received")
}
