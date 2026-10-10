package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestShowRouteLookup_RegisteredWireMethod verifies the `ze-iface:show-route-lookup`
// RPC is installed in the builtin registry (it moved here from the central
// show package).
func TestShowRouteLookup_RegisteredWireMethod(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-iface:show-route-lookup" {
			require.NotNil(t, r.Handler, "ze-iface:show-route-lookup handler must not be nil")
			found = true
			break
		}
	}
	require.True(t, found, "ze-iface:show-route-lookup not registered via pluginserver.RegisterRPCs")
}

// TestHandleRouteLookup_MissingArg verifies the handler rejects when no
// destination IP is supplied.
func TestHandleRouteLookup_MissingArg(t *testing.T) {
	resp, err := handleRouteLookup(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "usage")
}

// TestHandleRouteLookup_InvalidDest verifies a malformed destination IP
// rejects with a clear error rather than reaching the backend.
func TestHandleRouteLookup_InvalidDest(t *testing.T) {
	resp, err := handleRouteLookup(nil, commandtest.Args("not-an-ip"))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "invalid destination")
}
