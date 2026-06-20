package flowexport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// VALIDATES: the `show flow-export` RPC is registered with a handler, so the
// command is reachable rather than 404ing at dispatch time.
func TestShowFlowExportRPCRegistered(t *testing.T) {
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:flow-export" {
			assert.NotNil(t, r.Handler, "ze-show:flow-export must have a handler")
			return
		}
	}
	require.Fail(t, "ze-show:flow-export RPC is not registered")
}

// VALIDATES: with no exporter configured the handler reports not-configured
// rather than panicking on a nil exporter.
func TestShowFlowExportNotConfigured(t *testing.T) {
	resp, err := handleShowFlowExport(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}
