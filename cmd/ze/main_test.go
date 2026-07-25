//go:build ze_core

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestAvailablePlugins verifies the available plugin list is derived from the
// plugin registry.
//
// VALIDATES: Plugin discovery output uses the registry as its source of truth.
// PREVENTS: A second hardcoded production plugin list in cmd/ze.
func TestAvailablePlugins(t *testing.T) {
	got := plugin.AvailableInternalPlugins()
	require.NotEmpty(t, got, "AvailableInternalPlugins must return registered names")
	assert.Equal(t, registry.Names(), got)
}

// test-relax: TestLooksLikeConfig removed with the looksLikeConfig helper it tested (spec-fixit-config-file-positional-grammar AC-2, Thomas-confirmed remove-the-sink). The free-form position-1 config-path sink was deleted from zeDispatch; config launch now goes through `ze start <config-file>`. AC-2/AC-6 are proven by test/ui/bare-config-no-autoload.ci.

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
			content: "bgp {\n\tpeer peer1 { }\n}",
			want:    config.ConfigTypeBGP,
		},
		{
			name:    "bgp_with_environment",
			content: "environment { }\nbgp { peer peer1 { } }",
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
			content: "plugin { external x { } }\nbgp { peer peer1 { } }",
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

			got := detectConfigType(storage.NewFilesystem(), path)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsLocalhostPprof verifies pprof address localhost validation.
//
// VALIDATES: Only loopback addresses are accepted for pprof.
// PREVENTS: Exposing pprof on public interfaces (CWE-200).
func TestIsLocalhostPprof(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{"localhost_ipv4", "127.0.0.1:6060", true},
		{"localhost_ipv6", "[::1]:6060", true},
		{"localhost_name", "localhost:6060", true},
		{"all_interfaces", "0.0.0.0:6060", false},
		{"empty_host", ":6060", false},
		{"public_ip", "192.168.1.1:6060", false},
		{"ipv6_all", "[::]:6060", false},
		{"no_port", "127.0.0.1", false},
		{"garbage", "not-an-address", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalhostPprof(tt.addr)
			assert.Equal(t, tt.want, got, "isLocalhostPprof(%q)", tt.addr)
		})
	}
}

// TestDetectConfigTypeFileError verifies error handling for missing files.
//
// VALIDATES: Missing file returns ConfigTypeUnknown.
// PREVENTS: Panic on missing config file.
func TestDetectConfigTypeFileError(t *testing.T) {
	got := detectConfigType(storage.NewFilesystem(), "/nonexistent/path/config.conf")
	assert.Equal(t, config.ConfigTypeUnknown, got)
}
