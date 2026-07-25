package static

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: the `show static` proxy RPC is registered with its plugin command,
// so the command is reachable rather than 404ing at dispatch time.
func TestShowStaticRPCRegistered(t *testing.T) {
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:static" {
			assert.NotNil(t, r.Handler, "ze-show:static must have a handler")
			assert.Equal(t, "show static", r.PluginCommand)
			return
		}
	}
	require.Fail(t, "ze-show:static RPC is not registered")
}
