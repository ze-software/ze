// Design: docs/architecture/cli/plugin-modes.md — ze_distro build tag validation
//
//go:build ze_core && ze_distro && !ze_appliance && !ze_setup

package main

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func TestZeLinuxBinaryCommands(t *testing.T) {
	roots := registry.ListRoot()
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		seen[root.Name] = true
	}

	for _, name := range []string{"install", "uninstall", "connect"} {
		if !seen[name] {
			t.Fatalf("root %q missing in ze_distro build", name)
		}
	}
	if seen["appliance"] {
		t.Fatal("root \"appliance\" unexpectedly registered in ze_distro-only build")
	}
}
