// Design: docs/architecture/cli/command-verbs.md -- a clear that acts on a set takes an explicit all.
// Related: clear.go -- the clear interface counters handler under test.

package cmd_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger every module and RPC registration, so the dispatcher under test
	// holds the handlers and the schema the daemon holds.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// ifaceDispatcher answers a dispatcher loaded the way the daemon loads its
// builtins: every registered RPC at the path its YANG declares, with the
// argument definitions the same schema declares.
func ifaceDispatcher(t *testing.T) *pluginserver.Dispatcher {
	t.Helper()
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	d := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(d, yang.WireMethodToPath(loader),
		yang.PathToDescription(loader), yang.PathToHelp(loader), yang.PathToArgDefs(loader)))
	return d
}

// TestClearInterfaceCountersBareIsRefused proves `clear interface counters`
// alone reaches no handler: the dispatcher refuses the prefix and names the
// explicit form below it. Method: load every builtin RPC at its YANG path,
// dispatch the bare prefix, then dispatch `clear interface counters all` and
// prove it is not refused as incomplete. No backend is loaded here, so the
// whole form answers the backend's own refusal, which only the handler gives.
//
// VALIDATES: the interface counter clear declares only explicit forms
// (ai/rules/cli.md, owner rule 2026-10-17).
// PREVENTS: a typo, an early Enter or a completion that stops at the prefix
// resetting the counters of every interface.
func TestClearInterfaceCountersBareIsRefused(t *testing.T) {
	d := ifaceDispatcher(t)

	_, err := d.Dispatch(&pluginserver.CommandContext{}, "clear interface counters")
	var incomplete *pluginserver.IncompleteCommandError
	require.True(t, errors.As(err, &incomplete), "want IncompleteCommandError, got %T: %v", err, err)
	assert.Contains(t, incomplete.Forms, "clear interface counters all")
	assert.Contains(t, err.Error(), "\n  clear interface counters all")

	resp, err := d.Dispatch(&pluginserver.CommandContext{}, "clear interface counters all")
	require.False(t, errors.As(err, &incomplete), "the whole form was refused as incomplete: %v", err)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "iface: no backend loaded", resp.Error, "the whole form did not reach the handler")
}
