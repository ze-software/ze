// Design: docs/architecture/cli/plugin-modes.md — ze_setup build tag validation
//
//go:build ze_setup && !ze_distro && !ze_appliance && !ze_test && !ze_chaos && !ze_perf && !ze_analyze

package main

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func TestZeSetupBinaryCommands(t *testing.T) {
	roots := registry.ListRoot()
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		seen[root.Name] = true
	}

	if !seen["install"] {
		t.Fatal("root \"install\" missing in ze_setup build")
	}
	if !seen["appliance"] {
		t.Fatal("root \"appliance\" missing in ze_setup build")
	}
	if seen["uninstall"] {
		t.Fatal("root \"uninstall\" unexpectedly registered in ze_setup-only build")
	}
}
