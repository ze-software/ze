// Design: docs/architecture/cli/command-completion.md -- builtin argument position
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestPluginRestartCompletionRejectsSecondValueThroughRPC exercises the actual
// registered restart completer through the system completion RPC. A configured
// name is offered while its keyword awaits a value, and never after that value.
// This uses only the current API and stays unchanged when completed arguments
// become available to the registered producer.
func TestPluginRestartCompletionRejectsSecondValueThroughRPC(t *testing.T) {
	registration := pluginRestartRegistration(t)
	require.NotNil(t, registration.Complete)
	dispatcher := NewDispatcher()
	require.NoError(t, dispatcher.RegisterWithOptions("request plugin restart", registration.Handler,
		"Restart a configured plugin.", RegisterOptions{Complete: registration.Complete, Owner: registration.WireMethod}))
	require.NoError(t, dispatcher.RegisterWithOptions("system command complete", handleSystemCommandComplete,
		"Complete command arguments.", RegisterOptions{ReadOnly: true}))
	server := &Server{dispatcher: dispatcher}
	server.procManager.Store(process.NewProcessManager([]plugin.PluginConfig{{Name: "configured"}}))
	ctx := &CommandContext{Server: server, RequestContext: t.Context(), Sender: plugin.OperatorSender()}

	response, err := dispatcher.Dispatch(ctx, `system command complete args "request plugin restart" name ""`)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	data, ok := response.Data.(plugin.Map)
	require.True(t, ok)
	candidates, ok := data[fieldCompletions].([]Completion)
	require.True(t, ok)
	require.Equal(t, []Completion{{Value: "configured", ShortHelp: completionNotRunning}}, candidates)

	response, err = dispatcher.Dispatch(ctx, `system command complete args "request plugin restart" name configured ""`)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	data, ok = response.Data.(plugin.Map)
	require.True(t, ok)
	candidates, ok = data[fieldCompletions].([]Completion)
	require.True(t, ok)
	require.Empty(t, candidates, "restart takes one configured plugin name and has no second value")
}
