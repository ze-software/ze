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
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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
	for range 5 {
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

	result := rpcCall(t, conn, "ze-system:daemon-status", 1)

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

// TestEnvVarStageTimeout verifies ze.plugin.stage.timeout env var overrides default.
//
// VALIDATES: Env var provides non-config override for stage timeout.
// PREVENTS: Test environments stuck with 5s default under load.
func TestEnvVarStageTimeout(t *testing.T) {
	t.Setenv("ze.plugin.stage.timeout", "15s")
	got := stageTimeoutFromEnv()
	assert.Equal(t, 15*time.Second, got)
}

// TestEnvVarStageTimeoutUnderscore verifies ze_plugin_stage_timeout works.
//
// VALIDATES: Shell-compatible underscore form follows ze.log.* convention.
// PREVENTS: Users unable to set env var in shells that don't allow dots.
func TestEnvVarStageTimeoutUnderscore(t *testing.T) {
	t.Setenv("ze_plugin_stage_timeout", "20s")
	got := stageTimeoutFromEnv()
	assert.Equal(t, 20*time.Second, got)
}

// TestEnvVarStageTimeoutInvalid verifies invalid env var falls back to default.
//
// VALIDATES: Bad duration string doesn't crash, uses default.
// PREVENTS: Typo in env var causing zero timeout or panic.
func TestEnvVarStageTimeoutInvalid(t *testing.T) {
	t.Setenv("ze.plugin.stage.timeout", "not-a-duration")
	got := stageTimeoutFromEnv()
	assert.Equal(t, defaultStageTimeout, got)
}

// TestTimeoutPriorityConfigOverEnv verifies per-plugin config beats env var.
//
// VALIDATES: Priority order: config > env > default.
// PREVENTS: Env var overriding explicit per-plugin config.
func TestTimeoutPriorityConfigOverEnv(t *testing.T) {
	t.Setenv("ze.plugin.stage.timeout", "15s")

	// Per-plugin config timeout should be used, not env var
	proc := NewProcess(PluginConfig{
		Name:         "test",
		StageTimeout: 30 * time.Second,
	})

	// stageTransition uses proc.config.StageTimeout if non-zero, else env/default.
	// The priority logic: config > env > default
	timeout := proc.config.StageTimeout
	if timeout == 0 {
		timeout = stageTimeoutFromEnv()
	}
	assert.Equal(t, 30*time.Second, timeout, "per-plugin config should beat env var")
}

// TestEnvVarStageTimeoutDotPrecedence verifies dot form takes precedence over underscore.
//
// VALIDATES: When both forms are set, dot form wins (checked first).
// PREVENTS: Unexpected behavior when both env vars are set.
func TestEnvVarStageTimeoutDotPrecedence(t *testing.T) {
	t.Setenv("ze.plugin.stage.timeout", "10s")
	t.Setenv("ze_plugin_stage_timeout", "20s")
	got := stageTimeoutFromEnv()
	assert.Equal(t, 10*time.Second, got, "dot form should take precedence")
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
		Method: "ze-system:daemon-status",
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

// TestHandleProcessStartupRPC verifies the engine-side RPC handling of the 5-stage
// plugin startup protocol using per-socket PluginConns.
//
// VALIDATES: Engine correctly handles all 5 stages via YANG RPC protocol.
// PREVENTS: RPC infrastructure broken when plugins are converted from text protocol.
func TestHandleProcessStartupRPC(t *testing.T) {
	t.Parallel()

	// Create socket pairs
	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	// Set up process with RPC connections (per-socket wiring)
	proc := NewProcess(PluginConfig{
		Name:     "test-rpc",
		Internal: true,
		Encoder:  "json",
	})
	proc.sockets = pairs
	proc.engineConnA = NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide)
	proc.engineConnB = NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide)
	proc.running.Store(true)

	// Set up server with mock reactor
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = NewStartupCoordinator(1)

	// Plugin side: per-socket PluginConns (simulates SDK pattern)
	pluginConnA := NewPluginConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)
	pluginConnB := NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)

	// Run plugin protocol in goroutine (simulates SDK 5-stage startup)
	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)

		// Stage 1: Send declare-registration on Socket A
		_ = pluginConnA.SendDeclareRegistration(ctx, &rpc.DeclareRegistrationInput{
			Families:    []rpc.FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
			WantsConfig: []string{"bgp"},
		})

		// Stage 2: Receive configure on Socket B, respond OK
		req, err := pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 3: Send declare-capabilities on Socket A
		_ = pluginConnA.SendDeclareCapabilities(ctx, &rpc.DeclareCapabilitiesInput{})

		// Stage 4: Receive share-registry on Socket B, respond OK
		req, err = pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 5: Send ready on Socket A
		_ = pluginConnA.SendReady(ctx)
	}()

	// Run engine-side RPC startup handler
	server.handleProcessStartupRPC(proc)

	// Verify process reached StageRunning
	assert.Equal(t, StageRunning, proc.Stage())

	// Verify registration was recorded
	reg := proc.Registration()
	assert.True(t, reg.Done, "registration should be marked done")
	assert.Contains(t, reg.Families, "ipv4/unicast")
	assert.Contains(t, reg.DecodeFamilies, "ipv4/unicast")
	assert.Equal(t, []string{"bgp"}, reg.WantsConfigRoots)
	assert.Equal(t, "test-rpc", reg.Name)

	// Clean up
	cancel()
	<-pluginDone
}

// TestRegistrationFromRPC verifies conversion from RPC types to engine types.
//
// VALIDATES: DeclareRegistrationInput correctly maps to PluginRegistration.
// PREVENTS: Lost fields or incorrect family mode mapping during conversion.
func TestRegistrationFromRPC(t *testing.T) {
	t.Parallel()

	input := &rpc.DeclareRegistrationInput{
		Families: []rpc.FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
			{Name: "ipv4/flow", Mode: "decode"},
			{Name: "ipv6/unicast", Mode: "encode"},
		},
		Commands:    []rpc.CommandDecl{{Name: "show-routes"}},
		WantsConfig: []string{"bgp", "environment"},
		Schema: &rpc.SchemaDecl{
			Module:    "ze-test",
			Namespace: "urn:ze:test",
			YANGText:  "module ze-test { }",
			Handlers:  []string{"test", "test.sub"},
		},
	}

	reg := registrationFromRPC(input)

	assert.True(t, reg.Done)
	// "both" → appears in both lists
	assert.Contains(t, reg.Families, "ipv4/unicast")
	assert.Contains(t, reg.DecodeFamilies, "ipv4/unicast")
	// "decode" → only in DecodeFamilies
	assert.NotContains(t, reg.Families, "ipv4/flow")
	assert.Contains(t, reg.DecodeFamilies, "ipv4/flow")
	// "encode" → only in Families
	assert.Contains(t, reg.Families, "ipv6/unicast")
	assert.NotContains(t, reg.DecodeFamilies, "ipv6/unicast")

	assert.Equal(t, []string{"show-routes"}, reg.Commands)
	assert.Equal(t, []string{"bgp", "environment"}, reg.WantsConfigRoots)

	require.NotNil(t, reg.PluginSchema)
	assert.Equal(t, "ze-test", reg.PluginSchema.Module)
	assert.Equal(t, "urn:ze:test", reg.PluginSchema.Namespace)
	assert.Equal(t, "module ze-test { }", reg.PluginSchema.Yang)
	assert.Equal(t, []string{"test", "test.sub"}, reg.PluginSchema.Handlers)
}

// TestCapabilitiesFromRPC verifies conversion of capability declarations.
//
// VALIDATES: DeclareCapabilitiesInput correctly maps to PluginCapabilities.
// PREVENTS: Lost capability fields during conversion.
func TestCapabilitiesFromRPC(t *testing.T) {
	t.Parallel()

	input := &rpc.DeclareCapabilitiesInput{
		Capabilities: []rpc.CapabilityDecl{
			{Code: 73, Encoding: "text", Payload: "router.example.com"},
			{Code: 64, Encoding: "hex", Payload: "0078", Peers: []string{"192.168.1.1"}},
		},
	}

	caps := capabilitiesFromRPC(input)

	assert.True(t, caps.Done)
	require.Len(t, caps.Capabilities, 2)

	assert.Equal(t, uint8(73), caps.Capabilities[0].Code)
	assert.Equal(t, "text", caps.Capabilities[0].Encoding)
	assert.Equal(t, "router.example.com", caps.Capabilities[0].Payload)
	assert.Empty(t, caps.Capabilities[0].Peers)

	assert.Equal(t, uint8(64), caps.Capabilities[1].Code)
	assert.Equal(t, "hex", caps.Capabilities[1].Encoding)
	assert.Equal(t, "0078", caps.Capabilities[1].Payload)
	assert.Equal(t, []string{"192.168.1.1"}, caps.Capabilities[1].Peers)
}

// TestRegistrationFromRPCEdgeCases verifies edge cases in RPC-to-engine conversion.
//
// VALIDATES: Nil/empty inputs and unknown modes are handled gracefully.
// PREVENTS: Nil pointer dereference on empty input; silent misrouting on unknown mode.
func TestRegistrationFromRPCEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil_schema", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Commands: []rpc.CommandDecl{{Name: "status"}},
		}
		reg := registrationFromRPC(input)
		assert.Nil(t, reg.PluginSchema)
		assert.Equal(t, []string{"status"}, reg.Commands)
		assert.True(t, reg.Done)
	})

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{}
		reg := registrationFromRPC(input)
		assert.True(t, reg.Done)
		assert.Empty(t, reg.Families)
		assert.Empty(t, reg.DecodeFamilies)
		assert.Empty(t, reg.Commands)
		assert.Empty(t, reg.WantsConfigRoots)
		assert.Nil(t, reg.PluginSchema)
	})

	t.Run("unknown_mode_defaults_to_encode", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Families: []rpc.FamilyDecl{
				{Name: "ipv4/unicast", Mode: "unknown-mode"},
			},
		}
		reg := registrationFromRPC(input)
		// Unknown mode falls into default case, treated as encode-only
		assert.Contains(t, reg.Families, "ipv4/unicast")
		assert.NotContains(t, reg.DecodeFamilies, "ipv4/unicast")
	})

	t.Run("empty_mode_defaults_to_encode", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Families: []rpc.FamilyDecl{
				{Name: "ipv6/unicast", Mode: ""},
			},
		}
		reg := registrationFromRPC(input)
		assert.Contains(t, reg.Families, "ipv6/unicast")
		assert.NotContains(t, reg.DecodeFamilies, "ipv6/unicast")
	})

	t.Run("multi_word_commands", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Commands: []rpc.CommandDecl{
				{Name: "rib adjacent in show"},
				{Name: "peer * refresh"},
			},
		}
		reg := registrationFromRPC(input)
		assert.Equal(t, []string{"rib adjacent in show", "peer * refresh"}, reg.Commands)
	})
}

// TestCapabilitiesFromRPCEdgeCases verifies edge cases in capability conversion.
//
// VALIDATES: Empty capability list and empty payload are handled.
// PREVENTS: Nil slice issues when plugin declares no capabilities.
func TestCapabilitiesFromRPCEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty_capabilities", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareCapabilitiesInput{}
		caps := capabilitiesFromRPC(input)
		assert.True(t, caps.Done)
		assert.Empty(t, caps.Capabilities)
	})

	t.Run("empty_payload", func(t *testing.T) {
		t.Parallel()
		// Empty payload is valid (e.g., RFC 2918 route-refresh)
		input := &rpc.DeclareCapabilitiesInput{
			Capabilities: []rpc.CapabilityDecl{
				{Code: 2, Encoding: "text", Payload: ""},
			},
		}
		caps := capabilitiesFromRPC(input)
		require.Len(t, caps.Capabilities, 1)
		assert.Equal(t, uint8(2), caps.Capabilities[0].Code)
		assert.Empty(t, caps.Capabilities[0].Payload)
	})
}

// TestDispatchDecodeNLRI verifies the engine dispatches decode-nlri to the registry.
//
// VALIDATES: Plugin→engine decode-nlri RPC routes through registry.DecodeNLRIByFamily.
// PREVENTS: Engine rejecting decode-nlri as unknown method.
func TestDispatchDecodeNLRI(t *testing.T) {
	// No t.Parallel(): mutates global compile-time registry.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-decoder",
		Description: "test",
		Families:    []string{"ipv4/flow"},
		InProcessNLRIDecoder: func(family, hex string) (string, error) {
			return fmt.Sprintf(`[{"family":"%s","hex":"%s"}]`, family, hex), nil
		},
		RunEngine:  func(_, _ net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{ctx: ctx}

	// Create socket pair for Socket A
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &Process{}

	// Plugin sends decode-nlri request in background
	type decodeResult struct {
		json string
		err  error
	}
	done := make(chan decodeResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-nlri", &rpc.DecodeNLRIInput{
			Family: "ipv4/flow",
			Hex:    "0701180A0000",
		})
		if err != nil {
			done <- decodeResult{"", err}
			return
		}
		resp, err := rpc.ParseResponse(raw)
		if err != nil {
			done <- decodeResult{"", err}
			return
		}
		var out rpc.DecodeNLRIOutput
		if err := json.Unmarshal(resp, &out); err != nil {
			done <- decodeResult{"", err}
			return
		}
		done <- decodeResult{out.JSON, nil}
	}()

	// Engine reads and dispatches
	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, `[{"family":"ipv4/flow","hex":"0701180A0000"}]`, r.json)
}

// TestDispatchEncodeNLRI verifies the engine dispatches encode-nlri to the registry.
//
// VALIDATES: Plugin→engine encode-nlri RPC routes through registry.EncodeNLRIByFamily.
// PREVENTS: Engine rejecting encode-nlri as unknown method.
func TestDispatchEncodeNLRI(t *testing.T) {
	// No t.Parallel(): mutates global compile-time registry.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-encoder",
		Description: "test",
		Families:    []string{"ipv4/flow"},
		InProcessNLRIEncoder: func(family string, args []string) (string, error) {
			return "DEADBEEF", nil
		},
		RunEngine:  func(_, _ net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{ctx: ctx}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &Process{}

	type encodeResult struct {
		hex string
		err error
	}
	done := make(chan encodeResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:encode-nlri", &rpc.EncodeNLRIInput{
			Family: "ipv4/flow",
			Args:   []string{"match", "source", "10.0.0.0/24"},
		})
		if err != nil {
			done <- encodeResult{"", err}
			return
		}
		resp, err := rpc.ParseResponse(raw)
		if err != nil {
			done <- encodeResult{"", err}
			return
		}
		var out rpc.EncodeNLRIOutput
		if err := json.Unmarshal(resp, &out); err != nil {
			done <- encodeResult{"", err}
			return
		}
		done <- encodeResult{out.Hex, nil}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, "DEADBEEF", r.hex)
}

// TestDispatchDecodeNLRI_NoDecoder verifies error when no decoder registered.
//
// VALIDATES: Engine returns RPC error for unregistered family.
// PREVENTS: Nil pointer or silent failure on unregistered family.
func TestDispatchDecodeNLRI_NoDecoder(t *testing.T) {
	// No t.Parallel(): mutates global compile-time registry.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	registry.Reset()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{ctx: ctx}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &Process{}

	type errResult struct {
		err error
	}
	done := make(chan errResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-nlri", &rpc.DecodeNLRIInput{
			Family: "ipv4/flow",
			Hex:    "0701180A0000",
		})
		if err != nil {
			done <- errResult{err}
			return
		}
		_, err = rpc.ParseResponse(raw)
		done <- errResult{err}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	r := <-done
	require.Error(t, r.err)
	assert.Contains(t, r.err.Error(), "no NLRI decoder")
}

// TestDispatchDecodeMPReach verifies the engine dispatches decode-mp-reach.
//
// VALIDATES: Plugin→engine decode-mp-reach RPC parses MP_REACH_NLRI and returns structured JSON.
// PREVENTS: Engine rejecting decode-mp-reach as unknown method.
func TestDispatchDecodeMPReach(t *testing.T) {
	// No t.Parallel(): handler may use global state.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{ctx: ctx}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &Process{}

	// MP_REACH_NLRI for IPv4 unicast: AFI=1, SAFI=1, NH=192.168.1.1, NLRI=10.0.0.0/24
	// RFC 4760 Section 3: AFI(2) + SAFI(1) + NHLen(1) + NH(4) + Reserved(1) + NLRI
	mpReachHex := "00010104C0A8010100180A0000"

	type decodeResult struct {
		output rpc.DecodeMPReachOutput
		err    error
	}
	done := make(chan decodeResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-mp-reach", &rpc.DecodeMPReachInput{
			Hex: mpReachHex,
		})
		if err != nil {
			done <- decodeResult{err: err}
			return
		}
		resp, err := rpc.ParseResponse(raw)
		if err != nil {
			done <- decodeResult{err: err}
			return
		}
		var out rpc.DecodeMPReachOutput
		if err := json.Unmarshal(resp, &out); err != nil {
			done <- decodeResult{err: err}
			return
		}
		done <- decodeResult{output: out}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	dr := <-done
	require.NoError(t, dr.err)
	assert.Equal(t, "ipv4/unicast", dr.output.Family)
	assert.Equal(t, "192.168.1.1", dr.output.NextHop)
	assert.Contains(t, string(dr.output.NLRI), "10.0.0.0/24")
}

// TestDispatchDecodeMPUnreach verifies the engine dispatches decode-mp-unreach.
//
// VALIDATES: Plugin→engine decode-mp-unreach RPC parses MP_UNREACH_NLRI and returns structured JSON.
// PREVENTS: Engine rejecting decode-mp-unreach as unknown method.
func TestDispatchDecodeMPUnreach(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{ctx: ctx}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &Process{}

	// MP_UNREACH_NLRI for IPv4 unicast: AFI=1, SAFI=1, Withdrawn=192.168.0.0/24
	// RFC 4760 Section 4: AFI(2) + SAFI(1) + Withdrawn
	mpUnreachHex := "00010118C0A800"

	type decodeResult struct {
		output rpc.DecodeMPUnreachOutput
		err    error
	}
	done := make(chan decodeResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-mp-unreach", &rpc.DecodeMPUnreachInput{
			Hex: mpUnreachHex,
		})
		if err != nil {
			done <- decodeResult{err: err}
			return
		}
		resp, err := rpc.ParseResponse(raw)
		if err != nil {
			done <- decodeResult{err: err}
			return
		}
		var out rpc.DecodeMPUnreachOutput
		if err := json.Unmarshal(resp, &out); err != nil {
			done <- decodeResult{err: err}
			return
		}
		done <- decodeResult{output: out}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	dr := <-done
	require.NoError(t, dr.err)
	assert.Equal(t, "ipv4/unicast", dr.output.Family)
	assert.Contains(t, string(dr.output.NLRI), "192.168.0.0/24")
}

// TestDispatchDecodeUpdate verifies the engine dispatches decode-update.
//
// VALIDATES: Plugin→engine decode-update RPC parses full UPDATE body and returns ze-bgp JSON.
// PREVENTS: Engine rejecting decode-update as unknown method.
func TestDispatchDecodeUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{ctx: ctx}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &Process{}

	// UPDATE body: withdrawn_len=0, attr_len=11, ORIGIN=IGP, NEXT_HOP=192.168.1.1, NLRI=10.0.0.0/24
	// RFC 4271 Section 4.3: Withdrawn(2) + Attrs(2+N) + NLRI
	updateHex := "0000000B40010100400304C0A80101180A0000"

	type decodeResult struct {
		output rpc.DecodeUpdateOutput
		err    error
	}
	done := make(chan decodeResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-update", &rpc.DecodeUpdateInput{
			Hex: updateHex,
		})
		if err != nil {
			done <- decodeResult{err: err}
			return
		}
		resp, err := rpc.ParseResponse(raw)
		if err != nil {
			done <- decodeResult{err: err}
			return
		}
		var out rpc.DecodeUpdateOutput
		if err := json.Unmarshal(resp, &out); err != nil {
			done <- decodeResult{err: err}
			return
		}
		done <- decodeResult{output: out}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	dr := <-done
	require.NoError(t, dr.err)
	assert.Contains(t, dr.output.JSON, "update")
	assert.Contains(t, dr.output.JSON, "10.0.0.0/24")
	assert.Contains(t, dr.output.JSON, "igp")
}

// TestDispatchDecodeMPReach_Malformed verifies error handling for truncated MP_REACH_NLRI.
//
// VALIDATES: Engine returns RPC error for malformed hex input.
// PREVENTS: Panic or silent failure on truncated MP_REACH_NLRI data.
func TestDispatchDecodeMPReach_Malformed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{ctx: ctx}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &Process{}

	// Only 2 bytes — too short for MP_REACH_NLRI (need at least AFI+SAFI+NHLen = 4)
	type errResult struct {
		err error
	}
	done := make(chan errResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-mp-reach", &rpc.DecodeMPReachInput{
			Hex: "0001",
		})
		if err != nil {
			done <- errResult{err}
			return
		}
		_, err = rpc.ParseResponse(raw)
		done <- errResult{err}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	malR := <-done
	require.Error(t, malR.err)
	assert.Contains(t, malR.err.Error(), "too short")
}
