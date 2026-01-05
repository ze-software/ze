package api

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
	proc := NewProcess(ProcessConfig{
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
	proc := NewProcess(ProcessConfig{
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

// TestSessionReset verifies the session reset command.
//
// VALIDATES: Command resets session state and returns success.
//
// PREVENTS: Stale state persisting after reset request.
func TestSessionReset(t *testing.T) {
	proc := NewProcess(ProcessConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionReset(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestSessionPing verifies the session ping command.
//
// VALIDATES: Returns pong with daemon information.
//
// PREVENTS: Missing health check endpoint.
func TestSessionPing(t *testing.T) {
	proc := NewProcess(ProcessConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionPing(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "response should contain data map")
	assert.Contains(t, data, "pong", "response should contain pong field")
}

// TestSessionBye verifies the session bye command.
//
// VALIDATES: Returns success for client disconnect.
//
// PREVENTS: Error on client disconnect cleanup.
func TestSessionBye(t *testing.T) {
	proc := NewProcess(ProcessConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionBye(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestSessionCommandsRegistered verifies session commands are registered.
//
// VALIDATES: All session commands are accessible via dispatcher.
//
// PREVENTS: Session commands not wired up to dispatcher.
func TestSessionCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	commands := []string{
		"session sync enable",
		"session sync disable",
		"session api encoding",
		"session reset",
		"session ping",
		"session bye",
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
	proc := NewProcess(ProcessConfig{
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
	proc := NewProcess(ProcessConfig{
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
	proc := NewProcess(ProcessConfig{
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
	proc := NewProcess(ProcessConfig{
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

// TestSessionResetClearsEncoding verifies reset restores default encoding.
//
// VALIDATES: session reset restores encoding to hex (default).
//
// PREVENTS: Stale encoding persisting after reset.
func TestSessionResetClearsEncoding(t *testing.T) {
	proc := NewProcess(ProcessConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})
	// Change encoding
	proc.SetWireEncodingIn(WireEncodingCBOR)
	proc.SetWireEncodingOut(WireEncodingB64)

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionReset(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, WireEncodingHex, proc.WireEncodingIn(), "encoding should reset to hex")
	assert.Equal(t, WireEncodingHex, proc.WireEncodingOut(), "encoding should reset to hex")
}
