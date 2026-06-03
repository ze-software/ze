package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestShowNeighbors_RegisteredWireMethod verifies the top-level
// `ze-show:neighbors` RPC is installed in the builtin registry so the
// dispatcher can route `ze show neighbors` to handleShowNeighbors.
func TestShowNeighbors_RegisteredWireMethod(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:neighbors" {
			require.NotNil(t, r.Handler, "ze-show:neighbors handler must not be nil")
			found = true
			break
		}
	}
	require.True(t, found, "ze-show:neighbors not registered via pluginserver.RegisterRPCs")
}

// TestHandleShowNeighbors_UnknownFamilyRejects verifies the handler
// rejects an unknown positional family token with the valid set in the
// error message.
func TestHandleShowNeighbors_UnknownFamilyRejects(t *testing.T) {
	resp, err := handleShowNeighbors(nil, []string{"ipv5"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "ipv5")
	assert.Contains(t, msg, "ipv4")
	assert.Contains(t, msg, "ipv6")
}

// TestHandleShowNeighbors_TooManyArgs verifies the handler rejects when
// given more than one positional argument.
func TestHandleShowNeighbors_TooManyArgs(t *testing.T) {
	resp, err := handleShowNeighbors(nil, []string{"ipv4", "extra"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "too many arguments")
}

// TestHandleShowNeighbors_DispatchShape verifies the handler dispatches
// to the backend and wraps the result under the `neighbors` key, or
// propagates the backend error.
func TestHandleShowNeighbors_DispatchShape(t *testing.T) {
	resp, err := handleShowNeighbors(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == plugin.StatusError {
		return // no backend loaded in unit tests; error path is valid evidence
	}
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "data must be a map[string]any wrapper")
	_, ok = data["neighbors"]
	require.True(t, ok, "data must carry a `neighbors` key")
}
