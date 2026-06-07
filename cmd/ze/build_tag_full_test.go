// Design: docs/architecture/cli/plugin-modes.md — full (all tags) build validation
//
//go:build ze_distro && ze_appliance && ze_setup && !ze_test && !ze_chaos && !ze_perf && !ze_analyze

package main

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func TestZeFullBinaryCommands(t *testing.T) {
	roots := registry.ListRoot()
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		seen[root.Name] = true
	}

	for _, name := range []string{"install", "uninstall", "connect", "appliance"} {
		if !seen[name] {
			t.Fatalf("root %q missing in full (all-tags) build", name)
		}
	}
}
