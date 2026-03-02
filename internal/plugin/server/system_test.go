package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// TestHandleSystemDispatch verifies ze-system:dispatch routes text commands
// through the standard dispatcher, enabling API socket clients to reach
// plugin-registered commands.
//
// VALIDATES: Text commands from API socket clients reach the dispatcher.
// PREVENTS: API socket clients unable to invoke plugin commands.
func TestHandleSystemDispatch(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(_ *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone, Data: "ok"}, nil
	}
	d.Register("watchdog announce", handler, "Announce watchdog")

	srv := &Server{dispatcher: d}
	ctx := &CommandContext{Server: srv, Peer: "*"}

	resp, err := handleSystemDispatch(ctx, []string{"watchdog announce dnsr"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{"dnsr"}, receivedArgs)
}

// TestHandleSystemDispatchMissingCommand verifies error on empty args.
//
// VALIDATES: Missing command returns error.
// PREVENTS: Panic on nil/empty args.
func TestHandleSystemDispatchMissingCommand(t *testing.T) {
	ctx := &CommandContext{}

	resp, err := handleSystemDispatch(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandleSystemDispatchNoDispatcher verifies error when dispatcher unavailable.
//
// VALIDATES: Nil dispatcher returns error.
// PREVENTS: Panic when server has no dispatcher.
func TestHandleSystemDispatchNoDispatcher(t *testing.T) {
	ctx := &CommandContext{Server: &Server{}}

	resp, err := handleSystemDispatch(ctx, []string{"test"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandleSystemDispatchJoinsArgs verifies multiple args are joined.
//
// VALIDATES: Multiple args elements joined into single command string.
// PREVENTS: Only first arg used when multiple provided.
func TestHandleSystemDispatchJoinsArgs(t *testing.T) {
	d := NewDispatcher()

	var receivedArgs []string
	handler := func(_ *CommandContext, args []string) (*plugin.Response, error) {
		receivedArgs = args
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	d.Register("watchdog withdraw", handler, "Withdraw watchdog")

	srv := &Server{dispatcher: d}
	ctx := &CommandContext{Server: srv, Peer: "*"}

	resp, err := handleSystemDispatch(ctx, []string{"watchdog", "withdraw", "dnsr"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{"dnsr"}, receivedArgs)
}
