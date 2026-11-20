// Design: docs/architecture/api/commands.md — command-owned pipe operations
// Related: usage_model_test.go — the same dispatcher, built from the model

package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// The composition root registers the RIB command plugin's pipe operations
	// (registerPipeFilters, internal/component/bgp/plugins/cmd/rib/rib.go) and
	// the peer command module that declares `show bgp peer rib`.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// TestPeerRibRefusesScopeArgument dispatches the scope of the peer RIB typed
// as an argument, the form the removed YANG scope leaf accepted.
//
// VALIDATES: AC-34, refusal half. `received` is a pipe operation of
// `show bgp peer rib` (D-9), so the dispatcher's argument validator refuses it
// before forwardRibRoutes runs, names it as a pipe operation, and gives the
// line in the pipe form as its fix, the peer selector kept where it was typed.
// PREVENTS: the word reaching the plugin as an argument the model no longer
// declares, or a fix line the operator cannot type because it lost the peer.
func TestPeerRibRefusesScopeArgument(t *testing.T) {
	const path = "show bgp peer rib"
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	defs := yang.PathToArgDefs(loader)[path]

	called := false
	handler := func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	d := pluginserver.NewDispatcher()
	require.NoError(t, d.RegisterWithOptions(path, handler, "under test", pluginserver.RegisterOptions{ArgDefs: defs}))

	_, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, "show bgp peer edge1 rib received")
	require.Error(t, dispatchErr, "the scope typed as an argument was accepted")
	assert.Contains(t, dispatchErr.Error(), "received is a pipe operation of "+path+", not an argument")
	assert.Contains(t, dispatchErr.Error(), "\nfix: show bgp peer edge1 rib | received",
		"the fix is not the typed line in the pipe form")
	assert.False(t, called, "the handler ran on a refused argument")

	// A selector holding a space is one word of the fix, quoted as typed.
	_, quotedErr := d.Dispatch(&pluginserver.CommandContext{}, `show bgp peer "edge 1" rib received`)
	require.Error(t, quotedErr, "the scope typed as an argument was accepted")
	assert.Contains(t, quotedErr.Error(), "\nfix: show bgp peer \"edge 1\" rib | received",
		"the fix split or dropped the quoted selector")
	assert.False(t, called, "the handler ran on a refused argument")
}

// TestPeerRibHandsItsOperationsPastTheSelector dispatches the peer RIB with
// its scope and a reducer typed after a pipe, the peer selector inside the
// path, bare and quoted.
//
// VALIDATES: AC-34, answer half. The declaration sits on `show bgp peer rib`
// and the operator types `show bgp peer edge1 rib`, so the lookup resolves the
// typed words to the registered path the way the dispatcher matches it, and
// the handler receives [received, count] in chain order. A surface's own
// partition (command.ReadLocalLine) answers the same list for the same line.
// PREVENTS: the selector value hiding the declaration, which left `| received`
// residual and refused as an unknown operation before the handler ran.
func TestPeerRibHandsItsOperationsPastTheSelector(t *testing.T) {
	const path = "show bgp peer rib"
	loader, err := yang.DefaultLoader()
	require.NoError(t, err)
	defs := yang.PathToArgDefs(loader)[path]
	require.NotEmpty(t, defs, "the model declares no selector for %s", path)

	var received []command.PipeOperation
	var selector string
	calls := 0
	handler := func(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		received = args.PipeOperations()
		selector, _ = args.Value("selector")
		// The RIB's count override answers {"count": N}, the generic's
		// shape, which Dispatch checks (CheckOverrideAnswer); so does this one.
		for _, operation := range received {
			if operation.Name == "count" {
				return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(`{"count":0}`)}, nil
			}
		}
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	d := pluginserver.NewDispatcher()
	require.NoError(t, d.RegisterWithOptions(path, handler, "under test", pluginserver.RegisterOptions{ArgDefs: defs}))

	want := []command.PipeOperation{{Name: "received"}, {Name: "count"}}
	for _, tc := range []struct {
		line string
		peer string
	}{
		{line: "show bgp peer edge1 rib | received | count", peer: "edge1"},
		{line: `show bgp peer "edge 1" rib | received | count`, peer: "edge 1"},
	} {
		t.Run(tc.line, func(t *testing.T) {
			before := calls
			_, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, tc.line)
			require.NoError(t, dispatchErr)
			require.Equal(t, before+1, calls, "the handler did not run")
			assert.Equal(t, want, received, "the handler's declared list")
			assert.Equal(t, tc.peer, selector, "the selector the handler read")

			local, problem := command.ReadLocalLine(tc.line, "")
			require.Empty(t, problem, "the surface refused the chain")
			assert.Equal(t, want, local.DeclaredOperations(), "the surface's declared list")
		})
	}
}
