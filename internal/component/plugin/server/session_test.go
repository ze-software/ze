package server

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestPluginSessionPing verifies the plugin session ping command.
//
// VALIDATES: Returns the daemon process id under the pid key.
//
// PREVENTS: Missing health check endpoint.
func TestPluginSessionPing(t *testing.T) {
	ctx := &CommandContext{
		Server: &Server{reactor: &mockReactor{}},
	}

	resp, err := handlePluginSessionPing(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "response should contain data map")
	assert.Equal(t, os.Getpid(), data["pid"], "response should carry this process's pid")
}

// TestPluginSessionBye verifies the plugin session bye command.
//
// VALIDATES: Returns success for client disconnect.
//
// PREVENTS: Error on client disconnect cleanup.
func TestPluginSessionBye(t *testing.T) {
	ctx := &CommandContext{
		Server: &Server{reactor: &mockReactor{}},
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
		Server: &Server{reactor: &mockReactor{}},
	}

	resp, err := handlePluginSessionReady(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "ready acknowledged", data["api"])
}
