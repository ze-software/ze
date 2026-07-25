// Design: plan/learned/745-ipsec-10-cli-diag.md -- clear vpn ipsec handler tests

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestClearIPsecSA_RegisteredWireMethod(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-clear:vpn-ipsec-sa" {
			require.NotNil(t, r.Handler)
			found = true
			break
		}
	}
	require.True(t, found, "ze-clear:vpn-ipsec-sa not registered")
}

func TestClearIPsecSA_AllNoEngine(t *testing.T) {
	resp, err := handleClearIPsecSA(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "clear-all", data["action"])
	assert.Equal(t, 0, data["terminated"])
}

func TestClearIPsecSA_PeerMissingName(t *testing.T) {
	resp, err := handleClearIPsecSA(nil, []string{"peer"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestClearIPsecSA_PeerNotFound(t *testing.T) {
	resp, err := handleClearIPsecSA(nil, []string{"peer", "nonexistent"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "peer not found")
}
