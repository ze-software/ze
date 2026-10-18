// Design: docs/architecture/cli/command-verbs.md -- a clear that acts on a set takes an explicit all.
// Related: cmd_show.go -- the command constants the plugin declares.

package vrrp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestVRRPClearBareIsRefused proves `clear vrrp statistics` alone reaches no handler: the
// dispatcher refuses the prefix and names `clear vrrp statistics all`, the only form that
// clears every member. Method: register the plugin's own command declarations
// on a dispatcher, dispatch the bare prefix, then dispatch the whole form and prove the
// dispatcher routes it to the plugin rather than refusing it as incomplete (the
// plugin process is not running here, so the route answers ErrPluginProcessNotRunning).
//
// VALIDATES: the VRRP statistics clear declares only the explicit all form
// (ai/rules/cli.md, owner rule 2026-10-17).
// PREVENTS: a typo, an early Enter or a completion that stops at the prefix
// clearing every member.
func TestVRRPClearBareIsRefused(t *testing.T) {
	d := pluginserver.NewDispatcher()
	decls := commandDecls()
	defs := make([]pluginserver.CommandDef, 0, len(decls))
	for _, decl := range decls {
		defs = append(defs, pluginserver.CommandDef{Name: decl.Name})
	}
	proc := process.NewProcess(plugin.PluginConfig{Name: "vrrp"})
	for _, result := range d.Registry().Register(proc, defs) {
		require.True(t, result.OK, "%s: %s", result.Name, result.Error)
	}

	_, err := d.Dispatch(nil, "clear vrrp statistics")
	var incomplete *pluginserver.IncompleteCommandError
	require.True(t, errors.As(err, &incomplete), "want IncompleteCommandError, got %T: %v", err, err)
	assert.Contains(t, incomplete.Forms, "clear vrrp statistics all")
	assert.Contains(t, err.Error(), "\n  clear vrrp statistics all")

	_, err = d.Dispatch(nil, "clear vrrp statistics all")
	require.False(t, errors.As(err, &incomplete), "the whole form was refused as incomplete: %v", err)
	require.ErrorIs(t, err, pluginserver.ErrPluginProcessNotRunning, "clear vrrp statistics all did not route to the plugin")
}
