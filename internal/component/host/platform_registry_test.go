// Design: docs/features/ai-first.md -- platform name registration for doctor check validation
// VALIDATES: all platform type names are registered with the plugin registry
// for doctor check platform validation.
// PREVENTS: plugins registering doctor checks with valid platform names being
// rejected because the registry does not know the platform.

package host

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestPlatformNamesRegisteredForDoctorValidation(t *testing.T) {
	t.Cleanup(func() { registry.Restore(registry.Snapshot()) })

	for _, name := range platformTypeNames {
		registry.Reset()
		err := registry.Register(registry.Registration{
			Name:        "plat-test",
			Description: "test",
			RunEngine:   func(net.Conn) int { return 0 },
			CLIHandler:  func([]string) int { return 0 },
			DoctorChecks: []registry.DoctorCheckDef{{
				Name:         "plat-check",
				Phase:        rpc.DoctorPhasePostConfig,
				Order:        1,
				Dependencies: []string{"test"},
				Platforms:    []string{name},
				Codes:        []string{"doctor-test"},
				Check:        func(registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic { return nil },
			}},
		})
		require.NoError(t, err, "platform %q should be accepted by doctor check validation", name)
	}
}
