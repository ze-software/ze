package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionAckEnable verifies the session ack enable command.
//
// VALIDATES: Command enables ack and returns success response.
//
// PREVENTS: Ack remaining disabled after enable command.
func TestSessionAckEnable(t *testing.T) {
	proc := NewProcess(ProcessConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})
	// Start with ack disabled
	proc.SetAck(false)

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionAckEnable(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, proc.AckEnabled(), "ack should be enabled after command")
}

// TestSessionAckDisable verifies the session ack disable command.
//
// VALIDATES: Command disables ack and returns success response.
// Per ExaBGP behavior, a response IS sent for the disable command itself,
// then subsequent commands don't get responses.
//
// PREVENTS: Ack remaining enabled after disable command.
func TestSessionAckDisable(t *testing.T) {
	proc := NewProcess(ProcessConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})
	// Start with ack enabled (default)
	assert.True(t, proc.AckEnabled())

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionAckDisable(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.False(t, proc.AckEnabled(), "ack should be disabled after command")
}

// TestSessionAckSilence verifies the session ack silence command.
//
// VALIDATES: Command disables ack and returns ErrSilent (no output).
// Unlike disable, silence doesn't send a response for itself either.
//
// PREVENTS: Unexpected response for silence command.
func TestSessionAckSilence(t *testing.T) {
	proc := NewProcess(ProcessConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})
	// Start with ack enabled (default)
	assert.True(t, proc.AckEnabled())

	ctx := &CommandContext{
		Reactor: &mockReactor{},
		Process: proc,
	}

	resp, err := handleSessionAckSilence(ctx, nil)
	require.ErrorIs(t, err, ErrSilent, "silence should return ErrSilent")
	assert.Nil(t, resp, "silence should return nil response")
	assert.False(t, proc.AckEnabled(), "ack should be disabled after silence")
}

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
		"session ack enable",
		"session ack disable",
		"session ack silence",
		"session sync enable",
		"session sync disable",
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
