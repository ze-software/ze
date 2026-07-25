// Design: plan/spec-isis-13-cli-diag-interop.md -- `show isis ...` / `clear isis
// ...` proxy registration + contract tests.
// Related: cmd_show.go -- the proxy handlers under test.
//
// VALIDATES: every ze-show:isis-* / ze-clear:isis-* command declared in the
// grammar has a registered RPC handler with a PluginCommand (so the command is
// reachable rather than 404ing at dispatch, the project's recurring "unwired"
// defect); the proxy handlers reject extra args and a nil dispatcher gracefully.
// PREVENTS: a show/clear command whose wire method is declared but never wired,
// and a proxy that panics on a nil dispatcher or silently accepts stray args.

package isis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestISISShowClearRPCsRegistered: the full show/clear surface is registered,
// each with a handler and the PluginCommand that lets the engine claim the same
// command name (proxy contract).
func TestISISShowClearRPCsRegistered(t *testing.T) {
	want := map[string]string{
		"ze-show:isis-neighbor":        cmdShowNeighbor,
		"ze-show:isis-database":        cmdShowDatabase,
		"ze-show:isis-database-detail": cmdShowDatabaseDetail,
		"ze-show:isis-route":           cmdShowRoute,
		"ze-show:isis-route-ipv6":      cmdShowRouteIPv6,
		"ze-show:isis-interface":       cmdShowInterface,
		"ze-show:isis-hostname":        cmdShowHostname,
		"ze-show:isis-spf-log":         cmdShowSPFLog,
		"ze-clear:isis-adjacency":      cmdClearAdjacency,
		"ze-clear:isis-counters":       cmdClearCounters,
	}
	byMethod := make(map[string]pluginserver.RPCRegistration)
	for _, r := range pluginserver.AllBuiltinRPCs() {
		byMethod[r.WireMethod] = r
	}
	for method, pluginCmd := range want {
		r, ok := byMethod[method]
		require.Truef(t, ok, "RPC %s is registered", method)
		assert.NotNilf(t, r.Handler, "RPC %s has a handler", method)
		assert.Equalf(t, pluginCmd, r.PluginCommand, "RPC %s fronts the right plugin command", method)
	}
}

// TestISISProxyNilDispatcher: a proxy degrades gracefully when no dispatcher is
// wired instead of panicking on a nil dereference.
func TestISISProxyNilDispatcher(t *testing.T) {
	resp, err := forwardShowNeighbor(&pluginserver.CommandContext{}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "dispatcher unavailable")
}

// TestISISShowProxyArgsRejected: every proxy rejects extra args before touching
// the dispatcher (the proxied plugin commands take none).
func TestISISShowProxyArgsRejected(t *testing.T) {
	handlers := []func(*pluginserver.CommandContext, []string) (*plugin.Response, error){
		forwardShowNeighbor, forwardShowDatabase, forwardShowDatabaseDetail,
		forwardShowRoute, forwardShowRouteIPv6, forwardShowInterface, forwardShowHostname,
		forwardShowSPFLog, forwardClearAdjacency, forwardClearCounters,
	}
	for _, h := range handlers {
		resp, err := h(&pluginserver.CommandContext{}, []string{"extra"})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "unexpected argument")
	}
}
