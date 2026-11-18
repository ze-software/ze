// Design: docs/architecture/api/commands.md -- every alias publishes its actual typed inputs.
package hub

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestCommandAliasesPublishTheirOwnTypedParameters resolves real command/RPC
// declarations. The parent keeps its RPC-only parameter; each typed alias must
// publish its own leaf name, width, requirement and inline placement.
// MUTATION: join only WireMethodToPath or use RPC inputs for a typed CLI node.
func TestCommandAliasesPublishTheirOwnTypedParameters(t *testing.T) {
	schema := selectorAliasSchema(t)
	params, err := buildParamMeta(schema)
	require.NoError(t, err)
	require.Len(t, params["show selector-fixture"], 1)
	require.Equal(t, "session-id", params["show selector-fixture"][0].Name, "RPC-only declaration remains useful")
	for _, input := range []struct{ path, name, kind string }{
		{path: "show selector-fixture id", name: "id", kind: "uint16"},
		{path: "show selector-fixture name", name: "name", kind: "string"},
	} {
		t.Run(input.path, func(t *testing.T) {
			require.Len(t, params[input.path], 2)
			var selected *commandParam
			for index := range params[input.path] {
				if params[input.path][index].Name == input.name {
					selected = &params[input.path][index]
				}
			}
			require.NotNil(t, selected)
			require.Equal(t, input.kind, selected.Type)
			require.True(t, selected.Required)
			require.Empty(t, selected.Anchor, "an own leaf declares no explicit ancestor anchor")
			for _, parameter := range params[input.path] {
				if parameter.Name == "listener" {
					require.Empty(t, parameter.Anchor, "nonpath keyword has no invented anchor")
				}
			}
		})
	}
}

// TestCommandAliasMetadataInvokesTheTypedOriginalPath sends the published
// parameters through the common writer and actual typed dispatcher. A literal
// positive control verifies useful admitted output before the generated call.
// MUTATION: advertise RPC session-id or omit the alias's repeated-keyword anchor.
func TestCommandAliasMetadataInvokesTheTypedOriginalPath(t *testing.T) {
	schema := selectorAliasSchema(t)
	params, err := buildParamMeta(schema)
	require.NoError(t, err)
	const path = "show selector-fixture id"
	dispatcher := pluginserver.NewDispatcher()
	calls := 0
	require.NoError(t, dispatcher.RegisterWithOptions(path, func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		require.Equal(t, "7", ctx.Selector("id"))
		listener, found := args.Value("listener")
		require.True(t, found)
		require.Equal(t, "tunnel-id", listener)
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Read one original", pluginserver.RegisterOptions{ReadOnly: true, ArgDefs: yangloader.PathToArgDefs(schema)[path]}))
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{}, path+" 7 listener tunnel-id")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Equal(t, 1, calls)
	calls = 0
	metadata := buildCommandMeta(dispatcher.Commands(), nil, params, nil, nil)
	require.Len(t, metadata, 1)
	var definitions []command.InvocationArg
	for _, parameter := range metadata[0].Params {
		definitions = append(definitions, command.InvocationArg{Name: parameter.Name, Anchor: parameter.Anchor})
	}
	var output textbuf.Buffer
	command.WriteInvocation(&output, strings.Fields(path), definitions, map[string]string{"id": "7", "listener": "tunnel-id"})
	invocation := output.String()
	response, err = dispatcher.Dispatch(&pluginserver.CommandContext{}, invocation)
	require.NoError(t, err, "metadata generated %q", invocation)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Equal(t, 1, calls)
}

func selectorAliasSchema(t *testing.T) *yangloader.Resolved {
	t.Helper()
	loader := yangloader.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-selector-fixture-api", `module ze-selector-fixture-api {
 namespace "urn:ze:selector-fixture:api"; prefix api;
 import ze-extensions { prefix ze; }
 rpc read { ze:help "Read the fixture."; input {
  leaf session-id { type uint16; ze:help "RPC-only identity."; }
 } }
}`))
	require.NoError(t, loader.AddModuleFromText("ze-selector-fixture-cmd", `module ze-selector-fixture-cmd {
 namespace "urn:ze:selector-fixture:cmd"; prefix cmd;
 import ze-extensions { prefix ze; }
 container show { config false; ze:help "Read current state.";
  container selector-fixture { config false; ze:command "ze-selector-fixture:read"; ze:rpc "ze-selector-fixture-api:read"; ze:help "Read the fixture.";
   container id { config false; ze:command "ze-selector-fixture:read"; ze:rpc "ze-selector-fixture-api:read"; ze:help "Read by number.";
    leaf id { type uint16 { range "1..65535"; } mandatory true; ze:help "Original number."; }
    leaf listener { type string { length "1..253"; } ze:help "Original listener."; }
   }
   container name { config false; ze:command "ze-selector-fixture:read"; ze:rpc "ze-selector-fixture-api:read"; ze:help "Read by name.";
    leaf name { type string; mandatory true; ze:help "Original name."; }
    leaf listener { type string { length "1..253"; } ze:help "Original listener."; }
   }
  }
 }
}`))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	return schema
}
