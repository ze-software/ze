// Design: docs/architecture/cli/command-verbs.md -- a clear that acts on a set takes an explicit all.
// Related: ipsec.go -- the clear vpn ipsec sa handlers under test.

package cmd_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// The owning packages register the clear vpn ipsec sa RPCs and the YANG
	// that declares their paths and arguments. They are imported directly, not
	// through plugin/all, whose ike group builds only under the ze_ike tag.
	_ "github.com/ze-software/ze/internal/component/ike/cmd"
	_ "github.com/ze-software/ze/internal/component/ike/yang"
)

// ipsecDispatcher answers a dispatcher loaded the way the daemon loads its
// builtins: every registered RPC at the path its YANG declares, with the
// argument definitions the same schema declares.
func ipsecDispatcher(t *testing.T) *pluginserver.Dispatcher {
	t.Helper()
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	d := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(d, yang.WireMethodToPath(loader),
		yang.PathToDescription(loader), yang.PathToHelp(loader), yang.PathToArgDefs(loader)))
	return d
}

// TestClearIPsecSABareIsRefused proves `clear vpn ipsec sa` alone reaches no
// handler: the dispatcher refuses the prefix and names both explicit forms.
// Method: load every builtin RPC at its YANG path, dispatch the bare prefix,
// then dispatch `clear vpn ipsec sa all` and prove it is not refused as
// incomplete.
//
// VALIDATES: the IPsec SA clear declares only explicit forms (ai/rules/cli.md,
// owner rule 2026-10-17).
// PREVENTS: a typo, an early Enter or a completion that stops at the prefix
// terminating every SA.
func TestClearIPsecSABareIsRefused(t *testing.T) {
	d := ipsecDispatcher(t)

	_, err := d.Dispatch(&pluginserver.CommandContext{}, "clear vpn ipsec sa")
	var incomplete *pluginserver.IncompleteCommandError
	require.True(t, errors.As(err, &incomplete), "want IncompleteCommandError, got %T: %v", err, err)
	assert.Contains(t, incomplete.Forms, "clear vpn ipsec sa all")
	assert.Contains(t, incomplete.Forms, "clear vpn ipsec sa peer")
	assert.Contains(t, err.Error(), "\n  clear vpn ipsec sa all")

	resp, err := d.Dispatch(&pluginserver.CommandContext{}, "clear vpn ipsec sa all")
	require.False(t, errors.As(err, &incomplete), "the whole form was refused as incomplete: %v", err)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestClearIPsecSAPeerBothSpellings proves the peer leaf is reached whether the
// operator types the value alone or after its leaf keyword. Method: dispatch
// both spellings with no engine running and prove each answer names the peer
// it was asked about, which only a handler holding the bound leaf can do.
//
// VALIDATES: `clear vpn ipsec sa peer X` and `clear vpn ipsec sa peer name X`
// both bind X to the YANG leaf name.
// PREVENTS: the keyword spelling being refused as "give exactly one peer name".
func TestClearIPsecSAPeerBothSpellings(t *testing.T) {
	d := ipsecDispatcher(t)
	for _, line := range []string{
		"clear vpn ipsec sa peer site-b",
		"clear vpn ipsec sa peer name site-b",
	} {
		t.Run(line, func(t *testing.T) {
			resp, err := d.Dispatch(&pluginserver.CommandContext{}, line)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, "peer not found: site-b")
		})
	}
}
