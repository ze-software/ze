package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionSyncEnable verifies the session sync enable command.
//
// VALIDATES: Command enables sync mode and returns success.
//
// PREVENTS: Sync remaining disabled when user requests it.
func TestSessionSyncEnable(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})
	// Sync is disabled by default
	assert.False(t, proc.SyncEnabled())

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionSyncEnable(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, proc.SyncEnabled(), "sync should be enabled after command")
}

// TestSessionSyncDisable verifies the session sync disable command.
//
// VALIDATES: Command disables sync mode and returns success.
//
// PREVENTS: Sync remaining enabled when user disables it.
func TestSessionSyncDisable(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})
	// Enable sync first
	proc.SetSync(true)
	assert.True(t, proc.SyncEnabled())

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionSyncDisable(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.False(t, proc.SyncEnabled(), "sync should be disabled after command")
}

// TestPluginSessionPing verifies the plugin session ping command.
//
// VALIDATES: Returns pong with daemon information.
//
// PREVENTS: Missing health check endpoint.
func TestPluginSessionPing(t *testing.T) {
	ctx := &CommandContext{
		Reactor: &mockReactor{},
	}

	resp, err := handlePluginSessionPing(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "response should contain data map")
	assert.Contains(t, data, "pong", "response should contain pong field")
}

// TestPluginSessionBye verifies the plugin session bye command.
//
// VALIDATES: Returns success for client disconnect.
//
// PREVENTS: Error on client disconnect cleanup.
func TestPluginSessionBye(t *testing.T) {
	ctx := &CommandContext{
		Reactor: &mockReactor{},
	}

	resp, err := handlePluginSessionBye(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestPluginSessionReady verifies the plugin session ready command.
//
// VALIDATES: Returns success and signals reactor.
//
// PREVENTS: Plugin startup signal not being acknowledged.
func TestPluginSessionReady(t *testing.T) {
	ctx := &CommandContext{
		Reactor: &mockReactor{},
	}

	resp, err := handlePluginSessionReady(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ready acknowledged", data["api"])
}

// TestSessionCommandsRegistered verifies remaining session commands are registered.
//
// VALIDATES: Session sync/encoding commands are still accessible via dispatcher.
//
// PREVENTS: Session commands not wired up to dispatcher.
// NOTE: session ping/bye/reset moved to plugin namespace.
func TestSessionCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	// These commands remain in session namespace (Step 4 moves them to bgp plugin)
	commands := []string{
		"session sync enable",
		"session sync disable",
		"session api encoding",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.NotNil(t, c, "command %q should be registered", cmd)
		})
	}
}

// TestSessionAPIEncodingBoth verifies session api encoding sets both directions.
//
// VALIDATES: "session api encoding hex" sets inbound and outbound to hex.
//
// PREVENTS: Encoding not applied to both directions.
func TestSessionAPIEncodingBoth(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})
	// Default is hex
	assert.Equal(t, WireEncodingHex, proc.WireEncodingIn())
	assert.Equal(t, WireEncodingHex, proc.WireEncodingOut())

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	// Set to b64
	resp, err := handleSessionAPIEncoding(ctx, []string{"b64"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, WireEncodingB64, proc.WireEncodingIn())
	assert.Equal(t, WireEncodingB64, proc.WireEncodingOut())

	// Set to cbor
	_, err = handleSessionAPIEncoding(ctx, []string{"cbor"})
	require.NoError(t, err)
	assert.Equal(t, WireEncodingCBOR, proc.WireEncodingIn())
	assert.Equal(t, WireEncodingCBOR, proc.WireEncodingOut())

	// Set to text
	_, err = handleSessionAPIEncoding(ctx, []string{"text"})
	require.NoError(t, err)
	assert.Equal(t, WireEncodingText, proc.WireEncodingIn())
	assert.Equal(t, WireEncodingText, proc.WireEncodingOut())

	// Set back to hex explicitly
	_, err = handleSessionAPIEncoding(ctx, []string{"hex"})
	require.NoError(t, err)
	assert.Equal(t, WireEncodingHex, proc.WireEncodingIn())
	assert.Equal(t, WireEncodingHex, proc.WireEncodingOut())

	// Test "base64" alias
	_, err = handleSessionAPIEncoding(ctx, []string{"base64"})
	require.NoError(t, err)
	assert.Equal(t, WireEncodingB64, proc.WireEncodingIn())
	assert.Equal(t, WireEncodingB64, proc.WireEncodingOut())
}

// TestSessionAPIEncodingInbound verifies session api encoding inbound sets only inbound.
//
// VALIDATES: "session api encoding inbound hex" sets only inbound encoding.
//
// PREVENTS: Outbound being modified when only inbound specified.
func TestSessionAPIEncodingInbound(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	// Set inbound only to b64
	resp, err := handleSessionAPIEncoding(ctx, []string{"inbound", "b64"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, WireEncodingB64, proc.WireEncodingIn())
	assert.Equal(t, WireEncodingHex, proc.WireEncodingOut()) // unchanged
}

// TestSessionAPIEncodingOutbound verifies session api encoding outbound sets only outbound.
//
// VALIDATES: "session api encoding outbound b64" sets only outbound encoding.
//
// PREVENTS: Inbound being modified when only outbound specified.
func TestSessionAPIEncodingOutbound(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	// Set outbound only to cbor
	resp, err := handleSessionAPIEncoding(ctx, []string{"outbound", "cbor"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, WireEncodingHex, proc.WireEncodingIn()) // unchanged
	assert.Equal(t, WireEncodingCBOR, proc.WireEncodingOut())
}

// TestSessionAPIEncodingInvalid verifies invalid encoding is rejected.
//
// VALIDATES: Invalid encoding format returns error.
//
// PREVENTS: Accepting unknown encoding formats silently.
func TestSessionAPIEncodingInvalid(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	// Invalid format
	resp, err := handleSessionAPIEncoding(ctx, []string{"invalid"})
	require.Error(t, err)
	assert.Nil(t, resp)

	// Missing argument
	resp, err = handleSessionAPIEncoding(ctx, []string{})
	require.Error(t, err)
	assert.Nil(t, resp)
}
