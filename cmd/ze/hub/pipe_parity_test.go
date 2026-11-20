// Design: docs/architecture/api/commands.md -- one partition of the pipe chain serves every surface.
// Related: main_servers.go -- the per-surface dispatchers; api.go -- the REST and gRPC engine.

package hub

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestPipedLineParityAcrossSurfaces asks whether one piped line answers the
// same data on every data surface the daemon serves in process, and the data
// the local `ze` command answers for it (AC-33).
//
// Method: one daemon dispatcher serves a command answering two rows and
// declaring no pipe operation. Each line of the table is sent through the MCP
// surface dispatcher (serverDispatcher, as ze_execute calls it), the REST and
// gRPC engine (buildAPIEngine, which both transports share) and the
// dispatcher a plugin's dispatch-command reaches (Dispatch). Each answer is
// normalized to its decoded data, or its refusal text. The local answer is the
// one the generic operation computes over the two rows: a count and a count
// after a quoted match, each with the chain's pipe metadata and equal to what
// the rendering surface prints for the line with `| json` (AC-3), the same
// count for a line ending in `| json` (a format that prints a JSON document),
// and a refusal of an unknown operation. A line with a text format (`| table`,
// `| text`, `| yaml`) answers every surface the rendered text as a JSON
// string, byte for byte what the rendering surface prints (T-1, D-6, AC-5).
//
// The rendering surfaces (SSH exec, the web terminal, `ze cli -c`) and the
// forked dispatch-command transport are not reachable in process here; the
// functional tests of the same spec drive them.
//
// Red before the partition: Dispatch refuses any pipe outside quotes, so every
// line but the refused one answers a refusal instead of data.
// MUTATION: apply the residual chain on one surface only; that surface's row
// differs from the others.
func TestPipedLineParityAcrossSurfaces(t *testing.T) {
	const path = "show fixture pipe parity"
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.StartWithContext(t.Context()))
	rows := plugin.Map{"rows": []any{
		map[string]any{"name": "a", "state": "up"},
		map[string]any{"name": "b", "state": "link down"},
	}}
	require.NoError(t, server.Dispatcher().RegisterWithOptions(path, func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, rows), nil
	}, "Read two fixture rows", pluginserver.RegisterOptions{ReadOnly: true}))
	document, err := json.Marshal(rows)
	require.NoError(t, err)

	caller := plugin.CallerIdentity{Username: "alice", RemoteAddr: "192.0.2.25:1234"}
	mcp := serverDispatcher(server, audit.MCP)
	engine := buildAPIEngine(server)
	surfaces := []struct {
		name string
		send func(line string) (*plugin.Response, error)
	}{
		{"mcp", func(line string) (*plugin.Response, error) { return mcp(context.Background(), caller, line) }},
		{"rest and grpc", func(line string) (*plugin.Response, error) {
			return engine.Execute(t.Context(), &api.ExecuteRequest{Caller: caller, Command: line})
		}},
		{"dispatch-command", func(line string) (*plugin.Response, error) {
			return server.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: server}, line)
		}},
	}

	lines := []struct {
		name, chain string
		want        any    // the local answer's data; nil for the two rows below
		refusal     string // a word the refusal names, for a refused line
	}{
		{name: "generic count", chain: "| count", want: map[string]any{
			"count": float64(2),
			"pipe":  []any{map[string]any{"op": "count"}},
		}},
		{name: "quoted value", chain: `| match "link down" | count`, want: map[string]any{
			"count": float64(1),
			"pipe":  []any{map[string]any{"op": "match", "arg": "link down"}, map[string]any{"op": "count"}},
		}},
		{name: "json format", chain: "| count | json", want: map[string]any{
			"count": float64(2),
			"pipe":  []any{map[string]any{"op": "count"}},
		}},
		{name: "refused operation", chain: "| bogus", refusal: "bogus"},
		{name: "format", chain: "| table"},
		{name: "text format", chain: "| text"},
		{name: "yaml format", chain: "| yaml"},
	}
	for _, line := range lines {
		t.Run(line.name, func(t *testing.T) {
			input := path + " " + line.chain
			if line.want != nil {
				require.Equal(t, line.want, renderedAsJSON(t, input, document),
					"the rendering surface prints another answer for %q with | json", input)
			}
			formatted := ""
			if line.want == nil && line.refusal == "" {
				formatted = renderedText(t, input, document)
			}
			for _, surface := range surfaces {
				resp, sendErr := surface.send(input)
				data, refusal := parityOutcome(t, resp, sendErr)
				if line.refusal != "" {
					require.Contains(t, refusal, line.refusal, "%s answered %v for %q", surface.name, data, input)
					continue
				}
				require.Empty(t, refusal, "%s refused %q", surface.name, input)
				if line.want != nil {
					require.True(t, reflect.DeepEqual(line.want, data),
						"%s answered %v for %q, want %v", surface.name, data, input, line.want)
					continue
				}
				text, isText := data.(string)
				require.True(t, isText && strings.TrimSpace(text) != "",
					"%s answered %v for %q, want the rendered text", surface.name, data, input)
				require.Equal(t, formatted, text, "%s rendered %q other than `ze cli -c` prints it", surface.name, input)
			}
		})
	}
}

// renderedAsJSON answers what the rendering surface prints for input with
// `| json` over document, decoded: ProcessPipesDefaultFormatChecked is the
// formatter the SSH exec channel, and so `ze cli -c`, applies to the
// dispatcher's answer.
func renderedAsJSON(t *testing.T, input string, document []byte) any {
	t.Helper()
	if !command.HasFormatPipe(input) {
		input += " | json"
	}
	_, format, problem := command.ProcessPipesDefaultFormatChecked(input, "")
	require.Empty(t, problem, "the rendering surface refused %q", input)
	var printed any
	require.NoError(t, json.Unmarshal([]byte(format(string(document))), &printed),
		"the rendering surface printed no JSON for %q", input)
	return printed
}

// renderedText answers the text the rendering surface prints for input over
// document: the bytes `ze cli -c` prints for a line whose chain holds a text
// format.
func renderedText(t *testing.T, input string, document []byte) string {
	t.Helper()
	_, format, problem := command.ProcessPipesDefaultFormatChecked(input, "")
	require.Empty(t, problem, "the rendering surface refused %q", input)
	return format(string(document))
}

// parityOutcome normalizes one surface's answer to its decoded data, or to its
// refusal text when the surface refused, as plugin.ResponseJSON renders both.
func parityOutcome(t *testing.T, resp *plugin.Response, err error) (data any, refusal string) {
	t.Helper()
	output, renderErr := plugin.ResponseJSON(resp, err)
	if renderErr != nil {
		return nil, renderErr.Error()
	}
	require.NoError(t, json.Unmarshal([]byte(output), &data), "answer %q is not JSON", output)
	return data, ""
}
