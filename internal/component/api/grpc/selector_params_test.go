// Design: docs/architecture/api/architecture.md -- protobuf parameters retain typed selection.
package grpc

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestGRPCStructuredParametersRetainTypedOriginalSelector uses the existing
// joined gRPC fixture and real typed dispatcher. A raw control stays useful,
// legal keyword-shaped listener values remain data, and invalid IDs cannot act.
// MUTATION: convert protobuf Params to an untyped suffix before engine metadata.
func TestGRPCStructuredParametersRetainTypedOriginalSelector(t *testing.T) {
	const path = "show api-selector-fixture id"
	var calls atomic.Int32
	dispatcher := pluginserver.NewDispatcher()
	defs := []command.ArgDef{
		commandtest.Must(command.NewUintArg("id", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewUintArg("tunnel-id", 16, nil, command.ArgOptions{})),
		commandtest.Must(command.NewUintArg("expected-id", 16, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg("listener", nil, nil, command.ArgOptions{})),
	}
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		calls.Add(1)
		listener, _ := args.Value("listener")
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"id": ctx.Selector("id"), "listener": listener}), nil
	}, "Read one original", pluginserver.RegisterOptions{ReadOnly: true, ArgDefs: defs}))
	engine := api.NewAPIEngine(func(_ context.Context, _ api.CallerIdentity, input string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{}, input)
	}, func() ([]api.CommandMeta, error) {
		return []api.CommandMeta{{Name: path, ReadOnly: true, Params: []api.ParamMeta{{Name: "id", Type: "uint16", Required: true}, {Name: "tunnel-id", Type: "uint16"}, {Name: "expected-id", Type: "uint16"}, {Name: "listener", Type: "string"}}}}, nil
	}, nil, nil)
	client, _ := startTestServerWithEngine(t, "", engine)
	for _, input := range []struct {
		name, command, id, listener string
		good                        bool
	}{
		{name: "raw-control", command: path + " 7 listener edge", listener: "edge", good: true},
		{name: "mixed-control", command: path + " 7", listener: "edge", good: true},
		{name: "listener-value", command: path, id: "7", listener: "listener", good: true},
		{name: "tunnel-keyword-value", command: path, id: "7", listener: "tunnel-id", good: true},
		{name: "guard-keyword-value", command: path, id: "7", listener: "expected-id", good: true},
		{name: "zero-refused", command: path, id: "0"},
		{name: "wide-refused", command: path, id: "65536"},
		{name: "duplicate-refused", command: path + " 7", id: "8"},
	} {
		t.Run(input.name, func(t *testing.T) {
			before := calls.Load()
			params := make(map[string]string)
			if input.id != "" {
				params["id"] = input.id
			}
			if input.command != path+" 7 listener edge" {
				params["listener"] = input.listener
			}
			response, err := client.Execute(t.Context(), &zepb.CommandRequest{Command: input.command, Params: params})
			require.NoError(t, err)
			if !input.good {
				require.Equal(t, api.StatusError, response.Status)
				require.NotEmpty(t, response.Error)
				require.Equal(t, before, calls.Load())
				return
			}
			require.Equal(t, api.StatusDone, response.Status, response.Error)
			require.Equal(t, before+1, calls.Load())
			var original struct{ ID, Listener string }
			require.NoError(t, json.Unmarshal(response.Data, &original))
			require.Equal(t, "7", original.ID)
			require.Equal(t, input.listener, original.Listener)
		})
	}
}
