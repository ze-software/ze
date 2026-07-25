package policyroute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: the `show policy routes` proxy RPC is registered with its plugin
// command, so the command is reachable rather than 404ing at dispatch time.
func TestShowPolicyRoutesRPCRegistered(t *testing.T) {
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:policy-routes" {
			assert.NotNil(t, r.Handler, "ze-show:policy-routes must have a handler")
			assert.Equal(t, "show policy routes", r.PluginCommand)
			return
		}
	}
	require.Fail(t, "ze-show:policy-routes RPC is not registered")
}
