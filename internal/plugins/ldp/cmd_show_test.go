package ldp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: every `show ldp ...` ze:command declared in the grammar has a
// registered RPC handler, so the command is reachable rather than 404ing at
// dispatch time (the project's recurring "unwired" defect).
func TestLDPShowRPCsRegistered(t *testing.T) {
	want := []string{
		"ze-show:ldp-neighbor",
		"ze-show:ldp-binding",
	}
	byMethod := make(map[string]pluginserver.RPCRegistration)
	for _, r := range pluginserver.AllBuiltinRPCs() {
		byMethod[r.WireMethod] = r
	}
	for _, w := range want {
		r, ok := byMethod[w]
		require.Truef(t, ok, "RPC %s is registered", w)
		assert.NotNilf(t, r.Handler, "RPC %s has a handler", w)
	}
}

// VALIDATES: the proxy handler degrades gracefully when no dispatcher is wired
// (server unavailable) instead of panicking on a nil dereference.
func TestProxyShowNilDispatcher(t *testing.T) {
	resp, err := forwardShowNeighbor(&pluginserver.CommandContext{}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "dispatcher unavailable")
}

// VALIDATES: the proxy handler rejects extra arguments (the proxied plugin
// commands take none) before it ever touches the dispatcher.
func TestProxyShowRejectsArgs(t *testing.T) {
	resp, err := forwardShowBinding(&pluginserver.CommandContext{}, []string{"extra"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unexpected argument")
}
