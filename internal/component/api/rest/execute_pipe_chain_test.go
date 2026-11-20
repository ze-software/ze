// Design: docs/architecture/api/commands.md -- the engine applies the pipe chain for every caller.
package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestRESTExecuteAppliesPipeChain asks whether REST execute answers a piped
// line with the chain applied by the engine, in its done envelope (AC-3, AC-4).
//
// Method: the real dispatcher, behind the API engine and the REST server,
// serves a command that answers two rows and declares no pipe operation. The
// request sends the line with `| count`, a generic operation, so the data must
// be the generic's `{"count": 2}`.
//
// Red before the partition: Dispatch refuses any pipe outside quotes
// (pipeChainRefusal), so the envelope is an error.
// MUTATION: leave the residual chain unapplied in Dispatch; the data is the two
// rows, not the count.
func TestRESTExecuteAppliesPipeChain(t *testing.T) {
	const path = "show fixture pipe rows"
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"rows": []any{
			map[string]any{"name": "a", "state": "up"},
			map[string]any{"name": "b", "state": "down"},
		}}), nil
	}, "Read two fixture rows", pluginserver.RegisterOptions{ReadOnly: true}))
	engine := api.NewAPIEngine(func(_ context.Context, caller api.CallerIdentity, input string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{Username: caller.Username, RemoteAddr: caller.RemoteAddr, Authorizer: caller.Authorizer}, input)
	}, func() ([]api.CommandMeta, error) {
		return []api.CommandMeta{{Name: path, ReadOnly: true}}, nil
	}, nil, nil)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, nil, func() []byte { return nil })
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Shutdown(context.Background())) })
	body, err := json.Marshal(map[string]string{"command": path + " | count"})
	require.NoError(t, err)

	response := do(t, srv, "POST", "/api/v1/execute", string(body))

	require.Equal(t, http.StatusOK, response.Status, response.Body)
	var result struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  string          `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
	require.Equal(t, api.StatusDone, result.Status, "the piped line was refused: %s", result.Error)
	require.JSONEq(t, `{"count":2}`, string(result.Data), "the engine did not apply | count")
}
