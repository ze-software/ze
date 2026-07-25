// Design: docs/architecture/cli/plugin-modes.md — full (all tags) build validation
//
//go:build ze_core && ze_distro && ze_appliance && ze_setup

package main

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
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
