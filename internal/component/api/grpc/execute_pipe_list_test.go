// Design: docs/architecture/api/commands.md -- the ordered pipe list a structured route takes.
package grpc

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestGRPCExecutePipeList asks whether gRPC Execute takes the ordered pipe
// list of CommandRequest and answers what the typed line answers (AC-15,
// gRPC route).
//
// Method: the real dispatcher, behind the API engine and the gRPC server,
// serves a command that declares `received` and answers three rows, two of
// them received. The request's list `received` then `count` must answer the
// data the same RPC answers for `<path> | received | count`, a count of 2;
// with no list, the bare command's three rows; a name the command does not
// take is refused naming the valid ones.
//
// MUTATION: drop fromProtoPipe from fromProtoExecuteRequest; the answer is the
// three rows.
func TestGRPCExecutePipeList(t *testing.T) {
	const path = "show fixture grpc pipe list"
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
	engine := api.NewAPIEngine(func(_ context.Context, _ api.CallerIdentity, input string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{}, input)
	}, func() ([]api.CommandMeta, error) {
		return []api.CommandMeta{{Name: path, ReadOnly: true}}, nil
	}, nil, nil)
	client, _ := startTestServerWithEngine(t, "", engine)

	listed, err := client.Execute(t.Context(), &zepb.CommandRequest{Command: path, Pipe: []*zepb.PipeOperation{{Name: "received"}, {Name: "count"}}})
	require.NoError(t, err)
	require.Equal(t, api.StatusDone, listed.Status, "the pipe list was refused: %s", listed.Error)
	typed, err := client.Execute(t.Context(), &zepb.CommandRequest{Command: path + " | received | count"})
	require.NoError(t, err)
	require.JSONEq(t, string(typed.Data), string(listed.Data), "the pipe list answers other data than the typed line")
	var counted struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(listed.Data, &counted))
	require.Equal(t, 2, counted.Count, "received did not reach the handler: %s", listed.Data)

	bare, err := client.Execute(t.Context(), &zepb.CommandRequest{Command: path})
	require.NoError(t, err)
	require.Equal(t, api.StatusDone, bare.Status, bare.Error)
	require.NotContains(t, string(bare.Data), `"count"`, "an absent list changed the answer")

	refused, err := client.Execute(t.Context(), &zepb.CommandRequest{Command: path, Pipe: []*zepb.PipeOperation{{Name: "bogus"}}})
	message := ""
	if err != nil {
		message = err.Error()
	} else {
		require.NotEqual(t, api.StatusDone, refused.Status, "bogus was accepted")
		message = refused.Error
	}
	require.Contains(t, message, "received", "the refusal does not name the valid list")
	require.Contains(t, message, "match", "the refusal does not name the valid list")
}
