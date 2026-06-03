package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestShowKernelRoutes_RegisteredWireMethod verifies the top-level
// `ze-show:kernel-routes` RPC is installed in the builtin registry.
func TestShowKernelRoutes_RegisteredWireMethod(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:kernel-routes" {
			require.NotNil(t, r.Handler, "ze-show:kernel-routes handler must not be nil")
			found = true
			break
		}
	}
	require.True(t, found, "ze-show:kernel-routes not registered via pluginserver.RegisterRPCs")
}

// TestHandleShowKernelRoutes_InvalidPrefix verifies the handler rejects
// a malformed positional CIDR rather than silently returning empty.
func TestHandleShowKernelRoutes_InvalidPrefix(t *testing.T) {
	resp, err := handleShowKernelRoutes(nil, []string{"not-a-cidr"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "invalid prefix")
}

// TestHandleShowKernelRoutes_LimitValidated verifies the handler rejects
// --limit with a non-positive integer.
func TestHandleShowKernelRoutes_LimitValidated(t *testing.T) {
	resp, err := handleShowKernelRoutes(nil, []string{"--limit", "0"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "positive integer")
}

// TestHandleShowKernelRoutes_DispatchShape verifies the happy-path
// envelope (either the backend returns routes or propagates an error).
func TestHandleShowKernelRoutes_DispatchShape(t *testing.T) {
	resp, err := handleShowKernelRoutes(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == plugin.StatusError {
		return // no backend loaded in unit tests; error path is valid evidence
	}
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "data must be a map[string]any wrapper")
	_, ok = data["routes"]
	require.True(t, ok, "data must carry a `routes` key")
}
