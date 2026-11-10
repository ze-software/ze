package server

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// registerStage1DependencyPlugins registers a provider and a dependent whose
// registry row and Stage 1 declaration both name the provider as a hard
// dependency, the shape bgp-gr has with bgp-rib.
func registerStage1DependencyPlugins(t *testing.T, provider, dependent string) {
	t.Helper()
	registerLifecyclePlugin(t, provider, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(provider, conn)
		if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
			return 1
		}
		return 0
	})
	registerLifecyclePluginWithDeps(t, dependent, []string{provider}, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(dependent, conn)
		if err := p.Run(context.Background(), &sdk.Registration{Dependencies: []string{provider}}); err != nil {
			return 1
		}
		return 0
	})
}

// TestStage1DependencyAcceptsAutoLoadedProvider proves a Stage 1 dependency
// is satisfied by a provider the daemon auto-loaded, not only by one the
// operator wrote in a `plugin { }` block.
//
// VALIDATES: a plugin auto-loaded for a config root, whose dependency the
// registry resolution started beside it, passes its own Stage 1 declaration.
// PREVENTS: `ze start` refusing the web harness's BGP config with
// `plugin "bgp-gr" requires "bgp-rib"`: bgp-gr and bgp-rib are both
// auto-loaded for `bgp { }`, and the Stage 1 check read only the operator's
// plugin list.
// Method: one startup phase holding both plugins, as resolveAutoLoad builds
// it, over a server whose config names no plugin.
func TestStage1DependencyAcceptsAutoLoadedProvider(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const provider = "stage1-dep-provider"
	const dependent = "stage1-dep-dependent"
	registerStage1DependencyPlugins(t, provider, dependent)

	s, _ := newLifecycleStartupServer(t)
	require.False(t, s.hasConfiguredPlugin(provider), "the provider must come from auto-load, not the config")

	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: provider, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: dependent, Internal: true, Encoder: plugin.EncodingJSON},
	})
	require.NoError(t, err)
}

// TestStage1DependencyRefusesAbsentProvider proves the Stage 1 check still
// refuses a dependency nothing configured and nothing started.
//
// VALIDATES: the refusal names the plugin and the dependency it lacks, and,
// since the provider is in this build, the internal block that loads it
// (spec-operator-errors-name-the-fix AC-10), as an ErrMissingDependency.
// PREVENTS: the auto-load acceptance above turning into accepting any name.
// Method: a startup phase holding the dependent alone.
func TestStage1DependencyRefusesAbsentProvider(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const provider = "stage1-absent-provider"
	const dependent = "stage1-absent-dependent"
	registerStage1DependencyPlugins(t, provider, dependent)

	s, _ := newLifecycleStartupServer(t)
	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: dependent, Internal: true, Encoder: plugin.EncodingJSON},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, registry.ErrMissingDependency)
	var missing *registry.MissingDependencyError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, `plugin "stage1-absent-dependent" needs plugin "stage1-absent-provider", and the configuration does not load it`+"\n"+
		`"stage1-absent-dependent" does not work without "stage1-absent-provider", so ze refuses to start it`+"\n"+
		`fix: add 'plugin { internal stage1-absent-provider { use stage1-absent-provider; } }' to the config, `+
		`or remove the configuration that loads "stage1-absent-dependent"`, missing.Error())
	assert.Contains(t, err.Error(), missing.Error())
}
