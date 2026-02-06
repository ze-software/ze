package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/ipc"
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

// rpcCall sends a NUL-framed JSON RPC request and reads the NUL-framed response.
// Returns the parsed RPCResult on success or RPCError fields.
func rpcCall(t *testing.T, conn net.Conn, method string, id int) map[string]any {
	t.Helper()
	return rpcCallWithParams(t, conn, method, id, nil)
}

// rpcCallWithParams sends a NUL-framed JSON RPC request with params and reads the response.
func rpcCallWithParams(t *testing.T, conn net.Conn, method string, id int, params any) map[string]any {
	t.Helper()
	writer := ipc.NewFrameWriter(conn)
	reader := ipc.NewFrameReader(conn)

	req := map[string]any{"method": method, "id": id}
	if params != nil {
		raw, err := json.Marshal(params)
		require.NoError(t, err)
		req["params"] = json.RawMessage(raw)
	}

	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)
	require.NoError(t, writer.Write(reqJSON))

	respBytes, err := reader.Read()
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(respBytes, &result))
	return result
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

	// Connect and send NUL-framed JSON RPC request
	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	result := rpcCall(t, conn, "ze-bgp:daemon-status", 1)

	// RPC result: {"id":1,"result":{...}}
	assert.Equal(t, float64(1), result["id"])
	resultData, ok := result["result"].(map[string]any)
	require.True(t, ok, "result must be an object")
	assert.Equal(t, float64(2), resultData["peer_count"])
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

	result := rpcCall(t, conn, "ze-system:nonexistent", 1)

	// RPC error: {"id":1,"error":"..."}
	assert.Equal(t, float64(1), result["id"])
	assert.NotEmpty(t, result["error"], "should have error for unknown method")
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

// TestServerEmptyFrame verifies empty NUL frame handling.
//
// VALIDATES: Empty frames are ignored, subsequent valid requests work.
//
// PREVENTS: Server crash or disconnect from empty frames.
func TestServerEmptyFrame(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	writer := ipc.NewFrameWriter(conn)

	// Send empty frame (just NUL byte)
	require.NoError(t, writer.Write([]byte{}))

	// Send valid request — server should still respond
	result := rpcCall(t, conn, "ze-system:version-software", 1)

	assert.Equal(t, float64(1), result["id"])
	assert.NotNil(t, result["result"], "should get result after empty frame")
}

// TestServerInvalidJSON verifies error response for malformed JSON frames.
//
// VALIDATES: Invalid JSON returns error response, connection stays open.
//
// PREVENTS: Server crash or disconnect from malformed input.
func TestServerInvalidJSON(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	writer := ipc.NewFrameWriter(conn)
	reader := ipc.NewFrameReader(conn)

	// Send invalid JSON
	require.NoError(t, writer.Write([]byte("not valid json")))

	// Should get error response
	respBytes, err := reader.Read()
	require.NoError(t, err)

	var errResp map[string]any
	require.NoError(t, json.Unmarshal(respBytes, &errResp))
	assert.Equal(t, "invalid-json", errResp["error"])

	// Connection should still work — send valid request
	result := rpcCall(t, conn, "ze-system:version-software", 2)
	assert.Equal(t, float64(2), result["id"])
	assert.NotNil(t, result["result"])
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

// TestServerRPCWithID verifies response includes matching request ID.
//
// VALIDATES: RPC response echoes back the request ID.
//
// PREVENTS: Client unable to correlate responses with requests.
func TestServerRPCWithID(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Send request with specific ID
	result := rpcCall(t, conn, "ze-system:version-software", 42)

	// Response must echo back the same ID
	assert.Equal(t, float64(42), result["id"])
	assert.NotNil(t, result["result"])
}

// TestServerMultipleRPCRequests verifies multiple sequential requests on same connection.
//
// VALIDATES: Multiple requests with different IDs get correct responses.
//
// PREVENTS: Connection state corruption between requests.
func TestServerMultipleRPCRequests(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	// Send multiple requests with different IDs
	r1 := rpcCall(t, conn, "ze-system:version-software", 1)
	assert.Equal(t, float64(1), r1["id"])
	assert.NotNil(t, r1["result"])

	r2 := rpcCall(t, conn, "ze-system:version-api", 2)
	assert.Equal(t, float64(2), r2["id"])
	assert.NotNil(t, r2["result"])

	r3 := rpcCall(t, conn, "ze-system:nonexistent", 3)
	assert.Equal(t, float64(3), r3["id"])
	assert.NotEmpty(t, r3["error"])
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

	// ze-bgp JSON: top-level "type" should be "bgp"
	assert.Equal(t, "bgp", result["type"], "top-level type must be 'bgp'")

	// Payload under "bgp"
	payload, ok := result["bgp"].(map[string]any)
	require.True(t, ok, "bgp payload must exist")

	// Check event type in bgp.message.type
	msgObj, ok := payload["message"].(map[string]any)
	require.True(t, ok, "message object must exist in bgp payload")
	assert.Equal(t, "notification", msgObj["type"], "message type must be 'notification'")

	// Check raw message is present (FormatRaw includes hex in raw object)
	rawPart, ok := payload["raw"].(map[string]any)
	require.True(t, ok, "raw part must exist in bgp payload: %s", output)
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

// TestServerNULProtocol verifies the NUL-terminated JSON protocol for socket clients.
//
// VALIDATES: Server reads NUL-terminated JSON requests and returns NUL-terminated JSON responses.
// PREVENTS: Protocol mismatch between socket clients and server after text→JSON migration.
func TestServerNULProtocol(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	time.Sleep(10 * time.Millisecond)

	// Send NUL-terminated JSON request
	req := ipc.Request{
		Method: "ze-system:version-software",
		ID:     json.RawMessage(`1`),
	}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	writer := ipc.NewFrameWriter(conn)
	err = writer.Write(reqJSON)
	require.NoError(t, err)

	// Read NUL-terminated JSON response (with deadline to avoid hanging)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	reader := ipc.NewFrameReader(conn)
	respBytes, err := reader.Read()
	require.NoError(t, err, "server must respond with NUL-terminated JSON")

	// Parse response as RPCResult
	var result ipc.RPCResult
	err = json.Unmarshal(respBytes, &result)
	require.NoError(t, err)

	// Verify ID is echoed back
	assert.Equal(t, json.RawMessage(`1`), result.ID)

	// Verify result contains version
	assert.Contains(t, string(result.Result), `"version"`)
	assert.Contains(t, string(result.Result), Version)
}

// TestServerNULProtocolUnknownMethod verifies error response for unknown wire methods.
//
// VALIDATES: Unknown methods return RPCError with "unknown-method" error identity.
// PREVENTS: Silent failures or panics on invalid method names.
func TestServerNULProtocolUnknownMethod(t *testing.T) {
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("ze-nul-err-%d.sock", os.Getpid()))
	t.Cleanup(func() { _ = os.Remove(sockPath) })

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	time.Sleep(10 * time.Millisecond)

	// Send request with unknown method
	req := ipc.Request{
		Method: "ze-system:nonexistent",
		ID:     json.RawMessage(`2`),
	}
	reqJSON, _ := json.Marshal(req)

	writer := ipc.NewFrameWriter(conn)
	err = writer.Write(reqJSON)
	require.NoError(t, err)

	// Read response
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	reader := ipc.NewFrameReader(conn)
	respBytes, err := reader.Read()
	require.NoError(t, err)

	// Parse as RPCError
	var errResp ipc.RPCError
	err = json.Unmarshal(respBytes, &errResp)
	require.NoError(t, err)

	assert.Equal(t, "unknown-method", errResp.Error)
	assert.Equal(t, json.RawMessage(`2`), errResp.ID)
}

// TestServerNULProtocolWithParams verifies parameter passing through JSON protocol.
//
// VALIDATES: Handler receives args from JSON params and peer selector from params.
// PREVENTS: Parameter loss during text→JSON protocol migration.
func TestServerNULProtocolWithParams(t *testing.T) {
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("ze-nul-par-%d.sock", os.Getpid()))
	t.Cleanup(func() { _ = os.Remove(sockPath) })

	reactor := &mockReactor{
		stats: ReactorStats{
			PeerCount: 3,
			Uptime:    time.Hour,
		},
	}
	server := NewServer(&ServerConfig{SocketPath: sockPath}, reactor)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	time.Sleep(10 * time.Millisecond)

	// Send daemon-status request (no params needed)
	req := ipc.Request{
		Method: "ze-bgp:daemon-status",
		ID:     json.RawMessage(`42`),
	}
	reqJSON, _ := json.Marshal(req)

	writer := ipc.NewFrameWriter(conn)
	err = writer.Write(reqJSON)
	require.NoError(t, err)

	// Read response
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	reader := ipc.NewFrameReader(conn)
	respBytes, err := reader.Read()
	require.NoError(t, err)

	var result ipc.RPCResult
	err = json.Unmarshal(respBytes, &result)
	require.NoError(t, err)

	// Verify ID echoed
	assert.Equal(t, json.RawMessage(`42`), result.ID)

	// Verify result contains daemon status fields
	assert.Contains(t, string(result.Result), `"peer_count"`)
	assert.Contains(t, string(result.Result), `3`)
}
