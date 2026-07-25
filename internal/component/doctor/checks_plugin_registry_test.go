// Design: docs/features/ai-first.md -- plugin doctor check bridge tests
// VALIDATES: plugin doctor checks declared via Registration.DoctorChecks
// are bridged to the doctor runner and executed at the correct phase.
// PREVENTS: plugin checks registered through the new field being silently ignored.

package doctor

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func stubEngine(_ net.Conn) int { return 0 }
func stubCLI(_ []string) int    { return 0 }

func TestRunPluginRegistryChecks_PostConfig(t *testing.T) {
	t.Cleanup(func() { registry.Restore(registry.Snapshot()) })
	snap := registry.Snapshot()
	registry.Reset()

	called := false
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-bridge",
		Description: "bridge test",
		RunEngine:   stubEngine,
		CLIHandler:  stubCLI,
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "bridge-check",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        900,
			Dependencies: []string{"config-loaded"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-bridge-test"},
			Check: func(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
				called = true
				_, ok := ctx.Tree.(*config.Tree)
				if !ok {
					return nil
				}
				return []rpc.DoctorCheckDiagnostic{{
					Code:     "doctor-bridge-test",
					Severity: "warning",
					Message:  "bridge test fired",
				}}
			},
		}},
	}))

	ctx := doctorCheckContext{
		Tree:      config.NewTree(),
		ConfigDir: t.TempDir(),
		Platform:  &host.PlatformInfo{Type: host.PlatformDarwin},
	}
	diags := runPluginRegistryChecks(doctorCheckPhasePostConfig, ctx)

	assert.True(t, called, "plugin registry check was not called")
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-bridge-test", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Equal(t, "bridge test fired", diags[0].Message)
	registry.Restore(snap)
}

func TestRunPluginRegistryChecks_PhaseFiltering(t *testing.T) {
	t.Cleanup(func() { registry.Restore(registry.Snapshot()) })
	snap := registry.Snapshot()
	registry.Reset()

	called := false
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-phase-filter",
		Description: "phase filter test",
		RunEngine:   stubEngine,
		CLIHandler:  stubCLI,
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "post-only-check",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        100,
			Dependencies: []string{"config-loaded"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-phase-test"},
			Check: func(registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
				called = true
				return nil
			},
		}},
	}))

	ctx := doctorCheckContext{
		Tree:     config.NewTree(),
		Platform: &host.PlatformInfo{Type: host.PlatformDarwin},
	}
	_ = runPluginRegistryChecks(doctorCheckPhasePreConfig, ctx)
	assert.False(t, called, "post-config check should not run in pre-config phase")

	_ = runPluginRegistryChecks(doctorCheckPhasePostConfig, ctx)
	assert.True(t, called, "post-config check should run in post-config phase")
	registry.Restore(snap)
}

func TestRunPluginRegistryChecks_PlatformFiltering(t *testing.T) {
	t.Cleanup(func() { registry.Restore(registry.Snapshot()) })
	snap := registry.Snapshot()
	registry.Reset()

	called := false
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-platform-filter",
		Description: "platform filter test",
		RunEngine:   stubEngine,
		CLIHandler:  stubCLI,
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "linux-only-check",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        100,
			Dependencies: []string{"kernel"},
			Platforms:    []string{"plain-linux"},
			Codes:        []string{"doctor-platform-test"},
			Check: func(registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
				called = true
				return nil
			},
		}},
	}))

	ctx := doctorCheckContext{
		Tree:     config.NewTree(),
		Platform: &host.PlatformInfo{Type: host.PlatformDarwin},
	}
	_ = runPluginRegistryChecks(doctorCheckPhasePostConfig, ctx)
	assert.False(t, called, "linux-only check should not run on darwin")
	registry.Restore(snap)
}

func TestRunPluginRegistryChecks_Empty(t *testing.T) {
	t.Cleanup(func() { registry.Restore(registry.Snapshot()) })
	snap := registry.Snapshot()
	registry.Reset()

	ctx := doctorCheckContext{Tree: config.NewTree()}
	diags := runPluginRegistryChecks(doctorCheckPhasePostConfig, ctx)
	assert.Empty(t, diags)
	registry.Restore(snap)
}
