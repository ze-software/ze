// Design: docs/architecture/cli/plugin-modes.md — ze_linux build tag validation
//
//go:build ze_linux && !ze_appliance && !ze_setup

package main

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func TestZeLinuxBinaryCommands(t *testing.T) {
	roots := registry.ListRoot()
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		seen[root.Name] = true
	}

	for _, name := range []string{"install", "uninstall", "connect"} {
		if !seen[name] {
			t.Fatalf("root %q missing in ze_linux build", name)
		}
	}
	if seen["appliance"] {
		t.Fatal("root \"appliance\" unexpectedly registered in ze_linux-only build")
	}
}
