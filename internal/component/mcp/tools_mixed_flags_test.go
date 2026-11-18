// Design: docs/architecture/mcp/overview.md -- typed flags and raw arguments retain separate meanings.
package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestGeneratedToolMixedArgumentsNeverEnableFalseFlags counts actual handler
// actions through the generated tool and dispatcher, preserving raw ordinary
// boolean data while distinguishing true, false and malformed flag values.
// MUTATION: flatten a false structured flag as the words force false.
func TestGeneratedToolMixedArgumentsNeverEnableFalseFlags(t *testing.T) {
	const path = "request mcp-flag-fixture apply"
	defs := []command.ArgDef{
		commandtest.Must(command.NewUintArg("id", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewFlagArg("force", command.ArgOptions{})),
		commandtest.Must(command.NewEnumArg("enabled", []string{"true", "false"}, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("label", nil, nil, command.ArgOptions{})),
	}
	dispatcher := pluginserver.NewDispatcher()
	calls, forced := 0, 0
	var selected, enabled, label string
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		if args.Flag("force") {
			forced++
		}
		selected, _ = args.Value("id")
		enabled, _ = args.Value("enabled")
		label, _ = args.Value("label")
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"id": selected}), nil
	}, "Apply fixture", pluginserver.RegisterOptions{ArgDefs: defs}))
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{}, path+" id 7 force enabled false label original")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, forced)
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
			return dispatcher.Dispatch(&pluginserver.CommandContext{}, input)
		},
		commands: func() ([]CommandInfo, error) {
			return []CommandInfo{{Name: path, Params: []ParamInfo{
				{Name: "id", Type: "uint16", Required: true}, {Name: "force", Type: "empty", Flag: true},
				{Name: "enabled", Type: "boolean"}, {Name: "label", Type: "string"},
			}}}, nil
		},
	}
	for _, input := range []struct {
		name          string
		force         any
		good, enabled bool
	}{
		{"false", false, true, false}, {"true", true, true, true}, {"invalid", "maybe", false, false},
	} {
		t.Run(input.name, func(t *testing.T) {
			before, beforeForced := calls, forced
			body, err := json.Marshal(map[string]any{"id": 7, "force": input.force, "arguments": "enabled false label original"})
			require.NoError(t, err)
			result := s.dispatchGenerated(path, map[string]bool{"": false}, body)
			if !input.good {
				require.Equal(t, true, result["isError"], "%v", result)
				require.Contains(t, result, "content")
				message, err := json.Marshal(result["content"])
				require.NoError(t, err)
				require.Contains(t, string(message), "force")
				require.Contains(t, string(message), "No command ran")
				require.Contains(t, string(message), "ze help command")
				require.Equal(t, before, calls)
				require.Equal(t, beforeForced, forced)
				return
			}
			require.NotContains(t, result, "isError", "%v", result)
			require.Equal(t, before+1, calls)
			wantForced := beforeForced
			if input.enabled {
				wantForced++
			}
			require.Equal(t, wantForced, forced)
			require.Equal(t, "7", selected)
			require.Equal(t, "false", enabled)
			require.Equal(t, "original", label)
		})
	}
}
