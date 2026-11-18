// Design: docs/architecture/api/architecture.md -- typed parameters reach the registered selector.
package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestRESTStructuredParametersRetainTypedOriginalSelector sends real execute
// requests through REST, APIEngine and the actual typed dispatcher. Raw and
// mixed invocations remain useful controls; duplicate selectors cannot act.
// MUTATION: flatten parameters without the selected command's declared placement.
func TestRESTStructuredParametersRetainTypedOriginalSelector(t *testing.T) {
	const path = "show api-selector-fixture id"
	var calls atomic.Int32
	dispatcher := pluginserver.NewDispatcher()
	defs := []command.ArgDef{
		commandtest.Must(command.NewUintArg("id", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewUintArg("tunnel-id", 16, nil, command.ArgOptions{})),
		commandtest.Must(command.NewUintArg("expected-id", 16, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("listener", nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("label", nil, nil, command.ArgOptions{})),
	}
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		calls.Add(1)
		listener, _ := args.Value("listener")
		label, labelPresent := args.Value("label")
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"id": ctx.Selector("id"), "listener": listener, "label": label, "label-present": labelPresent}), nil
	}, "Read one original", pluginserver.RegisterOptions{ReadOnly: true, ArgDefs: defs}))
	engine := api.NewAPIEngine(func(_ context.Context, _ api.CallerIdentity, input string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{}, input)
	}, func() ([]api.CommandMeta, error) {
		return []api.CommandMeta{{Name: path, ReadOnly: true, Params: []api.ParamMeta{
			{Name: "id", Type: "uint16", Required: true}, {Name: "tunnel-id", Type: "uint16"}, {Name: "expected-id", Type: "uint16"}, {Name: "listener", Type: "string"}, {Name: "label", Type: "string"},
		}}}, nil
	}, nil, nil)
	srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, nil, func() []byte { return nil })
	require.NoError(t, err)
	for _, input := range []struct {
		name, command, listener, label string
		params                         map[string]any
		good, labelPresent             bool
	}{
		{name: "raw-control", command: path + " 7 tunnel-id 17 listener edge", listener: "edge", good: true},
		{name: "mixed-control", command: path + " 7", params: map[string]any{"tunnel-id": 17, "listener": "edge"}, listener: "edge", good: true},
		{name: "listener-value", command: path, params: map[string]any{"id": 7, "listener": "listener"}, listener: "listener", good: true},
		{name: "tunnel-keyword-value", command: path, params: map[string]any{"id": 7, "listener": "tunnel-id"}, listener: "tunnel-id", good: true},
		{name: "guard-keyword-value", command: path, params: map[string]any{"id": 7, "listener": "expected-id"}, listener: "expected-id", good: true},
		{name: "explicit-empty", command: path + " 7", params: map[string]any{"label": ""}, labelPresent: true, good: true},
		{name: "quoted-data", command: path + " 7", params: map[string]any{"label": `a"b|c`}, label: `a"b|c`, labelPresent: true, good: true},
		{name: "flaglike-data", command: path + " 7", params: map[string]any{"label": "-help"}, label: "-help", labelPresent: true, good: true},
		{name: "zero-refused", command: path, params: map[string]any{"id": 0}},
		{name: "wide-refused", command: path, params: map[string]any{"id": 65536}},
		{name: "duplicate-refused", command: path + " 7", params: map[string]any{"id": 8}},
	} {
		t.Run(input.name, func(t *testing.T) {
			before := calls.Load()
			body, err := json.Marshal(map[string]any{"command": input.command, "params": input.params})
			require.NoError(t, err)
			response := do(t, srv, "POST", "/api/v1/execute", string(body))
			require.Equal(t, http.StatusOK, response.Status)
			var result struct {
				Status string
				Error  string
				Data   struct {
					ID, Listener, Label string
					LabelPresent        bool `json:"label-present"`
				}
			}
			require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
			if !input.good {
				require.Equal(t, api.StatusError, result.Status)
				require.NotEmpty(t, result.Error)
				require.Equal(t, before, calls.Load())
				return
			}
			require.Equal(t, api.StatusDone, result.Status, result.Error)
			require.Equal(t, before+1, calls.Load())
			require.Equal(t, "7", result.Data.ID)
			require.Equal(t, input.listener, result.Data.Listener)
			require.Equal(t, input.label, result.Data.Label)
			require.Equal(t, input.labelPresent, result.Data.LabelPresent)
		})
	}
}
