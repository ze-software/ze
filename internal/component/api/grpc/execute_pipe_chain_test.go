// Design: docs/architecture/api/commands.md -- the engine applies the pipe chain for every caller.
package grpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestGRPCExecuteAppliesPipeChain asks whether gRPC `Execute` answers a piped
// line with the chain applied by the engine, in its done response (AC-3, AC-4).
//
// Method: the real dispatcher, behind the API engine and the gRPC server,
// serves a command that answers two rows and declares no pipe operation. The
// request sends the line with `| count`, a generic operation, so the data must
// be the generic's count of 2 with the chain's pipe metadata, and it must equal
// what the rendering surface (`ProcessPipesDefaultFormatChecked`, the SSH exec
// path `ze cli -c` takes) prints for the same line with `| json` over the same
// document.
//
// Red before the partition: Dispatch refuses any pipe outside quotes
// (pipeChainRefusal), so the response is an error.
// MUTATION: leave the residual chain unapplied in Dispatch; the data is the two
// rows, not the count.
func TestGRPCExecuteAppliesPipeChain(t *testing.T) {
	const path = "show fixture pipe rows"
	dispatcher := pluginserver.NewDispatcher()
	rows := plugin.Map{"rows": []any{
		map[string]any{"name": "a", "state": "up"},
		map[string]any{"name": "b", "state": "down"},
	}}
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, rows), nil
	}, "Read two fixture rows", pluginserver.RegisterOptions{ReadOnly: true}))
	engine := api.NewAPIEngine(func(_ context.Context, _ api.CallerIdentity, input string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{}, input)
	}, func() ([]api.CommandMeta, error) {
		return []api.CommandMeta{{Name: path, ReadOnly: true}}, nil
	}, nil, nil)
	client, _ := startTestServerWithEngine(t, "", engine)

	response, err := client.Execute(t.Context(), &zepb.CommandRequest{Command: path + " | count"})

	require.NoError(t, err)
	require.Equal(t, api.StatusDone, response.Status, "the piped line was refused: %s", response.Error)
	require.JSONEq(t, `{"count":2,"pipe":[{"op":"count"}]}`, string(response.Data), "the engine did not apply | count")

	document, err := json.Marshal(rows)
	require.NoError(t, err)
	_, format, problem := command.ProcessPipesDefaultFormatChecked(path+" | count | json", "")
	require.Empty(t, problem, "the rendering surface refused the line")
	require.JSONEq(t, format(string(document)), string(response.Data),
		"the data differs from what the rendering surface prints for the line with | json")
}
