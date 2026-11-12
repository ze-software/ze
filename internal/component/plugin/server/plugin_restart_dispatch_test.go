package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// restartLineDispatcher answers a dispatcher loaded the way the daemon loads
// its builtins: every registered handler, under the path, help and argument
// definitions the real YANG tree gives it. It is built before any test resets
// the plugin registry, so the schema it reads is the full one.
func restartLineDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load YANG")
	d := NewDispatcher()
	require.NoError(t, loadBuiltinsWithAliases(d, AllBuiltinRPCs(), yang.WireMethodToPaths(loader), yang.PathToDescription(loader),
		yang.PathToHelp(loader), yang.PathToArgDefs(loader), yang.BuildCommandTree(loader)))
	return d
}

// TestRestartCommandRunsFromTheOperatorLine types the fix line ze doctor
// prints, as the operator types it, into the real dispatcher.
//
// Goal: `request plugin restart <name>` reaches the handler and restarts the
// named plugin, in the positional form the command reference documents and
// in the keyword form the web admin form posts.
// Method: one internal plugin started through the real startup phase; each
// line is dispatched as text, and the process the manager holds afterwards
// must be a new one.
// PREVENTS: the command registering the peer-selector guard
// (RequiresSelector) over a leaf named `name`. The guard waits for a leaf
// named `selector`, which this command does not declare, so it refused every
// line with "requires a selector" and no doctor fix line could run.
func TestRestartCommandRunsFromTheOperatorLine(t *testing.T) {
	d := restartLineDispatcher(t)
	const name = "restart-line"
	s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
	ctx := &CommandContext{Server: s}

	for _, line := range []string{
		"request plugin restart " + name,
		"request plugin restart name " + name,
	} {
		before := s.procManager.Load().GetProcess(name)
		resp, err := d.Dispatch(ctx, line)
		require.NoError(t, err, "line %q", line)
		require.Equal(t, plugin.StatusDone, resp.Status, "line %q answered: %s", line, resp.Error)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "line %q: data %#v", line, resp.Data)
		assert.Equal(t, name, data[fieldRestartedPlugin], "line %q restarts the plugin it names", line)
		assert.Equal(t, true, data[fieldRestartWasRunning], "line %q: the plugin was running", line)
		after := s.procManager.Load().GetProcess(name)
		require.NotNil(t, after, "line %q left no process", line)
		assert.NotSame(t, before, after, "line %q did not replace the process", line)
	}
}

// TestRestartCommandLineRefusalsAndCompletion dispatches the lines an operator
// can get wrong, and the completion Tab asks, through the real dispatcher.
//
// Goal: the bare command is refused for the name it needs, before the handler
// runs and without naming a `selector` keyword the command does not have; two
// names reach the handler's own refusal; Tab still offers the configured
// plugins once the command is loaded from the YANG tree.
// Method: a process manager configured with two plugins, neither running, so
// no line can start anything.
func TestRestartCommandLineRefusalsAndCompletion(t *testing.T) {
	d := restartLineDispatcher(t)
	s := &Server{dispatcher: d}
	s.procManager.Store(process.NewProcessManager([]plugin.PluginConfig{{Name: "alpha"}, {Name: "beta"}}))
	ctx := &CommandContext{Server: s}

	t.Run("bare", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, "request plugin restart")
		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "required argument missing: name", "the refusal names the leaf the line left out")
		assert.NotContains(t, resp.Error, "selector", "the command has no selector keyword to name")
		assert.NotContains(t, resp.Error, "needs exactly one plugin name", "the handler must not run on a line the model refuses")
	})

	t.Run("two names", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, "request plugin restart alpha beta")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "request plugin restart needs exactly one plugin name")
	})

	t.Run("completion", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `system command complete args "request plugin restart"`)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status, "answer: %s", resp.Error)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "data: %#v", resp.Data)
		candidates, ok := data[fieldCompletions].([]Completion)
		require.True(t, ok, "completions: %#v", data[fieldCompletions])
		names := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			names = append(names, candidate.Value)
		}
		assert.ElementsMatch(t, []string{"alpha", "beta"}, names, "Tab offers every configured plugin")
	})
}
