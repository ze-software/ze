//go:build ze_stripped

package main

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func TestStrippedBuildOmitsSetupRoots(t *testing.T) {
	roots := registry.ListRoot()
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		seen[root.Name] = true
	}
	for _, name := range []string{"install", "service", "uninstall"} {
		if seen[name] {
			t.Fatalf("root %q unexpectedly registered in ze-stripped", name)
		}
	}
}
