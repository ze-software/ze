// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- clear vpn ipsec handler tests

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestClearIPsecSA_RegisteredWireMethod proves both explicit forms reach a
// handler: all terminates every SA, peer one peer's. The bare prefix has no
// wire method, so the dispatcher refuses it.
func TestClearIPsecSA_RegisteredWireMethod(t *testing.T) {
	for _, method := range []string{"ze-ike:clear-vpn-ipsec-sa", "ze-ike:clear-vpn-ipsec-sa-peer"} {
		found := false
		for _, r := range pluginserver.AllBuiltinRPCs() {
			if r.WireMethod == method {
				require.NotNil(t, r.Handler)
				found = true
				break
			}
		}
		require.True(t, found, method+" not registered")
	}
}

func TestClearIPsecSA_AllNoEngine(t *testing.T) {
	resp, err := handleClearIPsecSAAll(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "clear-all", data["action"])
	assert.Equal(t, 0, data["terminated"])
}

// TestClearIPsecSA_AllRefusesATail proves all takes nothing after it, so
// `clear vpn ipsec sa all peer-1` cannot be read as clearing everything.
func TestClearIPsecSA_AllRefusesATail(t *testing.T) {
	resp, err := handleClearIPsecSAAll(nil, commandtest.Args("peer-1"))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "clear vpn ipsec sa peer <name>")
}

func TestClearIPsecSA_PeerMissingName(t *testing.T) {
	resp, err := handleClearIPsecSAPeer(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// peerArgs answers tokens judged against the leaf the YANG model declares for
// `clear vpn ipsec sa peer`, as the dispatcher judges them.
func peerArgs(t *testing.T, tokens ...string) command.ValidatedArgs {
	t.Helper()
	defs := []command.ArgDef{commandtest.Must(command.NewStringArg("name",
		[]command.UintRange{{Min: 1, Max: 255}}, nil, command.ArgOptions{Mandatory: true}))}
	validated, err := command.ValidateArgs(tokens, defs, nil)
	require.NoError(t, err)
	return validated
}

// TestClearIPsecSA_PeerNotFound proves both spellings of the peer leaf reach
// the engine lookup, and the refusal names the peer and the command that lists
// the peers.
func TestClearIPsecSA_PeerNotFound(t *testing.T) {
	for _, tokens := range [][]string{{"nonexistent"}, {"name", "nonexistent"}} {
		resp, err := handleClearIPsecSAPeer(nil, peerArgs(t, tokens...))
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "peer not found: nonexistent")
		assert.Contains(t, resp.Error, "show vpn ipsec sa")
	}
}

// TestClearIPsecSA_PeerRefusesATail proves a word after the peer name is
// refused rather than ignored.
func TestClearIPsecSA_PeerRefusesATail(t *testing.T) {
	resp, err := handleClearIPsecSAPeer(nil, peerArgs(t, "name", "site-b", "extra"))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unexpected words after the peer name")
}
