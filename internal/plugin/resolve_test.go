package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			input:    "ze.bgp-rr",
			wantType: PluginTypeInternal,
			wantName: "bgp-rr",
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
	known := []string{"bgp-rib", "bgp-gr", "bgp-rr"}

	for _, name := range known {
		t.Run(name, func(t *testing.T) {
			assert.True(t, IsInternalPlugin(name), "plugin %s should be registered", name)
		})
	}

	t.Run("unknown", func(t *testing.T) {
		assert.False(t, IsInternalPlugin("unknown"))
	})
}
