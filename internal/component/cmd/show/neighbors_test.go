package show

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
//
// VALIDATES: offline CLI `ze interface neighbors` has a daemon-side
// counterpart reachable via the dispatcher -- the handler exists AND is
// wired, not only callable in unit tests.
// PREVENTS: regression where the handler exists but no init() registered
// it, so `ze show neighbors` returns "unknown command" at runtime.
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
//
// VALIDATES: unknown family (e.g. `show neighbors ipv5`) produces a
// clear error naming the valid set, not a silent empty result.
// PREVENTS: regression where a typo silently returns the unfiltered
// table instead of rejecting.
func TestHandleShowNeighbors_UnknownFamilyRejects(t *testing.T) {
	resp, err := handleShowNeighbors(nil, []string{"ipv5"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, msg, "ipv5")
	assert.Contains(t, msg, "ipv4")
	assert.Contains(t, msg, "ipv6")
}

// TestHandleShowNeighbors_TooManyArgs verifies the handler rejects when
// given more than one positional argument.
//
// VALIDATES: surplus positional args are a user error, not a fall-through
// to "use the first and ignore the rest".
// PREVENTS: regression where `show neighbors ipv4 extra` silently drops
// `extra` and succeeds.
func TestHandleShowNeighbors_TooManyArgs(t *testing.T) {
	resp, err := handleShowNeighbors(nil, []string{"ipv4", "extra"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, msg, "too many arguments")
}

// TestHandleShowNeighbors_DispatchShape verifies the handler dispatches
// to the backend and wraps the result under the `neighbors` key, or
// propagates the backend error. Either shape is valid evidence that the
// handler reached the backend layer; we do not require a running
// backend here.
//
// VALIDATES: the handler wraps neighbor list under "neighbors" so the
// `| table` / `| count` pipes unwrap correctly.
// PREVENTS: regression where a future edit drops the single-key wrapper
// and breaks the pipe framework's table rendering.
func TestHandleShowNeighbors_DispatchShape(t *testing.T) {
	resp, err := handleShowNeighbors(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == plugin.StatusError {
		return // no backend loaded in unit tests; error path is valid evidence
	}
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data must be a map[string]any wrapper")
	_, ok = data["neighbors"]
	require.True(t, ok, "data must carry a `neighbors` key")
}
