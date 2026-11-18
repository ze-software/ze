// Design: docs/architecture/api/commands.md -- invocation placement follows typed dispatch.
package command_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestInvocationWriterUsesTheTypedDispatchersExplicitSelector binds an id at
// its actual path keyword, then exercises the real dispatcher and validator.
// The manual positive control distinguishes a writer defect from setup failure.
// MUTATION: project only the explicit ArgDef.Anchor and lose repeated id placement.
func TestInvocationWriterUsesTheTypedDispatchersExplicitSelector(t *testing.T) {
	const path = "show invocation-fixture id"
	for _, listener := range []string{"edge", "listener", "tunnel-id", "expected-id"} {
		t.Run(listener, func(t *testing.T) {
			defs := []command.ArgDef{
				commandtest.Must(command.NewUintArg("id", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{Mandatory: true})),
				commandtest.Must(command.NewUintArg("tunnel-id", 16, []command.UintRange{{Min: 1, Max: 65535}}, command.ArgOptions{})),
				commandtest.Must(command.NewUintArg("expected-id", 16, nil, command.ArgOptions{})),
				commandtest.Must(command.NewUintArg("count", 16, nil, command.ArgOptions{})),
				commandtest.Must(command.NewStringArg("listener", nil, nil, command.ArgOptions{})),
			}
			dispatcher := pluginserver.NewDispatcher()
			calls := 0
			require.NoError(t, dispatcher.RegisterWithOptions(path, func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
				calls++
				require.Equal(t, "7", ctx.Selector("id"))
				value, present := args.Value("listener")
				require.True(t, present)
				require.Equal(t, listener, value)
				count, present := args.Value("count")
				require.True(t, present)
				require.Equal(t, "3", count, "a nonpath keyword keeps its value")
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "Read one original", pluginserver.RegisterOptions{ReadOnly: true, ArgDefs: defs}))
			response, err := dispatcher.Dispatch(&pluginserver.CommandContext{}, path+" 7 tunnel-id 17 listener "+listener+" count 3")
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, response.Status)
			require.Equal(t, 1, calls)
			calls = 0
			var output textbuf.Buffer
			command.WriteInvocation(&output, strings.Fields(path), command.InvocationArgs(defs), map[string]string{"id": "7", "tunnel-id": "17", "listener": listener, "count": "3"})
			invocation := output.String()
			response, err = dispatcher.Dispatch(&pluginserver.CommandContext{}, invocation)
			require.NoError(t, err, "writer produced %q", invocation)
			require.Equal(t, plugin.StatusDone, response.Status)
			require.Equal(t, 1, calls)
			words, _, err := command.ScanExecution(invocation)
			require.NoError(t, err)
			keywords := 0
			for _, word := range words {
				if word == "id" {
					keywords++
				}
			}
			require.Equal(t, 1, keywords)
		})
	}
}
