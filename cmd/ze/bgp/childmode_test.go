package bgp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestChildModeDetection verifies child mode detection logic.
//
// VALIDATES: Child mode detected via --child flag.
// PREVENTS: Incorrect mode detection.
func TestChildModeDetection(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envChild string
		wantMode bool
	}{
		{
			name:     "flag_explicit",
			args:     []string{"--child"},
			wantMode: true,
		},
		{
			name:     "env_var",
			args:     []string{},
			envChild: "1",
			wantMode: true,
		},
		{
			name:     "no_flag_no_env",
			args:     []string{},
			wantMode: false,
		},
		{
			name:     "config_file_standalone",
			args:     []string{"config.conf"},
			wantMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			// Set env var if specified
			if tt.envChild != "" {
				t.Setenv("ZE_CHILD_MODE", tt.envChild)
				env.ResetCache()
			}

			got := isChildMode(tt.args)
			assert.Equal(t, tt.wantMode, got)
		})
	}
}

// TestChildModeDeclare verifies child mode Stage 1 declaration.
//
// VALIDATES: BGP declares schema and handlers correctly.
// PREVENTS: Malformed protocol output.
func TestChildModeDeclare(t *testing.T) {
	var buf bytes.Buffer
	err := writeDeclare(&buf)
	require.NoError(t, err)

	output := buf.String()

	// Must contain required declarations
	assert.Contains(t, output, "declare module ze-bgp-conf")
	assert.Contains(t, output, "declare handler bgp")
	assert.Contains(t, output, "declare priority 100")
	assert.Contains(t, output, "declare done")
}

// TestChildModeCapability verifies child mode Stage 3 capability.
//
// VALIDATES: BGP reports capabilities correctly.
// PREVENTS: Missing capabilities in protocol.
func TestChildModeCapability(t *testing.T) {
	var buf bytes.Buffer
	err := writeCapability(&buf)
	require.NoError(t, err)

	output := buf.String()

	// Must contain capability declarations
	assert.Contains(t, output, "capability done")
}

// TestChildModeReady verifies child mode Stage 5 ready signal.
//
// VALIDATES: BGP signals ready correctly.
// PREVENTS: Hub waiting forever for ready.
func TestChildModeReady(t *testing.T) {
	var buf bytes.Buffer
	err := writeReady(&buf)
	require.NoError(t, err)

	output := buf.String()
	assert.Equal(t, "ready\n", output)
}

// TestChildModeParseConfig verifies config parsing from stdin.
//
// VALIDATES: JSON config parsed correctly.
// PREVENTS: Config parsing errors from hub.
func TestChildModeParseConfig(t *testing.T) {
	// Hub sends config as single line: config {...}
	configJSON := `config {"router-id": "1.2.3.4", "local-as": 65000, "peer": {"peer1": {"address": "192.0.2.1", "remote-as": 65001}}}
`

	reader := strings.NewReader(configJSON)
	cfg, err := parseChildConfig(reader)
	require.NoError(t, err)

	assert.Equal(t, "1.2.3.4", cfg["router-id"])
	assert.Equal(t, float64(65000), cfg["local-as"]) // JSON numbers are float64
}

// TestChildModeParseConfigDone verifies "config done" marker handling.
//
// VALIDATES: Config done marker returns empty config without error.
// PREVENTS: Error when hub sends no config data.
func TestChildModeParseConfigDone(t *testing.T) {
	reader := strings.NewReader("config done\n")
	cfg, err := parseChildConfig(reader)
	require.NoError(t, err)
	assert.Empty(t, cfg) // Empty map, not nil
}

// TestParseChildArgs verifies config path extraction from arguments.
//
// VALIDATES: Config path extracted from --config flag.
// PREVENTS: Incorrect argument parsing.
func TestParseChildArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{
			name:     "separate_flag_value",
			args:     []string{"--child", "--config", "/path/to/config.conf"},
			wantPath: "/path/to/config.conf",
		},
		{
			name:     "equals_syntax",
			args:     []string{"--child", "--config=/path/to/config.conf"},
			wantPath: "/path/to/config.conf",
		},
		{
			name:     "no_config",
			args:     []string{"--child"},
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseChildArgs(tt.args)
			assert.Equal(t, tt.wantPath, got)
		})
	}
}
