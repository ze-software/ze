// VALIDATES: `show bgp rib protocol` and `request bgp rib fastpath` declare
// their arguments in the model, so the generated usage line states them and
// the dispatcher refuses a value the model does not admit.
// PREVENTS: a grammar spelled in the summary sentence, where `<protocol>` is
// inline HTML under CommonMark and vanishes from the published reference
// (plan/journal/command-takes-an-untyped-positional-value.md, 2026-09-15).

package rib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/rib/yang"
	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// ribLoader loads the embedded modules and the rib command module, so the
// assertions below read the module a reviewer can open rather than whatever
// the rest of the binary happened to register.
func ribLoader(t *testing.T) *configyang.Loader {
	t.Helper()
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded(), "load the embedded modules")
	require.NoError(t, loader.AddModuleFromText("ze-rib-cmd", yang.ZeRibCmdYANG), "load the rib module")
	require.NoError(t, loader.Resolve(), "resolve the rib module")
	return loader
}

// TestProtocolAndFastpathStateTheirArguments pins the two generated usage
// lines and proves each wire method reaches a registered handler.
func TestProtocolAndFastpathStateTheirArguments(t *testing.T) {
	registered := map[string]bool{}
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		registered[reg.WireMethod] = true
	}
	for _, method := range []string{"ze-rib-api:protocol", "ze-rib-api:fastpath"} {
		assert.True(t, registered[method], "%s is not registered, so a command the model declares reaches no handler", method)
	}

	root := configyang.BuildCommandTree(ribLoader(t))
	cases := []struct {
		path string
		want string
	}{
		{"show bgp rib protocol", "show bgp rib protocol <protocol>"},
		{"request bgp rib fastpath", "request bgp rib fastpath <enable|disable|status>"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			node := command.FindNode(root, strings.Fields(c.path))
			require.NotNil(t, node, "the module declares no %q command", c.path)
			assert.Equal(t, c.want, command.UsageLine(command.Usage(strings.Fields(c.path), node)))
		})
	}
}

// TestProtocolAndFastpathDispatchTheDeclaredValues dispatches each command
// with a value the model admits and one it does not, through the argument
// definitions the model produces.
//
// The protocol value is bound to the leaf that repeats the keyword it follows,
// so the handler receives it as a selector and the pipeline words as the tail.
// The fastpath action is a closed set, so a word outside it is refused by name
// before any handler runs.
func TestProtocolAndFastpathDispatchTheDeclaredValues(t *testing.T) {
	argDefs := configyang.PathToArgDefs(ribLoader(t))

	cases := []struct {
		path      string
		input     string
		args      []string
		selectors map[string]string
		wantErr   string
	}{
		{
			path:      "show bgp rib protocol",
			input:     "show bgp rib protocol bmp peer 192.0.2.1 count",
			args:      []string{"peer", "192.0.2.1", "count"},
			selectors: map[string]string{"protocol": "bmp"},
		},
		{
			path:    "show bgp rib protocol",
			input:   "show bgp rib protocol",
			wantErr: "required argument missing: protocol",
		},
		{
			path:  "request bgp rib fastpath",
			input: "request bgp rib fastpath enable",
			args:  []string{"enable"},
		},
		{
			path:    "request bgp rib fastpath",
			input:   "request bgp rib fastpath on",
			wantErr: `"on"`,
		},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			defs := argDefs[c.path]
			require.NotEmpty(t, defs, "the model declares no argument for %q", c.path)

			var gotArgs []string
			var gotSelectors map[string]string
			called := false
			handler := func(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
				called = true
				gotArgs = args
				gotSelectors = ctx.Selectors
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}

			d := pluginserver.NewDispatcher()
			d.RegisterWithOptions(c.path, handler, "under test", pluginserver.RegisterOptions{ArgDefs: defs})

			resp, err := d.Dispatch(&pluginserver.CommandContext{}, c.input)
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				assert.False(t, called, "the handler ran on a value the model refuses")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "done", resp.Status)
			assert.Equal(t, c.args, gotArgs)
			assert.Equal(t, c.selectors, gotSelectors)
		})
	}
}

// TestProtocolForwarderCarriesTheBoundValue proves the value the dispatcher
// lifted out of the tail is put back in front of it, which is where the
// bgp-rib plugin's handler reads the protocol.
func TestProtocolForwarderCarriesTheBoundValue(t *testing.T) {
	ctx := &pluginserver.CommandContext{Selectors: map[string]string{protocolLeaf: "bmp"}}
	assert.Equal(t, []string{"bmp", "peer", "192.0.2.1"}, protocolArgs(ctx, []string{"peer", "192.0.2.1"}))
}
