// Design: docs/architecture/api/commands.md -- the engine applies the pipe chain for every caller.

package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestZeExecuteAppliesPipeChain asks whether MCP `ze_execute` answers a piped
// line with the chain applied by the engine, as `ze cli -c` prints the line with
// `| json` (AC-3, AC-4).
//
// Method: the real dispatcher serves a command that answers two rows and
// declares no pipe operation. `ze_execute` sends the line with `| count`, a
// generic operation, so the answer must be the generic's `{"count": 2}`.
//
// Red before the partition: Dispatch refuses any pipe outside quotes
// (pipeChainRefusal), so the tool answers an error.
// MUTATION: leave the residual chain unapplied in Dispatch; the answer is the
// two rows, not the count.
func TestZeExecuteAppliesPipeChain(t *testing.T) {
	const path = "show fixture pipe rows"
	dispatcher := pluginserver.NewDispatcher()
	if err := dispatcher.RegisterWithOptions(path, func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"rows": []any{
			map[string]any{"name": "a", "state": "up"},
			map[string]any{"name": "b", "state": "down"},
		}}), nil
	}, "Read two fixture rows", pluginserver.RegisterOptions{ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
			return dispatcher.Dispatch(&pluginserver.CommandContext{}, line)
		},
	}
	args, err := json.Marshal(map[string]string{"command": path + " | count"})
	if err != nil {
		t.Fatal(err)
	}

	result := toolHandlers[toolNameExecute](runner, args)

	content, _ := result["content"].([]map[string]any)
	if len(content) == 0 {
		t.Fatalf("result carries no content: %v", result)
	}
	text, _ := content[0]["text"].(string)
	if result["isError"] == true {
		t.Fatalf("the piped line was refused: %s", text)
	}
	var answer any
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatalf("answer %q is not JSON: %v", text, err)
	}
	if want := map[string]any{"count": float64(2)}; !reflect.DeepEqual(answer, want) {
		t.Errorf("answer = %v, want %v: the engine did not apply | count", answer, want)
	}
}
