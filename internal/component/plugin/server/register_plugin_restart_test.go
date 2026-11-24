// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: register_plugin_restart.go -- handlePluginRestart, completePluginRestart
package server

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRestartRefusesPluginAReloadRemoved verifies that once a reload removal is
// accepted, `request plugin restart <name>` refuses the removed plugin and its
// completion no longer offers the name.
//
// VALIDATES: spec-fib-dummy AC-17, R-9 -- after finishRemovalScope(scope, true)
// makes a reload removal final, handlePluginRestart refuses the removed name as
// not configured, starts no process for it, and completePluginRestart omits it.
// PREVENTS: an operator restarting a plugin (such as a displaced data-plane
// writer) that a committed reload removed.
func TestRestartRefusesPluginAReloadRemoved(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })

	const (
		keptName    = "restart-refused-kept"
		removedName = "restart-refused-removed"
		removedRoot = "restart-refused-root"
	)
	registerSimple := func(name string, roots []string) {
		require.NoError(t, registry.Register(registry.Registration{
			Name:        name,
			Description: "restart refusal after final reload removal test plugin",
			ConfigRoots: roots,
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
	}
	registerSimple(keptName, nil)
	registerSimple(removedName, []string{removedRoot})

	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, spawner := newLifecycleStartupServer(t)
	s.SetShutdownFunc(func() {})
	s.reactor = &removalRecoveryReactor{tree: map[string]any{removedRoot: map[string]any{"enabled": true}}}

	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: keptName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: removedName, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	s.dispatcher.Registry().Freeze()

	completionOffers := func(name string) bool {
		for _, candidate := range completePluginRestart(s, nil) {
			if candidate.Value == name {
				return true
			}
		}
		return false
	}

	// Control: before the reload removes it, completion offers the plugin.
	require.True(t, completionOffers(removedName))
	require.True(t, completionOffers(keptName))

	reloadCtx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false) //nolint:errcheck // Cleanup; accepted below.
	require.NoError(t, s.ReloadConfig(reloadCtx, map[string]any{}))
	require.NoError(t, finish(true))

	require.NotContains(t, spawner.pm.ConfiguredNames(), removedName)
	require.False(t, completionOffers(removedName),
		"completion still offered a plugin whose reload removal became final")
	require.True(t, completionOffers(keptName),
		"completion dropped a plugin that is still configured")

	resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", removedName))
	require.NotEqual(t, plugin.StatusDone, resp.Status,
		"request plugin restart accepted a plugin whose reload removal became final")
	require.Contains(t, resp.Error, "is not a configured plugin")
	require.Nil(t, spawner.pm.GetProcess(removedName),
		"the refused restart started a process for the removed plugin")
}
