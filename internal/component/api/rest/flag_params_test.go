// Design: docs/architecture/api/architecture.md -- false structured flags never enable an action.
package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestRESTMixedParametersNeverEnableFalseFlags uses REST, APIEngine and the
// actual dispatcher. Raw terminal and ancestor selectors remain useful; only
// a true structured flag may enable the fixture's counted action.
// MUTATION: omit the matched declaration when normalizing raw/mixed flags.
func TestRESTMixedParametersNeverEnableFalseFlags(t *testing.T) {
	for _, shape := range []struct {
		name, path, raw string
		selector        command.ArgDef
	}{
		{"terminal", "request flag-fixture id", "request flag-fixture id 7", commandtest.Must(command.NewUintArg("id", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{Mandatory: true}))},
		{"ancestor", "request flag-fixture peer apply", "request flag-fixture peer edge apply", commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true, Anchor: "peer"}))},
		{"implicit", "request flag-fixture interface apply", "request flag-fixture interface edge apply", commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true}))},
	} {
		t.Run(shape.name, func(t *testing.T) {
			defs := []command.ArgDef{shape.selector,
				commandtest.Must(command.NewFlagArg("force", command.ArgOptions{})),
				commandtest.Must(command.NewEnumArg("enabled", []string{"true", "false"}, command.ArgOptions{})),
				commandtest.Must(command.NewStringArg("label", nil, nil, command.ArgOptions{})),
			}
			dispatcher := pluginserver.NewDispatcher()
			calls, forced := 0, 0
			var gotSelector, enabled, label string
			var executed string
			require.NoError(t, dispatcher.RegisterWithOptions(shape.path, func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
				calls++
				if args.Flag("force") {
					forced++
				}
				gotSelector = ctx.Selector(shape.selector.Name())
				enabled, _ = args.Value("enabled")
				label, _ = args.Value("label")
				return plugin.NewResponse(plugin.StatusDone, plugin.Map{"selected": gotSelector}), nil
			}, "Apply fixture", pluginserver.RegisterOptions{ArgDefs: defs}))
			engine := api.NewAPIEngine(func(_ context.Context, _ api.CallerIdentity, input string) (*plugin.Response, error) {
				executed = input
				return dispatcher.Dispatch(&pluginserver.CommandContext{}, input)
			}, func() ([]api.CommandMeta, error) {
				return []api.CommandMeta{{Name: shape.path, ArgDefs: defs, Params: []api.ParamMeta{
					{Name: shape.selector.Name(), Type: "string", Required: true},
					{Name: "force", Type: "empty", Flag: true}, {Name: "enabled", Type: "boolean"}, {Name: "label", Type: "string"},
				}}}, nil
			}, nil, nil)
			srv, err := NewRESTServer(RESTConfig{ListenAddrs: []string{"127.0.0.1:0"}, Authentication: testAuthentication("fixture", nil, nil)}, engine, nil, func() []byte { return nil })
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, srv.Shutdown(context.Background())) })
			for _, input := range []struct {
				name, raw   string
				params      map[string]any
				good, force bool
			}{
				{name: "raw-useful", raw: shape.raw + " enabled false label original", good: true},
				{name: "raw-true-useful", raw: shape.raw + " force enabled false label original", good: true, force: true},
				{name: "mixed-false", raw: shape.raw, params: map[string]any{"force": false, "enabled": false, "label": "original"}, good: true},
				{name: "mixed-true", raw: shape.raw, params: map[string]any{"force": true, "enabled": false, "label": "original"}, good: true, force: true},
				{name: "mixed-invalid", raw: shape.raw, params: map[string]any{"force": "maybe", "enabled": false, "label": "original"}},
			} {
				t.Run(input.name, func(t *testing.T) {
					before, beforeForced := calls, forced
					body, err := json.Marshal(map[string]any{"command": input.raw, "params": input.params})
					require.NoError(t, err)
					response := doWithHeader(t, srv, "POST", "/api/v1/execute", string(body), map[string]string{"Content-Type": "application/json", "Authorization": "Bearer fixture"})
					require.Equal(t, http.StatusOK, response.Status)
					var result struct{ Status, Error string }
					require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
					if !input.good {
						require.Equal(t, api.StatusError, result.Status)
						require.Contains(t, result.Error, "force")
						require.Contains(t, result.Error, "No command ran")
						require.Contains(t, result.Error, "ze help command")
						require.Equal(t, before, calls)
						require.Equal(t, beforeForced, forced)
						return
					}
					require.Equal(t, api.StatusDone, result.Status, result.Error)
					require.Equal(t, before+1, calls)
					require.True(t, strings.HasPrefix(executed, input.raw), "raw text must remain unchanged: %q", executed)
					wantForced := beforeForced
					if input.force {
						wantForced++
					}
					require.Equal(t, wantForced, forced, "false must never enable the action")
					wantSelector := "edge"
					if shape.name == "terminal" {
						wantSelector = "7"
					}
					require.Equal(t, wantSelector, gotSelector)
					require.Equal(t, "false", enabled, "ordinary boolean data must survive")
					require.Equal(t, "original", label)
				})
			}
		})
	}
}
