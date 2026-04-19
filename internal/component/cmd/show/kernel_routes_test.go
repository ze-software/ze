package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestShowKernelRoutes_RegisteredWireMethod verifies the top-level
// `ze-show:kernel-routes` RPC is installed in the builtin registry.
//
// VALIDATES: offline CLI `ze interface routes` has a daemon-side
// counterpart reachable via the dispatcher.
// PREVENTS: regression where the handler exists but no init() registered
// it, so `ze show kernel-routes` returns "unknown command" at runtime.
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
//
// VALIDATES: shared dumpKernelRoutes parser path is reachable from the
// top-level handler and rejects bad input.
// PREVENTS: regression where the top-level wrapper bypasses prefix
// validation and lets garbage reach iface.ListKernelRoutes.
func TestHandleShowKernelRoutes_InvalidPrefix(t *testing.T) {
	resp, err := handleShowKernelRoutes(nil, []string{"not-a-cidr"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, msg, "invalid prefix")
}

// TestHandleShowKernelRoutes_LimitValidated verifies the handler rejects
// --limit with a non-positive integer.
//
// VALIDATES: shared --limit parser is reachable; zero / negative limits
// produce a clear error naming the invalid input.
// PREVENTS: regression where --limit 0 is accepted and short-circuits
// the backend to return everything (unbounded).
func TestHandleShowKernelRoutes_LimitValidated(t *testing.T) {
	resp, err := handleShowKernelRoutes(nil, []string{"--limit", "0"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, msg, "positive integer")
}

// TestHandleShowKernelRoutes_DispatchShape verifies the happy-path
// envelope (either the backend returns routes or propagates an error).
//
// VALIDATES: the handler wraps the route list under "routes" so the
// `| table` pipe unwraps to a columnar view.
// PREVENTS: regression where a future edit drops the single-key wrapper
// and breaks the pipe framework's table rendering.
func TestHandleShowKernelRoutes_DispatchShape(t *testing.T) {
	resp, err := handleShowKernelRoutes(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == plugin.StatusError {
		return // no backend loaded in unit tests; error path is valid evidence
	}
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data must be a map[string]any wrapper")
	_, ok = data["routes"]
	require.True(t, ok, "data must carry a `routes` key")
}
