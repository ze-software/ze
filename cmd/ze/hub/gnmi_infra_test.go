// Design: ai/rules/plugins.md -- ze_gnmi compile-out seam
//
// resolveGNMIListeners is always-on: the boot-time management-listener guard
// reads it before anything binds, and the gated builder calls the same function
// to bind. These tests pin the two questions it asks the config tree apart.

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
)

// setGNMIEnv sets one ze.gnmi.* variable for the duration of a test and puts
// the previous value back. env.Get reads a process-wide cache, so a leaked
// value would change every later test in this package.
func setGNMIEnv(t *testing.T, key, value string) {
	t.Helper()
	orig := env.Get(key)
	t.Cleanup(func() { _ = env.Set(key, orig) })
	require.NoError(t, env.Set(key, value))
}

// disabledGNMIBlockWithToken builds the config an operator writes when the
// listener comes from the environment: the block carries the authentication
// settings and the listen address, and `enabled false` because config itself
// must not start gNMI.
func disabledGNMIBlockWithToken(t *testing.T) *zeconfig.Tree {
	t.Helper()
	tree := disabledGNMIBlockTokenOnly(t)
	gnmi := tree.GetContainer("environment").GetContainer("gnmi")
	srv := zeconfig.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "9339")
	gnmi.AddListEntry("server", "main", srv)
	return tree
}

// disabledGNMIBlockTokenOnly is the same block naming no server, so the
// synthesized 0.0.0.0:9339 default is the address that binds.
func disabledGNMIBlockTokenOnly(t *testing.T) *zeconfig.Tree {
	t.Helper()
	tree := zeconfig.NewTree()
	gnmi := tree.GetOrCreateContainer("environment").GetOrCreateContainer("gnmi")
	gnmi.Set("enabled", "false")
	gnmi.Set("token", "gnmi-s3cret")
	return tree
}

// VALIDATES: the token of an environment.gnmi block reaches the resolver even
// when the block does not say `enabled true`, so an env-started gNMI listener
// authenticates the way the operator's config asked.
// PREVENTS: the boot refusal an operator cannot act on. Reading the token
// through the enable gate discarded it, the resolver fell back to the
// 0.0.0.0:9339 default with an empty token, and checkMgmtListeners refused to
// start while telling the operator to set the very token they had written.
func TestResolveGNMIListenersKeepsTokenFromDisabledBlock(t *testing.T) {
	setGNMIEnv(t, "ze.gnmi.enabled", "1")
	setGNMIEnv(t, "ze.gnmi.listen", "")
	setGNMIEnv(t, "ze.gnmi.token", "")

	addr, token, enabled := resolveGNMIListeners(disabledGNMIBlockTokenOnly(t))

	assert.True(t, enabled, "ze.gnmi.enabled starts gNMI")
	assert.Equal(t, "gnmi-s3cret", token, "the block's token must survive `enabled false`")
	// The block names no server, so the synthesized default binds. With the
	// token honored the guard passes it.
	assert.Equal(t, "0.0.0.0:9339", addr)
	assert.False(t, checkMgmtListeners([]mgmtListener{{
		service:       "gNMI",
		addrs:         []string{addr},
		authenticated: token != "",
	}}), "a gNMI listener whose config named a token must not be refused")
}

// VALIDATES: the address of an environment.gnmi block reaches the resolver even
// when the block does not say `enabled true`, so an env-started gNMI listener
// binds where the operator's config asked.
// PREVENTS: a wildcard bind over a named loopback address. Reading the address
// through the enable gate discarded it, the resolver fell back to the
// 0.0.0.0:9339 default, and a block that named 127.0.0.1 published a full
// config-mutation surface on every interface.
func TestResolveGNMIListenersKeepsAddressFromDisabledBlock(t *testing.T) {
	setGNMIEnv(t, "ze.gnmi.enabled", "1")
	setGNMIEnv(t, "ze.gnmi.listen", "")
	setGNMIEnv(t, "ze.gnmi.token", "")

	addr, token, enabled := resolveGNMIListeners(disabledGNMIBlockWithToken(t))

	assert.True(t, enabled)
	assert.Equal(t, "127.0.0.1:9339", addr, "the block's server must survive `enabled false`")
	assert.Equal(t, "gnmi-s3cret", token)
}

// VALIDATES: a block the operator did not enable, with nothing else asking for
// a listener, starts nothing and names no address. Reading the address as a
// setting must not turn a dormant block into a listener.
func TestResolveGNMIListenersDormantBlockStartsNothing(t *testing.T) {
	setGNMIEnv(t, "ze.gnmi.enabled", "")
	setGNMIEnv(t, "ze.gnmi.listen", "")
	setGNMIEnv(t, "ze.gnmi.token", "")

	addr, token, enabled := resolveGNMIListeners(disabledGNMIBlockWithToken(t))

	assert.False(t, enabled, "`enabled false` starts no gNMI listener")
	assert.Empty(t, addr, "a dormant block supplies no address to bind")
	assert.Equal(t, "gnmi-s3cret", token, "the token is read either way; nothing binds to use it")
}

// VALIDATES: the same token reaches the resolver when ze.gnmi.listen supplies a
// loopback address, and that address wins over the one the block names.
// PREVENTS: an unauthenticated gNMI Set surface -- a full config-mutation API
// (internal/component/gnmi/set.go) -- served over a config that asked for a
// token. The guard cannot catch this one: loopback passes on address alone.
func TestResolveGNMIListenersKeepsTokenWithEnvListenAddress(t *testing.T) {
	setGNMIEnv(t, "ze.gnmi.enabled", "1")
	setGNMIEnv(t, "ze.gnmi.listen", "127.0.0.1:19339")
	setGNMIEnv(t, "ze.gnmi.token", "")

	addr, token, enabled := resolveGNMIListeners(disabledGNMIBlockWithToken(t))

	assert.True(t, enabled)
	assert.Equal(t, "127.0.0.1:19339", addr, "the env address wins over the block's server")
	assert.Equal(t, "gnmi-s3cret", token, "the block's token must gate the env-supplied listener")
}

// VALIDATES: the env token still wins over the config token, and an enabled
// block still supplies its own address. The split must change which question
// each extractor answers, nothing else.
func TestResolveGNMIListenersPrecedenceUnchanged(t *testing.T) {
	setGNMIEnv(t, "ze.gnmi.enabled", "")
	setGNMIEnv(t, "ze.gnmi.listen", "")
	setGNMIEnv(t, "ze.gnmi.token", "env-wins")

	tree := zeconfig.NewTree()
	gnmi := tree.GetOrCreateContainer("environment").GetOrCreateContainer("gnmi")
	gnmi.Set("enabled", "true")
	gnmi.Set("token", "config-loses")
	srv := zeconfig.NewTree()
	srv.Set("ip", "10.0.0.1")
	srv.Set("port", "9339")
	gnmi.AddListEntry("server", "main", srv)

	addr, token, enabled := resolveGNMIListeners(tree)

	assert.True(t, enabled, "an enabled block starts gNMI without any env var")
	assert.Equal(t, "10.0.0.1:9339", addr, "an enabled block supplies its own address")
	assert.Equal(t, "env-wins", token)
}

// VALIDATES: a tree with no gnmi block and no env var starts nothing.
func TestResolveGNMIListenersDisabledByDefault(t *testing.T) {
	setGNMIEnv(t, "ze.gnmi.enabled", "")
	setGNMIEnv(t, "ze.gnmi.listen", "")
	setGNMIEnv(t, "ze.gnmi.token", "")

	addr, token, enabled := resolveGNMIListeners(zeconfig.NewTree())

	assert.False(t, enabled)
	assert.Empty(t, addr)
	assert.Empty(t, token)
}
