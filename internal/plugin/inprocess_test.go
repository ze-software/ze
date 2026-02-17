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
		{"bgp-rib", false},
		{"bgp-gr", false},
		{"bgp-rr", false},
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
		{"ze_plugin_rib", "ze plugin rib", "rib"},
		{"ze_plugin_gr", "ze plugin gr", "gr"},
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
		{"ze_plugin_missing_name", []string{"ze", "plugin"}, "ze"},
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

// TestGetPluginForFamily verifies family-to-plugin mapping.
//
// VALIDATES: Each family maps to the correct internal plugin.
// PREVENTS: Families not auto-loading their plugins.
func TestGetPluginForFamily(t *testing.T) {
	tests := []struct {
		family string
		want   string
	}{
		// FlowSpec families
		{"ipv4/flow", "bgp-flowspec"},
		{"ipv6/flow", "bgp-flowspec"},
		{"ipv4/flow-vpn", "bgp-flowspec"},
		{"ipv6/flow-vpn", "bgp-flowspec"},
		// EVPN family
		{"l2vpn/evpn", "bgp-evpn"},
		// Unknown families
		{"ipv4/unicast", ""},
		{"ipv6/unicast", ""},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.family, func(t *testing.T) {
			got := GetPluginForFamily(tt.family)
			assert.Equal(t, tt.want, got, "family %q should map to plugin %q", tt.family, tt.want)
		})
	}
}

// TestGetRequiredPlugins verifies plugin list generation from families.
//
// VALIDATES: GetRequiredPlugins returns correct plugins for families.
// PREVENTS: Missing plugin auto-loading when families are configured.
func TestGetRequiredPlugins(t *testing.T) {
	tests := []struct {
		name     string
		families []string
		want     []string
	}{
		{"empty", []string{}, nil},
		{"evpn_only", []string{"l2vpn/evpn"}, []string{"bgp-evpn"}},
		{"flowspec_only", []string{"ipv4/flow"}, []string{"bgp-flowspec"}},
		{"evpn_and_flowspec", []string{"l2vpn/evpn", "ipv4/flow"}, []string{"bgp-evpn", "bgp-flowspec"}},
		{"flowspec_dedupe", []string{"ipv4/flow", "ipv6/flow"}, []string{"bgp-flowspec"}},
		{"unknown_ignored", []string{"ipv4/unicast", "l2vpn/evpn"}, []string{"bgp-evpn"}},
		{"all_unknown", []string{"ipv4/unicast", "ipv6/unicast"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRequiredPlugins(tt.families)
			assert.Equal(t, tt.want, got)
		})
	}
}
