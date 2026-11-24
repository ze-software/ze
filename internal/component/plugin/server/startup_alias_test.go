// Design: docs/architecture/api/process-protocol.md -- plugin lifecycle helpers keyed by compiled registration identity.
// Related: startup.go -- configRefusalIsFatal and registryRow.
// Related: startup_autoload.go -- collectOrphanedDependencies and appendHeldNeedProviders.
// Related: doctor_snapshot.go -- PluginDoctorState.RegistrationName.
// Related: server.go -- isPluginLoaded and hasConfiguredPlugin.

package server

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAliasedPluginLifecycleHelpers verifies every server-side lifecycle helper
// named in AC-13 resolves compiled plugins by their compiled registration
// identity rather than endpoint alias, and never grants compiled identity to a
// foreign external program sharing the same endpoint name.
//
// VALIDATES: spec-explicit-plugin-and-attach-config AC-13 sibling sites:
//   - configRefusalIsFatal resolves by RegistrationName (true for aliased fatal
//     plugin, false for foreign external program sharing the fatal plugin name);
//   - isPluginLoaded recognizes aliased compiled plugins under their
//     registration name and rejects foreign external programs of the same name;
//   - collectOrphanedDependencies keeps dependencies alive while an aliased
//     dependent runs, discovers orphans when an aliased dependent stops, and
//     ignores a foreign external program sharing the dependent's name;
//   - appendHeldNeedProviders (via selectAutoLoad / isRegistrationRunning)
//     pulls providers for an aliased running owner and ignores a foreign
//     external program sharing the owner's name;
//   - doctorSnapshot populates RegistrationName for running, stopped, and
//     disabled aliased compiled plugins and leaves it empty for foreign
//     external programs.
//
// PREVENTS: endpoint aliases breaking fatal-config enforcement, auto-load
// deduplication, orphan cleanup, conditional provider loading, or doctor
// snapshots, and foreign external programs spoofing compiled registrations.
func TestAliasedPluginLifecycleHelpers(t *testing.T) {
	t.Run("configRefusalIsFatal", func(t *testing.T) {
		snap := registry.Snapshot()
		t.Cleanup(func() { registry.Restore(snap) })

		const fatalReg = "test-alias-fatal-reg"
		require.NoError(t, registry.Register(registry.Registration{
			Name:               fatalReg,
			Description:        "fatal config error test fixture",
			FatalOnConfigError: true,
			RunEngine:          func(net.Conn) int { return 0 },
			CLIHandler:         func([]string) int { return 0 },
		}))

		refusalErr := &rpc.RPCCallError{Code: "config-invalid", Message: "bad setting"}
		transportErr := errors.New("connection reset")

		aliasedInternal := process.NewProcess(plugin.PluginConfig{
			Name:     "my-fatal",
			Internal: true,
			Run:      fatalReg,
		})
		aliasedSeparate := process.NewProcess(plugin.PluginConfig{
			Name:            "my-fatal-sep",
			CompiledPlugin:  fatalReg,
			SeparateProcess: true,
		})
		foreignSameName := process.NewProcess(plugin.PluginConfig{
			Name: fatalReg,
			Run:  "/usr/local/bin/foreign-fatal",
		})

		assert.True(t, configRefusalIsFatal(aliasedInternal.RegistrationName(), refusalErr),
			"aliased internal fatal plugin must trigger fatal refusal")
		assert.True(t, configRefusalIsFatal(aliasedSeparate.RegistrationName(), refusalErr),
			"aliased separate-process fatal plugin must trigger fatal refusal")
		assert.False(t, configRefusalIsFatal(aliasedInternal.RegistrationName(), transportErr),
			"transport error must never be treated as a fatal config refusal")
		assert.False(t, configRefusalIsFatal(foreignSameName.RegistrationName(), refusalErr),
			"foreign external program named after a fatal plugin must not trigger fatal refusal")
	})

	t.Run("isPluginLoaded", func(t *testing.T) {
		const compiledReg = "test-alias-loaded-reg"

		s := sessionReadyTestServer()
		pm := s.procManager.Load()

		// Foreign external sharing the compiled registration's name is NOT
		// treated as that compiled plugin.
		foreign := process.NewProcess(plugin.PluginConfig{
			Name: compiledReg,
			Run:  "/usr/local/bin/foreign-copy",
		})
		require.NoError(t, pm.AddProcess(compiledReg, foreign))
		s.markPluginConfigLoaded(foreign.Config())
		assert.False(t, s.isPluginLoaded(compiledReg),
			"foreign external program named after compiled plugin must not satisfy isPluginLoaded")

		// Aliased compiled plugin (in-daemon or separate-process) IS recognized
		// under its compiled registration identity.
		aliased := process.NewProcess(plugin.PluginConfig{
			Name:            "custom-alias",
			CompiledPlugin:  compiledReg,
			SeparateProcess: true,
		})
		require.NoError(t, pm.AddProcess("custom-alias", aliased))
		s.markPluginConfigLoaded(aliased.Config())
		assert.True(t, s.isPluginLoaded(compiledReg),
			"aliased compiled plugin must satisfy isPluginLoaded under its registration identity")

		// Removing the aliased process and unmarking clears the registration
		// mark even while the foreign external remains.
		pm.RemoveProcess("custom-alias")
		s.unmarkPluginConfigLoaded(aliased.Config())
		assert.False(t, s.isPluginLoaded(compiledReg),
			"removing the aliased compiled plugin must clear isPluginLoaded")
	})

	t.Run("collectOrphanedDependencies", func(t *testing.T) {
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		const (
			depReg    = "test-alias-shared-dep"
			ownerAReg = "test-alias-owner-a"
			ownerBReg = "test-alias-owner-b"
		)
		for _, reg := range []registry.Registration{
			{
				Name:        depReg,
				Description: "shared dependency fixture",
				RunEngine:   func(net.Conn) int { return 0 },
				CLIHandler:  func([]string) int { return 0 },
			},
			{
				Name:         ownerAReg,
				Description:  "dependent A fixture",
				Dependencies: []string{depReg},
				RunEngine:    func(net.Conn) int { return 0 },
				CLIHandler:   func([]string) int { return 0 },
			},
			{
				Name:         ownerBReg,
				Description:  "dependent B fixture",
				Dependencies: []string{depReg},
				RunEngine:    func(net.Conn) int { return 0 },
				CLIHandler:   func([]string) int { return 0 },
			},
		} {
			require.NoError(t, registry.Register(reg))
		}

		// Case 1: aliased dependent B keeps shared dependency alive when A stops.
		s1 := sessionReadyTestServer()
		pm1 := s1.procManager.Load()
		require.NoError(t, pm1.AddProcess(depReg, process.NewProcess(plugin.PluginConfig{Name: depReg, Internal: true})))
		require.NoError(t, pm1.AddProcess("alias-a", process.NewProcess(plugin.PluginConfig{Name: "alias-a", CompiledPlugin: ownerAReg})))
		require.NoError(t, pm1.AddProcess("alias-b", process.NewProcess(plugin.PluginConfig{Name: "alias-b", CompiledPlugin: ownerBReg, SeparateProcess: true})))

		collected := s1.collectOrphanedDependencies(pm1, map[string]bool{"alias-a": true})
		assert.True(t, collected["alias-a"])
		assert.False(t, collected[depReg], "aliased dependent B must keep shared dependency alive")

		// Case 2: stopping the sole aliased dependent discovers its orphaned dependency.
		s2 := sessionReadyTestServer()
		pm2 := s2.procManager.Load()
		require.NoError(t, pm2.AddProcess(depReg, process.NewProcess(plugin.PluginConfig{Name: depReg, Internal: true})))
		require.NoError(t, pm2.AddProcess("alias-a", process.NewProcess(plugin.PluginConfig{Name: "alias-a", CompiledPlugin: ownerAReg})))
		// A foreign external named after ownerBReg must NOT keep depReg alive.
		require.NoError(t, pm2.AddProcess(ownerBReg, process.NewProcess(plugin.PluginConfig{Name: ownerBReg, Run: "/usr/local/bin/foreign-b"})))

		collected = s2.collectOrphanedDependencies(pm2, map[string]bool{"alias-a": true})
		assert.True(t, collected["alias-a"])
		assert.True(t, collected[depReg],
			"stopping aliased dependent must orphan shared dependency even if a foreign external shares another dependent's name")
	})

	t.Run("appendHeldNeedProviders", func(t *testing.T) {
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		const (
			ownerReg    = "test-alias-need-owner"
			providerReg = "test-alias-need-provider"
		)
		require.NoError(t, registry.Register(registry.Registration{
			Name:                 ownerReg,
			Description:          "owner with conditional need",
			OptionalDependencies: []string{providerReg},
			ConfigNeeds: []registry.ConfigNeed{{
				Path:      []string{"need-root", "enable"},
				Value:     "true",
				Providers: []string{providerReg},
			}},
			RunEngine:  func(net.Conn) int { return 0 },
			CLIHandler: func([]string) int { return 0 },
		}))
		require.NoError(t, registry.Register(registry.Registration{
			Name:        providerReg,
			Description: "optional provider fixture",
			RunEngine:   func(net.Conn) int { return 0 },
			CLIHandler:  func([]string) int { return 0 },
		}))

		tree := map[string]any{"need-root": map[string]any{"enable": "true"}}
		needHolds := config.ConfigNeedHolds(tree)

		// Aliased compiled owner running in pm holds its need and pulls providerReg.
		sAliased := sessionReadyTestServer()
		pmAliased := sAliased.procManager.Load()
		require.NoError(t, pmAliased.AddProcess("my-owner", process.NewProcess(plugin.PluginConfig{
			Name:            "my-owner",
			CompiledPlugin:  ownerReg,
			SeparateProcess: true,
		})))
		needed := appendHeldNeedProviders(nil, map[string]bool{}, needHolds, func(name string) bool {
			return sAliased.isRegistrationRunning(pmAliased, name)
		})
		assert.Equal(t, []string{providerReg}, needed,
			"aliased running compiled owner must hold its need and pull its provider")

		// Foreign external named after ownerReg does NOT hold ownerReg's compiled need.
		sForeign := sessionReadyTestServer()
		pmForeign := sForeign.procManager.Load()
		require.NoError(t, pmForeign.AddProcess(ownerReg, process.NewProcess(plugin.PluginConfig{
			Name: ownerReg,
			Run:  "/usr/local/bin/foreign-owner",
		})))
		neededForeign := appendHeldNeedProviders(nil, map[string]bool{}, needHolds, func(name string) bool {
			return sForeign.isRegistrationRunning(pmForeign, name)
		})
		assert.Empty(t, neededForeign,
			"foreign external named after compiled owner must not pull compiled owner's need provider")
	})

	t.Run("doctorSnapshotRegistrationName", func(t *testing.T) {
		const (
			runningAlias = "my-running-ddos"
			stoppedAlias = "my-stopped-ddos"
			foreignName  = "ddos-flowspec"
			compiledReg  = "ddos-flowspec"
		)

		configs := []plugin.PluginConfig{
			{Name: runningAlias, CompiledPlugin: compiledReg, SeparateProcess: true},
			{Name: stoppedAlias, Internal: true, Run: compiledReg},
			{Name: foreignName, Run: "/usr/local/bin/fake-ddos"},
		}
		pm := process.NewProcessManager(configs)
		runningProc := process.NewProcess(configs[0])
		runningProc.SetRunning(true)
		require.NoError(t, pm.AddProcess(runningAlias, runningProc))
		// stoppedAlias is configured in pm.configs with no live process (stopped).
		pm.RecordStopped(stoppedAlias, process.PluginStop{Cause: process.StopPolicyExit, Detail: "exited"})
		foreignProc := process.NewProcess(configs[2])
		foreignProc.SetRunning(true)
		require.NoError(t, pm.AddProcess(foreignName, foreignProc))

		s := &Server{
			config:        &ServerConfig{Plugins: configs},
			registry:      plugin.NewPluginRegistry(),
			loadedPlugins: make(map[string]bool),
		}
		s.procManager.Store(pm)

		snapshot, err := s.doctorSnapshot()
		require.NoError(t, err)

		byName := make(map[string]PluginDoctorState, len(snapshot.Plugins))
		for _, p := range snapshot.Plugins {
			byName[p.Name] = p
		}

		require.Contains(t, byName, runningAlias)
		assert.Equal(t, compiledReg, byName[runningAlias].RegistrationName,
			"running aliased compiled plugin must report its compiled RegistrationName")
		assert.True(t, byName[runningAlias].Running)

		require.Contains(t, byName, stoppedAlias)
		assert.Equal(t, compiledReg, byName[stoppedAlias].RegistrationName,
			"stopped aliased compiled plugin must still report its compiled RegistrationName")
		assert.False(t, byName[stoppedAlias].HasProcess)
		assert.True(t, byName[stoppedAlias].Down)

		require.Contains(t, byName, foreignName)
		assert.Empty(t, byName[foreignName].RegistrationName,
			"foreign external program named after a compiled plugin must report empty RegistrationName")
	})
}
