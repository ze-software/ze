// Design: docs/architecture/api/json-format.md -- a format operation makes data the rendered text.
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

// TestGRPCExecuteAnswersRenderedText asks whether gRPC `Execute` answers a
// line whose chain holds a text format with the rendered text as the
// response's data, a JSON string, byte for byte what `ze cli -c` prints (T-1,
// AC-5).
//
// Method: the real dispatcher behind the API engine and the gRPC server
// answers two rows; the request sends the line with `| table`. The data is
// decoded as a string and compared with the rendering surface's formatter
// (ProcessPipesDefaultFormatChecked) over the same rows.
//
// Red before the fix: the engine carried the text as plugin.RawJSON, which
// refuses a JSON string, so the data could not be encoded.
func TestGRPCExecuteAnswersRenderedText(t *testing.T) {
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
	line := path + " | table"

	response, err := client.Execute(t.Context(), &zepb.CommandRequest{Command: line})

	require.NoError(t, err)
	require.Equal(t, api.StatusDone, response.Status, "the formatted line was refused: %s", response.Error)
	var text string
	require.NoError(t, json.Unmarshal(response.Data, &text), "data %s is not one JSON string", response.Data)

	document, err := json.Marshal(rows)
	require.NoError(t, err)
	_, format, problem := command.ProcessPipesDefaultFormatChecked(line, "")
	require.Empty(t, problem, "the rendering surface refused the line")
	require.Equal(t, format(string(document)), text, "the text the rendering surface prints")
}
