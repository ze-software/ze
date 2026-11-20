// Design: docs/architecture/api/json-format.md -- a format operation makes data the rendered text.

package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestZeExecuteAnswersRenderedText asks whether MCP `ze_execute` answers a
// line whose chain holds a text format with the rendered text, carried as a
// JSON string in the tool's text item, byte for byte what `ze cli -c` prints
// (T-1, D-6, AC-5).
//
// Method: the real dispatcher answers two rows; `ze_execute` sends the line
// with `| table`. The first text item is decoded as a string and compared with
// the rendering surface's formatter (ProcessPipesDefaultFormatChecked) over
// the same rows.
//
// Red before the fix: the engine carried the text as plugin.RawJSON, which
// refuses a JSON string, so the tool answered "marshal response" as an error.
func TestZeExecuteAnswersRenderedText(t *testing.T) {
	const path = "show fixture pipe rows"
	dispatcher := pluginserver.NewDispatcher()
	rows := plugin.Map{"rows": []any{
		map[string]any{"name": "a", "state": "up"},
		map[string]any{"name": "b", "state": "down"},
	}}
	if err := dispatcher.RegisterWithOptions(path, func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, rows), nil
	}, "Read two fixture rows", pluginserver.RegisterOptions{ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
			return dispatcher.Dispatch(&pluginserver.CommandContext{}, line)
		},
	}
	line := path + " | table"
	args, err := json.Marshal(map[string]string{"command": line})
	if err != nil {
		t.Fatal(err)
	}

	result := toolHandlers[toolNameExecute](runner, args)

	content, _ := result["content"].([]map[string]any)
	if len(content) == 0 {
		t.Fatalf("result carries no content: %v", result)
	}
	item, _ := content[0]["text"].(string)
	if result["isError"] == true {
		t.Fatalf("the formatted line was refused: %s", item)
	}
	var text string
	if err := json.Unmarshal([]byte(item), &text); err != nil {
		t.Fatalf("answer %q is not one JSON string: %v", item, err)
	}

	document, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	_, format, problem := command.ProcessPipesDefaultFormatChecked(line, "")
	if problem != "" {
		t.Fatalf("the rendering surface refused the line: %s", problem)
	}
	if want := format(string(document)); text != want {
		t.Errorf("answer = %q, the rendering surface prints %q", text, want)
	}
}
