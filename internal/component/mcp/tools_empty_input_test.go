// Design: docs/architecture/api/commands.md -- explicit input on every surface.
package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestMCPGeneratedCommandPreservesExplicitEmptyContent reaches a real typed
// dispatcher with empty and nonempty supplied input, retaining both outcomes.
func TestMCPGeneratedCommandPreservesExplicitEmptyContent(t *testing.T) {
	const path = "show fixture bytes"
	def := commandtest.Must(command.NewStringArg("content", []command.UintRange{{Min: 0, Max: 32}}, nil, command.ArgOptions{Mandatory: true}))
	dispatcher := pluginserver.NewDispatcher()
	var received string
	called := false
	if err := dispatcher.RegisterWithOptions(path, func(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		var found bool
		received, found = args.Keyed("content")
		called = found
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"octets": len(received)}), nil
	}, "Read supplied fixture bytes", pluginserver.RegisterOptions{ReadOnly: true, ArgDefs: []command.ArgDef{def}}); err != nil {
		t.Fatal(err)
	}
	server := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
			return dispatcher.Dispatch(&pluginserver.CommandContext{}, line)
		},
		commands: func() ([]CommandInfo, error) {
			return []CommandInfo{{Name: path, Params: []ParamInfo{{Name: "content", Type: "string", Required: true}}}}, nil
		},
	}
	for _, content := range []string{"", "YQ=="} {
		called = false
		payload, err := json.Marshal(map[string]string{"action": "bytes", "content": content})
		if err != nil {
			t.Fatal(err)
		}
		result := server.dispatchGenerated("show fixture", map[string]bool{"bytes": false}, payload)
		if _, failed := result["isError"]; failed {
			t.Fatalf("explicit content %q was refused: %v", content, result)
		}
		if !called || received != content {
			t.Fatalf("content %q reached handler=%t as %q", content, called, received)
		}
	}
}
