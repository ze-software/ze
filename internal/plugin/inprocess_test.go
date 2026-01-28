package plugin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInternalPluginRunnerRegistry verifies all internal plugins have runners.
//
// VALIDATES: Each internal plugin has a corresponding runner function.
// PREVENTS: Missing runner registrations.
func TestInternalPluginRunnerRegistry(t *testing.T) {
	// All plugins from AvailableInternalPlugins should have runners
	for _, name := range AvailableInternalPlugins() {
		t.Run(name, func(t *testing.T) {
			runner := GetInternalPluginRunner(name)
			require.NotNil(t, runner, "plugin %s should have a runner", name)
		})
	}

	// Unknown plugins should return nil
	t.Run("unknown", func(t *testing.T) {
		runner := GetInternalPluginRunner("unknown")
		assert.Nil(t, runner)
	})
}

// TestGetInternalPluginRunner verifies runner lookup.
//
// VALIDATES: All known plugins return non-nil runners.
// PREVENTS: Missing runner registrations breaking in-process execution.
func TestGetInternalPluginRunner(t *testing.T) {
	tests := []struct {
		name    string
		wantNil bool
	}{
		{"rib", false},
		{"gr", false},
		{"rr", false},
		{"unknown", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := GetInternalPluginRunner(tt.name)
			if tt.wantNil {
				assert.Nil(t, runner)
			} else {
				assert.NotNil(t, runner)
			}
		})
	}
}

// TestDeriveName verifies plugin name derivation from command.
//
// VALIDATES: Name is correctly derived from various command formats.
// PREVENTS: Name collisions or incorrect naming.
func TestDeriveName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ze_bgp_plugin_rib", "ze bgp plugin rib", "rib"},
		{"ze_bgp_plugin_gr", "ze bgp plugin gr", "gr"},
		{"local_path", "./myplugin", "myplugin"},
		{"local_nested", "./path/to/plugin", "plugin"},
		{"absolute_path", "/usr/lib/ze/myplugin", "myplugin"},
		{"single_command", "myplugin", "myplugin"},
		{"with_args", "plugin --flag value", "plugin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.Fields(tt.input)
			got := deriveName(parts)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDeriveNameEdgeCases verifies edge cases in name derivation.
//
// VALIDATES: Edge cases are handled gracefully.
// PREVENTS: Panics or incorrect names for unusual inputs.
func TestDeriveNameEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"ze_bgp_plugin_missing_name", []string{"ze", "bgp", "plugin"}, "ze"},
		{"trailing_slash", []string{"./path/to/"}, "to"}, // filepath.Base of "./path/to/" is "to"
		{"single_dot", []string{"."}, "."},               // Current dir
		{"double_dot", []string{".."}, ".."},             // Parent dir
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveName(tt.parts)
			assert.Equal(t, tt.want, got)
		})
	}
}
