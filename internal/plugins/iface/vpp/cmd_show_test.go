// Design: plan/spec-backend-command-dispatch.md -- VPP show handler wiring tests

package ifacevpp

import (
	"testing"

	"github.com/stretchr/testify/require"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func TestShowVPP_RegisteredWireMethods(t *testing.T) {
	wanted := map[string]bool{
		"ze-show:vpp-trace-start": false,
		"ze-show:vpp-trace-show":  false,
		"ze-show:vpp-trace-clear": false,
		"ze-show:vpp-runtime":     false,
	}
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if _, ok := wanted[r.WireMethod]; ok {
			require.NotNil(t, r.Handler, "%s handler must not be nil", r.WireMethod)
			wanted[r.WireMethod] = true
		}
	}
	for wm, seen := range wanted {
		require.True(t, seen, "%s not registered via pluginserver.RegisterRPCs", wm)
	}
}

func TestHandleVPPTraceStart_InvalidNodeName(t *testing.T) {
	resp, err := handleVPPTraceStart(nil, []string{"node", "invalid;name"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Contains(t, resp.Data, "invalid node name")
}
