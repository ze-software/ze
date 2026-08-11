// Design: ai/rules/plugins.md -- ze_rest / ze_grpc compile-out seam
//
// resolveAPIListeners is always-on: the boot-time management-listener guard
// reads it before anything binds, and the gated builders consume the same
// struct. These tests pin the two questions it asks the config tree apart.

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
)

// clearAPIEnv blanks every ze.api-server.* variable this resolver reads and
// restores it afterwards. env.Get answers from a process-wide cache, so a
// leaked value would change every later test in this package.
func clearAPIEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ze.api-server.rest.enabled",
		"ze.api-server.rest.listen",
		"ze.api-server.grpc.enabled",
		"ze.api-server.grpc.listen",
	} {
		orig := env.Get(key)
		t.Cleanup(func() { _ = env.Set(key, orig) })
		require.NoError(t, env.Set(key, ""))
	}
}

// dormantAPIBlock builds the config an operator writes when the transport comes
// from the environment: the block carries the token, the listen address and the
// gRPC TLS pair, and no `enabled true` because config itself must not start the
// API.
func dormantAPIBlock(t *testing.T, withServers bool) *zeconfig.Tree {
	t.Helper()
	tree := zeconfig.NewTree()
	api := tree.GetOrCreateContainer("environment").GetOrCreateContainer("api-server")
	api.Set("token", "api-s3cret")

	rest := api.GetOrCreateContainer("rest")
	grpc := api.GetOrCreateContainer("grpc")
	grpc.Set("tls-cert", "/etc/ze/grpc.pem")
	grpc.Set("tls-key", "/etc/ze/grpc.key")
	if withServers {
		restSrv := zeconfig.NewTree()
		restSrv.Set("ip", "127.0.0.1")
		restSrv.Set("port", "18095")
		rest.AddListEntry("server", "main", restSrv)

		grpcSrv := zeconfig.NewTree()
		grpcSrv.Set("ip", "127.0.0.1")
		grpcSrv.Set("port", "50052")
		grpc.AddListEntry("server", "main", grpcSrv)
	}
	return tree
}

// VALIDATES: the token and the listen address of an environment.api-server
// block reach the resolver even when no transport says `enabled true`, so an
// env-started REST listener authenticates and binds where the config asked.
// PREVENTS: the boot refusal an operator cannot act on. Reading the settings
// through the enable gate discarded them, the resolver fell back to
// 0.0.0.0:8081 with an empty token, and checkMgmtListeners refused to start
// while telling the operator to set the very token they had written.
func TestResolveAPIListenersKeepsSettingsFromDormantBlock(t *testing.T) {
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.rest.enabled", "1"))

	cfg, ok, err := resolveAPIListeners(dormantAPIBlock(t, true))

	require.NoError(t, err)
	assert.True(t, ok, "ze.api-server.rest.enabled starts REST")
	assert.True(t, cfg.RESTOn)
	assert.False(t, cfg.GRPCOn, "an env var for one transport must not start the other")
	assert.Equal(t, "api-s3cret", cfg.Token, "the block's token must survive the enable gate")
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "127.0.0.1:18095", cfg.REST[0].Listen(), "the block's server must survive the enable gate")
	assert.False(t, checkMgmtListeners([]mgmtListener{{
		service:       "API",
		addrs:         apiGuardAddrs(cfg),
		authenticated: cfg.Token != "",
	}}), "an API listener whose config named a token must not be refused")
}

// VALIDATES: the gRPC TLS pair of a dormant block reaches the resolver, so an
// env-started gRPC transport serves the operator's certificate.
// PREVENTS: management gRPC in CLEAR. tls-cert and tls-key were read inside the
// transport's `enabled true` branch while the token was read above it, so
// ze.api-server.grpc.enabled produced an authenticated server with no
// transport encryption -- the bearer token crossing the wire in plaintext
// (plan/journal/enabled-gate-discards-settings.md).
func TestResolveAPIListenersKeepsGRPCTLSFromDormantBlock(t *testing.T) {
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.grpc.enabled", "1"))

	cfg, ok, err := resolveAPIListeners(dormantAPIBlock(t, true))

	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, cfg.GRPCOn)
	assert.Equal(t, "/etc/ze/grpc.pem", cfg.GRPCTLSCert, "the block's TLS certificate must reach the gRPC builder")
	assert.Equal(t, "/etc/ze/grpc.key", cfg.GRPCTLSKey)
	require.Len(t, cfg.GRPC, 1)
	assert.Equal(t, "127.0.0.1:50052", cfg.GRPC[0].Listen())
}

// VALIDATES: a block the operator did not enable, with no env var asking for a
// transport, starts nothing and declares no address. Reading the address as a
// setting must not turn a dormant transport into a listener.
func TestResolveAPIListenersDormantBlockStartsNothing(t *testing.T) {
	clearAPIEnv(t)

	cfg, ok, err := resolveAPIListeners(dormantAPIBlock(t, true))

	require.NoError(t, err)
	assert.False(t, ok, "a block with no enabled leaf starts no API listener")
	assert.False(t, cfg.RESTOn)
	assert.False(t, cfg.GRPCOn)
	assert.Empty(t, apiGuardAddrs(cfg), "a dormant transport declares no address to the guard")
	assert.Equal(t, "api-s3cret", cfg.Token, "the token is read either way; nothing binds to use it")
}

// VALIDATES: the 0.0.0.0 defaults are unchanged for a block that names no
// server, which is what makes reading the address outside the enable gate a
// strict narrowing: the settings path can only ever supply an address the
// operator wrote.
func TestResolveAPIListenersDefaultsWhenBlockNamesNoServer(t *testing.T) {
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.rest.enabled", "1"))
	require.NoError(t, env.Set("ze.api-server.grpc.enabled", "1"))

	cfg, ok, err := resolveAPIListeners(dormantAPIBlock(t, false))

	require.NoError(t, err)
	assert.True(t, ok)
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "0.0.0.0:8081", cfg.REST[0].Listen())
	require.Len(t, cfg.GRPC, 1)
	assert.Equal(t, "0.0.0.0:50051", cfg.GRPC[0].Listen())
}

// VALIDATES: precedence is unchanged. The listen env var still replaces the
// config list, an enabled transport still supplies its own address, and the
// config token still wins over ze.api-server.token (applied by the caller).
func TestResolveAPIListenersEnvListenWins(t *testing.T) {
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.rest.enabled", "1"))
	require.NoError(t, env.Set("ze.api-server.rest.listen", "127.0.0.1:18096"))

	cfg, ok, err := resolveAPIListeners(dormantAPIBlock(t, true))

	require.NoError(t, err)
	assert.True(t, ok)
	require.Len(t, cfg.REST, 1)
	assert.Equal(t, "127.0.0.1:18096", cfg.REST[0].Listen(), "the env address wins over the block's server")
}

// VALIDATES: an unparseable listen address is a startup error, never a silent
// fallback to a wildcard default.
func TestResolveAPIListenersRejectsBadListen(t *testing.T) {
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.rest.enabled", "1"))
	require.NoError(t, env.Set("ze.api-server.rest.listen", "no-port-here"))

	_, ok, err := resolveAPIListeners(dormantAPIBlock(t, true))

	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "ze.api-server.rest.listen", "the error must name the registered env key, so a grep for it finds the entry")
}

// VALIDATES: a tree with no api-server block and no env var starts nothing.
func TestResolveAPIListenersDisabledByDefault(t *testing.T) {
	clearAPIEnv(t)

	cfg, ok, err := resolveAPIListeners(zeconfig.NewTree())

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, cfg.Token)
	assert.Empty(t, apiGuardAddrs(cfg))
}
