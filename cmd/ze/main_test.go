//go:build ze_core

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestLooksLikeConfig removed with the looksLikeConfig helper it tested (spec-fixit-config-file-positional-grammar AC-2, Thomas-confirmed remove-the-sink). The free-form position-1 config-path sink was deleted from zeDispatch; config launch now goes through `ze start <config-file>`. AC-2/AC-6 are proven by test/ui/bare-config-no-autoload.ci.

// TestDetectConfigType and TestDetectConfigTypeFileError removed with the detectConfigType helper both tested. ProbeConfigType no longer selects a runtime -- every config the YANG schema accepts boots on one daemon path (cmd/ze/hub/main.go Run) -- so the helper and the --web config-type gate it fed both went. The probe's own behavior is covered by TestProbeConfigType (internal/component/config/probe_test.go), the boot path by test/plugin/config-validate-agrees-with-boot.ci, and the unreadable-config case by the "error: read config" branch the daemon now reaches for every config.

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
