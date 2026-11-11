package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// pluginRestartRegistration answers the builtin registration of
// `request plugin restart`, as init() handed it to RegisterRPCs, so the test
// judges the completer the daemon really loads.
func pluginRestartRegistration(t *testing.T) RPCRegistration {
	t.Helper()
	for _, reg := range AllBuiltinRPCs() {
		if reg.WireMethod == "ze-plugin:system-plugin-restart" {
			return reg
		}
	}
	t.Fatal("ze-plugin:system-plugin-restart is not registered")
	return RPCRegistration{}
}

// TestPluginRestartCompletesConfiguredPlugins proves Tab after
// `request plugin restart` offers the configured plugin names.
//
// Goal: the name leaf completes from the daemon's configured plugin set, on
// both routes a completion takes: the `system command complete args` RPC an
// outside caller (`ze cli`) asks, and the value hint WireArgCompletion hangs
// on an SSH session's tree.
// Method: a process manager configured with two plugins, neither running; the
// builtin is loaded through its real registration (Complete included), then
// both routes are asked. The completable flag `system command list` publishes
// is checked too, because the `ze cli` client wires the hint from it.
func TestPluginRestartCompletesConfiguredPlugins(t *testing.T) {
	reg := pluginRestartRegistration(t)
	require.NotNil(t, reg.Complete, "request plugin restart must declare an argument completer")

	d := NewDispatcher()
	require.NoError(t, d.RegisterWithOptions("request plugin restart", reg.Handler, "Start a stopped plugin again.",
		RegisterOptions{RequiresSelector: reg.RequiresSelector, Owner: reg.WireMethod, Complete: reg.Complete}))
	s := &Server{dispatcher: d}
	s.procManager.Store(process.NewProcessManager([]plugin.PluginConfig{{Name: "alpha"}, {Name: "beta"}}))
	ctx := &CommandContext{Server: s}

	values := func(resp *plugin.Response) []string {
		t.Helper()
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "the completion answer is a map")
		candidates, ok := data[fieldCompletions].([]Completion)
		require.True(t, ok, "the completion answer carries its candidates")
		names := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			names = append(names, candidate.Value)
		}
		return names
	}

	resp, err := handleSystemCommandComplete(ctx, commandtest.Args("args", "request plugin restart"))
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.ElementsMatch(t, []string{"alpha", "beta"}, values(resp), "no partial lists every configured plugin")

	resp, err = handleSystemCommandComplete(ctx, commandtest.Args("args", "request plugin restart", "al"))
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha"}, values(resp), "a partial keeps the names it starts")

	tree := &command.Node{Children: map[string]*command.Node{
		"request": {Name: "request", Children: map[string]*command.Node{
			"plugin": {Name: "plugin", Children: map[string]*command.Node{
				"restart": {Name: "restart"},
			}},
		}},
	}}
	s.WireArgCompletion(tree)
	node := command.FindNode(tree, []string{"request", "plugin", "restart"})
	require.NotNil(t, node.ValueHints, "the SSH tree's restart node must complete its argument")
	hinted := make([]string, 0, 2)
	for _, suggestion := range node.ValueHints() {
		hinted = append(hinted, suggestion.Text)
	}
	assert.ElementsMatch(t, []string{"alpha", "beta"}, hinted, "the SSH hint offers the configured plugins")

	rows := make(map[string]map[string]any)
	for record := range rpc.HeldRecords(commandRows(d, false)) {
		require.Nil(t, record.Fault, "commandRows yielded a rejected row")
		var row map[string]any
		require.NoError(t, json.Unmarshal(record.Item, &row))
		value, ok := row["value"].(string)
		require.True(t, ok, "every command row names its command")
		rows[value] = row
	}
	row, ok := rows["request plugin restart"]
	require.True(t, ok, "command list carries the restart row")
	assert.Equal(t, true, row["completable"],
		"command list marks the restart command completable, which ze cli wires the hint from")
}
