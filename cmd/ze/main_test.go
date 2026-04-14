package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestAvailablePlugins verifies the available plugins list.
//
// VALIDATES: All expected plugins are listed.
// PREVENTS: Missing plugin entries in discovery output.
func TestAvailablePlugins(t *testing.T) {
	// Plugins that register unconditionally (no build tags).
	expected := []string{"bfd", "bgp", "bgp-adj-rib-in", "bgp-aigp", "bgp-bmp", "bgp-filter-aspath", "bgp-filter-community", "bgp-filter-community-match", "bgp-filter-modify", "bgp-filter-prefix", "bgp-gr", "bgp-healthcheck", "bgp-hostname", "bgp-llnh", "bgp-nlri-evpn", "bgp-nlri-flowspec", "bgp-nlri-labeled", "bgp-nlri-ls", "bgp-nlri-mup", "bgp-nlri-mvpn", "bgp-nlri-rtc", "bgp-nlri-vpls", "bgp-nlri-vpn", "bgp-persist", "bgp-redistribute", "bgp-rib", "bgp-role", "bgp-route-refresh", "bgp-rpki", "bgp-rpki-decorator", "bgp-rr", "bgp-rs", "bgp-softver", "bgp-watchdog", "fib-kernel", "fib-p4", "fib-vpp", "interface", "loop", "ntp", "rib", "sysctl", "vpp"}
	// iface-dhcp only registers on linux (//go:build linux).
	linuxOnly := []string{"iface-dhcp"}

	got := plugin.AvailableInternalPlugins()
	// Every expected plugin must be registered.
	for _, want := range expected {
		assert.Contains(t, got, want, "expected plugin %q not registered", want)
	}
	// Every registered plugin must be in expected or linuxOnly.
	all := append(append([]string{}, expected...), linuxOnly...)
	for _, name := range got {
		assert.True(t, slices.Contains(all, name), "unexpected plugin %q registered (add to expected list)", name)
	}
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
