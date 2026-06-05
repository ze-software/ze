// Design: docs/architecture/cli/plugin-modes.md — ze_appliance build tag validation
//
//go:build ze_appliance && !ze_setup

package main

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func TestZeApplianceBinaryCommands(t *testing.T) {
	roots := registry.ListRoot()
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		seen[root.Name] = true
	}

	if seen["appliance"] {
		t.Fatal("root \"appliance\" unexpectedly registered in ze_appliance-only build; appliance commands are in ze_setup")
	}
}
