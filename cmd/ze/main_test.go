package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// TestAvailablePlugins verifies the available plugins list.
//
// VALIDATES: All expected plugins are listed.
// PREVENTS: Missing plugin entries in discovery output.
func TestAvailablePlugins(t *testing.T) {
	// Expected plugins (sorted - AvailableInternalPlugins returns sorted list)
	expected := []string{"bgp-evpn", "bgp-flowspec", "bgp-gr", "bgp-hostname", "bgp-labeled", "bgp-llnh", "bgp-ls", "bgp-mup", "bgp-mvpn", "bgp-rib", "bgp-route-refresh", "bgp-rr", "bgp-rtc", "bgp-softver", "bgp-vpls", "bgp-vpn", "role"}

	got := plugin.AvailableInternalPlugins()
	assert.Equal(t, expected, got)
}

// TestLooksLikeConfig verifies config file detection.
//
// VALIDATES: Config file patterns are correctly identified.
// PREVENTS: False positives/negatives in config detection.
func TestLooksLikeConfig(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{"stdin", "-", true},
		{"conf_extension", "config.conf", true},
		{"cfg_extension", "config.cfg", true},
		{"yaml_extension", "config.yaml", true},
		{"yml_extension", "config.yml", true},
		{"json_extension", "config.json", true},
		{"no_extension", "config", false},
		{"command", "bgp", false},
		{"relative_path_nonexistent", "./nonexistent-test-file-xyz", false},
		{"absolute_path_nonexistent", "/nonexistent-path-xyz/config", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeConfig(tt.arg)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDetectConfigType verifies config type detection from file content.
//
// VALIDATES: Detects bgp, hub, unknown from top-level blocks.
// PREVENTS: Wrong daemon started for config type.
func TestDetectConfigType(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    config.ConfigType
	}{
		{
			name:    "bgp_block",
			content: "bgp {\n\tpeer 127.0.0.1 { }\n}",
			want:    config.ConfigTypeBGP,
		},
		{
			name:    "bgp_with_environment",
			content: "environment { }\nbgp { peer 127.0.0.1 { } }",
			want:    config.ConfigTypeBGP,
		},
		{
			name:    "plugin_external",
			content: "plugin {\n\texternal bgp { run \"ze bgp\"; }\n}",
			want:    config.ConfigTypeHub,
		},
		{
			name:    "unknown_empty",
			content: "",
			want:    config.ConfigTypeUnknown,
		},
		{
			name:    "unknown_only_environment",
			content: "environment { log { level info; } }",
			want:    config.ConfigTypeUnknown,
		},
		{
			name:    "unknown_comments_only",
			content: "# just a comment\n# another comment",
			want:    config.ConfigTypeUnknown,
		},
		{
			name:    "bgp_precedence_over_plugin",
			content: "plugin { external x { } }\nbgp { peer 1.2.3.4 { } }",
			want:    config.ConfigTypeBGP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write content to temp file
			dir := t.TempDir()
			path := filepath.Join(dir, "config.conf")
			err := os.WriteFile(path, []byte(tt.content), 0o600)
			require.NoError(t, err)

			got := detectConfigType(path)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDetectConfigTypeFileError verifies error handling for missing files.
//
// VALIDATES: Missing file returns ConfigTypeUnknown.
// PREVENTS: Panic on missing config file.
func TestDetectConfigTypeFileError(t *testing.T) {
	got := detectConfigType("/nonexistent/path/config.conf")
	assert.Equal(t, config.ConfigTypeUnknown, got)
}
