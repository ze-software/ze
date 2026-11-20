// Design: docs/architecture/api/commands.md -- the ordered pipe list a structured route takes.
package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestRESTExecutePipeList asks whether REST execute takes the ordered pipe
// list and answers what the typed line answers (AC-15, REST route).
//
// Method: the real dispatcher, behind the API engine and the REST server,
// serves a command that declares `received` and answers three rows, two of
// them received. The body's list `received` then `count` must answer the data
// the same route answers for `<path> | received | count`, a count of 2; with
// no list, the bare command's three rows; a name the command does not take is
// refused naming the valid ones; a misspelt element key is refused.
//
// MUTATION: drop the Pipe field from the domain request; the answer is the
// three rows.
func TestRESTExecutePipeList(t *testing.T) {
	const path = "show fixture rest pipe list"
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "received", Description: "Only the received rows", Argument: command.ArgNone})
	dispatcher := pluginserver.NewDispatcher()
	all := []any{
		map[string]any{"name": "a", "state": "received"},
		map[string]any{"name": "b", "state": "received"},
		map[string]any{"name": "c", "state": "advertised"},
	}
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		rows := all
		if slices.ContainsFunc(args.PipeOperations(), func(op command.PipeOperation) bool { return op.Name == "received" }) {
			rows = all[:2]
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"rows": rows}), nil
	}, "Read three fixture rows", pluginserver.RegisterOptions{ReadOnly: true}))
	engine := api.NewAPIEngine(func(_ context.Context, caller api.CallerIdentity, input string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{Username: caller.Username, RemoteAddr: caller.RemoteAddr, Authorizer: caller.Authorizer}, input)
	}, func() ([]api.CommandMeta, error) {
		return []api.CommandMeta{{Name: path, ReadOnly: true}}, nil
	}, nil, nil)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, nil, func() []byte { return nil })
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Shutdown(context.Background())) })

	execute := func(body map[string]any) (int, string, json.RawMessage, string) {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		response := do(t, srv, "POST", "/api/v1/execute", string(encoded))
		var result struct {
			Status string          `json:"status"`
			Data   json.RawMessage `json:"data"`
			Error  string          `json:"error"`
		}
		if response.Status == http.StatusOK {
			require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
		}
		return response.Status, result.Status, result.Data, response.Body
	}

	code, status, listed, body := execute(map[string]any{
		"command": path, command.PipeListParameter: []map[string]any{{"name": "received"}, {"name": "count"}},
	})
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, api.StatusDone, status, body)
	_, _, typed, body := execute(map[string]any{"command": path + " | received | count"})
	require.JSONEq(t, string(typed), string(listed), "the pipe list answers other data than the typed line: %s", body)
	var counted struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(listed, &counted))
	require.Equal(t, 2, counted.Count, "received did not reach the handler: %s", listed)

	_, _, bare, _ := execute(map[string]any{"command": path})
	_, _, absent, body := execute(map[string]any{"command": path, command.PipeListParameter: nil})
	require.JSONEq(t, string(bare), string(absent), "an absent list changed the answer: %s", body)

	code, status, _, body = execute(map[string]any{
		"command": path, command.PipeListParameter: []map[string]any{{"name": "bogus"}},
	})
	require.False(t, code == http.StatusOK && status == api.StatusDone, "bogus was accepted: %s", body)
	require.Contains(t, body, "received", "the refusal does not name the valid list")
	require.Contains(t, body, "match", "the refusal does not name the valid list")

	code, _, _, body = execute(map[string]any{
		"command": path, command.PipeListParameter: []map[string]any{{"name": "match", "arguments": []string{"a"}}},
	})
	require.Equal(t, http.StatusBadRequest, code, "a misspelt element key was accepted: %s", body)
}
