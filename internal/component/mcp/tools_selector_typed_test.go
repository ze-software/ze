// Design: docs/architecture/mcp/overview.md -- structured tool values reach typed dispatch unchanged.
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

// TestGeneratedToolRetainsTypedSelectorAndLiteralValues exercises generated
// MCP action assembly against the real dispatcher. A manual call establishes
// useful typed output before the tool's structured call is judged.
// MUTATION: drop metadata placement, flatten flags, or reinterpret literal data.
func TestGeneratedToolRetainsTypedSelectorAndLiteralValues(t *testing.T) {
	const prefix = "show mcp-selector-fixture"
	const path = prefix + " id"
	defs := []command.ArgDef{
		commandtest.Must(command.NewUintArg("id", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewUintArg("tunnel-id", 16, nil, command.ArgOptions{})),
		commandtest.Must(command.NewUintArg("expected-id", 16, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("listener", nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("label", nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewFlagArg("force", command.ArgOptions{})),
		commandtest.Must(command.NewEnumArg("enabled", []string{"true", "false"}, command.ArgOptions{})),
	}
	dispatcher := pluginserver.NewDispatcher()
	calls := 0
	var selected, listener, label, enabled string
	var labelPresent, force bool
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		selected = ctx.Selector("id")
		listener, _ = args.Value("listener")
		label, labelPresent = args.Value("label")
		enabled, _ = args.Value("enabled")
		force = args.Flag("force")
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"id": selected}), nil
	}, "Read one original", pluginserver.RegisterOptions{ReadOnly: true, ArgDefs: defs}))
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{}, path+" 7 listener edge force enabled false")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Equal(t, 1, calls)
	require.True(t, force)
	require.Equal(t, "false", enabled)
	s := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
			return dispatcher.Dispatch(&pluginserver.CommandContext{}, input)
		},
		commands: func() ([]CommandInfo, error) {
			return []CommandInfo{{Name: path, ReadOnly: true, Params: []ParamInfo{
				{Name: "id", Type: "uint16", Required: true},
				{Name: "tunnel-id", Type: "uint16"}, {Name: "expected-id", Type: "uint16"},
				{Name: "listener", Type: "string"}, {Name: "label", Type: "string"},
				{Name: "force", Type: "empty", Flag: true}, {Name: "enabled", Type: "boolean"},
			}}}, nil
		},
	}
	for _, input := range []struct {
		name, listener, label     string
		values                    map[string]any
		good, force, labelPresent bool
	}{
		{name: "listener-keyword", values: map[string]any{"listener": "listener"}, listener: "listener", good: true},
		{name: "tunnel-keyword", values: map[string]any{"listener": "tunnel-id"}, listener: "tunnel-id", good: true},
		{name: "guard-keyword", values: map[string]any{"listener": "expected-id"}, listener: "expected-id", good: true},
		{name: "empty", values: map[string]any{"label": ""}, labelPresent: true, good: true},
		{name: "quote-pipe", values: map[string]any{"label": `a"b|c`}, label: `a"b|c`, labelPresent: true, good: true},
		{name: "flaglike", values: map[string]any{"label": "-help"}, label: "-help", labelPresent: true, good: true},
		{name: "flag-true", values: map[string]any{"force": true, "enabled": false}, force: true, good: true},
		{name: "flag-false", values: map[string]any{"force": false, "enabled": false}, good: true},
		{name: "flag-invalid", values: map[string]any{"force": "maybe"}},
		{name: "zero", values: map[string]any{"id": 0}},
		{name: "wide", values: map[string]any{"id": 65536}},
	} {
		t.Run(input.name, func(t *testing.T) {
			before := calls
			values := map[string]any{"action": "id", "id": 7}
			for key, value := range input.values {
				values[key] = value
			}
			body, err := json.Marshal(values)
			require.NoError(t, err)
			result := s.dispatchGenerated(prefix, map[string]bool{"id": false}, body)
			if !input.good {
				require.Equal(t, true, result["isError"], "%v", result)
				require.Equal(t, before, calls)
				return
			}
			require.NotContains(t, result, "isError", "%v", result)
			require.Equal(t, before+1, calls)
			require.Equal(t, "7", selected)
			require.Equal(t, input.listener, listener)
			require.Equal(t, input.label, label)
			require.Equal(t, input.labelPresent, labelPresent)
			require.Equal(t, input.force, force)
			if _, present := input.values["enabled"]; present {
				require.Equal(t, "false", enabled)
			}
		})
	}
}
