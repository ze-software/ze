package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestClearInterfaceCounters_RegisteredWireMethod verifies the
// `ze-iface:clear-interface-counters` RPC is installed in the builtin
// registry so `ze clear interface ... counters` is reachable via the
// dispatcher.
//
// VALIDATES: clear verb wiring -- command is reachable at runtime.
// PREVENTS: regression where the handler exists but no init()
// registered it.
func TestClearInterfaceCounters_RegisteredWireMethod(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-iface:clear-interface-counters" {
			found = true
			require.NotNil(t, r.Handler)
			break
		}
	}
	require.True(t, found, "ze-iface:clear-interface-counters not registered")
}

// TestHandleClearInterfaceCounters_Scopes proves both whole forms pass the
// argument check and reach the counter reset with the scope they name:
// `clear interface counters all` arrives with no tokens and no selector, and
// `clear interface name <name> counters` with the name as its selector. The
// bare `clear interface counters` reaches no handler: the dispatcher refuses it
// (TestClearInterfaceCountersBareIsRefused). Method: call the handler with each
// form's validated arguments. No backend is loaded in this package's tests, so
// the reset answers the backend's own refusal, which only a call that got past
// the argument check can produce; a loaded backend answers the scope instead.
//
// VALIDATES: each whole form reaches iface.ResetCounters, never the usage line.
// PREVENTS: an argument check that refuses a correctly typed whole form.
func TestHandleClearInterfaceCounters_Scopes(t *testing.T) {
	tests := []struct {
		name string
		ctx  *pluginserver.CommandContext
		want string // the "cleared" value a loaded backend answers
	}{
		{"all", nil, "all"},
		{"name", &pluginserver.CommandContext{Selectors: map[string]string{"name": "eth0"}}, "eth0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleClearInterfaceCounters(tt.ctx, commandtest.Args())
			require.NoError(t, err)
			require.NotNil(t, resp)
			if resp.Status == plugin.StatusDone {
				data, ok := resp.Data.(plugin.Map)
				require.True(t, ok)
				assert.Equal(t, tt.want, data["cleared"])
				return
			}
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Equal(t, "iface: no backend loaded", resp.Error, "the form was refused before it reached the reset")
		})
	}
}

// TestHandleClearInterfaceCounters_RejectBadGrammar verifies that an
// invocation with more than two words, or with two words where neither
// is the `counters` keyword, rejects with the usage line.
//
// VALIDATES: AC (op-0 command 5) -- operator typos are surfaced rather
// than silently executed on the wrong scope.
func TestHandleClearInterfaceCounters_RejectBadGrammar(t *testing.T) {
	bad := [][]string{
		{"eth0", "packets"},     // unknown keyword after name
		{"one", "two", "three"}, // too many args
		{"foo", "bar"},          // no `counters` token anywhere
	}
	for _, args := range bad {
		resp, err := handleClearInterfaceCounters(nil, commandtest.Args(args...))
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status, "args=%v should reject", args)
		msg := resp.Error
		assert.Contains(t, msg, "usage", "args=%v error should include usage line", args)
	}
}
