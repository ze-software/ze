package main

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"github.com/stretchr/testify/assert"
)

// TestAvailablePlugins verifies the available plugins list.
//
// VALIDATES: All expected plugins are listed.
// PREVENTS: Missing plugin entries in discovery output.
func TestAvailablePlugins(t *testing.T) {
	// Expected plugins (sorted - AvailableInternalPlugins returns sorted list)
	expected := []string{"gr", "rib", "rr"}

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
