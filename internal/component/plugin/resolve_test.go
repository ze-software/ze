package plugin

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestResolvePlugin verifies plugin string resolution.
//
// VALIDATES: Plugin strings are correctly categorized and resolved.
// PREVENTS: Incorrect routing of internal vs external plugins.
func TestResolvePlugin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType PluginType
		wantName string
		wantCmd  []string
		wantErr  bool
	}{
		// Internal plugins (ze.X)
		{
			name:     "internal_rib",
			input:    "ze.bgp-rib",
			wantType: PluginTypeInternal,
			wantName: "bgp-rib",
		},
		{
			name:     "internal_gr",
			input:    "ze.bgp-gr",
			wantType: PluginTypeInternal,
			wantName: "bgp-gr",
		},
		{
			name:     "internal_rr",
			input:    "ze.bgp-rs",
			wantType: PluginTypeInternal,
			wantName: "bgp-rs",
		},
		// Local path (./path)
		{
			name:     "local_path",
			input:    "./myplugin",
			wantType: PluginTypeExternal,
			wantName: "myplugin",
			wantCmd:  []string{"./myplugin"},
		},
		{
			name:     "local_path_nested",
			input:    "./path/to/plugin",
			wantType: PluginTypeExternal,
			wantName: "plugin",
			wantCmd:  []string{"./path/to/plugin"},
		},
		// Absolute path (/path)
		{
			name:     "absolute_path",
			input:    "/usr/lib/ze/myplugin",
			wantType: PluginTypeExternal,
			wantName: "myplugin",
			wantCmd:  []string{"/usr/lib/ze/myplugin"},
		},
		// Command with args
		{
			name:     "command_with_args",
			input:    "ze plugin rib",
			wantType: PluginTypeExternal,
			wantName: "rib",
			wantCmd:  []string{"ze", "plugin", "rib"},
		},
		{
			name:     "command_single",
			input:    "myplugin",
			wantType: PluginTypeExternal,
			wantName: "myplugin",
			wantCmd:  []string{"myplugin"},
		},
		// Auto discovery
		{
			name:     "auto",
			input:    "auto",
			wantType: PluginTypeAuto,
		},
		// Errors
		{
			name:    "empty_string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "internal_unknown",
			input:   "ze.unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := ResolvePlugin(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, resolved.Type)
			if tt.wantName != "" {
				assert.Equal(t, tt.wantName, resolved.Name)
			}
			if tt.wantCmd != nil {
				assert.Equal(t, tt.wantCmd, resolved.Command)
			}
		})
	}
}

// TestResolvePluginBoundary verifies boundary conditions.
//
// VALIDATES: Plugin name length limits.
// PREVENTS: Buffer overflow or excessive memory usage.
func TestResolvePluginBoundary(t *testing.T) {
	// 64 char name - last valid
	name64 := "ze." + string(make([]byte, 61)) // ze. + 61 = 64 total, but we check the name part
	for i := 3; i < 64; i++ {
		name64 = name64[:i] + "a" + name64[i+1:]
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "max_valid_length",
			input:   "./plugin_" + string(repeatByte('a', 54)), // 64 total
			wantErr: false,
		},
		{
			name:    "empty_invalid",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolvePlugin(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func repeatByte(b byte, n int) []byte {
	result := make([]byte, n)
	for i := range result {
		result[i] = b
	}
	return result
}

// TestInternalPluginRegistry verifies the internal plugin registry.
//
// VALIDATES: All known internal plugins are registered.
// PREVENTS: Missing plugin registrations.
func TestInternalPluginRegistry(t *testing.T) {
	known := []string{"bgp-rib", "bgp-gr", "bgp-rs"}

	for _, name := range known {
		t.Run(name, func(t *testing.T) {
			assert.True(t, IsInternalPlugin(name), "plugin %s should be registered", name)
		})
	}

	t.Run("unknown", func(t *testing.T) {
		assert.False(t, IsInternalPlugin("unknown"))
	})
}

// TestRegistryNames pins the one rule that answers which registry row a process
// configuration names.
//
// VALIDATES: every legal spelling of an in-tree implementation resolves to that
// implementation, and an external command line leaves the process name as the
// answer.
// PREVENTS: a caller growing its own reading of PluginConfig.Run. Four of them
// had one before this function existed, and they disagreed about the `ze.`
// prefix: the session-ready barrier answered "declares nothing" for
// `run ze.bgp-rib`, which let a peer's End-of-RIB claim an initial routing
// update the plugin had not finished (RFC 4724 Section 4).
func TestRegistryNames(t *testing.T) {
	// A throwaway registration keeps every case independent of the feature tags
	// this binary was built with. The registry is restored when the test ends.
	const implementation = "test-registry-names-implementation"
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:        implementation,
		Description: "registry-name resolution test fixture",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	tests := []struct {
		name      string
		config    PluginConfig
		wantNames []string
		wantName  string
	}{
		{
			name:      "bare_registry_name",
			config:    PluginConfig{Name: implementation},
			wantNames: []string{implementation},
			wantName:  implementation,
		},
		{
			name:      "use_spelling_under_an_alias",
			config:    PluginConfig{Name: "ribout", Run: implementation},
			wantNames: []string{implementation, "ribout"},
			wantName:  implementation,
		},
		{
			name:      "ze_dot_spelling_under_an_alias",
			config:    PluginConfig{Name: "ribout", Run: "ze." + implementation},
			wantNames: []string{implementation, "ribout"},
			wantName:  implementation,
		},
		{
			name:      "ze_plugin_verb_pair",
			config:    PluginConfig{Name: "ribout", Run: "ze plugin " + implementation},
			wantNames: []string{implementation, "ribout"},
			wantName:  implementation,
		},
		{
			name:      "external_command_line",
			config:    PluginConfig{Name: "watcher", Run: "/usr/bin/watcher --verbose"},
			wantNames: []string{"watcher"},
			wantName:  "watcher",
		},
		{
			// The registry holds no row for it, so the process name is the answer
			// and startInternal reports "unknown internal plugin" rather than
			// starting something else.
			name:      "ze_dot_spelling_of_no_registered_plugin",
			config:    PluginConfig{Name: "ribout", Run: "ze.not-a-plugin"},
			wantNames: []string{"ribout"},
			wantName:  "ribout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantNames, RegistryNames(tt.config))
			assert.Equal(t, tt.wantName, RegistryName(tt.config))
		})
	}
}
