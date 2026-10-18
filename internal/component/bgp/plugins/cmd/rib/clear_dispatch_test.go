// Design: docs/architecture/cli/command-verbs.md -- a clear that acts on a set takes an explicit selector.
// Related: rib.go -- the clear bgp rib in and out forwarders under test.

package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// ribClearDispatcher answers a dispatcher loaded the way the daemon loads its
// builtins, and the argument definitions the same schema declares per path.
func ribClearDispatcher(t *testing.T) (*pluginserver.Dispatcher, map[string][]command.ArgDef) {
	t.Helper()
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	defs := yang.PathToArgDefs(loader)
	d := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(d, yang.WireMethodToPath(loader),
		yang.PathToDescription(loader), yang.PathToHelp(loader), defs))
	return d, defs
}

// TestClearBgpRibBareIsRefused proves `clear bgp rib in` and `clear bgp rib
// out` alone reach no handler. Method: load every builtin RPC at its YANG path
// and dispatch each bare prefix; the selector guard refuses it before the
// forwarder runs, and its refusal names the explicit every-peer form.
//
// VALIDATES: both RIB clears declare their selector mandatory, in the YANG
// leaf and in RequiresSelector (the explicit-all CLI rule, 2026-10-17), and
// the refusal names `<selector>` with the leaf's own summary, which names `*`.
// PREVENTS: a typo or an early Enter clearing or replaying a RIB, and a
// refusal that leaves the operator guessing how every peer is said.
func TestClearBgpRibBareIsRefused(t *testing.T) {
	d, _ := ribClearDispatcher(t)
	for _, line := range []string{cmdRibClearIn, cmdRibClearOut} {
		t.Run(line, func(t *testing.T) {
			resp, err := d.Dispatch(&pluginserver.CommandContext{}, line)
			require.Error(t, err)
			assert.Nil(t, resp)
			assert.Equal(t, line+" requires a selector: "+line+
				" <selector> (* for every peer, or an address, a peer name or an AS pattern)", err.Error())
		})
	}
}

// TestClearBgpRibUnkeyedValuesAreRefused proves a line whose values carry no
// keyword is refused with the selector form named, for each shape the
// keyword-form example does not cover. Method: dispatch three or more unkeyed
// values to clear bgp rib out, and two to clear bgp rib in, which declares no
// other leaf.
//
// VALIDATES: every multi-token refusal says why no selector was taken and
// names `<selector>` with `*`, then any other keyword the command declares.
// PREVENTS: a refusal with no fix in it (ai/rules/user-facing-errors.md).
func TestClearBgpRibUnkeyedValuesAreRefused(t *testing.T) {
	d, _ := ribClearDispatcher(t)
	const form = " <selector> (* for every peer, or an address, a peer name or an AS pattern)"
	tests := []struct {
		line string
		want string
	}{
		{cmdRibClearIn + " * extra", cmdRibClearIn + " requires a selector: * extra are 2 values typed without a keyword," +
			" so none is taken as the selector. Type " + cmdRibClearIn + form},
		{cmdRibClearOut + " * ipv4/unicast extra", cmdRibClearOut + " requires a selector: * ipv4/unicast extra are 3 values" +
			" typed without a keyword, so none is taken as the selector. Type " + cmdRibClearOut + form +
			", then each other value after its keyword: family"},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			resp, err := d.Dispatch(&pluginserver.CommandContext{}, tt.line)
			require.Error(t, err)
			assert.Nil(t, resp)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

// TestClearBgpRibOutPositionalFamilyIsRefused proves the family typed with no
// keyword reaches no handler, and that the refusal names the keyword form.
// Method: dispatch `clear bgp rib out * ipv4/unicast` through the builtin
// dispatcher. Two spare positionals leave the selector unbound, so the
// dispatcher's selector guard refuses it, the path a daemon takes.
//
// VALIDATES: the refusal says why `*` was not taken and names `family`.
// PREVENTS: a refusal that tells an operator who typed `*` they gave no selector.
func TestClearBgpRibOutPositionalFamilyIsRefused(t *testing.T) {
	d, _ := ribClearDispatcher(t)
	resp, err := d.Dispatch(&pluginserver.CommandContext{}, cmdRibClearOut+" * ipv4/unicast")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "none is taken as the selector")
	assert.Contains(t, err.Error(), cmdRibClearOut+" * family ipv4/unicast")
}

// TestClearBgpRibWholeFormsReachThePlugin proves every whole form passes the
// dispatcher's typed validation, its selector guard and the plugin hand-off's
// judgement (route R9, routeToProcess), the path the daemon takes. Method:
// build the server the daemon builds, register the bgp-rib plugin's two clear
// commands for a process that is not running, and dispatch each line; a call
// that passes every judgement fails only because no process answers.
//
// VALIDATES: `*` is a valid selector, and a family after its keyword binds,
// at the builtin and again at the plugin's path.
// PREVENTS: `clear bgp rib out *` refused in the daemon as an invalid family,
// because the hand-off judged the forwarded selector against the family leaf.
func TestClearBgpRibWholeFormsReachThePlugin(t *testing.T) {
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	d := srv.Dispatcher()
	proc := process.NewProcess(plugin.PluginConfig{Name: "bgp-rib"})
	for _, result := range d.Registry().Register(proc, []pluginserver.CommandDef{{Name: cmdRibClearIn}, {Name: cmdRibClearOut}}) {
		require.True(t, result.OK, "register %s: %s", result.Name, result.Error)
	}
	for _, line := range []string{
		cmdRibClearIn + " *",
		cmdRibClearIn + " 10.0.0.1",
		cmdRibClearOut + " *",
		cmdRibClearOut + " 10.0.0.1",
		cmdRibClearOut + " * family ipv4/unicast",
	} {
		t.Run(line, func(t *testing.T) {
			resp, err := d.Dispatch(&pluginserver.CommandContext{Server: srv, Sender: plugin.OperatorSender()}, line)
			if resp != nil {
				assert.Empty(t, resp.Error)
			}
			require.ErrorIs(t, err, pluginserver.ErrPluginProcessNotRunning)
		})
	}
}

// TestClearBgpRibArgs proves each whole form hands the plugin the selector and
// family it was typed with. Method: bind the line's tail the way the
// dispatcher does (command.ValidateArgs over the schema's own definitions),
// put the selector where the dispatcher puts it, and read ribClearArgs.
//
// VALIDATES: the forwarders read the bound leaves, never the first raw token.
// PREVENTS: a family typed after its keyword arriving as the selector.
func TestClearBgpRibArgs(t *testing.T) {
	_, defs := ribClearDispatcher(t)
	tests := []struct {
		name string
		path string
		tail []string
		want []string
	}{
		{"in every peer", cmdRibClearIn, []string{"*"}, []string{"*"}},
		{"in one peer", cmdRibClearIn, []string{"10.0.0.1"}, []string{"10.0.0.1"}},
		{"out every peer", cmdRibClearOut, []string{"*"}, []string{"*"}},
		{"out one family", cmdRibClearOut, []string{"*", "family", "ipv4/unicast"}, []string{"*", "ipv4/unicast"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validated, err := command.ValidateArgs(tt.tail, defs[tt.path], nil)
			require.NoError(t, err)
			ctx := &pluginserver.CommandContext{Selectors: map[string]string{"selector": tt.tail[0]}, Peer: tt.tail[0]}
			args, refusal := ribClearArgs(ctx, validated, tt.path)
			require.Nil(t, refusal)
			assert.Equal(t, tt.want, args)
		})
	}
}

// TestClearBgpRibArgsExtraToken proves a token no leaf took is refused rather
// than dropped. Method: bind `* ipv4/unicast`, where the family arrives with no
// keyword and two spare positionals leave neither leaf singly bound, and read
// ribClearArgs.
//
// VALIDATES: the refusal names the keyword form of the family.
// PREVENTS: a replay of every family for an operator who named one.
func TestClearBgpRibArgsExtraToken(t *testing.T) {
	_, defs := ribClearDispatcher(t)
	validated, err := command.ValidateArgs([]string{"*", "ipv4/unicast"}, defs[cmdRibClearOut], nil)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Peer: "*"}
	args, refusal := ribClearArgs(ctx, validated, cmdRibClearOut)
	assert.Nil(t, args)
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Error, cmdRibClearOut+" * family ipv4/unicast")
}

// TestClearBgpRibArgsNoSelector proves a route that skipped the dispatcher's
// guard is refused by the forwarder, and the refusal names *, rather than the
// forwarder reading the "*" CommandContext.PeerSelector answers for no peer.
//
// VALIDATES: the forwarder never turns a missing selector into every peer.
// PREVENTS: the silent everything default the explicit-all rule removed.
func TestClearBgpRibArgsNoSelector(t *testing.T) {
	for _, path := range []string{cmdRibClearIn, cmdRibClearOut} {
		args, refusal := ribClearArgs(&pluginserver.CommandContext{}, command.ValidatedArgs{}, path)
		assert.Nil(t, args)
		require.NotNil(t, refusal)
		assert.Equal(t, plugin.StatusError, refusal.Status)
		assert.Contains(t, refusal.Error, path+" * for every peer")
	}
}
