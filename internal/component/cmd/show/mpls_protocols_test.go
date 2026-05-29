// Design: plan/spec-mpls-2-ldp.md, plan/spec-mpls-3-rsvp-te.md -- show grammar wiring
package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// VALIDATES: every `show rsvp-te ...` / `show ldp ...` ze:command declared in
// the grammar has a registered RPC handler, so the command is reachable rather
// than 404ing at dispatch time (the project's recurring "unwired" defect).
func TestMPLSProtocolShowRPCsRegistered(t *testing.T) {
	want := []string{
		"ze-show:rsvp-te-lsp",
		"ze-show:rsvp-te-interface",
		"ze-show:rsvp-te-tunnel",
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

// VALIDATES: the proxy handler rejects extra arguments (the proxied plugin
// commands take none).
func TestProxyShowRejectsArgs(t *testing.T) {
	h := proxyShowToPlugin("rsvp-te show-session")
	resp, err := h(&pluginserver.CommandContext{}, []string{"extra"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unexpected argument")
}

// VALIDATES: the proxy handler degrades gracefully when no dispatcher is wired
// (server unavailable) instead of panicking on a nil dereference.
func TestProxyShowNilDispatcher(t *testing.T) {
	h := proxyShowToPlugin("ldp show-neighbor")
	resp, err := h(&pluginserver.CommandContext{}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "dispatcher unavailable")
}
