// Design: docs/architecture/api/process-protocol.md -- separate-process execution for compiled-in plugins.
// Related: process.go -- buildSeparateProcessCmd, startSeparateProcess, stopExitGrace.
// Related: manager.go -- RegistrationFor, ProcessServing, BlocksServing.

package process

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestSeparateProcessExecutionContract verifies the separate-process transport
// contract for compiled-in plugins.
//
// VALIDATES: spec-explicit-plugin-and-attach-config S-2 / AC-13:
//   - a registration declaring RunsInProcessOnly: true is refused with an
//     operator error naming the registration and its in-daemon requirement;
//   - an unknown compiled registration is refused naming the registration;
//   - a forkable compiled registration constructs a direct child command via
//     os.Executable() with args ["plugin", regName] and no shell invocation;
//   - stopExitGrace follows transport (RunsInDaemon), so a SeparateProcess
//     compiled plugin receives the forked exit grace rather than 0;
//   - RegistrationFor returns the compiled registration identity for running
//     and stopped/disabled endpoints and empty string for foreign external
//     programs, while ProcessServing("") and BlocksServing("") return nil.
//
// PREVENTS: separate-process compiled plugins invoking a shell, silently
// running RunsInProcessOnly plugins out-of-process, receiving the in-daemon
// zero exit grace, or foreign external programs matching empty registrations.
func TestSeparateProcessExecutionContract(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	const (
		inProcessOnlyReg = "test-sep-in-process-only"
		forkableReg      = "test-sep-forkable"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:              inProcessOnlyReg,
		Description:       "in-process-only test fixture",
		RunsInProcessOnly: true,
		RunEngine:         func(net.Conn) int { return 0 },
		CLIHandler:        func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        forkableReg,
		Description: "forkable compiled plugin test fixture",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	t.Run("refuses-in-process-only", func(t *testing.T) {
		proc := NewProcess(plugin.PluginConfig{
			Name:            "alias-inproc",
			CompiledPlugin:  inProcessOnlyReg,
			SeparateProcess: true,
		})
		cmd, err := proc.buildSeparateProcessCmd(context.Background())
		require.Nil(t, cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), inProcessOnlyReg)
		assert.Contains(t, err.Error(), "RunsInProcessOnly")
		assert.Contains(t, err.Error(), "in-daemon")
	})

	t.Run("refuses-unknown-registration", func(t *testing.T) {
		const unknownReg = "test-sep-does-not-exist"
		proc := NewProcess(plugin.PluginConfig{
			Name:            "alias-unknown",
			CompiledPlugin:  unknownReg,
			SeparateProcess: true,
		})
		cmd, err := proc.buildSeparateProcessCmd(context.Background())
		require.Nil(t, cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), unknownReg)
	})

	t.Run("constructs-direct-ze-exec-without-shell", func(t *testing.T) {
		exe, err := os.Executable()
		require.NoError(t, err)

		proc := NewProcess(plugin.PluginConfig{
			Name:            "alias-forkable",
			CompiledPlugin:  forkableReg,
			SeparateProcess: true,
			WorkDir:         "/tmp",
		})
		assert.Equal(t, forkableReg, proc.RegistrationName())

		cmd, err := proc.buildSeparateProcessCmd(context.Background())
		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, exe, cmd.Path)
		assert.Equal(t, []string{exe, "plugin", forkableReg}, cmd.Args)
		assert.Equal(t, "/tmp", cmd.Dir)
		assert.NotEqual(t, plugin.Shell, cmd.Path, "separate process must exec ze directly, never a shell")
	})

	t.Run("stopExitGrace-follows-transport", func(t *testing.T) {
		inDaemon := NewProcess(plugin.PluginConfig{
			Name:           "in-daemon",
			CompiledPlugin: forkableReg,
		})
		separate := NewProcess(plugin.PluginConfig{
			Name:            "separate",
			CompiledPlugin:  forkableReg,
			SeparateProcess: true,
		})
		foreign := NewProcess(plugin.PluginConfig{
			Name: "foreign",
			Run:  "/usr/local/bin/foreign",
		})

		assert.Zero(t, inDaemon.stopExitGrace(), "in-daemon compiled plugin must have zero exit grace")
		assert.Equal(t, foreign.stopExitGrace(), separate.stopExitGrace(),
			"separate-process compiled plugin must share forked exit grace with external processes")
		if stopGivesExitGrace {
			assert.Equal(t, pluginExitGrace, separate.stopExitGrace())
		}
	})

	t.Run("manager-registration-and-empty-guards", func(t *testing.T) {
		configs := []plugin.PluginConfig{
			{Name: "running-alias", CompiledPlugin: forkableReg, SeparateProcess: true},
			{Name: "stopped-alias", Internal: true, Run: forkableReg},
			{Name: forkableReg, Run: "/usr/local/bin/foreign-copy"},
		}
		pm := NewProcessManager(configs)
		require.NoError(t, pm.AddProcess("running-alias", NewProcess(configs[0])))
		require.NoError(t, pm.AddProcess(forkableReg, NewProcess(configs[2])))

		assert.Equal(t, forkableReg, pm.RegistrationFor("running-alias"))
		assert.Equal(t, forkableReg, pm.RegistrationFor("stopped-alias"),
			"RegistrationFor must resolve stopped/disabled configured endpoints")
		assert.Empty(t, pm.RegistrationFor(forkableReg),
			"RegistrationFor must return empty string for foreign external programs")
		assert.Empty(t, pm.RegistrationFor("unknown-endpoint"))

		serving, err := pm.ProcessServing("")
		require.NoError(t, err)
		assert.Nil(t, serving, "ProcessServing must guard empty registration name")
		assert.Nil(t, pm.BlocksServing(""), "BlocksServing must guard empty registration name")

		assert.Equal(t, []string{"running-alias"}, pm.BlocksServing(forkableReg),
			"BlocksServing must return only the compiled endpoint and ignore foreign external of same name")
	})
}
