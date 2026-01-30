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

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
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

	// Send command (Step 5: daemon moved to bgp daemon)
	_, err = conn.Write([]byte("bgp daemon status\n"))
	require.NoError(t, err)

	// Read response
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// IPC 2.0: Response wrapped in {"type":"response","response":{...}}
	assert.Contains(t, response, `"type":"response"`)
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

	// IPC 2.0: Response wrapped
	assert.Contains(t, response, `"type":"response"`)
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
	_, err = conn.Write([]byte("\n\n\nsystem version software\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// IPC 2.0: Should get wrapped version response, not errors from empty lines
	assert.Contains(t, response, `"type":"response"`)
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
	_, err = conn.Write([]byte("# this is a comment\nsystem version software\n"))
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
		{"no prefix", "system version software", "", "system version software"},
		{"numeric serial", "#1 system version software", "1", "system version software"},
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
		{"system version software", false},
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
	_, err = conn.Write([]byte("system version software\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// IPC 2.0: Wrapped response, should NOT have serial field (omitempty)
	assert.Contains(t, response, `"type":"response"`)
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
	_, err = conn.Write([]byte("#42 system version software\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	require.NoError(t, err)

	// IPC 2.0: Should have serial in JSON body inside response wrapper
	assert.Contains(t, response, `"type":"response"`)
	assert.Contains(t, response, `"serial":"42"`)
	assert.Contains(t, response, `"status":"done"`)
}

// TestFormatNotificationJSON verifies NOTIFICATION message JSON format.
//
// VALIDATES: Notification JSON output has correct structure for subscription delivery.
// PREVENTS: Plugin receiving malformed JSON or missing fields for NOTIFICATION events.
// NOTE: FormatMessage with FormatRaw returns JSON for non-UPDATE messages.
// FormatParsed returns text format for backwards compatibility.
func TestFormatNotificationJSON(t *testing.T) {
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
		Type:      message.TypeNOTIFICATION,
		RawBytes:  rawBytes,
		MessageID: 42,
		Direction: "received",
	}

	// Use FormatRaw for JSON output (FormatParsed returns text for non-UPDATE)
	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   FormatRaw,
	}

	output := FormatMessage(peer, msg, content, "")

	// Parse JSON to verify structure
	var result map[string]any
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "JSON must be valid: %s", output)

	// IPC 2.0: top-level "type" should be "bgp"
	assert.Equal(t, "bgp", result["type"], "top-level type must be 'bgp'")

	// Payload under "bgp"
	payload, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "bgp payload must exist")

	// Check event type in payload
	assert.Equal(t, "notification", payload["type"], "event type must be 'notification'")

	// Check raw message is present (FormatRaw includes hex)
	rawPart, ok := payload["raw"].(map[string]any)
	require.True(t, ok, "raw part must exist in bgp payload")
	assert.NotEmpty(t, rawPart["message"], "raw message must be present")
}

// mustParseAddr parses an IP address or fails the test.
func mustParseAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	require.NoError(t, err)
	return addr
}

// TestEncodeNLRI_NotConfigured verifies error when server has no plugin support.
//
// VALIDATES: EncodeNLRI returns error when registry/procManager nil.
//
// PREVENTS: Nil pointer dereference when plugins not configured.
func TestEncodeNLRI_NotConfigured(t *testing.T) {
	// Server without plugins
	server := NewServer(&ServerConfig{}, nil)

	_, err := server.EncodeNLRI(nlri.IPv4FlowSpec, []string{"destination", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured for plugins")
}

// TestEncodeNLRI_NoPlugin verifies error when no plugin registered for family.
//
// VALIDATES: EncodeNLRI returns clear error for unregistered families.
//
// PREVENTS: Confusing errors or silent failures.
func TestEncodeNLRI_NoPlugin(t *testing.T) {
	// Create server with plugins config but no actual plugins
	server := NewServer(&ServerConfig{
		Plugins: []PluginConfig{{Name: "test"}},
	}, nil)
	// Initialize registry manually (normally done in Start)
	server.registry = NewPluginRegistry()
	server.procManager = NewProcessManager(nil)

	_, err := server.EncodeNLRI(nlri.IPv4FlowSpec, []string{"destination", "10.0.0.0/24"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no plugin registered for family")
}

// TestDecodeNLRI_NotConfigured verifies error when server has no plugin support.
//
// VALIDATES: DecodeNLRI returns error when registry/procManager nil.
//
// PREVENTS: Nil pointer dereference when plugins not configured.
func TestDecodeNLRI_NotConfigured(t *testing.T) {
	server := NewServer(&ServerConfig{}, nil)

	_, err := server.DecodeNLRI(nlri.IPv4FlowSpec, "0701180A0000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured for plugins")
}

// TestDecodeNLRI_NoPlugin verifies error when no plugin registered for family.
//
// VALIDATES: DecodeNLRI returns clear error for unregistered families.
//
// PREVENTS: Confusing errors or silent failures.
func TestDecodeNLRI_NoPlugin(t *testing.T) {
	server := NewServer(&ServerConfig{
		Plugins: []PluginConfig{{Name: "test"}},
	}, nil)
	server.registry = NewPluginRegistry()
	server.procManager = NewProcessManager(nil)

	_, err := server.DecodeNLRI(nlri.IPv4FlowSpec, "0701180A0000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no plugin registered for family")
}
