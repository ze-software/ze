// Design: docs/architecture/api/architecture.md -- ambiguous metadata grants no command execution.
package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestRESTAmbiguousInvocationMetadataNeverExecutes establishes a useful
// read-only literal command, then supplies words matching two distinct paths
// of equal length. One metadata generation must refuse before either handler,
// preserving the transport's caller restriction rather than choosing an owner.
// MUTATION: accept the last equally long declaration rather than refusing.
func TestRESTAmbiguousInvocationMetadataNeverExecutes(t *testing.T) {
	defs := []command.ArgDef{commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true}))}
	dispatcher := pluginserver.NewDispatcher()
	calls, generations := 0, 0
	for _, registered := range []struct {
		path string
		args []command.ArgDef
	}{
		{"show foo bar", nil}, {"show bar baz", defs},
	} {
		require.NoError(t, dispatcher.RegisterWithOptions(registered.path, func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
			calls++
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"called": true}), nil
		}, "Read fixture", pluginserver.RegisterOptions{ReadOnly: true, ArgDefs: registered.args}))
	}
	engine := api.NewAPIEngine(func(_ context.Context, caller api.CallerIdentity, input string) (*plugin.Response, error) {
		require.True(t, caller.ReadOnly, "transport identity remains read-only")
		return dispatcher.Dispatch(&pluginserver.CommandContext{Username: caller.Username, RemoteAddr: caller.RemoteAddr, Authorizer: caller.Authorizer}, input)
	}, func() ([]api.CommandMeta, error) {
		generations++
		return []api.CommandMeta{
			{Name: "show foo bar", ReadOnly: true},
			{Name: "show bar baz", ReadOnly: true, ArgDefs: defs, Params: []api.ParamMeta{{Name: "selector", Type: "string", Required: true}}},
		}, nil
	}, nil, nil)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, nil, func() []byte { return nil })
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Shutdown(context.Background())) })
	for _, input := range []struct {
		name, line string
		good       bool
	}{
		{"unique-useful", "show foo bar", true}, {"ambiguous-refused", "show foo bar baz", false},
	} {
		t.Run(input.name, func(t *testing.T) {
			beforeCalls, beforeGenerations := calls, generations
			body, err := json.Marshal(map[string]string{"command": input.line})
			require.NoError(t, err)
			response := do(t, srv, "POST", "/api/v1/execute", string(body))
			require.Equal(t, http.StatusOK, response.Status)
			var result struct{ Status, Error string }
			require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
			require.Equal(t, beforeGenerations+1, generations, "one metadata generation per request")
			if input.good {
				require.Equal(t, api.StatusDone, result.Status, result.Error)
				require.Equal(t, beforeCalls+1, calls)
				return
			}
			require.Equal(t, api.StatusError, result.Status)
			require.Contains(t, result.Error, "both")
			require.Contains(t, result.Error, "No command ran")
			require.Contains(t, result.Error, "ze help command")
			require.Equal(t, beforeCalls, calls, "ambiguous metadata cannot select either action")
		})
	}
}
