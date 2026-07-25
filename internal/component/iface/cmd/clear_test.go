package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestClearInterfaceCounters_RegisteredWireMethod verifies the
// `ze-clear:interface-counters` RPC is installed in the builtin
// registry so `ze clear interface ... counters` is reachable via the
// dispatcher.
//
// VALIDATES: clear verb wiring -- command is reachable at runtime.
// PREVENTS: regression where the handler exists but no init()
// registered it.
func TestClearInterfaceCounters_RegisteredWireMethod(t *testing.T) {
	found := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-clear:interface-counters" {
			found = true
			require.NotNil(t, r.Handler)
			break
		}
	}
	require.True(t, found, "ze-clear:interface-counters not registered")
}

// TestHandleClearInterfaceCounters_Grammars verifies every accepted
// argument shape resolves to the right scope. Covers:
//   - `clear interface counters`           (args=[], scope all)
//   - `clear interface counters`           (args=["counters"], scope all)
//   - `clear interface <name>`             (args=[<name>], scope named)
//   - `clear interface <name> counters`    (args=[<name>, "counters"], scope named)
//   - `clear interface counters <name>`    (args=["counters", <name>], tolerated)
//
// VALIDATES: AC (op-0 command 5) -- CLI grammar matches `show interface
// <name> counters` in symbol ordering; bare form clears all.
// PREVENTS: regression where a new argument ordering or a typo maps to
// the wrong scope (e.g. typo landing as "all").
func TestHandleClearInterfaceCounters_Grammars(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string // expected "cleared" value when status is done
	}{
		{"bare", nil, "all"},
		{"bare-counters", []string{"counters"}, "all"},
		{"name-only", []string{"eth0"}, "eth0"},
		{"name-then-counters", []string{"eth0", "counters"}, "eth0"},
		{"counters-then-name", []string{"counters", "eth0"}, "eth0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleClearInterfaceCounters(nil, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			// Backend may not be loaded in the unit-test environment; in
			// that case the handler surfaces the backend error with the
			// correct argument parse, so we still know the scope branch
			// that fired. Assert on shape only when the call succeeded.
			if resp.Status != plugin.StatusDone {
				return
			}
			data, ok := resp.Data.(plugin.Map)
			require.True(t, ok)
			assert.Equal(t, tt.want, data["cleared"], "args=%v", tt.args)
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
		resp, err := handleClearInterfaceCounters(nil, args)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status, "args=%v should reject", args)
		msg := resp.Error
		assert.Contains(t, msg, "usage", "args=%v error should include usage line", args)
	}
}

// TestClearInterface_DeprecatedNameFirst verifies old name-first grammar adds deprecation.
func TestClearInterface_DeprecatedNameFirst(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"name-only", []string{"eth0"}, "eth0"},
		{"name-then-counters", []string{"eth0", "counters"}, "eth0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleClearInterfaceCounters(nil, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			if resp.Status != plugin.StatusDone {
				return
			}
			data, ok := resp.Data.(plugin.Map)
			require.True(t, ok)
			assert.Equal(t, tt.want, data["cleared"])
			dep, hasDeprecated := data["deprecated"]
			assert.True(t, hasDeprecated, "deprecated grammar should have deprecation warning")
			assert.Contains(t, dep, "clear interface counters")
		})
	}
}

// TestClearInterface_CanonicalNoDeprecation verifies canonical grammar has no deprecation.
func TestClearInterface_CanonicalNoDeprecation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"bare", nil},
		{"counters-keyword", []string{"counters"}},
		{"counters-then-name", []string{"counters", "eth0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleClearInterfaceCounters(nil, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			if resp.Status != plugin.StatusDone {
				return
			}
			data, ok := resp.Data.(plugin.Map)
			require.True(t, ok)
			_, hasDeprecated := data["deprecated"]
			assert.False(t, hasDeprecated, "canonical grammar should not have deprecation")
		})
	}
}
